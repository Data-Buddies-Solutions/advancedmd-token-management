package advancedmd

import (
	"context"
	"net/http"
	"testing"
	"time"

	"advancedmd-token-management/internal/clients"
	"advancedmd-token-management/internal/domain"
	"advancedmd-token-management/internal/safeerrors"
)

type noWriteTransport struct{ t *testing.T }

func (f noWriteTransport) RoundTrip(*http.Request) (*http.Response, error) {
	f.t.Error("mutation sent without reconciliation budget")
	return nil, context.DeadlineExceeded
}

func TestNoMutationStartsWithoutReconciliationBudget(t *testing.T) {
	client := &http.Client{Transport: noWriteTransport{t}}
	adapter := NewAdapter(staticSession{token: &domain.TokenData{}}, clients.NewAdvancedMDClient(client), clients.NewAdvancedMDRestClient(client))
	mutations := map[string]func(context.Context) error{
		"create patient": func(ctx context.Context) error {
			_, err := adapter.CreatePatient(ctx, domain.PatientCreate{OfficeID: "spring_hill", FirstName: "Jane", LastName: "Doe", DOB: "01/15/1980", Phone: "5551234567"})
			return err
		},
		"add insurance": func(ctx context.Context) error {
			return adapter.AddPatientInsurance(ctx, domain.PatientInsurance{PatientID: "12345", RespPartyID: "123", CarrierID: "car40906", SubscriberNum: "test"})
		},
		"end insurance": func(ctx context.Context) error {
			return adapter.EndDatePatientInsurance(ctx, domain.PatientInsuranceEnd{PatientID: "12345", InsPlanID: "ins123"})
		},
		"book": func(ctx context.Context) error {
			_, err := adapter.BookAppointment(ctx, Booking{OfficeID: "spring_hill", PatientID: 12345, ColumnID: 1513, ProfileID: 620, Start: time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC), Duration: 15, ProviderAppointmentTypeID: 1007, AppointmentColor: "#FFFFFF"})
			return err
		},
		"cancel": func(ctx context.Context) error {
			return adapter.CancelAppointment(ctx, Cancellation{PatientID: "12345", AppointmentID: 98765, OfficeID: "spring_hill"})
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err := mutate(ctx)
			if CategoryOf(err) != safeerrors.CategoryTimeout || IsAmbiguousWrite(err) {
				t.Fatalf("err=%v ambiguous=%v", err, IsAmbiguousWrite(err))
			}
		})
	}
}
