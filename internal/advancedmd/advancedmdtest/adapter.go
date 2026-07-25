// Package advancedmdtest provides a deterministic adapter for the true
// external AdvancedMD seam.
package advancedmdtest

import (
	"context"

	"advancedmd-token-management/internal/advancedmd"
	"advancedmd-token-management/internal/domain"
)

// Adapter returns caller-controlled domain results without provider I/O.
type Adapter struct {
	PatientSearches     map[domain.PatientSearch][]domain.Patient
	PatientErrors       map[domain.PatientSearch]error
	Demographics        map[string]domain.PatientDemographics
	DemographicErrors   map[string]error
	Appointments        map[string][]domain.PatientAppointment
	AppointmentErrors   map[string]error
	SchedulerSetup      domain.SchedulerSetup
	SchedulerSetupError error
	SchedulerSetupCalls int
	ScheduleReads       map[string]domain.ScheduleReadResult
	ScheduleReadErrors  map[string]error
	ScheduleReadQueries []domain.ScheduleReadQuery
}

func NewAdapter() *Adapter {
	return &Adapter{
		PatientSearches:    make(map[domain.PatientSearch][]domain.Patient),
		PatientErrors:      make(map[domain.PatientSearch]error),
		Demographics:       make(map[string]domain.PatientDemographics),
		DemographicErrors:  make(map[string]error),
		Appointments:       make(map[string][]domain.PatientAppointment),
		AppointmentErrors:  make(map[string]error),
		ScheduleReads:      make(map[string]domain.ScheduleReadResult),
		ScheduleReadErrors: make(map[string]error),
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
	if err := a.AppointmentErrors[query.PatientID]; err != nil {
		return nil, err
	}
	return append([]domain.PatientAppointment(nil), a.Appointments[query.PatientID]...), nil
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

var _ advancedmd.PatientRecords = (*Adapter)(nil)
var _ advancedmd.SchedulingRecords = (*Adapter)(nil)
