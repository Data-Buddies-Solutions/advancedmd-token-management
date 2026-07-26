package advancedmd

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"advancedmd-token-management/internal/clients"
	"advancedmd-token-management/internal/domain"
	"advancedmd-token-management/internal/safeerrors"
	"advancedmd-token-management/internal/session"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var eastern = domain.EasternLocation()

// Adapter is the production adapter for AdvancedMD domain records.
type Adapter struct {
	session    session.Session
	xmlClient  *clients.AdvancedMDClient
	restClient *clients.AdvancedMDRestClient
	now        func() time.Time
}

func NewAdapter(
	amdSession session.Session,
	xmlClient *clients.AdvancedMDClient,
	restClient *clients.AdvancedMDRestClient,
	clock ...func() time.Time,
) *Adapter {
	now := time.Now
	if len(clock) > 0 && clock[0] != nil {
		now = clock[0]
	}
	return &Adapter{
		session:    amdSession,
		xmlClient:  xmlClient,
		restClient: restClient,
		now:        now,
	}
}

func (a *Adapter) SearchPatients(ctx context.Context, search domain.PatientSearch) ([]domain.Patient, error) {
	token, err := a.token(ctx)
	if err != nil {
		return nil, err
	}
	if a.xmlClient == nil {
		return nil, NewError(safeerrors.CategoryInternal)
	}

	var patients []domain.Patient
	if search.Phone != "" {
		patients, err = a.xmlClient.LookupPatientByPhone(ctx, token, search.Phone)
	} else {
		patients, err = a.xmlClient.LookupPatient(ctx, token, search.LastName, search.FirstName)
	}
	if err != nil {
		return nil, classify(err)
	}
	return patients, nil
}

func (a *Adapter) GetPatientDemographics(ctx context.Context, patientID string) (domain.PatientDemographics, error) {
	token, err := a.token(ctx)
	if err != nil {
		return domain.PatientDemographics{}, err
	}
	if a.xmlClient == nil {
		return domain.PatientDemographics{}, NewError(safeerrors.CategoryInternal)
	}

	result, err := a.xmlClient.GetDemographic(ctx, token, patientID)
	if err != nil {
		return domain.PatientDemographics{}, classify(err)
	}
	if result == nil {
		return domain.PatientDemographics{}, nil
	}
	return domain.PatientDemographics{
		CarrierName: result.CarrierName,
		CarrierID:   result.CarrierID,
		InsPlanID:   result.InsPlanID,
		RespPartyID: result.RespPartyID,
		DOB:         result.DOB,
	}, nil
}

func (a *Adapter) GetUpcomingAppointments(ctx context.Context, query domain.PatientAppointmentsQuery) ([]domain.PatientAppointment, error) {
	read, err := a.readPatientAppointments(ctx, query)
	return read.Appointments, err
}

func (a *Adapter) ReadPatientAppointments(ctx context.Context, query domain.PatientAppointmentsQuery) (AppointmentRead, error) {
	return a.readPatientAppointments(ctx, query)
}

func (a *Adapter) ReadPatientAppointmentsForMonth(
	ctx context.Context,
	query AppointmentMonthQuery,
) (AppointmentRead, error) {
	token, err := a.token(ctx)
	if err != nil {
		return AppointmentRead{}, err
	}
	if a.restClient == nil || query.Month.IsZero() {
		return AppointmentRead{}, NewError(safeerrors.CategoryInternal)
	}
	patientID, err := strconv.Atoi(query.PatientID)
	if err != nil {
		return AppointmentRead{}, NewError(safeerrors.CategoryInternal)
	}

	read := AppointmentRead{
		Appointments: make([]domain.PatientAppointment, 0),
		Complete:     true,
	}
	for _, officeID := range query.OfficeIDs {
		office, ok := domain.LookupOfficeByID(officeID)
		if !ok {
			return AppointmentRead{}, NewError(safeerrors.CategoryInternal)
		}
		officeRead, err := a.patientAppointmentsForOfficeMonth(
			ctx,
			token,
			patientID,
			office,
			query.Month,
		)
		if err != nil {
			return AppointmentRead{}, err
		}
		read.Appointments = append(read.Appointments, officeRead.Appointments...)
		read.Complete = read.Complete && officeRead.Complete
	}
	return read, nil
}

