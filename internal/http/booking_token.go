package http

import (
	"time"

	"advancedmd-token-management/internal/domain"
	schedulingmodule "advancedmd-token-management/internal/scheduling"
)

func (w *schedulingWorkflow) applyBookingToken(
	req *BookAppointmentRequest,
	requestedOffice *domain.OfficeConfig,
	now time.Time,
) (*domain.OfficeConfig, error) {
	policy, err := schedulingmodule.VerifySlotToken(w.bookingTokenSecret, req.BookingToken, now)
	if err != nil {
		return nil, err
	}
	office, ok := lookupOfficeByID(policy.OfficeID)
	if !ok {
		return nil, schedulingmodule.ErrSlotTokenInvalid
	}
	if requestedOffice != nil && requestedOffice.ID != office.ID {
		return nil, schedulingmodule.ErrSlotTokenInvalid
	}
	if policy.DOB != "" && req.DOB != "" {
		if _, valid := domain.AgeYears(req.DOB); valid && domain.NormalizeDOB(req.DOB) != policy.DOB {
			return nil, schedulingmodule.ErrSlotTokenInvalid
		}
	}

	req.ColumnID = policy.ColumnID
	req.ProfileID = policy.ProfileID
	req.StartDatetime = policy.StartDatetime
	req.Duration = policy.Duration
	req.Routing = policy.Routing
	if policy.DOB != "" && (req.DOB == "" || domain.NormalizeDOB(req.DOB) == policy.DOB) {
		req.DOB = policy.DOB
	}
	req.bookingRequiresForce = policy.RequiresForce
	req.bookingAppointmentTypeIDs = append([]int(nil), policy.AppointmentTypeIDs...)
	return office, nil
}

func lookupOfficeByID(officeID string) (*domain.OfficeConfig, bool) {
	return domain.LookupOfficeByID(officeID)
}
