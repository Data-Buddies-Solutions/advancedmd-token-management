package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"log"

	"advancedmd-token-management/internal/auth"
	"advancedmd-token-management/internal/clients"
	"advancedmd-token-management/internal/domain"
)

// ErrorResponse is the JSON response structure for error conditions.
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// VerifyPatientRequest is the expected JSON body for patient verification.
type VerifyPatientRequest struct {
	LastName  string `json:"lastName"`
	DOB       string `json:"dob"`
	FirstName string `json:"firstName,omitempty"`
}

// VerifyPatientResponse is returned on successful patient verification.
type VerifyPatientResponse struct {
	Status           string         `json:"status"`
	PatientID        string         `json:"patientId,omitempty"`
	Name             string         `json:"name,omitempty"`
	DOB              string         `json:"dob,omitempty"`
	Phone            string         `json:"phone,omitempty"`
	InsuranceCarrier string         `json:"insuranceCarrier,omitempty"`
	Message          string         `json:"message,omitempty"`
	Matches          []PatientMatch `json:"matches,omitempty"`
}

// PatientMatch provides minimal info for disambiguation.
type PatientMatch struct {
	FirstName string `json:"firstName"`
}

// Handlers holds the dependencies for HTTP handlers.
type Handlers struct {
	tokenManager *auth.TokenManager
	amdClient    *clients.AdvancedMDClient
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(tm *auth.TokenManager, amdClient *clients.AdvancedMDClient) *Handlers {
	return &Handlers{
		tokenManager: tm,
		amdClient:    amdClient,
	}
}

// HandleHealth returns a simple health check response.
func (h *Handlers) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// HandleGetToken returns the cached AdvancedMD token.
func (h *Handlers) HandleGetToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Method not allowed"})
		return
	}

	tokenData, err := h.tokenManager.GetToken(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "Failed to get token",
			Details: err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(tokenData.ToResponse())
}

// AddPatientRequest is the expected JSON body for patient creation.
type AddPatientRequest struct {
	FirstName         string `json:"firstName"`
	LastName          string `json:"lastName"`
	DOB               string `json:"dob"`
	Email             string `json:"email"`
	Phone             string `json:"phone"`
	InsuranceProvider string `json:"insuranceProvider"`
	SubscriberNum     string `json:"subscriberNum"`
}

// AddPatientResponse is returned after creating a patient.
type AddPatientResponse struct {
	Status    string `json:"status"`
	PatientID string `json:"patientId,omitempty"`
	Name      string `json:"name,omitempty"`
	DOB       string `json:"dob,omitempty"`
	Message   string `json:"message,omitempty"`
}

// HandleAddPatient creates a new patient in AdvancedMD and attaches insurance.
func (h *Handlers) HandleAddPatient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req AddPatientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:  "error",
			Message: "Invalid JSON body",
		})
		return
	}

	// Validate required fields
	missing := []string{}
	if req.FirstName == "" {
		missing = append(missing, "firstName")
	}
	if req.LastName == "" {
		missing = append(missing, "lastName")
	}
	if req.DOB == "" {
		missing = append(missing, "dob")
	}
	if req.Email == "" {
		missing = append(missing, "email")
	}
	if req.Phone == "" {
		missing = append(missing, "phone")
	}
	if req.InsuranceProvider == "" {
		missing = append(missing, "insuranceProvider")
	}
	if req.SubscriberNum == "" {
		missing = append(missing, "subscriberNum")
	}
	if len(missing) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:  "error",
			Message: fmt.Sprintf("Missing required fields: %s", strings.Join(missing, ", ")),
		})
		return
	}

	// Normalize inputs
	normalizedDOB := domain.NormalizeDOB(req.DOB)
	formattedPhone := domain.FormatPhone(req.Phone)

	// Look up carrier ID
	carrierID, ok := domain.LookupCarrierID(req.InsuranceProvider)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:  "error",
			Message: fmt.Sprintf("Unknown insurance provider: %s", req.InsuranceProvider),
		})
		return
	}

	// Get auth token
	tokenData, err := h.tokenManager.GetToken(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:  "error",
			Message: "Failed to get authentication token: " + err.Error(),
		})
		return
	}

	// Create patient in AMD
	rawPatientID, respPartyID, patientName, err := h.amdClient.AddPatient(r.Context(), tokenData, req.FirstName, req.LastName, normalizedDOB, formattedPhone, req.Email)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:  "error",
			Message: "Failed to create patient: " + err.Error(),
		})
		return
	}

	strippedID := domain.StripPatientPrefix(rawPatientID)

	// Attach insurance
	if err := h.amdClient.AddInsurance(r.Context(), tokenData, rawPatientID, respPartyID, carrierID, req.SubscriberNum); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:    "partial",
			PatientID: strippedID,
			Name:      patientName,
			DOB:       normalizedDOB,
			Message:   "Patient created but insurance failed: " + err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(AddPatientResponse{
		Status:    "created",
		PatientID: strippedID,
		Name:      patientName,
		DOB:       normalizedDOB,
		Message:   "Patient created and insurance attached successfully",
	})
}

