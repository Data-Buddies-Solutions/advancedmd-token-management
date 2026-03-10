package http

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"advancedmd-token-management/internal/auth"
	"advancedmd-token-management/internal/clients"
	"advancedmd-token-management/internal/domain"
)

// eastern is the America/New_York timezone, loaded once at startup.
var eastern *time.Location

func init() {
	var err error
	eastern, err = time.LoadLocation("America/New_York")
	if err != nil {
		eastern = time.FixedZone("EST", -5*3600)
	}
}

// ErrorResponse is the JSON response structure for error conditions.
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// ElevenLabsWebhookResponse is the response format for ElevenLabs conversation initiation webhook.
type ElevenLabsWebhookResponse struct {
	Type             string            `json:"type"`
	DynamicVariables map[string]interface{} `json:"dynamic_variables"`
}

// VerifyPatientRequest is the expected JSON body for patient verification.
type VerifyPatientRequest struct {
	LastName  string `json:"lastName"`
	DOB       string `json:"dob"`
	FirstName string `json:"firstName,omitempty"`
}

// VerifyPatientResponse is returned on successful patient verification.
type VerifyPatientResponse struct {
	Status             string         `json:"status"`
	PatientID          string         `json:"patientId,omitempty"`
	Name               string         `json:"name,omitempty"`
	DOB                string         `json:"dob,omitempty"`
	Phone              string         `json:"phone,omitempty"`
	InsuranceCarrier   string         `json:"insuranceCarrier,omitempty"`
	InsuranceCarrierID string         `json:"insuranceCarrierId,omitempty"`
	Routing            string         `json:"routing,omitempty"`
	AllowedProviders   []string       `json:"allowedProviders,omitempty"`
	RoutingAmbiguous   bool           `json:"routingAmbiguous,omitempty"`
	Message            string         `json:"message,omitempty"`
	Matches            []PatientMatch `json:"matches,omitempty"`
}

// PatientMatch provides minimal info for disambiguation.
type PatientMatch struct {
	FirstName string `json:"firstName"`
}

// Handlers holds the dependencies for HTTP handlers.
type Handlers struct {
	tokenManager  *auth.TokenManager
	amdClient     *clients.AdvancedMDClient
	amdRestClient *clients.AdvancedMDRestClient
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(tm *auth.TokenManager, amdClient *clients.AdvancedMDClient, amdRestClient *clients.AdvancedMDRestClient) *Handlers {
	return &Handlers{
		tokenManager:  tm,
		amdClient:     amdClient,
		amdRestClient: amdRestClient,
	}
}

// HandleHealth returns a simple health check response.
func (h *Handlers) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// HandleGetToken returns the cached AdvancedMD token for ElevenLabs agents.
// Accepts POST only (for ElevenLabs conversation initiation webhook).
func (h *Handlers) HandleGetToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	log.Printf("token: received webhook request")

	tokenData, err := h.tokenManager.GetToken(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "Failed to get token",
			Details: err.Error(),
		})
		return
	}

	resp := tokenData.ToResponse()

	nowEST := time.Now().In(eastern)

	dynamicVars := map[string]interface{}{
		"amd_token":         resp.Token,
		"amd_rest_api_base": resp.RestApiBase,
		"patient_id":        "1",
		"current_date":      nowEST.Format("Monday, January 2, 2006"),
		"current_time":      nowEST.Format("3:04 PM"),
	}

	json.NewEncoder(w).Encode(ElevenLabsWebhookResponse{
		Type:             "conversation_initiation_client_data",
		DynamicVariables: dynamicVars,
	})
}

// AddPatientRequest is the expected JSON body for patient creation.
type AddPatientRequest struct {
	FirstName      string `json:"firstName"`
	LastName       string `json:"lastName"`
	DOB            string `json:"dob"`
	Phone          string `json:"phone"`
	Email          string `json:"email"`
	Street         string `json:"street"`
	AptSuite       string `json:"aptSuite"`
	City           string `json:"city"`
	State          string `json:"state"`
	Zip            string `json:"zip"`
	Sex            string `json:"sex"`
	Insurance      string `json:"insurance"`
	SubscriberName string `json:"subscriberName"`
	SubscriberNum  string `json:"subscriberNum"`
}

