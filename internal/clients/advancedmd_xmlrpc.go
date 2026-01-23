package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"advancedmd-token-management/internal/domain"
)

// AMDLookupRequest is the XMLRPC request format for lookuppatient.
type AMDLookupRequest struct {
	PPMDMsg AMDLookupMsg `json:"ppmdmsg"`
}

// AMDLookupMsg contains the lookuppatient action parameters.
type AMDLookupMsg struct {
	Action string `json:"@action"`
	Class  string `json:"@class"`
	Name   string `json:"@name"`
}

// AMDLookupResponse represents the AdvancedMD lookuppatient response (array format).
type AMDLookupResponse struct {
	PPMDResults struct {
		Results struct {
			PatientList struct {
				ItemCount string       `json:"@itemcount"`
				Patients  []AMDPatient `json:"patient"`
			} `json:"patientlist"`
		} `json:"Results"`
		Error interface{} `json:"Error"`
	} `json:"PPMDResults"`
}

// AMDLookupResponseSingle handles single patient response.
type AMDLookupResponseSingle struct {
	PPMDResults struct {
		Results struct {
			PatientList struct {
				ItemCount string     `json:"@itemcount"`
				Patient   AMDPatient `json:"patient"`
			} `json:"patientlist"`
		} `json:"Results"`
		Error interface{} `json:"Error"`
	} `json:"PPMDResults"`
}

// AMDPatient represents a patient record from AdvancedMD.
type AMDPatient struct {
	ID          string `json:"@id"`
	Name        string `json:"@name"`
	DOB         string `json:"@dob"`
	Gender      string `json:"@gender"`
	Chart       string `json:"@chart"`
	ContactInfo struct {
		HomePhone string `json:"@homephone"`
	} `json:"contactinfo"`
}

// AdvancedMDClient handles XMLRPC calls to AdvancedMD.
type AdvancedMDClient struct {
	httpClient *http.Client
}

// NewAdvancedMDClient creates a new AdvancedMD XMLRPC client.
func NewAdvancedMDClient(httpClient *http.Client) *AdvancedMDClient {
	return &AdvancedMDClient{httpClient: httpClient}
}

// LookupPatient searches for patients by last name.
func (c *AdvancedMDClient) LookupPatient(ctx context.Context, tokenData *domain.TokenData, lastName string) ([]domain.Patient, error) {
	reqBody := AMDLookupRequest{
		PPMDMsg: AMDLookupMsg{
			Action: "lookuppatient",
			Class:  "api",
			Name:   lastName,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := "https://" + tokenData.XmlrpcURL

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Cookie", tokenData.CookieToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Try array response first
	var arrayResp AMDLookupResponse
	if err := json.Unmarshal(body, &arrayResp); err == nil {
		if arrayResp.PPMDResults.Results.PatientList.Patients != nil {
			return convertPatients(arrayResp.PPMDResults.Results.PatientList.Patients), nil
		}
	}

	// Try single patient response
	var singleResp AMDLookupResponseSingle
	if err := json.Unmarshal(body, &singleResp); err == nil {
		if singleResp.PPMDResults.Results.PatientList.Patient.ID != "" {
			return convertPatients([]AMDPatient{singleResp.PPMDResults.Results.PatientList.Patient}), nil
		}
	}

	return []domain.Patient{}, nil
}

// convertPatients converts AMD patient records to domain patients.
func convertPatients(amdPatients []AMDPatient) []domain.Patient {
	patients := make([]domain.Patient, len(amdPatients))
	for i, p := range amdPatients {
		patients[i] = domain.Patient{
			ID:        domain.StripPatientPrefix(p.ID),
			FullName:  p.Name,
			FirstName: domain.ParseFirstName(p.Name),
			DOB:       p.DOB,
			Phone:     p.ContactInfo.HomePhone,
		}
	}
	return patients
}
