package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"advancedmd-token-management/internal/domain"
)

// ParseDateTime parses an AMD datetime string trying multiple known formats.
// Returns timezone-stripped wall-clock time for consistent comparison with slot times.
func ParseDateTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty datetime string")
	}
	for _, layout := range []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.999999999",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(),
				t.Hour(), t.Minute(), t.Second(), 0, time.UTC), nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse datetime %q", s)
}

// AdvancedMDRestClient handles REST API calls to AdvancedMD.
type AdvancedMDRestClient struct {
	httpClient *http.Client
}

// NewAdvancedMDRestClient creates a new AdvancedMD REST client.
func NewAdvancedMDRestClient(httpClient *http.Client) *AdvancedMDRestClient {
	return &AdvancedMDRestClient{httpClient: httpClient}
}

// AMDAppointmentResponse represents a single appointment from the REST API.
type AMDAppointmentResponse struct {
	ID               int     `json:"id"`
	StartDateTime    string  `json:"startdatetime"`
	Duration         int     `json:"duration"`
	ColumnID         int     `json:"columnid"`
	ProfileID        int     `json:"profileid"`
	Provider         string  `json:"provider"`
	Heading          string  `json:"heading"`
	Facility         string  `json:"facility"`
	FacilityID       int     `json:"facilityid"`
	AppointmentTypes []int   `json:"appointmenttypeids"`
	PatientID        int     `json:"patientid"`
	FirstName        string  `json:"firstname"`
	LastName         string  `json:"lastname"`
	ConfirmDate      *string `json:"confirmdate"`
	ConfirmMethod    *string `json:"confirmmethod"`
}

// GetAppointments fetches appointments for a column within a date range.
// startDate should be in YYYY-MM-DD format.
func (c *AdvancedMDRestClient) GetAppointments(ctx context.Context, tokenData *domain.TokenData, columnID string, startDate string) ([]domain.Appointment, error) {
	url := fmt.Sprintf("https://%s/scheduler/appointments?columnId=%s&forView=day&isLegacy=true&startDate=%s",
		tokenData.RestApiBase, columnID, startDate)

	body, err := c.getResponseBody(ctx, tokenData, url)
	if err != nil {
		return nil, err
	}

	// Handle AMD single-vs-array response quirk
	var amdAppts []AMDAppointmentResponse
	if err := json.Unmarshal(body, &amdAppts); err != nil {
		var single AMDAppointmentResponse
		if err2 := json.Unmarshal(body, &single); err2 != nil {
			return nil, fmt.Errorf("failed to parse appointments (array: %v, single: %v)", err, err2)
		}
		amdAppts = []AMDAppointmentResponse{single}
	}

	var appointments []domain.Appointment
	for _, a := range amdAppts {
		startTime, err := ParseDateTime(a.StartDateTime)
		if err != nil {
			return nil, fmt.Errorf("invalid response: appointment start time")
		}

		if a.Duration <= 0 {
			return nil, fmt.Errorf("invalid response: appointment duration")
		}

		appointments = append(appointments, domain.Appointment{
			ID:            a.ID,
			StartDateTime: startTime,
			Duration:      a.Duration,
			ColumnID:      a.ColumnID,
			ProfileID:     a.ProfileID,
			PatientID:     a.PatientID,
		})
	}

	return appointments, nil
}

// GetAppointmentsForColumns preserves partial data and returns the first failure.
func (c *AdvancedMDRestClient) GetAppointmentsForColumns(ctx context.Context, tokenData *domain.TokenData, columnIDs []string, startDate string) (map[string][]domain.Appointment, error) {
	return readColumns(ctx, columnIDs, func(ctx context.Context, id string) ([]domain.Appointment, error) {
		return c.GetAppointments(ctx, tokenData, id, startDate)
	})
}

// AMDBlockHoldResponse represents a block hold from the REST API.
type AMDBlockHoldResponse struct {
	ID            int    `json:"id"`
	StartDateTime string `json:"startdatetime"`
	EndDateTime   string `json:"enddatetime"`
	Duration      int    `json:"duration"`
	ColumnID      int    `json:"columnid"`
	Note          string `json:"note"`
	Recurrence    struct {
		RecurrenceType int `json:"recurrencetype"`
	} `json:"recurrence"`
}

// GetBlockHolds fetches block holds for a column within a date range.
// startDate should be in YYYY-MM-DD format.
func (c *AdvancedMDRestClient) GetBlockHolds(ctx context.Context, tokenData *domain.TokenData, columnID string, startDate string) ([]domain.BlockHold, error) {
	url := fmt.Sprintf("https://%s/scheduler/blockholds?columnId=%s&forView=day&startDate=%s",
		tokenData.RestApiBase, columnID, startDate)

	body, err := c.getResponseBody(ctx, tokenData, url)
	if err != nil {
		return nil, err
	}

	// Handle AMD single-vs-array response quirk
	var amdHolds []AMDBlockHoldResponse
	if err := json.Unmarshal(body, &amdHolds); err != nil {
		var single AMDBlockHoldResponse
		if err2 := json.Unmarshal(body, &single); err2 != nil {
			return nil, fmt.Errorf("failed to parse block holds (array: %v, single: %v)", err, err2)
		}
		amdHolds = []AMDBlockHoldResponse{single}
	}

	var holds []domain.BlockHold
	for _, h := range amdHolds {
		startTime, err := ParseDateTime(h.StartDateTime)
		if err != nil {
			return nil, fmt.Errorf("invalid response: block hold start time")
		}

		endTime, err := ParseDateTime(h.EndDateTime)
		if err != nil {
			endTime = startTime.Add(time.Duration(h.Duration) * time.Minute)
		}
		// For recurring holds, AMD's enddatetime is the recurrence series end,
		// not the end of this day's occurrence.
		if h.Recurrence.RecurrenceType > 0 && h.Duration > 0 {
			endTime = startTime.Add(time.Duration(h.Duration) * time.Minute)
		}

		if !endTime.After(startTime) {
			return nil, fmt.Errorf("invalid response: block hold end time")
		}

		holds = append(holds, domain.BlockHold{
			ID:            h.ID,
			StartDateTime: startTime,
			EndDateTime:   endTime,
			ColumnID:      h.ColumnID,
			Note:          h.Note,
		})
	}

	return holds, nil
}

