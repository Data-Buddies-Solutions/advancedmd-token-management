package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"advancedmd-token-management/internal/domain"
	"advancedmd-token-management/internal/scheduling"
)

type deadlineScheduling struct {
	scheduling.Scheduling
	received context.Context
}

func (s *deadlineScheduling) List(ctx context.Context, _ scheduling.ListCommand) (domain.AvailabilityResponse, error) {
	s.received = ctx
	return domain.AvailabilityResponse{Status: domain.AvailabilityStatusSuccess}, nil
}

func TestInventoryRequestReceivesTotalDeadline(t *testing.T) {
	for _, shorterParent := range []bool{false, true} {
		scheduler := &deadlineScheduling{}
		router := NewRouter(NewHandlers(nil, nil, scheduler), "test-secret", nil)
		req := httptest.NewRequest(http.MethodPost, "/api/scheduler/slots", strings.NewReader(`{"office":"Spring Hill"}`))
		req.Header.Set("Authorization", "Bearer test-secret")
		limit := time.Now().Add(50 * time.Second)
		if shorterParent {
			limit = time.Now().Add(time.Second)
			ctx, cancel := context.WithDeadline(req.Context(), limit)
			defer cancel()
			req = req.WithContext(ctx)
		}
		router.ServeHTTP(httptest.NewRecorder(), req)
		if scheduler.received == nil {
			t.Fatal("workflow not reached")
		}
		deadline, ok := scheduler.received.Deadline()
		if !ok || deadline.After(limit.Add(100*time.Millisecond)) {
			t.Fatalf("deadline=%v present=%v", deadline, ok)
		}
		if scheduler.received.Err() != context.Canceled {
			t.Fatalf("completed request context not canceled: %v", scheduler.received.Err())
		}
	}
}
