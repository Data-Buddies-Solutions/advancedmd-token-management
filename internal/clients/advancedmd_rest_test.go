package clients

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"advancedmd-token-management/internal/domain"
)

// newTestRestClient creates a TLS test server and REST client wired together.
// The handler receives all requests. Returns the client, tokenData pointing at the server, and a cleanup func.
func newTestRestClient(t *testing.T, handler http.Handler) (*AdvancedMDRestClient, *domain.TokenData, func()) {
	t.Helper()
	server := httptest.NewTLSServer(handler)

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Strip "https://" from server.URL to match RestApiBase format
	restBase := server.URL[8:]

	tokenData := &domain.TokenData{
		Token:       "Bearer test-token",
		RestApiBase: restBase,
	}

	return NewAdvancedMDRestClient(httpClient), tokenData, server.Close
}

func TestGetAppointmentsForColumns_Concurrent(t *testing.T) {
	var callCount int64

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&callCount, 1)
		// Simulate AMD latency
		time.Sleep(50 * time.Millisecond)

		colID := r.URL.Query().Get("columnId")
		appts := []AMDAppointmentResponse{
			{
				ID:            1,
				StartDateTime: fmt.Sprintf("2026-03-03T09:00:00"),
				Duration:      15,
				ColumnID:      0,
				PatientID:     100,
			},
		}
		// Tag the response with the column ID so we can verify correct mapping
		if colID == "1513" {
			appts[0].ColumnID = 1513
		} else if colID == "1551" {
			appts[0].ColumnID = 1551
		} else if colID == "1550" {
			appts[0].ColumnID = 1550
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(appts)
	})

	client, tokenData, cleanup := newTestRestClient(t, handler)
	defer cleanup()

	columnIDs := []string{"1513", "1551", "1550"}

	start := time.Now()
	result, err := client.GetAppointmentsForColumns(context.Background(), tokenData, columnIDs, "2026-03-03")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("GetAppointmentsForColumns failed: %v", err)
	}

	// Verify all 3 columns returned
	if len(result) != 3 {
		t.Fatalf("Expected 3 columns in result, got %d", len(result))
	}

	// Verify correct mapping (each column got its own data)
	for _, colID := range columnIDs {
		appts, ok := result[colID]
		if !ok {
			t.Errorf("Missing results for column %s", colID)
			continue
		}
		if len(appts) != 1 {
			t.Errorf("Column %s: expected 1 appointment, got %d", colID, len(appts))
		}
	}

	// Verify all 3 calls were made
	if atomic.LoadInt64(&callCount) != 3 {
		t.Errorf("Expected 3 HTTP calls, got %d", callCount)
	}

	// Verify concurrency: 3 calls x 50ms each should take ~50-100ms concurrent, not ~150ms+ sequential
	if elapsed > 140*time.Millisecond {
		t.Errorf("Expected concurrent execution (~50-100ms), but took %v (likely sequential)", elapsed)
	}
}

func TestGetBlockHoldsForColumns_Concurrent(t *testing.T) {
	var callCount int64

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&callCount, 1)
		time.Sleep(50 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]AMDBlockHoldResponse{})
	})

	client, tokenData, cleanup := newTestRestClient(t, handler)
	defer cleanup()

	columnIDs := []string{"1513", "1551", "1550"}

	start := time.Now()
	result, err := client.GetBlockHoldsForColumns(context.Background(), tokenData, columnIDs, "2026-03-03")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("GetBlockHoldsForColumns failed: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("Expected 3 columns in result, got %d", len(result))
	}

	if atomic.LoadInt64(&callCount) != 3 {
		t.Errorf("Expected 3 HTTP calls, got %d", callCount)
	}

	if elapsed > 140*time.Millisecond {
		t.Errorf("Expected concurrent execution (~50-100ms), but took %v (likely sequential)", elapsed)
	}
}

func TestGetAppointmentsForColumns_ErrorPropagation(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		colID := r.URL.Query().Get("columnId")
		if colID == "1551" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("AMD is down"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]AMDAppointmentResponse{})
	})

	client, tokenData, cleanup := newTestRestClient(t, handler)
	defer cleanup()

	_, err := client.GetAppointmentsForColumns(context.Background(), tokenData, []string{"1513", "1551", "1550"}, "2026-03-03")
	if err == nil {
		t.Fatal("Expected error when one column fails, got nil")
	}
}

func TestGetAppointmentsForColumns_EmptyColumns(t *testing.T) {
	client, tokenData, cleanup := newTestRestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("No HTTP calls should be made for empty column list")
	}))
	defer cleanup()

	result, err := client.GetAppointmentsForColumns(context.Background(), tokenData, []string{}, "2026-03-03")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d entries", len(result))
	}
}

func TestGetAppointmentsForColumns_SingleColumn(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]AMDAppointmentResponse{
			{ID: 1, StartDateTime: "2026-03-03T10:00:00", Duration: 15, ColumnID: 1513},
		})
	})

	client, tokenData, cleanup := newTestRestClient(t, handler)
	defer cleanup()

	result, err := client.GetAppointmentsForColumns(context.Background(), tokenData, []string{"1513"}, "2026-03-03")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("Expected 1 column, got %d", len(result))
	}
	if len(result["1513"]) != 1 {
		t.Errorf("Expected 1 appointment for column 1513, got %d", len(result["1513"]))
	}
}
