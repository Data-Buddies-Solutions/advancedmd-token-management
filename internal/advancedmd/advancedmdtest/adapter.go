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
	PatientSearches   map[domain.PatientSearch][]domain.Patient
	PatientErrors     map[domain.PatientSearch]error
	Demographics      map[string]domain.PatientDemographics
	DemographicErrors map[string]error
	Appointments      map[string][]domain.PatientAppointment
	AppointmentErrors map[string]error
}

func NewAdapter() *Adapter {
	return &Adapter{
		PatientSearches:   make(map[domain.PatientSearch][]domain.Patient),
		PatientErrors:     make(map[domain.PatientSearch]error),
		Demographics:      make(map[string]domain.PatientDemographics),
		DemographicErrors: make(map[string]error),
		Appointments:      make(map[string][]domain.PatientAppointment),
		AppointmentErrors: make(map[string]error),
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

var _ advancedmd.PatientRecords = (*Adapter)(nil)
