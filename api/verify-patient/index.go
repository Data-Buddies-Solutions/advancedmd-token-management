package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"advancedmd-token-management/pkg/advancedmd"
	"advancedmd-token-management/pkg/redis"
)

// VerifyPatientRequest is the expected JSON body for patient verification.
type VerifyPatientRequest struct {
	LastName  string `json:"lastName"`
	DOB       string `json:"dob"`
	FirstName string `json:"firstName,omitempty"`
}

// VerifyPatientResponse is returned on successful patient verification.
type VerifyPatientResponse struct {
	Status    string         `json:"status"`
	PatientID string         `json:"patientId,omitempty"`
	Name      string         `json:"name,omitempty"`
	DOB       string         `json:"dob,omitempty"`
	Phone     string         `json:"phone,omitempty"`
	Message   string         `json:"message,omitempty"`
	Matches   []PatientMatch `json:"matches,omitempty"`
}

// PatientMatch provides minimal info for disambiguation.
type PatientMatch struct {
	FirstName string `json:"firstName"`
}

// AMDLookupRequest is the XMLRPC request format for lookuppatient
type AMDLookupRequest struct {
	PPMDMsg AMDLookupMsg `json:"ppmdmsg"`
}

// AMDLookupMsg contains the lookuppatient action parameters
type AMDLookupMsg struct {
	Action string `json:"@action"`
	Class  string `json:"@class"`
	Name   string `json:"@name"`
}

// AMDLookupResponse represents the AdvancedMD lookuppatient response
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

// AMDLookupResponseSingle handles single patient response
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

// AMDPatient represents a patient record from AdvancedMD
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

// Handler is the Vercel serverless function handler for /api/verify-patient
func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(VerifyPatientResponse{
			Status:  "error",
			Message: "Method not allowed. Use POST.",
		})
		return
	}

	// Verify API secret
	auth := r.Header.Get("Authorization")
	apiSecret := os.Getenv("API_SECRET")
	expectedBearer := "Bearer " + apiSecret
	if auth != expectedBearer && auth != apiSecret {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(VerifyPatientResponse{
			Status:  "error",
			Message: "Unauthorized",
		})
		return
	}

	// Parse request body
	var req VerifyPatientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(VerifyPatientResponse{
			Status:  "error",
			Message: "Invalid JSON body",
		})
		return
	}

	// Validate required fields
	if req.LastName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(VerifyPatientResponse{
			Status:  "error",
			Message: "lastName is required",
		})
		return
	}
	if req.DOB == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(VerifyPatientResponse{
			Status:  "error",
			Message: "dob is required",
		})
		return
	}

	// Normalize DOB to MM/DD/YYYY format
	normalizedDOB := normalizeDOB(req.DOB)

	// Get token from cache or authenticate
	tokenData, err := getOrRefreshToken()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(VerifyPatientResponse{
			Status:  "error",
			Message: "Failed to get authentication token: " + err.Error(),
		})
		return
	}

	// Call AdvancedMD lookuppatient API
	patients, err := lookupPatient(tokenData, req.LastName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(VerifyPatientResponse{
			Status:  "error",
			Message: "Failed to lookup patient: " + err.Error(),
		})
		return
	}

	// Filter patients by DOB
	var matchingPatients []AMDPatient
	for _, p := range patients {
		if p.DOB == normalizedDOB {
			matchingPatients = append(matchingPatients, p)
		}
	}

	// Handle results
	switch len(matchingPatients) {
	case 0:
		json.NewEncoder(w).Encode(VerifyPatientResponse{
			Status:  "not_found",
			Message: "No patient found with that last name and date of birth",
		})
		return

	case 1:
		p := matchingPatients[0]
		json.NewEncoder(w).Encode(VerifyPatientResponse{
			Status:    "verified",
			PatientID: stripPatientPrefix(p.ID),
			Name:      p.Name,
			DOB:       p.DOB,
			Phone:     p.ContactInfo.HomePhone,
		})
		return

	default:
		if req.FirstName != "" {
			upperFirstName := strings.ToUpper(req.FirstName)
			for _, p := range matchingPatients {
				parts := strings.SplitN(p.Name, ",", 2)
				if len(parts) == 2 {
					patientFirstName := strings.TrimSpace(parts[1])
					if strings.HasPrefix(patientFirstName, upperFirstName) {
						json.NewEncoder(w).Encode(VerifyPatientResponse{
							Status:    "verified",
							PatientID: stripPatientPrefix(p.ID),
							Name:      p.Name,
							DOB:       p.DOB,
							Phone:     p.ContactInfo.HomePhone,
						})
						return
					}
				}
			}
			json.NewEncoder(w).Encode(VerifyPatientResponse{
				Status:  "not_found",
				Message: "No patient found matching that first name",
			})
			return
		}

		var matches []PatientMatch
		for _, p := range matchingPatients {
			parts := strings.SplitN(p.Name, ",", 2)
			firstName := ""
			if len(parts) == 2 {
				firstName = strings.TrimSpace(parts[1])
			}
			matches = append(matches, PatientMatch{FirstName: firstName})
		}
		json.NewEncoder(w).Encode(VerifyPatientResponse{
			Status:  "multiple_matches",
			Message: fmt.Sprintf("Found %d patients with that last name and DOB. Please provide first name.", len(matchingPatients)),
			Matches: matches,
		})
		return
	}
}

