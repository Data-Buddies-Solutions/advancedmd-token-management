// Package advancedmdtest provides a deterministic adapter for the true
// external AdvancedMD seam.
package advancedmdtest

import (
	"context"

	"advancedmd-token-management/internal/advancedmd"
	"advancedmd-token-management/internal/domain"
)

type AppointmentResult struct {
	Read advancedmd.AppointmentRead
	Err  error
}

type AppointmentStateResult struct {
	State advancedmd.AppointmentState
	Err   error
}

// Adapter returns caller-controlled domain results without provider I/O.
type Adapter struct {
	PatientSearches         map[domain.PatientSearch][]domain.Patient
	PatientErrors           map[domain.PatientSearch]error
	Demographics            map[string]domain.PatientDemographics
	DemographicErrors       map[string]error
	AppointmentResults      map[string]AppointmentResult
	AppointmentStateResults map[int]AppointmentStateResult
	AppointmentStateQueries []advancedmd.AppointmentStateQuery
	SchedulerSetup          domain.SchedulerSetup
	SchedulerSetupError     error
	SchedulerSetupCalls     int
	ScheduleReads           map[string]domain.ScheduleReadResult
	ScheduleReadErrors      map[string]error
	ScheduleReadQueries     []domain.ScheduleReadQuery
	BookAppointmentID       int
	BookAppointmentErr      error
	Bookings                []advancedmd.Booking
	CancelAppointmentErr    error
	Cancellations           []int
}

func NewAdapter() *Adapter {
	return &Adapter{
		PatientSearches:         make(map[domain.PatientSearch][]domain.Patient),
		PatientErrors:           make(map[domain.PatientSearch]error),
		Demographics:            make(map[string]domain.PatientDemographics),
		DemographicErrors:       make(map[string]error),
		AppointmentResults:      make(map[string]AppointmentResult),
		AppointmentStateResults: make(map[int]AppointmentStateResult),
		ScheduleReads:           make(map[string]domain.ScheduleReadResult),
		ScheduleReadErrors:      make(map[string]error),
	}
}

func (a *Adapter) SearchPatients(_ context.Context, search domain.PatientSearch) ([]domain.Patient, error) {
	if err := a.PatientErrors[search]; err != nil {
		return nil, err
	}
	return append([]domain.Patient(nil), a.PatientSearches[search]...), nil
}

func (a *Adapter) GetPatientDemographics(_ context.Context, patientID string) (domain.PatientDemographics, error) {
	if err := a.DemographicErrors[patientID]; err != nil {
		return domain.PatientDemographics{}, err
	}
	return a.Demographics[patientID], nil
}

func (a *Adapter) GetUpcomingAppointments(_ context.Context, query domain.PatientAppointmentsQuery) ([]domain.PatientAppointment, error) {
	read, err := a.nextAppointmentRead(query.PatientID)
	return read.Appointments, err
}

func (a *Adapter) ReadPatientAppointments(_ context.Context, query domain.PatientAppointmentsQuery) (advancedmd.AppointmentRead, error) {
	return a.nextAppointmentRead(query.PatientID)
}

func (a *Adapter) nextAppointmentRead(patientID string) (advancedmd.AppointmentRead, error) {
	result, configured := a.AppointmentResults[patientID]
	if result.Err != nil {
		return advancedmd.AppointmentRead{}, result.Err
	}
	read := result.Read
	read.Appointments = append([]domain.PatientAppointment(nil), read.Appointments...)
	if !configured {
		read.Complete = true
	}
	return read, nil
}

func (a *Adapter) ReadAppointmentState(_ context.Context, query advancedmd.AppointmentStateQuery) (advancedmd.AppointmentState, error) {
	a.AppointmentStateQueries = append(a.AppointmentStateQueries, query)
	result, configured := a.AppointmentStateResults[query.AppointmentID]
	if result.Err != nil {
		return advancedmd.AppointmentState{}, result.Err
	}
	if !configured {
		result.State.Complete = true
	}
	return result.State, nil
}

func (a *Adapter) GetSchedulerSetup(_ context.Context) (domain.SchedulerSetup, error) {
	a.SchedulerSetupCalls++
	if a.SchedulerSetupError != nil {
		return domain.SchedulerSetup{}, a.SchedulerSetupError
	}
	return a.SchedulerSetup, nil
}

func (a *Adapter) ReadSchedule(_ context.Context, query domain.ScheduleReadQuery) (domain.ScheduleReadResult, error) {
	query.ColumnIDs = append([]string(nil), query.ColumnIDs...)
	a.ScheduleReadQueries = append(a.ScheduleReadQueries, query)
	if err := a.ScheduleReadErrors[query.Date]; err != nil {
		return domain.ScheduleReadResult{}, err
	}
	read := a.ScheduleReads[query.Date]
	result := domain.ScheduleReadResult{
		Columns: make(map[string]domain.ColumnSchedule, len(query.ColumnIDs)),
	}
	for _, columnID := range query.ColumnIDs {
		if column, ok := read.Columns[columnID]; ok {
			result.Columns[columnID] = column
		}
	}
	return result, nil
}

func (a *Adapter) BookAppointment(_ context.Context, booking advancedmd.Booking) (int, error) {
	a.Bookings = append(a.Bookings, booking)
	if a.BookAppointmentErr != nil {
		return 0, a.BookAppointmentErr
	}
	return a.BookAppointmentID, nil
}

func (a *Adapter) CancelAppointment(_ context.Context, appointmentID int) error {
	a.Cancellations = append(a.Cancellations, appointmentID)
	return a.CancelAppointmentErr
}

var _ advancedmd.PatientRecords = (*Adapter)(nil)
var _ advancedmd.SchedulingRecords = (*Adapter)(nil)
