package advancedmd

import (
	"context"
	"strconv"
	"strings"
	"time"

	"advancedmd-token-management/internal/clients"
	"advancedmd-token-management/internal/domain"
	"advancedmd-token-management/internal/safeerrors"
	"advancedmd-token-management/internal/session"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var eastern = loadEastern()

// Adapter is the production adapter for AdvancedMD patient records.
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
	token, err := a.token(ctx)
	if err != nil {
		return nil, err
	}
	if a.restClient == nil {
		return nil, NewError(safeerrors.CategoryInternal)
	}
	patientIDNumber, err := strconv.Atoi(query.PatientID)
	if err != nil {
		return nil, NewError(safeerrors.CategoryInternal)
	}

	appointments := make([]domain.PatientAppointment, 0)
	for _, officeID := range query.OfficeIDs {
		office, ok := domain.LookupOfficeByID(officeID)
		if !ok {
			return nil, NewError(safeerrors.CategoryInternal)
		}
		officeAppointments, err := a.upcomingAppointmentsForOffice(ctx, token, patientIDNumber, office)
		if err != nil {
			return nil, err
		}
		appointments = append(appointments, officeAppointments...)
	}
	return appointments, nil
}

func (a *Adapter) upcomingAppointmentsForOffice(
	ctx context.Context,
	token *domain.TokenData,
	patientID int,
	office *domain.OfficeConfig,
) ([]domain.PatientAppointment, error) {
	if office == nil {
		return nil, NewError(safeerrors.CategoryInternal)
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
			return nil, classify(result.err)
		}
		rawAppointments = append(rawAppointments, result.appointments...)
	}

	appointments := make([]domain.PatientAppointment, 0)
	for _, raw := range rawAppointments {
		if raw.PatientID != patientID {
			continue
		}
		start, err := clients.ParseDateTime(raw.StartDateTime)
		if err != nil || !start.After(cutoff) {
			continue
		}

		typeID := 0
		typeName := ""
		if len(raw.AppointmentTypes) > 0 {
			typeID = raw.AppointmentTypes[0]
			if canonicalID, ok := domain.CanonicalAppointmentTypeID(typeID); ok {
				typeID = canonicalID
			}
			typeName, _ = office.AppointmentTypeName(typeID)
		}
		facility := friendlyFacilityName(raw.Facility)
		if facility == "" {
			facility = office.DisplayName
		}

		appointments = append(appointments, domain.PatientAppointment{
			ID:                raw.ID,
			Start:             start,
			Provider:          office.FriendlyProviderName(raw.Provider),
			Type:              typeName,
			AppointmentTypeID: typeID,
			Facility:          facility,
			OfficeID:          office.ID,
			Office:            office.DisplayName,
		})
	}
	return appointments, nil
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

func friendlyFacilityName(name string) string {
	if name == "" {
		return ""
	}
	return cases.Title(language.English).String(strings.ToLower(name))
}

func loadEastern() *time.Location {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.FixedZone("EST", -5*60*60)
	}
	return location
}

var _ PatientRecords = (*Adapter)(nil)