func getOrRefreshToken() (*redis.TokenData, error) {
	tokenData, err := redis.GetToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token from cache: %w", err)
	}

	if tokenData == nil {
		token, webserverURL, err := advancedmd.Authenticate()
		if err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}

		tokenData = &redis.TokenData{
			Token:        "Bearer " + token,
			CookieToken:  "token=" + token,
			WebserverURL: strings.TrimPrefix(webserverURL, "https://"),
			XmlrpcURL:    strings.TrimPrefix(webserverURL+"/xmlrpc/processrequest.aspx", "https://"),
			RestApiBase:  strings.TrimPrefix(strings.Replace(webserverURL, "/processrequest/", "/api/", 1), "https://"),
			EhrApiBase:   strings.TrimPrefix(strings.Replace(webserverURL, "/processrequest/", "/ehr-api/", 1), "https://"),
			CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		}

		if err := redis.SaveToken(tokenData); err != nil {
			fmt.Printf("Warning: failed to cache token: %v\n", err)
		}
	}

	return tokenData, nil
}

func lookupPatient(tokenData *redis.TokenData, lastName string) ([]AMDPatient, error) {
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

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	rawToken := strings.TrimPrefix(tokenData.Token, "Bearer ")
	req.Header.Set("Cookie", "token="+rawToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var arrayResp AMDLookupResponse
	if err := json.Unmarshal(body, &arrayResp); err == nil {
		if arrayResp.PPMDResults.Results.PatientList.Patients != nil {
			return arrayResp.PPMDResults.Results.PatientList.Patients, nil
		}
	}

	var singleResp AMDLookupResponseSingle
	if err := json.Unmarshal(body, &singleResp); err == nil {
		if singleResp.PPMDResults.Results.PatientList.Patient.ID != "" {
			return []AMDPatient{singleResp.PPMDResults.Results.PatientList.Patient}, nil
		}
	}

	return []AMDPatient{}, nil
}

func normalizeDOB(dob string) string {
	if len(dob) == 10 && dob[2] == '/' && dob[5] == '/' {
		return dob
	}

	formats := []string{
		"2006-01-02",
		"01-02-2006",
		"1/2/2006",
		"01/02/2006",
		"January 2 2006",
		"January 2, 2006",
		"Jan 2 2006",
		"Jan 2, 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dob); err == nil {
			return t.Format("01/02/2006")
		}
	}

	return dob
}

// stripPatientPrefix removes the "pat" prefix from patient IDs
// AMD returns IDs like "pat45" but the booking API expects just "45"
func stripPatientPrefix(id string) string {
	return strings.TrimPrefix(id, "pat")
}
