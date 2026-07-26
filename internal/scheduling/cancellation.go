package scheduling

import (
	"context"
	"log"
	"strconv"
	"strings"

	"advancedmd-token-management/internal/advancedmd"
	"advancedmd-token-management/internal/domain"
	"advancedmd-token-management/internal/safeerrors"
)

// CancelCommand preserves the public cancellation request while Scheduling
// owns patient ownership and reconciliation.
type CancelCommand struct {
	AppointmentID int    `json:"appointmentId"`
	PatientID     string `json:"patientId,omitempty"`
	Office        string `json:"office,omitempty"`
}

// CancelReceipt is the stable cancellation result returned to HTTP callers.
type CancelReceipt struct {
	Status        string `json:"status"`
	Outcome       string `json:"outcome,omitempty"`
	AppointmentID int    `json:"appointmentId,omitempty"`
	Message       string `json:"message"`
}

func (s *service) Cancel(ctx context.Context, command CancelCommand) (CancelReceipt, error) {
	if command.AppointmentID == 0 {
		return CancelReceipt{}, schedulingError("appointmentId is required")
	}
	command.PatientID = domain.StripPatientPrefix(strings.TrimSpace(command.PatientID))
	if command.PatientID == "" {
		return CancelReceipt{}, schedulingError("patientId is required")
	}
	if _, err := strconv.Atoi(command.PatientID); err != nil {
		return CancelReceipt{}, schedulingError("patientId must be numeric")
	}
	office, err := domain.ResolveOffice(command.Office)
	if err != nil {
		return CancelReceipt{}, schedulingError(err.Error())
	}
	if s.records == nil {
		return CancelReceipt{}, ownershipCheckError()
	}

	read, err := s.records.ReadPatientAppointments(ctx, domain.PatientAppointmentsQuery{
		PatientID: command.PatientID,
		OfficeIDs: appointmentOfficeIDs(office),
	})
	if err != nil {
		return CancelReceipt{}, ownershipCheckError()
	}
	appointment, owningOffice, found, knownOffice := ownedAppointment(
		read.Appointments,
		command.AppointmentID,
		office,
	)
	if !knownOffice {
		return CancelReceipt{}, ownershipCheckError()
	}
	if !found {
		if !read.Complete {
			return CancelReceipt{}, ownershipCheckError()
		}
		return CancelReceipt{}, categorizedError(
			CategoryOwnershipMismatch,
			"No upcoming appointment matches that patient and appointment ID. Please load appointments again and choose the appointment to cancel.",
		)
	}

	err = s.records.CancelAppointment(ctx, command.AppointmentID)
	if err == nil {
		return cancellationReceipt(command.AppointmentID), nil
	}
	if advancedmd.IsAmbiguousWrite(err) {
		return s.reconcileCancellation(ctx, command.AppointmentID, appointment, owningOffice)
	}

	providerFailure := providerCategory(err)
	switch providerFailure {
	case safeerrors.CategoryConflict:
		return CancelReceipt{}, categorizedProviderError(
			CategoryProviderConflict,
			providerFailure,
			"AdvancedMD could not cancel the appointment because its state changed. Please load appointments again.",
		)
	case safeerrors.CategoryRejected:
		return CancelReceipt{}, categorizedProviderError(
			CategoryProviderRejected,
			providerFailure,
			"AdvancedMD rejected the cancellation. Please load appointments again or contact the office.",
		)
	case safeerrors.CategoryAuthentication, safeerrors.CategoryUnavailable:
		return CancelReceipt{}, categorizedProviderError(
			CategoryWriteFailed,
			providerFailure,
			"Service authentication is temporarily unavailable. Please try again.",
		)
	default:
		return CancelReceipt{}, categorizedProviderError(
			CategoryWriteFailed,
			providerFailure,
			"Failed to cancel appointment in AdvancedMD. Please try again or contact the office.",
		)
	}
}

func (s *service) reconcileCancellation(
	ctx context.Context,
	appointmentID int,
	appointment domain.PatientAppointment,
	office *domain.OfficeConfig,
) (CancelReceipt, error) {
	state, err := s.records.ReadAppointmentState(ctx, advancedmd.AppointmentStateQuery{
		AppointmentID: appointmentID,
		OfficeID:      office.ID,
		Start:         appointment.Start,
	})
	if err != nil {
		log.Printf("cancellation: reconciliation outcome=indeterminate category=%s", providerCategory(err))
		return CancelReceipt{}, categorizedError(
			CategoryIndeterminateWrite,
			"AdvancedMD may have cancelled the appointment, but the result could not be verified. Do not retry automatically; load appointments before taking another action.",
		)
	}
	if state.Exists {
		log.Printf("cancellation: reconciliation outcome=failure")
		return CancelReceipt{}, categorizedError(
			CategoryWriteFailed,
			"AdvancedMD did not cancel the appointment. Please load appointments again before retrying.",
		)
	}
	if !state.Complete {
		log.Printf("cancellation: reconciliation outcome=indeterminate category=incomplete_read")
		return CancelReceipt{}, categorizedError(
			CategoryIndeterminateWrite,
			"AdvancedMD may have cancelled the appointment, but the result could not be verified. Do not retry automatically; load appointments before taking another action.",
		)
	}

	log.Printf("cancellation: reconciliation outcome=success")
	return cancellationReceipt(appointmentID), nil
}

func ownershipCheckError() error {
	return categorizedError(
		CategoryWriteFailed,
		"Unable to verify appointment before cancellation. Please load appointments again and choose the appointment to cancel.",
	)
}

func ownedAppointment(
	appointments []domain.PatientAppointment,
	appointmentID int,
	requestedOffice *domain.OfficeConfig,
) (domain.PatientAppointment, *domain.OfficeConfig, bool, bool) {
	for _, appointment := range appointments {
		if appointment.ID != appointmentID {
			continue
		}
		if appointment.OfficeID == "" {
			return appointment, requestedOffice, true, true
		}
		office, ok := domain.LookupOfficeByID(appointment.OfficeID)
		return appointment, office, true, ok
	}
	return domain.PatientAppointment{}, nil, false, true
}

func appointmentOfficeIDs(office *domain.OfficeConfig) []string {
	offices := domain.AppointmentLookupOffices(office)
	officeIDs := make([]string, 0, len(offices))
	for _, candidate := range offices {
		officeIDs = append(officeIDs, candidate.ID)
	}
	return officeIDs
}

func cancellationReceipt(appointmentID int) CancelReceipt {
	return CancelReceipt{
		Status:        "cancelled",
		AppointmentID: appointmentID,
		Message:       "Appointment cancelled successfully",
	}
}
