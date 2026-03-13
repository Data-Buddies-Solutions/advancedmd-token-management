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
	ID               int    `json:"id"`
	StartDateTime    string `json:"startdatetime"`
	Duration         int    `json:"duration"`
	ColumnID         int    `json:"columnid"`
	ProfileID        int    `json:"profileid"`
	Provider         string `json:"provider"`
	Heading          string `json:"heading"`
	Facility         string `json:"facility"`
	FacilityID       int    `json:"facilityid"`
	AppointmentTypes []int  `json:"appointmenttypeids"`
	PatientID        int    `json:"patientid"`
	FirstName        string `json:"firstname"`
	LastName         string `json:"lastname"`
	ConfirmDate      *string `json:"confirmdate"`
	ConfirmMethod    *string `json:"confirmmethod"`
}

// GetAppointments fetches appointments for a column within a date range.
// startDate should be in YYYY-MM-DD format.
func (c *AdvancedMDRestClient) GetAppointments(ctx context.Context, tokenData *domain.TokenData, columnID string, startDate string) ([]domain.Appointment, error) {
	url := fmt.Sprintf("https://%s/scheduler/appointments?columnId=%s&forView=day&isLegacy=true&startDate=%s",
		tokenData.RestApiBase, columnID, startDate)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var amdAppts []AMDAppointmentResponse
	if err := json.Unmarshal(body, &amdAppts); err != nil {
		return nil, fmt.Errorf("failed to parse appointments: %w", err)
	}

	appointments := make([]domain.Appointment, len(amdAppts))
	for i, a := range amdAppts {
		startTime, err := time.Parse("2006-01-02T15:04:05", a.StartDateTime)
		if err != nil {
			// Try alternate format
			startTime, err = time.Parse("2006-01-02T15:04", a.StartDateTime)
			if err != nil {
				continue
			}
		}

		appointments[i] = domain.Appointment{
			ID:            a.ID,
			StartDateTime: startTime,
			Duration:      a.Duration,
			ColumnID:      a.ColumnID,
			ProfileID:     a.ProfileID,
			PatientID:     a.PatientID,
		}
	}

	return appointments, nil
}

// GetAppointmentsForColumns fetches appointments for multiple columns concurrently.
func (c *AdvancedMDRestClient) GetAppointmentsForColumns(ctx context.Context, tokenData *domain.TokenData, columnIDs []string, startDate string) (map[string][]domain.Appointment, error) {
	result := make(map[string][]domain.Appointment)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error

	for _, colID := range columnIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			appts, err := c.GetAppointments(ctx, tokenData, id, startDate)
			mu.Lock()
			defer mu.Unlock()
			if err != nil && firstErr == nil {
				firstErr = fmt.Errorf("failed to get appointments for column %s: %w", id, err)
				return
			}
			result[id] = appts
		}(colID)
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return result, nil
}

// AMDBlockHoldResponse represents a block hold from the REST API.
type AMDBlockHoldResponse struct {
	ID            int    `json:"id"`
	StartDateTime string `json:"startdatetime"`
	EndDateTime   string `json:"enddatetime"`
	Duration      int    `json:"duration"`
	ColumnID      int    `json:"columnid"`
	Note          string `json:"note"`
}

// GetBlockHolds fetches block holds for a column within a date range.
// startDate should be in YYYY-MM-DD format.
func (c *AdvancedMDRestClient) GetBlockHolds(ctx context.Context, tokenData *domain.TokenData, columnID string, startDate string) ([]domain.BlockHold, error) {
	url := fmt.Sprintf("https://%s/scheduler/blockholds?columnId=%s&forView=day&startDate=%s",
		tokenData.RestApiBase, columnID, startDate)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var amdHolds []AMDBlockHoldResponse
	if err := json.Unmarshal(body, &amdHolds); err != nil {
		return nil, fmt.Errorf("failed to parse block holds: %w", err)
	}

	holds := make([]domain.BlockHold, len(amdHolds))
	for i, h := range amdHolds {
		startTime, err := time.Parse("2006-01-02T15:04:05", h.StartDateTime)
		if err != nil {
			startTime, err = time.Parse("2006-01-02T15:04", h.StartDateTime)
			if err != nil {
				continue
			}
		}

		endTime, err := time.Parse("2006-01-02T15:04:05", h.EndDateTime)
		if err != nil {
			endTime, err = time.Parse("2006-01-02T15:04", h.EndDateTime)
			if err != nil {
				// Fall back to computing from duration
				endTime = startTime.Add(time.Duration(h.Duration) * time.Minute)
			}
		}

		holds[i] = domain.BlockHold{
			ID:            h.ID,
			StartDateTime: startTime,
			EndDateTime:   endTime,
			ColumnID:      h.ColumnID,
			Note:          h.Note,
		}
	}

	return holds, nil
}

// GetBlockHoldsForColumns fetches block holds for multiple columns concurrently.
func (c *AdvancedMDRestClient) GetBlockHoldsForColumns(ctx context.Context, tokenData *domain.TokenData, columnIDs []string, startDate string) (map[string][]domain.BlockHold, error) {
	result := make(map[string][]domain.BlockHold)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error

	for _, colID := range columnIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			holds, err := c.GetBlockHolds(ctx, tokenData, id, startDate)
			mu.Lock()
			defer mu.Unlock()
			if err != nil && firstErr == nil {
				firstErr = fmt.Errorf("failed to get block holds for column %s: %w", id, err)
				return
			}
			result[id] = holds
		}(colID)
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return result, nil
}

// GetAppointmentsByMonth fetches all appointments for the given columns for a full month.
// columnIDs should be dash-separated (e.g., "1513-1550-1551").
// startDate should be the first of the month in YYYY-MM-DD format.
func (c *AdvancedMDRestClient) GetAppointmentsByMonth(ctx context.Context, tokenData *domain.TokenData, columnIDs string, startDate string) ([]AMDAppointmentResponse, error) {
	url := fmt.Sprintf("https://%s/scheduler/appointments?columnId=%s&forView=month&isLegacy=true&startDate=%s",
		tokenData.RestApiBase, columnIDs, startDate)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var appts []AMDAppointmentResponse
	if err := json.Unmarshal(body, &appts); err != nil {
		return nil, fmt.Errorf("failed to parse appointments: %w", err)
	}

	return appts, nil
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
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