// GetBlockHoldsForColumns preserves partial data and returns the first failure.
func (c *AdvancedMDRestClient) GetBlockHoldsForColumns(ctx context.Context, tokenData *domain.TokenData, columnIDs []string, startDate string) (map[string][]domain.BlockHold, error) {
	return readColumns(ctx, columnIDs, func(ctx context.Context, id string) ([]domain.BlockHold, error) {
		return c.GetBlockHolds(ctx, tokenData, id, startDate)
	})
}

// A failed column cancels sibling reads; callers must not treat partial data as
// a complete schedule. Both occupancy endpoints use the same failure policy.
func readColumns[T any](ctx context.Context, columnIDs []string, read func(context.Context, string) ([]T, error)) (map[string][]T, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	result := make(map[string][]T, len(columnIDs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error
	for _, id := range columnIDs {
		wg.Go(func() {
			rows, err := read(ctx, id)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
					cancel()
				}
				return
			}
			result[id] = rows
		})
	}
	wg.Wait()
	return result, firstErr
}

// GetAppointmentsByMonth fetches all appointments for the given columns for a full month.
// columnIDs should be dash-separated (e.g., "1513-1550-1551").
// startDate should be the first of the month in YYYY-MM-DD format.
func (c *AdvancedMDRestClient) GetAppointmentsByMonth(ctx context.Context, tokenData *domain.TokenData, columnIDs string, startDate string) ([]AMDAppointmentResponse, error) {
	url := fmt.Sprintf("https://%s/scheduler/appointments?columnId=%s&forView=month&isLegacy=true&startDate=%s",
		tokenData.RestApiBase, columnIDs, startDate)

	body, err := c.getResponseBody(ctx, tokenData, url)
	if err != nil {
		return nil, err
	}

	var appts []AMDAppointmentResponse
	if err := json.Unmarshal(body, &appts); err != nil {
		return nil, fmt.Errorf("failed to parse appointments: %w", err)
	}

	return appts, nil
}

// BookAppointmentParams holds the parameters for booking an appointment.
type BookAppointmentParams struct {
	PatientID       int    `json:"patientid"`
	ColumnID        int    `json:"columnid"`
	ProfileID       int    `json:"profileid"`
	StartDatetime   string `json:"startdatetime"`
	Duration        int    `json:"duration"`
	AppointmentType []struct {
		ID int `json:"id"`
	} `json:"type"`
	EpisodeID  int    `json:"episodeid"`
	FacilityID int    `json:"facilityid"`
	Color      string `json:"color"`
	Force      int    `json:"force,omitempty"`
	Comments   string `json:"comments,omitempty"`
}

// BookAppointmentResponse represents the AMD response after booking.
type BookAppointmentResponse struct {
	ID int `json:"id"`
}

// BookAppointment creates an appointment via AMD's REST API.
// Returns the appointment ID on success.
func (c *AdvancedMDRestClient) BookAppointment(ctx context.Context, tokenData *domain.TokenData, params BookAppointmentParams) (int, error) {
	url := fmt.Sprintf("https://%s/scheduler/Appointments", tokenData.RestApiBase)

	bodyBytes, err := json.Marshal(params)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", tokenData.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, newMutationError(
			MutationDispositionAmbiguous,
			fmt.Errorf("request failed: %w", err),
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, newMutationError(
			mutationDispositionForStatus(resp.StatusCode),
			fmt.Errorf("unexpected status %d from AMD booking API", resp.StatusCode),
		)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, newMutationError(
			MutationDispositionAmbiguous,
			fmt.Errorf("failed to read response: %w", err),
		)
	}

	var result BookAppointmentResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, newMutationError(
			MutationDispositionAmbiguous,
			fmt.Errorf("failed to parse response: %w", err),
		)
	}
	if result.ID <= 0 {
		return 0, newMutationError(
			MutationDispositionAmbiguous,
			fmt.Errorf("invalid booking response"),
		)
	}

	return result.ID, nil
}

// CancelAppointment cancels an appointment via AMD's REST API.
func (c *AdvancedMDRestClient) CancelAppointment(ctx context.Context, tokenData *domain.TokenData, appointmentID int) error {
	url := fmt.Sprintf("https://%s/scheduler/appointments/%d/cancel",
		tokenData.RestApiBase, appointmentID)

	reqBody := map[string]interface{}{
		"id": appointmentID,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", tokenData.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return newMutationError(
			MutationDispositionAmbiguous,
			fmt.Errorf("request failed: %w", err),
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newMutationError(
			mutationDispositionForStatus(resp.StatusCode),
			fmt.Errorf("unexpected status %d from AMD cancellation API", resp.StatusCode),
		)
	}

	return nil
}

func (c *AdvancedMDRestClient) getResponseBody(ctx context.Context, tokenData *domain.TokenData, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", tokenData.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPStatusError{StatusCode: resp.StatusCode}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if bytes.Equal(bytes.TrimSpace(body), []byte("null")) {
		return nil, fmt.Errorf("invalid response: null schedule")
	}
	return body, nil
}