// AddPatientResponse is returned after creating a patient.
type AddPatientResponse struct {
	Status           string   `json:"status"`
	PatientID        string   `json:"patientId,omitempty"`
	Name             string   `json:"name,omitempty"`
	DOB              string   `json:"dob,omitempty"`
	Routing          string   `json:"routing,omitempty"`
	AllowedProviders []string `json:"allowedProviders,omitempty"`
	Message          string   `json:"message,omitempty"`
}

// HandleAddPatient creates a new patient in AdvancedMD and attaches insurance.
func (h *Handlers) HandleAddPatient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req AddPatientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("add-patient: failed to decode JSON: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:  "error",
			Message: "Invalid JSON body",
		})
		return
	}

	log.Printf("add-patient: received request: firstName=%q lastName=%q dob=%q phone=%q email=%q street=%q aptSuite=%q city=%q state=%q zip=%q sex=%q insurance=%q subscriberName=%q subscriberNum=%q",
		req.FirstName, req.LastName, req.DOB, req.Phone, req.Email, req.Street, req.AptSuite, req.City, req.State, req.Zip, req.Sex, req.Insurance, req.SubscriberName, req.SubscriberNum)

	// Validate required fields (aptSuite is optional)
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
	if req.Phone == "" {
		missing = append(missing, "phone")
	}
	if req.Email == "" {
		missing = append(missing, "email")
	}
	if req.Street == "" {
		missing = append(missing, "street")
	}
	if req.City == "" {
		missing = append(missing, "city")
	}
	if req.State == "" {
		missing = append(missing, "state")
	}
	if req.Zip == "" {
		missing = append(missing, "zip")
	}
	if req.Sex == "" {
		missing = append(missing, "sex")
	}
	if req.Insurance == "" {
		missing = append(missing, "insurance")
	}
	if req.SubscriberName == "" {
		missing = append(missing, "subscriberName")
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
	normalizedSex := domain.NormalizeSex(req.Sex)
	normalizedFirstName := domain.StripDiacritics(req.FirstName)
	normalizedLastName := domain.StripDiacritics(req.LastName)

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
	rawPatientID, respPartyID, patientName, err := h.amdClient.AddPatient(r.Context(), tokenData, clients.AddPatientParams{
		FirstName: normalizedFirstName,
		LastName:  normalizedLastName,
		DOB:       normalizedDOB,
		Phone:     formattedPhone,
		Email:     req.Email,
		Street:    req.Street,
		AptSuite:  req.AptSuite,
		City:      req.City,
		State:     strings.ToUpper(req.State),
		Zip:       req.Zip,
		Sex:       normalizedSex,
	})
	if err != nil {
		log.Printf("add-patient: AMD error: %v", err)
		if strings.Contains(err.Error(), "Duplicate name/DOB") {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(AddPatientResponse{
				Status:  "error",
				Message: "A patient with this name and date of birth already exists in the system. Please try verifying the patient again instead of registering.",
			})
			return
		}
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "error from AMD") {
			status = http.StatusConflict
		}
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:  "error",
			Message: "Failed to create patient: " + err.Error(),
		})
		return
	}

	strippedID := domain.StripPatientPrefix(rawPatientID)

	// Look up insurance entry from name
	insEntry, ok := domain.LookupInsurance(req.Insurance)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:    "partial",
			PatientID: strippedID,
			Name:      patientName,
			DOB:       normalizedDOB,
			Message:   fmt.Sprintf("Patient created but insurance not recognized: %q. Please use an insurance name from the accepted list.", req.Insurance),
		})
		return
	}

	// Reject insurance not accepted at Spring Hill
	if insEntry.Routing == domain.RoutingNotAccepted {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:    "partial",
			PatientID: strippedID,
			Name:      patientName,
			DOB:       normalizedDOB,
			Message:   fmt.Sprintf("%s is not accepted at Spring Hill. The patient may self-pay or contact the office for options.", req.Insurance),
		})
		return
	}

	// Attach insurance
	if err := h.amdClient.AddInsurance(r.Context(), tokenData, rawPatientID, respPartyID, insEntry.CarrierID, req.SubscriberNum); err != nil {
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

	// Pediatric override: under-18 patients → Dr. Bach only
	routing := insEntry.Routing
	if domain.IsMinor(normalizedDOB) && routing != domain.RoutingNotAccepted {
		routing = domain.RoutingBachOnly
	}

	json.NewEncoder(w).Encode(AddPatientResponse{
		Status:           "created",
		PatientID:        strippedID,
		Name:             patientName,
		DOB:              normalizedDOB,
		Routing:          string(routing),
		AllowedProviders: domain.ProvidersForRouting(routing),
		Message:          "Patient created and insurance attached successfully",
	})
}