func (a *Adapter) readPatientAppointments(ctx context.Context, query domain.PatientAppointmentsQuery) (AppointmentRead, error) {
	token, err := a.token(ctx)
	if err != nil {
		return AppointmentRead{}, err
	}
	if a.restClient == nil {
		return AppointmentRead{}, NewError(safeerrors.CategoryInternal)
	}
	patientIDNumber, err := strconv.Atoi(query.PatientID)
	if err != nil {
		return AppointmentRead{}, NewError(safeerrors.CategoryInternal)
	}

	read := AppointmentRead{
		Appointments: make([]domain.PatientAppointment, 0),
		Complete:     true,
	}
	for _, officeID := range query.OfficeIDs {
		office, ok := domain.LookupOfficeByID(officeID)
		if !ok {
			return AppointmentRead{}, NewError(safeerrors.CategoryInternal)
		}
		officeRead, err := a.upcomingAppointmentsForOffice(ctx, token, patientIDNumber, office)
		if err != nil {
			return AppointmentRead{}, err
		}
		read.Appointments = append(read.Appointments, officeRead.Appointments...)
		read.Complete = read.Complete && officeRead.Complete
	}
	return read, nil
}

func (a *Adapter) upcomingAppointmentsForOffice(
	ctx context.Context,
	token *domain.TokenData,
	patientID int,
	office *domain.OfficeConfig,
) (AppointmentRead, error) {
	if office == nil {
		return AppointmentRead{}, NewError(safeerrors.CategoryInternal)
	}

	now := a.now().In(eastern)
	cutoff := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second(), 0, time.UTC)
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, eastern)
	columnIDs := strings.Join(office.AllowedColumnIDs(), "-")

	type monthResult struct {
		appointments []clients.AMDAppointmentResponse
		err          error
	}
	results := make(chan monthResult, 6)
	for month := 0; month < 6; month++ {
		startDate := thisMonth.AddDate(0, month, 0).Format("2006-01-02")
		go func() {
			appointments, err := a.restClient.GetAppointmentsByMonth(ctx, token, columnIDs, startDate)
			results <- monthResult{appointments: appointments, err: err}
		}()
	}

	var rawAppointments []clients.AMDAppointmentResponse
	for range 6 {
		result := <-results
		if result.err != nil {
			return AppointmentRead{}, classify(result.err)
		}
		rawAppointments = append(rawAppointments, result.appointments...)
	}

	return patientAppointmentRead(rawAppointments, patientID, office, &cutoff), nil
}

func (a *Adapter) patientAppointmentsForOfficeMonth(
	ctx context.Context,
	token *domain.TokenData,
	patientID int,
	office *domain.OfficeConfig,
	month time.Time,
) (AppointmentRead, error) {
	if office == nil || month.IsZero() {
		return AppointmentRead{}, NewError(safeerrors.CategoryInternal)
	}
	rawAppointments, err := a.restClient.GetAppointmentsByMonth(
		ctx,
		token,
		strings.Join(office.AllowedColumnIDs(), "-"),
		firstOfMonth(month),
	)
	if err != nil {
		return AppointmentRead{}, classify(err)
	}
	return patientAppointmentRead(rawAppointments, patientID, office, nil), nil
}

