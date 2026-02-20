package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	AppointmentTypes []int  `json:"appointmenttypeids"`
	PatientID        int    `json:"patientid"`
	FirstName        string `json:"firstname"`
	LastName         string `json:"lastname"`
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

// GetAppointmentsForColumns fetches appointments for multiple columns.
func (c *AdvancedMDRestClient) GetAppointmentsForColumns(ctx context.Context, tokenData *domain.TokenData, columnIDs []string, startDate string) (map[string][]domain.Appointment, error) {
	result := make(map[string][]domain.Appointment)

	for _, colID := range columnIDs {
		appts, err := c.GetAppointments(ctx, tokenData, colID, startDate)
		if err != nil {
			return nil, fmt.Errorf("failed to get appointments for column %s: %w", colID, err)
		}
		result[colID] = appts
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

// GetBlockHoldsForColumns fetches block holds for multiple columns.
func (c *AdvancedMDRestClient) GetBlockHoldsForColumns(ctx context.Context, tokenData *domain.TokenData, columnIDs []string, startDate string) (map[string][]domain.BlockHold, error) {
	result := make(map[string][]domain.BlockHold)

	for _, colID := range columnIDs {
		holds, err := c.GetBlockHolds(ctx, tokenData, colID, startDate)
		if err != nil {
			return nil, fmt.Errorf("failed to get block holds for column %s: %w", colID, err)
		}
		result[colID] = holds
	}

	return result, nil
}

