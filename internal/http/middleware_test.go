package http

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"advancedmd-token-management/internal/advancedmd"
	"advancedmd-token-management/internal/advancedmd/advancedmdtest"
	patientmodule "advancedmd-token-management/internal/patient"
	"advancedmd-token-management/internal/safeerrors"
	"advancedmd-token-management/internal/safelog"
	schedulingmodule "advancedmd-token-management/internal/scheduling"

	"github.com/go-chi/chi/v5"
)

func TestRequestIDMiddlewareHashesCallerValueForLogs(t *testing.T) {
	var requestID string
	var logRequestID string
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID = GetRequestID(r.Context())
		logRequestID = GetLogRequestID(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Request-ID", "patientId=17604634 token=secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if got := w.Header().Get("X-Request-ID"); got != "patientId=17604634 token=secret" {
		t.Fatalf("X-Request-ID = %q, want caller value echoed", got)
	}
	if requestID != "patientId=17604634 token=secret" {
		t.Fatalf("request ID = %q, want caller value retained", requestID)
	}
	if !strings.HasPrefix(logRequestID, "external-") {
		t.Fatalf("log request ID = %q, want hashed external ID", logRequestID)
	}
	for _, forbidden := range []string{"17604634", "secret"} {
		if strings.Contains(logRequestID, forbidden) {
			t.Fatalf("log request ID %q exposed %q", logRequestID, forbidden)
		}
	}
}

func TestRequestLogIsStructuredAndPHISafe(t *testing.T) {
	var logs bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(safelog.NewWriter(&logs))
	t.Cleanup(func() { log.SetOutput(previousWriter) })

	router := NewRouter(
		NewHandlers(unavailableSession{}, nil, nil, nil, nil),
		"test-secret",
		nil,
	)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/patient/resolve",
		strings.NewReader(`{"patientId":"17604634"}`),
	)
	req.Header.Set("Authorization", "Bearer test-secret")
	req.Header.Set("X-Request-ID", "patientId=17604634 token=secret")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	entry := decodeLastLogEntry(t, logs.String())
	want := map[string]any{
		"route_template":            "/api/patient/resolve",
		"outcome_category":          "provider_failure",
		"session_state":             "unavailable",
		"provider_failure_category": "unavailable",
	}
	for field, expected := range want {
		if entry[field] != expected {
			t.Errorf("%s = %v, want %v", field, entry[field], expected)
		}
	}
	if _, ok := entry["latency_ms"].(float64); !ok {
		t.Errorf("latency_ms = %T, want JSON number", entry["latency_ms"])
	}
	if requestID, ok := entry["request_id"].(string); !ok || !strings.HasPrefix(requestID, "external-") {
		t.Errorf("request_id = %v, want hashed external ID", entry["request_id"])
	}
	if len(entry) != 6 {
		t.Errorf("log fields = %v, want fixed six-field schema", entry)
	}
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		if !json.Valid([]byte(line)) {
			t.Errorf("log line is not structured JSON: %q", line)
		}
	}
	for _, forbidden := range []string{"17604634", "secret", "patientId", "https://provider.example/private"} {
		if strings.Contains(logs.String(), forbidden) {
			t.Errorf("logs exposed %q: %s", forbidden, logs.String())
		}
	}
}

func TestRequestLogUsesSafeFallbackForUnmatchedRoute(t *testing.T) {
	var logs bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousWriter) })

	router := NewRouter(NewHandlers(nil, nil, nil, nil, nil), "test-secret", nil)
	req := httptest.NewRequest(http.MethodGet, "/patients/17604634", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	entry := decodeLastLogEntry(t, logs.String())
	if entry["route_template"] != "unmatched" {
		t.Errorf("route_template = %v, want unmatched", entry["route_template"])
	}
	if entry["outcome_category"] != "not_found" {
		t.Errorf("outcome_category = %v, want not_found", entry["outcome_category"])
	}
	if strings.Contains(logs.String(), "17604634") {
		t.Errorf("logs exposed unmatched URL: %s", logs.String())
	}
}

func TestRequestLogRecoversPanicWithoutLoggingRawError(t *testing.T) {
	var logs bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousWriter) })

	router := chi.NewRouter()
	router.Use(RequestIDMiddleware)
	router.Use(LoggingMiddleware(nil))
	router.Use(recoveryMiddleware)
	router.Get("/panic", func(http.ResponseWriter, *http.Request) {
		panic("patientId=17604634 https://provider.example/private")
	})
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	entry := decodeLastLogEntry(t, logs.String())
	if entry["outcome_category"] != "internal_failure" {
		t.Errorf("outcome_category = %v, want internal_failure", entry["outcome_category"])
	}
	for _, forbidden := range []string{"17604634", "provider.example", "private"} {
		if strings.Contains(logs.String(), forbidden) {
			t.Errorf("logs exposed panic detail %q: %s", forbidden, logs.String())
		}
	}
}

