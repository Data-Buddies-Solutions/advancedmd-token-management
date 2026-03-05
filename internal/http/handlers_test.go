package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"advancedmd-token-management/internal/clients"
	"advancedmd-token-management/internal/domain"
)

func TestHandleHealth(t *testing.T) {
	handlers := &Handlers{}

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handlers.HandleHealth(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", body["status"])
	}
}

func TestHandleVerifyPatient_ValidationErrors(t *testing.T) {
	handlers := &Handlers{}

	tests := []struct {
		name           string
		method         string
		body           string
		expectedStatus int
		expectedMsg    string
	}{
		{
			name:           "invalid JSON",
			method:         "POST",
			body:           "not json",
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "Invalid JSON body",
		},
		{
			name:           "missing lastName",
			method:         "POST",
			body:           `{"dob":"01/15/1980"}`,
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "lastName is required",
		},
		{
			name:           "missing dob",
			method:         "POST",
			body:           `{"lastName":"Smith"}`,
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "dob is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/verify-patient", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handlers.HandleVerifyPatient(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			var body VerifyPatientResponse
			json.NewDecoder(resp.Body).Decode(&body)
			if body.Message != tt.expectedMsg {
				t.Errorf("Expected message '%s', got '%s'", tt.expectedMsg, body.Message)
			}
		})
	}
}

func TestAuthMiddleware(t *testing.T) {
	apiSecret := "test-secret-123"
	middleware := AuthMiddleware(apiSecret)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "no auth header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "wrong secret",
			authHeader:     "Bearer wrong-secret",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "valid bearer token",
			authHeader:     "Bearer test-secret-123",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "valid raw secret",
			authHeader:     "test-secret-123",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/token", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request ID is in context
		requestID := GetRequestID(r.Context())
		if requestID == "" {
			t.Error("Expected request ID in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("generates new request ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		requestID := w.Header().Get("X-Request-ID")
		if requestID == "" {
			t.Error("Expected X-Request-ID header")
		}
	})

	t.Run("uses existing request ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Request-ID", "existing-id-123")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		requestID := w.Header().Get("X-Request-ID")
		if requestID != "existing-id-123" {
			t.Errorf("Expected 'existing-id-123', got '%s'", requestID)
		}
	})
}

func TestCalculateAvailableSlots_AllBlocked(t *testing.T) {
	eastern, _ := time.LoadLocation("America/New_York")
	// Use a future Monday so it's not "today"
	date := time.Date(2026, 6, 1, 0, 0, 0, 0, eastern)
	nowEastern := time.Date(2026, 3, 3, 10, 0, 0, 0, eastern)

	col := domain.SchedulerColumn{
		ID:              "1513",
		Name:            "DR. BACH - BP",
		StartTime:       "08:00",
		EndTime:         "17:00",
		Interval:        15,
		MaxApptsPerSlot: 0,
		Workweek:        62, // Mon-Fri
	}

	// Block hold covering the entire work day
	blockHolds := []domain.BlockHold{
		{
			StartDateTime: time.Date(2026, 6, 1, 8, 0, 0, 0, eastern),
			EndDateTime:   time.Date(2026, 6, 1, 17, 0, 0, 0, eastern),
			Note:          "OUT OF THE OFFICE",
		},
	}

	slots := calculateAvailableSlots(col, nil, blockHolds, date, nowEastern)

	if len(slots) != 0 {
		t.Errorf("Expected 0 slots when entire day is blocked, got %d", len(slots))
	}
}

func TestCalculateAvailableSlots_AllBookedAtMax(t *testing.T) {
	eastern, _ := time.LoadLocation("America/New_York")
	nowEastern := time.Date(2026, 3, 3, 10, 0, 0, 0, eastern)

	col := domain.SchedulerColumn{
		ID:              "1551",
		Name:            "DR. LICHT",
		StartTime:       "09:00",
		EndTime:         "10:00",
		Interval:        15,
		MaxApptsPerSlot: 2, // Max 2 per slot
		Workweek:        24, // Wed-Thu
	}

	// June 3 2026 is a Wednesday
	date := time.Date(2026, 6, 3, 0, 0, 0, 0, eastern)

	// Fill every slot with 2 appointments
	var appointments []domain.Appointment
	for h := 9; h < 10; h++ {
		for m := 0; m < 60; m += 15 {
			for i := 0; i < 2; i++ {
				appointments = append(appointments, domain.Appointment{
					StartDateTime: time.Date(2026, 6, 3, h, m, 0, 0, eastern),
					Duration:      15,
				})
			}
		}
	}

	slots := calculateAvailableSlots(col, appointments, nil, date, nowEastern)

	if len(slots) != 0 {
		t.Errorf("Expected 0 slots when all slots at max capacity, got %d", len(slots))
	}
}