func patientAppointmentRead(
	rawAppointments []clients.AMDAppointmentResponse,
	patientID int,
	office *domain.OfficeConfig,
	cutoff *time.Time,
) AppointmentRead {
	read := AppointmentRead{
		Appointments: make([]domain.PatientAppointment, 0),
		Complete:     true,
	}
	for _, raw := range rawAppointments {
		if raw.PatientID <= 0 {
			read.Complete = false
			continue
		}
		if raw.PatientID != patientID {
			continue
		}
		if raw.ID <= 0 {
			read.Complete = false
			continue
		}
		start, err := clients.ParseDateTime(raw.StartDateTime)
		if err != nil {
			read.Complete = false
			continue
		}
		if cutoff != nil && !start.After(*cutoff) {
			continue
		}

		typeID := 0
		typeName := ""
		if len(raw.AppointmentTypes) > 0 {
			providerTypeID := raw.AppointmentTypes[0]
			canonicalID, ok := domain.CanonicalAppointmentTypeID(providerTypeID)
			if !ok {
				read.Complete = false
			} else {
				typeID = canonicalID
				var named bool
				typeName, named = office.AppointmentTypeName(typeID)
				if !named {
					read.Complete = false
				}
			}
		} else {
			read.Complete = false
		}
		provider := office.FriendlyProviderName(raw.Provider)
		if strings.TrimSpace(raw.Provider) == "" || provider == "" {
			read.Complete = false
		}
		facility := friendlyFacilityName(raw.Facility)
		if facility == "" {
			facility = office.DisplayName
		}

		read.Appointments = append(read.Appointments, domain.PatientAppointment{
			ID:                raw.ID,
			Start:             start,
			Provider:          provider,
			Type:              typeName,
			AppointmentTypeID: typeID,
			Facility:          facility,
			OfficeID:          office.ID,
			Office:            office.DisplayName,
		})
	}
	return read
}

func (a *Adapter) ReadAppointmentState(
	ctx context.Context,
	query AppointmentStateQuery,
) (AppointmentState, error) {
	token, err := a.token(ctx)
	if err != nil {
		return AppointmentState{}, err
	}
	if a.restClient == nil || query.AppointmentID <= 0 || query.Start.IsZero() {
		return AppointmentState{}, NewError(safeerrors.CategoryInternal)
	}
	office, ok := domain.LookupOfficeByID(query.OfficeID)
	if !ok {
		return AppointmentState{}, NewError(safeerrors.CategoryInternal)
	}

	rawAppointments, err := a.restClient.GetAppointmentsByMonth(
		ctx,
		token,
		strings.Join(office.AllowedColumnIDs(), "-"),
		firstOfMonth(query.Start),
	)
	if err != nil {
		return AppointmentState{}, classify(err)
	}

	state := AppointmentState{Complete: true}
	for _, raw := range rawAppointments {
		if raw.ID == query.AppointmentID {
			state.Exists = true
			return state, nil
		}
		if raw.ID <= 0 {
			state.Complete = false
		}
	}
	return state, nil
}

func firstOfMonth(value time.Time) string {
	return time.Date(
		value.Year(),
		value.Month(),
		1,
		0,
		0,
		0,
		0,
		value.Location(),
	).Format("2006-01-02")
}

func (a *Adapter) GetSchedulerSetup(ctx context.Context) (domain.SchedulerSetup, error) {
	token, err := a.token(ctx)
	if err != nil {
		return domain.SchedulerSetup{}, err
	}
	if a.xmlClient == nil {
		return domain.SchedulerSetup{}, NewError(safeerrors.CategoryInternal)
	}

	setup, err := a.xmlClient.GetSchedulerSetup(ctx, token)
	if err != nil {
		return domain.SchedulerSetup{}, classify(err)
	}
	if setup == nil {
		return domain.SchedulerSetup{}, NewError(safeerrors.CategoryInvalidResponse)
	}
	return *setup, nil
}

func (a *Adapter) ReadSchedule(ctx context.Context, query domain.ScheduleReadQuery) (domain.ScheduleReadResult, error) {
	token, err := a.token(ctx)
	if err != nil {
		return domain.ScheduleReadResult{}, err
	}
	if a.restClient == nil {
		return domain.ScheduleReadResult{}, NewError(safeerrors.CategoryInternal)
	}

	var appointments map[string][]domain.Appointment
	var blockHolds map[string][]domain.BlockHold
	var reads sync.WaitGroup
	reads.Add(2)
	go func() {
		defer reads.Done()
		appointments = a.restClient.GetAppointmentsForColumns(ctx, token, query.ColumnIDs, query.Date)
	}()
	go func() {
		defer reads.Done()
		blockHolds = a.restClient.GetBlockHoldsForColumns(ctx, token, query.ColumnIDs, query.Date)
	}()
	reads.Wait()

	result := domain.ScheduleReadResult{
		Columns: make(map[string]domain.ColumnSchedule, len(query.ColumnIDs)),
	}
	for _, columnID := range query.ColumnIDs {
		columnAppointments, appointmentsComplete := appointments[columnID]
		columnBlockHolds, blockHoldsComplete := blockHolds[columnID]
		result.Columns[columnID] = domain.ColumnSchedule{
			Appointments:         columnAppointments,
			BlockHolds:           columnBlockHolds,
			AppointmentsComplete: appointmentsComplete,
			BlockHoldsComplete:   blockHoldsComplete,
		}
	}
	return result, nil
}

