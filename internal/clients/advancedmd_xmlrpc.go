package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

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
	RespParty   string `json:"@respparty"`
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

// doXMLRPCRequest marshals payload to JSON, POSTs to the XMLRPC endpoint, and returns the raw response body.
func (c *AdvancedMDClient) doXMLRPCRequest(ctx context.Context, tokenData *domain.TokenData, payload interface{}) ([]byte, error) {
	jsonBody, err := json.Marshal(payload)
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

	return body, nil
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

	body, err := c.doXMLRPCRequest(ctx, tokenData, reqBody)
	if err != nil {
		return nil, err
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

// AddPatient creates a new patient in AdvancedMD.
// Returns the raw patient ID (with "pat" prefix), responsible party ID, and patient name.
func (c *AdvancedMDClient) AddPatient(ctx context.Context, tokenData *domain.TokenData, firstName, lastName, dob, phone, email string) (string, string, string, error) {
	name := lastName + "," + firstName
	msgTime := time.Now().Format("01/02/2006 03:04:05 PM")

	payload := map[string]interface{}{
		"ppmdmsg": map[string]interface{}{
			"@action":   "addpatient",
			"@class":    "api",
			"@msgtime":  msgTime,
			"@nocookie": "0",
			"patientlist": map[string]interface{}{
				"patient": map[string]interface{}{
					"@respparty":         "SELF",
					"@name":              name,
					"@sex":               "U",
					"@relationship":      "1",
					"@hipaarelationship": "18",
					"@dob":               dob,
					"@ssn":               "",
					"@chart":             "AUTO",
					"@profile":           "3",
					"contactinfo": map[string]interface{}{
						"@homephone": phone,
						"@email":     email,
					},
				},
			},
		},
	}

	body, err := c.doXMLRPCRequest(ctx, tokenData, payload)
	if err != nil {
		return "", "", "", fmt.Errorf("addpatient request failed: %w", err)
	}

	// Try single patient response first (most likely for addpatient)
	var singleResp AMDLookupResponseSingle
	if err := json.Unmarshal(body, &singleResp); err == nil {
		if singleResp.PPMDResults.Results.PatientList.Patient.ID != "" {
			p := singleResp.PPMDResults.Results.PatientList.Patient
			return p.ID, p.RespParty, p.Name, nil
		}
	}

	// Try array response
	var arrayResp AMDLookupResponse
	if err := json.Unmarshal(body, &arrayResp); err == nil {
		if len(arrayResp.PPMDResults.Results.PatientList.Patients) > 0 {
			p := arrayResp.PPMDResults.Results.PatientList.Patients[0]
			return p.ID, p.RespParty, p.Name, nil
		}
	}

	return "", "", "", fmt.Errorf("addpatient returned unexpected response: %s", string(body))
}

// AddInsurance attaches an insurance record to an existing patient in AdvancedMD.
func (c *AdvancedMDClient) AddInsurance(ctx context.Context, tokenData *domain.TokenData, patientID, respPartyID, carrierID, subscriberNum string) error {
	msgTime := time.Now().Format("01/02/2006 03:04:05 PM")

	payload := map[string]interface{}{
		"ppmdmsg": map[string]interface{}{
			"@action":  "addinsurance",
			"@class":   "api",
			"@msgtime": msgTime,
			"patient": map[string]interface{}{
				"@id":      patientID,
				"@changed": "1",
				"insplanlist": map[string]interface{}{
					"insplan": map[string]interface{}{
						"@id":                 "",
						"@carrier":            carrierID,
						"@subscriber":         respPartyID,
						"@subscribernum":      subscriberNum,
						"@hipaarelationship":  "18",
						"@relationship":       "1",
						"@copay":              "0.00",
						"@coverage":           "3",
					},
				},
			},
		},
	}

	body, err := c.doXMLRPCRequest(ctx, tokenData, payload)
	if err != nil {
		return fmt.Errorf("addinsurance request failed: %w", err)
	}

	// Check for error in response
	var errResp struct {
		PPMDResults struct {
			Error interface{} `json:"Error"`
		} `json:"PPMDResults"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil {
		if errResp.PPMDResults.Error != nil {
			if errStr, ok := errResp.PPMDResults.Error.(string); ok && errStr != "" {
				return fmt.Errorf("addinsurance error: %s", errStr)
			}
		}
	}

	return nil
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
