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

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Method not allowed"})
		return
	}

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

	// Return only auth token variables
	dynamicVars := map[string]interface{}{
		"amd_token":         resp.Token,
		"amd_cookie_token":  resp.CookieToken,
		"amd_xmlrpc_url":    resp.XmlrpcURL,
		"amd_rest_api_base": resp.RestApiBase,
		"amd_ehr_api_base":  resp.EhrApiBase,
		"patient_verified":  "not_found",
		"patient_id":        "1",
		"booking_confirmed": 0,
	}

	json.NewEncoder(w).Encode(ElevenLabsWebhookResponse{
		Type:             "conversation_initiation_client_data",
		DynamicVariables: dynamicVars,
	})
}

// AddPatientRequest is the expected JSON body for patient creation.
type AddPatientRequest struct {
	FirstName         string `json:"firstName"`
	LastName          string `json:"lastName"`
	DOB               string `json:"dob"`
	Phone             string `json:"phone"`
	Email             string `json:"email"`
	Street            string `json:"street"`
	AptSuite          string `json:"aptSuite"`
	City              string `json:"city"`
	State             string `json:"state"`
	Zip               string `json:"zip"`
	Sex               string `json:"sex"`
	CarrierID      string `json:"carrierId"`
	SubscriberName string `json:"subscriberName"`
	SubscriberNum  string `json:"subscriberNum"`
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
		log.Printf("add-patient: failed to decode JSON: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:  "error",
			Message: "Invalid JSON body",
		})
		return
	}

	log.Printf("add-patient: received request: firstName=%q lastName=%q dob=%q phone=%q email=%q street=%q aptSuite=%q city=%q state=%q zip=%q sex=%q carrierId=%q subscriberName=%q subscriberNum=%q",
		req.FirstName, req.LastName, req.DOB, req.Phone, req.Email, req.Street, req.AptSuite, req.City, req.State, req.Zip, req.Sex, req.CarrierID, req.SubscriberName, req.SubscriberNum)

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
	if req.CarrierID == "" {
		missing = append(missing, "carrierId")
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
		FirstName: req.FirstName,
		LastName:  req.LastName,
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
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "error from AMD") {
			status = http.StatusConflict // 409 for AMD-reported errors (e.g., duplicate patient)
		}
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:  "error",
			Message: "Failed to create patient: " + err.Error(),
		})
		return
	}

	strippedID := domain.StripPatientPrefix(rawPatientID)

	// Look up carrier ID from name
	carrierID, ok := domain.LookupCarrierID(req.CarrierID)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:    "partial",
			PatientID: strippedID,
			Name:      patientName,
			DOB:       normalizedDOB,
			Message:   fmt.Sprintf("Patient created but insurance carrier not recognized: %q. Valid carriers: %s", req.CarrierID, strings.Join(domain.ValidCarrierNames(), ", ")),
		})
		return
	}

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
		json.NewEncoder(w).Encode(VerifyPatientResponse{
			Status:    "verified",
			PatientID: p.ID,
			Name:      p.FullName,
			DOB:       p.DOB,
			Phone:     p.Phone,
		})
		return

	default:
		// Multiple matches - try to disambiguate by first name
		if req.FirstName != "" {
			upperFirstName := strings.ToUpper(req.FirstName)
			for _, p := range matchingPatients {
				if strings.HasPrefix(p.FirstName, upperFirstName) {
					json.NewEncoder(w).Encode(VerifyPatientResponse{
						Status:    "verified",
						PatientID: p.ID,
						Name:      p.FullName,
						DOB:       p.DOB,
						Phone:     p.Phone,
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

// AvailabilityRequest is the expected JSON body for availability lookup.
type AvailabilityRequest struct {
	Date     string `json:"date"`     // Required: YYYY-MM-DD format
	Provider string `json:"provider"` // Optional: filter by provider name
	Days     int    `json:"days"`     // Optional: how many days to search (default 7)
	Office   string `json:"office"`   // Optional: filter by office name (e.g., "Spring Hill", "Hollywood")
}

// providerDisplayNames maps profile IDs to friendly display names.
var providerDisplayNames = map[string]string{
	"1135": "Dr. Austin Bach",
	"1141": "Dr. J. Licht",
	"1137": "Dr. D. Noel",
}

// HandleGetAvailability returns available appointment slots for providers.
func (h *Handlers) HandleGetAvailability(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Method not allowed. Use POST."})
		return
	}

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

	// Default to 3 days if not specified
	days := req.Days
	if days <= 0 {
		days = 3
	}
	if days > 30 {
		days = 30 // Cap at 30 days
	}

	log.Printf("availability: date=%s provider=%q days=%d office=%q", req.Date, req.Provider, days, req.Office)

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

	// Get scheduler setup
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
			// If office filter specified, check facility ID
			if facilityFilter != "" && col.FacilityID != facilityFilter {
				continue
			}
			// If provider filter specified, check if name matches
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
		// Default to first column's facility if no filter
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
			Date:      startDate.Format("Monday, January 2, 2006"),
			Location:  locationName,
			Providers: []domain.ProviderAvailability{},
		})
		return
	}

	// Get column IDs for appointments query
	columnIDs := make([]string, len(allowedColumns))
	for i, col := range allowedColumns {
		columnIDs[i] = col.ID
	}

	// Get existing appointments for all columns
	appointmentsByColumn, err := h.amdRestClient.GetAppointmentsForColumns(r.Context(), tokenData, columnIDs, req.Date)
	if err != nil {
		log.Printf("availability: failed to get appointments: %v", err)
		// Continue without appointments - will show all slots as available
		appointmentsByColumn = make(map[string][]domain.Appointment)
	}

	// Get block holds for all columns
	blockHoldsByColumn, err := h.amdRestClient.GetBlockHoldsForColumns(r.Context(), tokenData, columnIDs, req.Date)
	if err != nil {
		log.Printf("availability: failed to get block holds: %v", err)
		// Continue without block holds
		blockHoldsByColumn = make(map[string][]domain.BlockHold)
	}

	// Calculate availability for each provider
	var providers []domain.ProviderAvailability

	for _, col := range allowedColumns {
		profile := profileMap[col.ProfileID]
		facility := facilityMap[col.FacilityID]

		// Get display name
		displayName := providerDisplayNames[col.ProfileID]
		if displayName == "" {
			displayName = profile.Name
		}

		// Calculate available slots (with block holds)
		slots := calculateAvailableSlots(col, appointmentsByColumn[col.ID], blockHoldsByColumn[col.ID], startDate, days)

		// Build schedule description
		schedule := buildScheduleDescription(col)

		colID, _ := strconv.Atoi(col.ID)
		profID, _ := strconv.Atoi(col.ProfileID)

		providers = append(providers, domain.ProviderAvailability{
			Name:           displayName,
			ColumnID:       colID,
			ProfileID:      profID,
			Facility:       facility.Name,
			Schedule:       schedule,
			SlotDuration:   col.Interval,
			AvailableSlots: slots,
		})
	}

	json.NewEncoder(w).Encode(domain.AvailabilityResponse{
		Date:      startDate.Format("Monday, January 2, 2006"),
		Location:  locationName,
		Providers: providers,
	})
}

// calculateAvailableSlots generates available time slots for a column.
func calculateAvailableSlots(col domain.SchedulerColumn, appointments []domain.Appointment, blockHolds []domain.BlockHold, startDate time.Time, days int) []domain.AvailableSlot {
	var slots []domain.AvailableSlot

	// Build appointment count map: datetime string -> count
	apptCounts := make(map[string]int)
	for _, appt := range appointments {
		key := appt.StartDateTime.Format("2006-01-02T15:04")
		apptCounts[key]++
	}

	// Generate slots for each day
	for d := 0; d < days; d++ {
		date := startDate.AddDate(0, 0, d)

		// Skip if provider doesn't work this day
		if !col.WorksOnDay(date.Weekday()) {
			continue
		}

		// Get work hours
		workStart, workEnd, err := col.ParseWorkHours(date)
		if err != nil {
			continue
		}

		// Generate slots at interval
		interval := time.Duration(col.Interval) * time.Minute
		if interval == 0 {
			interval = 15 * time.Minute // Default to 15 min
		}

		for slotTime := workStart; slotTime.Before(workEnd); slotTime = slotTime.Add(interval) {
			// Skip if blocked by a hold (lunch, meetings, etc.)
			if domain.IsBlockedByHold(slotTime, blockHolds) {
				continue
			}

			// Check if slot is available (count < max)
			slotKey := slotTime.Format("2006-01-02T15:04")
			count := apptCounts[slotKey]

			maxAppts := col.MaxApptsPerSlot
			if maxAppts == 0 {
				maxAppts = 1 // Treat 0 as 1 for safety
			}

			if count >= maxAppts {
				continue // Slot is full
			}

			slots = append(slots, domain.AvailableSlot{
				Date:     domain.FormatSlotDate(slotTime),
				Time:     domain.FormatSlotTime(slotTime),
				DateTime: domain.FormatSlotDateTime(slotTime),
			})
		}
	}

	return slots
}

// buildScheduleDescription creates a human-readable schedule string.
func buildScheduleDescription(col domain.SchedulerColumn) string {
	// Parse work days from workweek bitmask
	days := []string{}
	dayNames := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

	for i := 0; i < 7; i++ {
		if col.Workweek&(1<<i) != 0 {
			days = append(days, dayNames[i])
		}
	}

	daysStr := "Varies"
	if len(days) > 0 {
		if len(days) == 5 && days[0] == "Monday" && days[4] == "Friday" {
			daysStr = "Monday-Friday"
		} else if len(days) == 2 {
			daysStr = days[0] + "-" + days[1]
		} else {
			daysStr = strings.Join(days, ", ")
		}
	}

	// Format times
	startTime := formatTimeForDisplay(col.StartTime)
	endTime := formatTimeForDisplay(col.EndTime)

	return fmt.Sprintf("%s, %s - %s", daysStr, startTime, endTime)
}

// formatTimeForDisplay converts 24h time to 12h format.
func formatTimeForDisplay(t string) string {
	parsed, err := time.Parse("15:04", t)
	if err != nil {
		return t
	}
	return parsed.Format("3:04 PM")
}