func TestRequestLogRecordsInvalidJSONWithoutInspectingBodies(t *testing.T) {
	paths := []string{
		"/api/patient/resolve",
		"/api/add-patient",
		"/api/scheduler/availability",
		"/api/appointment/book",
		"/api/appointment/cancel",
		"/api/patient/update-insurance",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			var logs bytes.Buffer
			previousWriter := log.Writer()
			log.SetOutput(&logs)
			t.Cleanup(func() { log.SetOutput(previousWriter) })

			router := NewRouter(NewHandlers(nil, nil, nil, nil, nil), "test-secret", nil)
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"patientId":"17604634"`))
			req.Header.Set("Authorization", "Bearer test-secret")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			entry := decodeLastLogEntry(t, logs.String())
			if entry["outcome_category"] != "invalid_request" {
				t.Errorf("outcome_category = %v, want invalid_request", entry["outcome_category"])
			}
			if entry["provider_failure_category"] != "none" {
				t.Errorf("provider_failure_category = %v, want none", entry["provider_failure_category"])
			}
			if strings.Contains(logs.String(), "17604634") {
				t.Errorf("logs exposed request body: %s", logs.String())
			}
		})
	}
}

func TestRequestLogPreservesAvailabilityProviderFailureCategory(t *testing.T) {
	var logs bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousWriter) })

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	records := advancedmdtest.NewAdapter()
	records.SchedulerSetupError = advancedmd.NewError(safeerrors.CategoryAuthentication)
	scheduler := schedulingmodule.New(records, "test-booking-secret", func() time.Time { return now })
	router := NewRouter(&Handlers{scheduling: scheduler}, "test-secret", nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/scheduler/availability",
		strings.NewReader(`{"date":"2026-06-03","office":"Spring Hill","routing":"bach_only"}`),
	)
	req.Header.Set("Authorization", "Bearer test-secret")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	entry := decodeLastLogEntry(t, logs.String())
	if entry["outcome_category"] != "provider_failure" {
		t.Errorf("outcome_category = %v, want provider_failure", entry["outcome_category"])
	}
	if entry["provider_failure_category"] != "authentication" {
		t.Errorf("provider_failure_category = %v, want authentication", entry["provider_failure_category"])
	}
}

func TestRequestLogCategorizesPatientMutationFailures(t *testing.T) {
	patients := &patientStub{
		createResult: patientmodule.CreateResult{
			Status:  patientmodule.CreateStatusError,
			Outcome: patientmodule.MutationUnavailable,
		},
		updateResult: patientmodule.UpdateInsuranceResult{
			Status:  patientmodule.UpdateInsuranceStatusError,
			Outcome: patientmodule.MutationRejected,
		},
	}
	router := NewRouter(&Handlers{patient: patients}, "test-secret", nil)
	tests := []struct {
		path            string
		body            string
		providerFailure string
	}{
		{
			path:            "/api/add-patient",
			body:            `{}`,
			providerFailure: "unavailable",
		},
		{
			path:            "/api/patient/update-insurance",
			body:            `{}`,
			providerFailure: "rejected",
		},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			var logs bytes.Buffer
			previousWriter := log.Writer()
			log.SetOutput(&logs)
			t.Cleanup(func() { log.SetOutput(previousWriter) })

			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Authorization", "Bearer test-secret")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			entry := decodeLastLogEntry(t, logs.String())
			if entry["outcome_category"] != "provider_failure" {
				t.Errorf("outcome_category = %v, want provider_failure", entry["outcome_category"])
			}
			if entry["provider_failure_category"] != tt.providerFailure {
				t.Errorf(
					"provider_failure_category = %v, want %s",
					entry["provider_failure_category"],
					tt.providerFailure,
				)
			}
		})
	}
}

func TestRequestLogPreservesPartialPatientProviderFailure(t *testing.T) {
	var logs bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousWriter) })

	patients := &patientStub{resolveResult: patientmodule.ResolveResult{
		Status:             patientmodule.StatusVerified,
		AppointmentsStatus: patientmodule.AppointmentsError,
		Appointments:       []patientmodule.Appointment{},
		ProviderFailure:    safeerrors.CategoryUpstreamStatus,
	}}
	router := NewRouter(&Handlers{patient: patients}, "test-secret", nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/patient/resolve",
		strings.NewReader(`{"patientId":"17604634"}`),
	)
	req.Header.Set("Authorization", "Bearer test-secret")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	entry := decodeLastLogEntry(t, logs.String())
	if entry["outcome_category"] != "provider_failure" {
		t.Errorf("outcome_category = %v, want provider_failure", entry["outcome_category"])
	}
	if entry["provider_failure_category"] != "upstream_status" {
		t.Errorf("provider_failure_category = %v, want upstream_status", entry["provider_failure_category"])
	}
}

func TestRequestLogTreatsSuccessfulPatientResolveAsSuccess(t *testing.T) {
	var logs bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousWriter) })

	patients := &patientStub{resolveResult: patientmodule.ResolveResult{
		Status: patientmodule.StatusVerified,
	}}
	router := NewRouter(&Handlers{patient: patients}, "test-secret", nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/patient/resolve",
		strings.NewReader(`{"patientId":"17604634"}`),
	)
	req.Header.Set("Authorization", "Bearer test-secret")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	entry := decodeLastLogEntry(t, logs.String())
	if entry["outcome_category"] != "success" {
		t.Errorf("outcome_category = %v, want success", entry["outcome_category"])
	}
	if entry["provider_failure_category"] != "none" {
		t.Errorf("provider_failure_category = %v, want none", entry["provider_failure_category"])
	}
}

func decodeLastLogEntry(t *testing.T, output string) map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		t.Fatalf("last log line is not JSON: %q: %v", lines[len(lines)-1], err)
	}
	return entry
}