// HandleVerifyPatient looks up a patient by name and DOB.
func (h *Handlers) HandleVerifyPatient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(VerifyPatientResponse{
			Status:  "error",
			Message: "Method not allowed. Use POST.",
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

	// Normalize DOB
	normalizedDOB := domain.NormalizeDOB(req.DOB)

	// Get token
	tokenData, err := h.tokenManager.GetToken(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(VerifyPatientResponse{
			Status:  "error",
			Message: "Failed to get authentication token: " + err.Error(),
		})
		return
	}

	// Call AdvancedMD lookuppatient API
	patients, err := h.amdClient.LookupPatient(r.Context(), tokenData, req.LastName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(VerifyPatientResponse{
			Status:  "error",
			Message: "Failed to lookup patient: " + err.Error(),
		})
		return
	}

	// Filter patients by DOB
	var matchingPatients []domain.Patient
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
		insuranceCarrier, err := h.amdClient.GetDemographic(r.Context(), tokenData, p.ID)
		if err != nil {
			log.Printf("WARNING: failed to get demographics for patient %s: %v", p.ID, err)
		}
		json.NewEncoder(w).Encode(VerifyPatientResponse{
			Status:           "verified",
			PatientID:        p.ID,
			Name:             p.FullName,
			DOB:              p.DOB,
			Phone:            p.Phone,
			InsuranceCarrier: insuranceCarrier,
		})
		return

	default:
		// Multiple matches - try to disambiguate by first name
		if req.FirstName != "" {
			upperFirstName := strings.ToUpper(req.FirstName)
			for _, p := range matchingPatients {
				if strings.HasPrefix(p.FirstName, upperFirstName) {
					insuranceCarrier, err := h.amdClient.GetDemographic(r.Context(), tokenData, p.ID)
					if err != nil {
						log.Printf("WARNING: failed to get demographics for patient %s: %v", p.ID, err)
					}
					json.NewEncoder(w).Encode(VerifyPatientResponse{
						Status:           "verified",
						PatientID:        p.ID,
						Name:             p.FullName,
						DOB:              p.DOB,
						Phone:            p.Phone,
						InsuranceCarrier: insuranceCarrier,
					})
					return
				}
			}
			json.NewEncoder(w).Encode(VerifyPatientResponse{
				Status:  "not_found",
				Message: "No patient found matching that first name",
			})
			return
		}

		// Return list of first names for disambiguation
		var matches []PatientMatch
		for _, p := range matchingPatients {
			matches = append(matches, PatientMatch{FirstName: p.FirstName})
		}
		json.NewEncoder(w).Encode(VerifyPatientResponse{
			Status:  "multiple_matches",
			Message: fmt.Sprintf("Found %d patients with that last name and DOB. Please provide first name.", len(matchingPatients)),
			Matches: matches,
		})
	}
}