// HandleVerifyPatient looks up a patient by name and DOB.
func (h *Handlers) HandleVerifyPatient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

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

	// Normalize inputs
	normalizedDOB := domain.NormalizeDOB(req.DOB)
	normalizedLastName := domain.StripDiacritics(req.LastName)
	normalizedFirstName := domain.StripDiacritics(req.FirstName)

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
	patients, err := h.amdClient.LookupPatient(r.Context(), tokenData, normalizedLastName, normalizedFirstName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(VerifyPatientResponse{
			Status:  "error",
			Message: "Failed to lookup patient: " + err.Error(),
		})
		return
	}

	log.Printf("verify-patient: lookup lastName=%q returned %d patients", normalizedLastName, len(patients))
	for i, p := range patients {
		log.Printf("verify-patient: result[%d] id=%s name=%q dob=%q", i, p.ID, p.FullName, p.DOB)
	}

	// Filter patients by DOB (normalize both sides — AMD may return different formats)
	var matchingPatients []domain.Patient
	for _, p := range patients {
		if domain.NormalizeDOB(p.DOB) == normalizedDOB {
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
		carrierName, carrierID, err := h.amdClient.GetDemographic(r.Context(), tokenData, p.ID)
		if err != nil {
			log.Printf("WARNING: failed to get demographics for patient %s: %v", p.ID, err)
		}

		resp := VerifyPatientResponse{
			Status:           "verified",
			PatientID:        p.ID,
			Name:             p.FullName,
			DOB:              p.DOB,
			Phone:            p.Phone,
			InsuranceCarrier: carrierName,
		}

		if carrierID != "" {
			resp.InsuranceCarrierID = carrierID
			routing, ambiguous := domain.RoutingForCarrierID(carrierID)
			resp.Routing = string(routing)
			resp.AllowedProviders = domain.ProvidersForRouting(routing)
			resp.RoutingAmbiguous = ambiguous
		}

		// Pediatric override: under-18 patients → Dr. Bach only
		if domain.IsMinor(p.DOB) && resp.Routing != "" && resp.Routing != string(domain.RoutingNotAccepted) {
			resp.Routing = string(domain.RoutingBachOnly)
			resp.AllowedProviders = domain.ProvidersForRouting(domain.RoutingBachOnly)
			resp.RoutingAmbiguous = false
		}

		json.NewEncoder(w).Encode(resp)
		return

	default:
		// Multiple matches - try to disambiguate by first name
		if req.FirstName != "" {
			upperFirstName := strings.ToUpper(req.FirstName)
			for _, p := range matchingPatients {
				if strings.HasPrefix(p.FirstName, upperFirstName) {
					carrierName, carrierID, err := h.amdClient.GetDemographic(r.Context(), tokenData, p.ID)
					if err != nil {
						log.Printf("WARNING: failed to get demographics for patient %s: %v", p.ID, err)
					}

					resp := VerifyPatientResponse{
						Status:           "verified",
						PatientID:        p.ID,
						Name:             p.FullName,
						DOB:              p.DOB,
						Phone:            p.Phone,
						InsuranceCarrier: carrierName,
					}

					if carrierID != "" {
						resp.InsuranceCarrierID = carrierID
						routing, ambiguous := domain.RoutingForCarrierID(carrierID)
						resp.Routing = string(routing)
						resp.AllowedProviders = domain.ProvidersForRouting(routing)
						resp.RoutingAmbiguous = ambiguous
					}

					// Pediatric override: under-18 patients → Dr. Bach only
					if domain.IsMinor(p.DOB) && resp.Routing != "" && resp.Routing != string(domain.RoutingNotAccepted) {
						resp.Routing = string(domain.RoutingBachOnly)
						resp.AllowedProviders = domain.ProvidersForRouting(domain.RoutingBachOnly)
						resp.RoutingAmbiguous = false
					}

					json.NewEncoder(w).Encode(resp)
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

// AvailabilityRequest is the expected JSON body for availability lookup.
type AvailabilityRequest struct {
	Date     string `json:"date"`     // Required: YYYY-MM-DD format
	Provider string `json:"provider"` // Optional: filter by provider name
	Office   string `json:"office"`   // Optional: filter by office name (e.g., "Spring Hill", "Hollywood")
	Routing  string `json:"routing"`  // Optional: routing rule from verify/add-patient (e.g., "bach_only")
}

// providerDisplayNames maps profile IDs to friendly display names.
var providerDisplayNames = map[string]string{
	"620":  "Dr. Austin Bach",
	"2064": "Dr. J. Licht",
	"2076": "Dr. D. Noel",
}

// HandleGetAvailability returns available appointment slots for providers.
func (h *Handlers) HandleGetAvailability(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse request body
	var req AvailabilityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Invalid JSON body"})
		return
	}

	// Validate required date field
	if req.Date == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "date is required (YYYY-MM-DD format)"})
		return
	}

	// Parse start date
	startDate, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Invalid date format. Use YYYY-MM-DD."})
		return
	}

	// Reject same-day appointment searches
	todayEastern := time.Now().In(eastern).Format("2006-01-02")
	if startDate.Format("2006-01-02") == todayEastern {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Same-day appointments are not available. Please search for tomorrow or later."})
		return
	}

	log.Printf("availability: date=%s provider=%q office=%q routing=%q", req.Date, req.Provider, req.Office, req.Routing)

	// Get auth token
	tokenData, err := h.tokenManager.GetToken(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "Failed to get authentication token",
			Details: err.Error(),
		})
		return
	}

	// Get scheduler setup (1 XMLRPC call)
	setup, err := h.amdClient.GetSchedulerSetup(r.Context(), tokenData)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "Failed to get scheduler setup",
			Details: err.Error(),
		})
		return
	}

	// Build lookup maps
	profileMap := make(map[string]domain.SchedulerProfile)
	for _, p := range setup.Profiles {
		profileMap[p.ID] = p
	}

	facilityMap := make(map[string]domain.SchedulerFacility)
	for _, f := range setup.Facilities {
		facilityMap[f.ID] = f
	}

	// Resolve office filter to facility ID if provided
	var facilityFilter string
	if req.Office != "" {
		facilityID, ok := domain.LookupFacilityID(req.Office)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{
				Error: fmt.Sprintf("Unknown office: %q. Valid options: %s", req.Office, strings.Join(domain.ValidOfficeNames(), ", ")),
			})
			return
		}
		facilityFilter = facilityID
	}

	// Filter columns to allowed providers
	var allowedColumns []domain.SchedulerColumn
	for _, col := range setup.Columns {
		if domain.IsAllowedColumn(col.ID) {
			if facilityFilter != "" && col.FacilityID != facilityFilter {
				continue
			}
			if req.Provider != "" {
				profile, ok := profileMap[col.ProfileID]
				if !ok {
					continue
				}
				normalizedProvider := strings.ToUpper(domain.NormalizeForLookup(req.Provider))
				if !strings.Contains(strings.ToUpper(domain.NormalizeForLookup(profile.Name)), normalizedProvider) &&
					!strings.Contains(strings.ToUpper(domain.NormalizeForLookup(col.Name)), normalizedProvider) {
					continue
				}
			}
			allowedColumns = append(allowedColumns, col)
		}
	}

	// Apply routing filter (insurance-based provider restriction)
	if req.Routing != "" {
		rule := domain.ParseRoutingRule(req.Routing)
		routingColumns := domain.ColumnsForRouting(rule)
		if routingColumns != nil {
			var filtered []domain.SchedulerColumn
			for _, col := range allowedColumns {
				if routingColumns[col.ID] {
					filtered = append(filtered, col)
				}
			}
			allowedColumns = filtered
		} else {
			// RoutingNotAccepted — no columns allowed
			allowedColumns = nil
		}
	}

	// Determine location name for response
	locationName := "All Locations"
	if facilityFilter != "" {
		for _, f := range setup.Facilities {
			if f.ID == facilityFilter {
				locationName = f.Name
				break
			}
		}
	} else if len(allowedColumns) > 0 {
		if fac, ok := facilityMap[allowedColumns[0].FacilityID]; ok {
			locationName = fac.Name
		}
	}

	if len(allowedColumns) == 0 {
		if req.Provider != "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{
				Error: fmt.Sprintf("No provider found matching %q. Valid providers: %s",
					req.Provider, strings.Join(domain.ValidProviderNames(), ", ")),
			})
			return
		}
		json.NewEncoder(w).Encode(domain.AvailabilityResponse{
			SearchedDate: req.Date,
			Date:         startDate.Format("Monday, January 2, 2006"),
			Location:     locationName,
			Providers:    []domain.ProviderAvailability{},
		})
		return
	}

	nowEastern := time.Now().In(eastern)

	// Try the requested date first, then auto-search forward up to 14 days
	searchDate := startDate
	var providers []domain.ProviderAvailability

	for attempt := 0; attempt <= 14; attempt++ {
		dateStr := searchDate.Format("2006-01-02")

		// Fetch appointments and block holds per column
		columnIDs := make([]string, len(allowedColumns))
		for i, col := range allowedColumns {
			columnIDs[i] = col.ID
		}

		appointmentsByColumn, err := h.amdRestClient.GetAppointmentsForColumns(r.Context(), tokenData, columnIDs, dateStr)
		if err != nil {
			log.Printf("availability: failed to get appointments: %v", err)
			appointmentsByColumn = make(map[string][]domain.Appointment)
		}

		blockHoldsByColumn, err := h.amdRestClient.GetBlockHoldsForColumns(r.Context(), tokenData, columnIDs, dateStr)
		if err != nil {
			log.Printf("availability: failed to get block holds: %v", err)
			blockHoldsByColumn = make(map[string][]domain.BlockHold)
		}

		// Calculate availability for each provider
		providers = nil
		for _, col := range allowedColumns {
			profile := profileMap[col.ProfileID]
			facility := facilityMap[col.FacilityID]

			displayName := providerDisplayNames[col.ProfileID]
			if displayName == "" {
				displayName = profile.Name
			}

			allSlots := calculateAvailableSlots(col, appointmentsByColumn[col.ID], blockHoldsByColumn[col.ID], searchDate, nowEastern)

			colID, _ := strconv.Atoi(col.ID)
			profID, _ := strconv.Atoi(col.ProfileID)

			pa := domain.ProviderAvailability{
				Name:           displayName,
				ColumnID:       colID,
				ProfileID:      profID,
				Facility:       facility.Name,
				SlotDuration:   col.Interval,
				TotalAvailable: len(allSlots),
			}

			if len(allSlots) > 0 {
				pa.FirstAvailable = allSlots[0].Time
				pa.LastAvailable = allSlots[len(allSlots)-1].Time
				if len(allSlots) > 5 {
					pa.Slots = allSlots[:5]
				} else {
					pa.Slots = allSlots
				}
			} else {
				pa.Slots = []domain.AvailableSlot{}
			}

			providers = append(providers, pa)
		}

		// Check if any provider has availability
		hasAvailability := false
		for _, p := range providers {
			if p.TotalAvailable > 0 {
				hasAvailability = true
				break
			}
		}

		if hasAvailability || attempt == 14 {
			break
		}

		// No availability — try the next day
		searchDate = searchDate.AddDate(0, 0, 1)
		log.Printf("availability: no slots on %s, searching forward to %s", dateStr, searchDate.Format("2006-01-02"))
	}

	// Check if any provider has availability after the search loop
	hasAnyAvailability := false
	for _, p := range providers {
		if p.TotalAvailable > 0 {
			hasAnyAvailability = true
			break
		}
	}

	if !hasAnyAvailability {
		json.NewEncoder(w).Encode(domain.AvailabilityResponse{
			SearchedDate: req.Date,
			Date:         "",
			Location:     locationName,
			Message:      "No availability found within 14 days of requested date",
			Providers:    []domain.ProviderAvailability{},
		})
		return
	}

	json.NewEncoder(w).Encode(domain.AvailabilityResponse{
		SearchedDate: req.Date,
		Date:         searchDate.Format("Monday, January 2, 2006"),
		Location:     locationName,
		Providers:    providers,
	})
}

