package patient_test

import (
	"context"
	"testing"

	"advancedmd-token-management/internal/advancedmd"
	"advancedmd-token-management/internal/advancedmd/advancedmdtest"
	"advancedmd-token-management/internal/domain"
	"advancedmd-token-management/internal/patient"
	"advancedmd-token-management/internal/safeerrors"
)

func TestResolveReportsIncompleteAppointmentsAsUnavailable(t *testing.T) {
	for _, partial := range []bool{false, true} {
		records := advancedmdtest.NewAdapter()
		read := advancedmd.AppointmentRead{Complete: false, ProviderReads: 6}
		if partial {
			read.Appointments = []domain.PatientAppointment{{ID: 123}}
		}
		records.AppointmentResults["123"] = advancedmdtest.AppointmentResult{Read: read}
		result, err := patient.New(records).Resolve(context.Background(), patient.ResolveCommand{PatientID: "123", OfficeID: "spring_hill"})
		if err != nil || result.Status != patient.StatusVerified || result.AppointmentsStatus != patient.AppointmentsError || len(result.Appointments) != 0 || result.ProviderFailure != safeerrors.CategoryInvalidResponse || result.Observation.AppointmentOutcome != "error" || result.Observation.AppointmentReads != 6 {
			t.Fatalf("partial=%v: err=%v result=%+v", partial, err, result)
		}
	}
}