func TestNoAvailabilityResponse_HasMessageAndEmptyProviders(t *testing.T) {
	resp := domain.AvailabilityResponse{
		SearchedDate: "2026-05-15",
		Date:         "",
		Location:     "ABITA EYE GROUP SPRING HILL",
		Message:      "No availability found within 14 days of requested date",
		Providers:    []domain.ProviderAvailability{},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	var decoded map[string]interface{}
	json.Unmarshal(data, &decoded)

	if decoded["date"] != "" {
		t.Errorf("Expected empty date, got %q", decoded["date"])
	}
	if decoded["message"] != "No availability found within 14 days of requested date" {
		t.Errorf("Expected no-availability message, got %q", decoded["message"])
	}
	providers := decoded["providers"].([]interface{})
	if len(providers) != 0 {
		t.Errorf("Expected empty providers array, got %d", len(providers))
	}
}

func TestAvailabilityResponse_OmitsMessageWhenEmpty(t *testing.T) {
	resp := domain.AvailabilityResponse{
		SearchedDate: "2026-05-15",
		Date:         "Monday, June 1, 2026",
		Location:     "ABITA EYE GROUP SPRING HILL",
		Providers:    []domain.ProviderAvailability{},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	var decoded map[string]interface{}
	json.Unmarshal(data, &decoded)

	if _, exists := decoded["message"]; exists {
		t.Error("Expected message field to be omitted when empty")
	}
}

func TestCountOverlappingAppointments(t *testing.T) {
	eastern, _ := time.LoadLocation("America/New_York")

	tests := []struct {
		name         string
		slotTime     time.Time
		appointments []domain.Appointment
		expected     int
	}{
		{
			name:         "no appointments",
			slotTime:     time.Date(2026, 3, 6, 9, 30, 0, 0, eastern),
			appointments: nil,
			expected:     0,
		},
		{
			name:     "30-min appt only counts at its own slot",
			slotTime: time.Date(2026, 3, 6, 9, 30, 0, 0, eastern),
			appointments: []domain.Appointment{
				{StartDateTime: time.Date(2026, 3, 6, 9, 0, 0, 0, eastern), Duration: 30},
			},
			expected: 0, // 9:00+30min=9:30, slot 9:30 is NOT inside [9:00, 9:30)
		},
		{
			name:     "60-min appt spans into next slot",
			slotTime: time.Date(2026, 3, 6, 9, 30, 0, 0, eastern),
			appointments: []domain.Appointment{
				{StartDateTime: time.Date(2026, 3, 6, 9, 0, 0, 0, eastern), Duration: 60},
			},
			expected: 1, // 9:30 falls within [9:00, 10:00)
		},
		{
			name:     "60-min appt does not span past its end",
			slotTime: time.Date(2026, 3, 6, 10, 0, 0, 0, eastern),
			appointments: []domain.Appointment{
				{StartDateTime: time.Date(2026, 3, 6, 9, 0, 0, 0, eastern), Duration: 60},
			},
			expected: 0, // 10:00 is NOT inside [9:00, 10:00)
		},
		{
			name:     "Dr Noel 3/6 scenario — 60-min Vargas + 30-min Prater at 9:30",
			slotTime: time.Date(2026, 3, 6, 9, 30, 0, 0, eastern),
			appointments: []domain.Appointment{
				{StartDateTime: time.Date(2026, 3, 6, 9, 0, 0, 0, eastern), Duration: 60},  // Vargas
				{StartDateTime: time.Date(2026, 3, 6, 9, 30, 0, 0, eastern), Duration: 30}, // Prater
			},
			expected: 2, // both occupy 9:30
		},
		{
			name:     "multiple overlapping appointments at same slot",
			slotTime: time.Date(2026, 3, 6, 10, 0, 0, 0, eastern),
			appointments: []domain.Appointment{
				{StartDateTime: time.Date(2026, 3, 6, 9, 30, 0, 0, eastern), Duration: 60},  // spans 9:30-10:30
				{StartDateTime: time.Date(2026, 3, 6, 10, 0, 0, 0, eastern), Duration: 30}, // starts at 10:00
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countOverlappingAppointments(tt.slotTime, tt.appointments)
			if got != tt.expected {
				t.Errorf("Expected %d overlapping appointments, got %d", tt.expected, got)
			}
		})
	}
}

func TestCalculateAvailableSlots_MultiSlotAppointment(t *testing.T) {
	eastern, _ := time.LoadLocation("America/New_York")
	// Use a future Friday so it's not "today"
	date := time.Date(2026, 3, 6, 0, 0, 0, 0, eastern)
	nowEastern := time.Date(2026, 3, 5, 10, 0, 0, 0, eastern) // day before

	// Dr. Noel: 30-min intervals, max 2 per slot, 8:30-16:30
	col := domain.SchedulerColumn{
		ID:              "1550",
		Name:            "DR. NOEL",
		StartTime:       "08:30",
		EndTime:         "10:30",
		Interval:        30,
		MaxApptsPerSlot: 2,
		Workweek:        62, // Mon-Fri
	}

	// Simulate: 60-min appt at 9:00 (Vargas) + 30-min appt at 9:30 (Prater)
	// This should fill the 9:30 slot (2 overlapping = max)
	appointments := []domain.Appointment{
		{StartDateTime: time.Date(2026, 3, 6, 9, 0, 0, 0, eastern), Duration: 60},  // Vargas 9:00-10:00
		{StartDateTime: time.Date(2026, 3, 6, 9, 30, 0, 0, eastern), Duration: 30}, // Prater 9:30-10:00
	}

	// Block hold at 8:30 (OUT OF THE OFFICE)
	blockHolds := []domain.BlockHold{
		{
			StartDateTime: time.Date(2026, 3, 6, 8, 30, 0, 0, eastern),
			EndDateTime:   time.Date(2026, 3, 6, 9, 0, 0, 0, eastern),
			Note:          "OUT OF THE OFFICE",
		},
	}

	slots := calculateAvailableSlots(col, appointments, blockHolds, date, nowEastern)

	// Available slots should be: 10:00 (Vargas ends, Prater ends)
	// 8:30 — blocked by hold
	// 9:00 — 1 appt (Vargas), under max 2 → available
	// 9:30 — 2 appts (Vargas overlap + Prater), at max → blocked
	// 10:00 — 0 appts → available

	expectedTimes := map[string]bool{
		"9:00 AM":  true,
		"10:00 AM": true,
	}

	if len(slots) != len(expectedTimes) {
		t.Errorf("Expected %d available slots, got %d: %v", len(expectedTimes), len(slots), slots)
		return
	}

	for _, slot := range slots {
		if !expectedTimes[slot.Time] {
			t.Errorf("Unexpected slot: %s", slot.Time)
		}
	}
}

func TestCalculateAvailableSlots_UnlimitedMaxAppts(t *testing.T) {
	eastern, _ := time.LoadLocation("America/New_York")
	date := time.Date(2026, 6, 1, 0, 0, 0, 0, eastern) // Monday
	nowEastern := time.Date(2026, 5, 31, 10, 0, 0, 0, eastern)

	// Dr. Bach: max=0 (unlimited), 15-min intervals
	col := domain.SchedulerColumn{
		ID:              "1513",
		Name:            "DR. BACH - BP",
		StartTime:       "09:00",
		EndTime:         "09:30",
		Interval:        15,
		MaxApptsPerSlot: 0, // unlimited
		Workweek:        62,
	}

	// Even with many appointments stacked, all slots should remain available
	appointments := []domain.Appointment{
		{StartDateTime: time.Date(2026, 6, 1, 9, 0, 0, 0, eastern), Duration: 15},
		{StartDateTime: time.Date(2026, 6, 1, 9, 0, 0, 0, eastern), Duration: 15},
		{StartDateTime: time.Date(2026, 6, 1, 9, 0, 0, 0, eastern), Duration: 15},
		{StartDateTime: time.Date(2026, 6, 1, 9, 15, 0, 0, eastern), Duration: 15},
	}

	slots := calculateAvailableSlots(col, appointments, nil, date, nowEastern)

	// Both 9:00 and 9:15 should be available despite appointments
	if len(slots) != 2 {
		t.Errorf("Expected 2 slots with unlimited max, got %d", len(slots))
	}
}

func TestRouter(t *testing.T) {
	// Create minimal handlers for testing
	amdClient := clients.NewAdvancedMDClient(&http.Client{})
	handlers := NewHandlers(nil, amdClient, nil) // nil token manager - can't test full flow

	router := NewRouter(handlers, "test-secret")

	t.Run("health endpoint no auth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
	})

	t.Run("api endpoints require auth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/token", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 without auth, got %d", w.Code)
		}
	})

	t.Run("api endpoints with auth", func(t *testing.T) {
		// Skip this test - it requires a real token manager
		// The important thing is that auth middleware works (tested above)
		t.Skip("Requires non-nil token manager")
	})
}