func (a *Adapter) BookAppointment(ctx context.Context, booking Booking) (int, error) {
	token, err := a.token(ctx)
	if err != nil {
		return 0, err
	}
	if a.restClient == nil {
		return 0, NewError(safeerrors.CategoryInternal)
	}
	office, ok := domain.LookupOfficeByID(booking.OfficeID)
	if !ok {
		return 0, NewError(safeerrors.CategoryInternal)
	}
	facilityID, err := strconv.Atoi(office.FacilityID)
	if err != nil {
		return 0, NewError(safeerrors.CategoryInternal)
	}
	providerTypeID, ok := domain.ResolveAppointmentTypeID(booking.AppointmentTypeID)
	if !ok {
		return 0, NewError(safeerrors.CategoryInternal)
	}
	color, ok := office.AppointmentColor(booking.AppointmentTypeID)
	if !ok {
		return 0, NewError(safeerrors.CategoryInternal)
	}
	force := 0
	if booking.Force {
		force = 1
	}

	appointmentID, err := a.restClient.BookAppointment(ctx, token, clients.BookAppointmentParams{
		PatientID:     booking.PatientID,
		ColumnID:      booking.ColumnID,
		ProfileID:     booking.ProfileID,
		StartDatetime: domain.FormatSlotDateTime(booking.Start),
		Duration:      booking.Duration,
		AppointmentType: []struct {
			ID int `json:"id"`
		}{{ID: providerTypeID}},
		EpisodeID:  1,
		FacilityID: facilityID,
		Color:      color,
		Force:      force,
		Comments:   booking.Comments,
	})
	if err != nil {
		return 0, classifyMutation(err)
	}
	return appointmentID, nil
}

func (a *Adapter) CancelAppointment(ctx context.Context, appointmentID int) error {
	token, err := a.token(ctx)
	if err != nil {
		return err
	}
	if a.restClient == nil {
		return NewError(safeerrors.CategoryInternal)
	}
	if err := a.restClient.CancelAppointment(ctx, token, appointmentID); err != nil {
		return classifyMutation(err)
	}
	return nil
}

func (a *Adapter) token(ctx context.Context) (*domain.TokenData, error) {
	if a.session == nil {
		return nil, NewError(safeerrors.CategoryUnavailable)
	}
	token, err := a.session.Get(ctx)
	if err != nil {
		return nil, NewError(safeerrors.CategoryUnavailable)
	}
	if token == nil {
		return nil, NewError(safeerrors.CategoryUnavailable)
	}
	return token, nil
}

func classify(err error) error {
	if err == nil {
		return nil
	}
	return NewError(safeerrors.Classify(err))
}

func classifyMutation(err error) error {
	switch clients.MutationDispositionOf(err) {
	case clients.MutationDispositionAuthentication:
		return NewError(safeerrors.CategoryAuthentication)
	case clients.MutationDispositionConflict:
		return NewError(safeerrors.CategoryConflict)
	case clients.MutationDispositionRejected:
		return NewError(safeerrors.CategoryRejected)
	case clients.MutationDispositionAmbiguous:
		return NewAmbiguousWriteError(safeerrors.Classify(err))
	default:
		return classify(err)
	}
}

func friendlyFacilityName(name string) string {
	if name == "" {
		return ""
	}
	return cases.Title(language.English).String(strings.ToLower(name))
}

var _ PatientRecords = (*Adapter)(nil)
var _ SchedulingRecords = (*Adapter)(nil)
