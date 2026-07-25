// Package advancedmd defines the domain-oriented seam around the external
// AdvancedMD dependency.
package advancedmd

import (
	"context"
	"errors"

	"advancedmd-token-management/internal/domain"
	"advancedmd-token-management/internal/safeerrors"
)

// Error preserves a stable category while keeping provider details behind the
// AdvancedMD seam.
type Error struct {
	category safeerrors.Category
}

func NewError(category safeerrors.Category) error {
	return &Error{category: category}
}

func (e *Error) Error() string {
	return string(e.category)
}

func CategoryOf(err error) safeerrors.Category {
	var classified *Error
	if errors.As(err, &classified) {
		return classified.category
	}
	return safeerrors.CategoryInternal
}

// PatientRecords is the AdvancedMD surface required by the Patient module.
// Implementations own authentication, provider endpoints, request formats, and
// response parsing.
type PatientRecords interface {
	SearchPatients(ctx context.Context, search domain.PatientSearch) ([]domain.Patient, error)
	GetPatientDemographics(ctx context.Context, patientID string) (domain.PatientDemographics, error)
	GetUpcomingAppointments(ctx context.Context, query domain.PatientAppointmentsQuery) ([]domain.PatientAppointment, error)
}

// SchedulingRecords is the AdvancedMD surface required by the Scheduling
// module. Implementations keep authentication, endpoints, and provider payloads
// behind domain scheduler setup and schedule-read results.
type SchedulingRecords interface {
	GetSchedulerSetup(ctx context.Context) (domain.SchedulerSetup, error)
	ReadSchedule(ctx context.Context, query domain.ScheduleReadQuery) (domain.ScheduleReadResult, error)
}
