package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"advancedmd-token-management/internal/clients"
	"advancedmd-token-management/internal/domain"
	schedulingmodule "advancedmd-token-management/internal/scheduling"
	"advancedmd-token-management/internal/session"
)

func TestSchedulingWorkflowBooksSignedSlotWithItsPolicyAndReceipt(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	token, err := schedulingmodule.SignSlotToken("test-booking-secret", schedulingmodule.SlotPolicy{
		OfficeID:           "spring_hill",
		Routing:            string(domain.RoutingBachOnly),
		ColumnID:           1513,
		ProfileID:          620,
		StartDatetime:      "2026-06-03T09:00",
		Duration:           15,
		DOB:                "01/15/1980",
		AppointmentTypeIDs: []int{1007},
		SameStartBooked:    1,
		SameStartCapacity:  2,
		RequiresForce:      true,
		Provider:           "Dr. Austin Bach",
		IssuedAt:           now.Unix(),
		ExpiresAt:          now.Add(15 * time.Minute).Unix(),
	})
	if err != nil {
		t.Fatalf("SignSlotToken error = %v", err)
	}

	status := http.StatusOK
	var payload map[string]any
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/scheduler/Appointments" {
			http.NotFound(w, r)
			return
		}
		if status == http.StatusConflict {
			http.Error(w, "conflict", status)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode booking payload: %v", err)
		}
		w.Write([]byte(`{"id":98765}`))
	}))
	defer server.Close()

	workflow := newSchedulingWorkflow(
		bookingStaticSession{token: &domain.TokenData{
			Token:       "Bearer test-token",
			RestApiBase: strings.TrimPrefix(server.URL, "https://"),
		}},
		clients.NewAdvancedMDRestClient(server.Client()),
		"test-booking-secret",
	)

	_, workflowErr := workflow.Book(context.Background(), BookAppointmentRequest{
		PatientID:         "123",
		BookingToken:      token,
		AppointmentTypeID: 1004,
	}, now.Add(time.Minute))
	if workflowErr == nil {
		t.Fatalf("changed appointment-type policy error = %#v", workflowErr)
	}
	_, workflowErr = workflow.Book(context.Background(), BookAppointmentRequest{
		PatientID:         "123",
		DOB:               "01/15/2010",
		BookingToken:      token,
		AppointmentTypeID: 1007,
	}, now.Add(time.Minute))
	if workflowErr == nil || workflowErr.outcome != "invalid_booking_token" {
		t.Fatalf("changed DOB error = %#v", workflowErr)
	}

	receipt, workflowErr := workflow.Book(context.Background(), BookAppointmentRequest{
		PatientID:         "123",
		PatientName:       "SMITH,JANE",
		BookingToken:      token,
		AppointmentTypeID: 1007,
	}, now.Add(time.Minute))
	if workflowErr != nil {
		t.Fatalf("Book error = %#v", workflowErr)
	}
	if receipt.Status != "booked" ||
		receipt.AppointmentID != 98765 ||
		receipt.PatientName != "Jane Smith" ||
		receipt.ProviderName != "Dr. Austin Bach" {
		t.Fatalf("receipt = %#v", receipt)
	}
	if payload["force"] != float64(1) {
		t.Fatalf("booking force = %#v, payload = %#v", payload["force"], payload)
	}

	status = http.StatusConflict
	_, workflowErr = workflow.Book(context.Background(), BookAppointmentRequest{
		PatientID:         "123",
		BookingToken:      token,
		AppointmentTypeID: 1007,
	}, now.Add(2*time.Minute))
	if workflowErr == nil || workflowErr.outcome != "slot_unavailable" {
		t.Fatalf("conflict error = %#v, want slot_unavailable", workflowErr)
	}
}

type bookingStaticSession struct {
	token *domain.TokenData
}

func (s bookingStaticSession) Get(context.Context) (*domain.TokenData, error) {
	return s.token, nil
}

func (bookingStaticSession) Maintain(context.Context) error {
	return nil
}

func (bookingStaticSession) Status() session.SessionStatus {
	return session.SessionStatus{}
}