// calculateAvailableSlots generates available time slots for a column on a single day.
// nowEastern is used to filter out past slots when the date is today.
func calculateAvailableSlots(col domain.SchedulerColumn, appointments []domain.Appointment, blockHolds []domain.BlockHold, date time.Time, nowEastern time.Time) []domain.AvailableSlot {
	var slots []domain.AvailableSlot

	// Skip if provider doesn't work this day
	if !col.WorksOnDay(date.Weekday()) {
		return slots
	}

	// Get work hours
	workStart, workEnd, err := col.ParseWorkHours(date)
	if err != nil {
		return slots
	}

	// Determine cutoff for past slots: if date is today, skip slots before now + 30 min
	today := nowEastern.Format("2006-01-02")
	isToday := date.Format("2006-01-02") == today
	cutoff := nowEastern.Add(30 * time.Minute)

	interval := time.Duration(col.Interval) * time.Minute
	if interval == 0 {
		interval = 15 * time.Minute
	}

	maxAppts := col.MaxApptsPerSlot

	for slotTime := workStart; slotTime.Before(workEnd); slotTime = slotTime.Add(interval) {
		// Filter past slots
		if isToday {
			slotInEastern := time.Date(slotTime.Year(), slotTime.Month(), slotTime.Day(),
				slotTime.Hour(), slotTime.Minute(), 0, 0, nowEastern.Location())
			if slotInEastern.Before(cutoff) {
				continue
			}
		}

		if domain.IsBlockedByHold(slotTime, interval, blockHolds) {
			continue
		}

		// AMD 4101: Block if any appointment from a different start time overlaps this slot
		if hasOverlappingAppointment(slotTime, appointments) {
			continue
		}

		// AMD 4186: Check same-start-time appointment count against maxApptsPerSlot
		if maxAppts > 0 {
			count := countSameStartAppointments(slotTime, appointments)
			if count >= maxAppts {
				continue
			}
		}

		slots = append(slots, domain.AvailableSlot{
			Time:     domain.FormatSlotTime(slotTime),
			DateTime: domain.FormatSlotDateTime(slotTime),
		})
	}

	return slots
}

// hasOverlappingAppointment checks if any existing appointment's duration covers
// this slot. AMD returns error 4101 ("Overlaps existing appointment") for ANY
// overlap, including same-start times. This means maxApptsPerSlot (4186) is
// effectively unreachable when appointments have duration > 0.
func hasOverlappingAppointment(slotTime time.Time, appointments []domain.Appointment) bool {
	for _, appt := range appointments {
		// Block if this slot falls within [apptStart, apptStart+duration)
		apptEnd := appt.StartDateTime.Add(time.Duration(appt.Duration) * time.Minute)
		if !slotTime.Before(appt.StartDateTime) && slotTime.Before(apptEnd) {
			return true
		}
	}
	return false
}

// countSameStartAppointments counts appointments that start at exactly the given slot time.
// AMD returns error 4186 when this count exceeds maxApptsPerSlot.
func countSameStartAppointments(slotTime time.Time, appointments []domain.Appointment) int {
	count := 0
	for _, appt := range appointments {
		if appt.StartDateTime.Equal(slotTime) {
			count++
		}
	}
	return count
}
