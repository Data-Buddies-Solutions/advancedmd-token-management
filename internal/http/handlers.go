package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"advancedmd-token-management/internal/advancedmd"
	"advancedmd-token-management/internal/clients"
	"advancedmd-token-management/internal/domain"
	patientmodule "advancedmd-token-management/internal/patient"
	"advancedmd-token-management/internal/safeerrors"
	schedulingmodule "advancedmd-token-management/internal/scheduling"
	"advancedmd-token-management/internal/session"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var eastern = domain.EasternLocation()

// ErrorResponse is the JSON response structure for error conditions.
// Returns 200 OK with status:"error" so ElevenLabs passes the message to the LLM.
type ErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// PatientResolveRequest is the single patient-read request shape. It supports
// pre-call phone lookup, verification by phone/name/DOB, and direct loading for
// an already verified patient ID.
type PatientResolveRequest struct {
	PatientID string `json:"patientId,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	DOB       string `json:"dob,omitempty"`
	FirstName string `json:"firstName,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Office    string `json:"office,omitempty"`
}

// PatientResolveResponse is returned by /api/patient/resolve.
type PatientResolveResponse struct {
	Status              string                   `json:"status"`
	PatientID           string                   `json:"patientId,omitempty"`
	Name                string                   `json:"name,omitempty"`
	DOB                 string                   `json:"dob,omitempty"`
	Phone               string                   `json:"phone,omitempty"`
	InsuranceCarrier    string                   `json:"insuranceCarrier,omitempty"`
	InsuranceCarrierID  string                   `json:"insuranceCarrierId,omitempty"`
	InsPlanID           string                   `json:"insPlanId,omitempty"`
	RespPartyID         string                   `json:"respPartyId,omitempty"`
	Routing             string                   `json:"routing,omitempty"`
	AllowedProviders    []string                 `json:"allowedProviders,omitempty"`
	RoutingAmbiguous    bool                     `json:"routingAmbiguous,omitempty"`
	PreauthRequired     bool                     `json:"preauthRequired,omitempty"`
	AppointmentsStatus  string                   `json:"appointmentsStatus,omitempty"`
	Appointments        []PatientApptDetail      `json:"appointments"`
	AppointmentsMessage string                   `json:"appointmentsMessage,omitempty"`
	Message             string                   `json:"message,omitempty"`
	Matches             []PatientResolveResponse `json:"matches,omitempty"`
}

// Handlers holds the dependencies for HTTP handlers.
type Handlers struct {
	session            session.Session
	amdClient          *clients.AdvancedMDClient
	amdRestClient      *clients.AdvancedMDRestClient
	patient            patientmodule.Patient
	scheduling         schedulingmodule.Scheduling
	schedulingWorkflow *schedulingWorkflow
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(
	amdSession session.Session,
	amdClient *clients.AdvancedMDClient,
	amdRestClient *clients.AdvancedMDRestClient,
	patient patientmodule.Patient,
	scheduling schedulingmodule.Scheduling,
	bookingTokenSecret ...string,
) *Handlers {
	secret := ""
	if len(bookingTokenSecret) > 0 {
		secret = bookingTokenSecret[0]
	}
	handlers := &Handlers{
		session:       amdSession,
		amdClient:     amdClient,
		amdRestClient: amdRestClient,
		patient:       patient,
		scheduling:    scheduling,
	}
	handlers.schedulingWorkflow = newSchedulingWorkflow(amdSession, amdRestClient, secret)
	return handlers
}

// SetAllowRawSlotBooking enables the legacy raw scheduler field booking path.
func (h *Handlers) SetAllowRawSlotBooking(allow bool) {
	h.workflow().allowRawBooking = allow
}

func (h *Handlers) workflow() *schedulingWorkflow {
	if h.schedulingWorkflow == nil {
		h.schedulingWorkflow = newSchedulingWorkflow(h.session, h.amdRestClient, "")
	}
	return h.schedulingWorkflow
}

// HandleLive reports process liveness without calling AdvancedMD.
func (h *Handlers) HandleLive(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// HandleReady reports that local initialization completed and HTTP traffic can
// be accepted. AdvancedMD session health remains a separate signal.
func (h *Handlers) HandleReady(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ready"}`))
}

// HandleSessionMaintenance refreshes the process-local AdvancedMD session
// without returning credentials, tokens, or provider endpoints.
func (h *Handlers) HandleSessionMaintenance(w http.ResponseWriter, r *http.Request) {
	if err := h.session.Maintain(r.Context()); err != nil {
		log.Printf("session maintenance failed category=%s", safeerrors.Classify(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"unavailable"}`))
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	SSN            string `json:"ssn,omitempty"`
	Insurance      string `json:"insurance"`
	CoverageType   string `json:"coverageType,omitempty"`
	SubscriberName string `json:"subscriberName"`
	SubscriberNum  string `json:"subscriberNum"`
	Office         string `json:"office,omitempty"`
}

// AddPatientResponse is returned after creating a patient.
type AddPatientResponse struct {
	Status           string   `json:"status"`
	PatientID        string   `json:"patientId,omitempty"`
	Name             string   `json:"name,omitempty"`
	DOB              string   `json:"dob,omitempty"`
	Routing          string   `json:"routing,omitempty"`
	AllowedProviders []string `json:"allowedProviders,omitempty"`
	PreauthRequired  bool     `json:"preauthRequired,omitempty"`
	Message          string   `json:"message,omitempty"`
}

// HandleAddPatient creates a new patient in AdvancedMD and attaches insurance.
func (h *Handlers) HandleAddPatient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req AddPatientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("add-patient: failed to decode JSON")
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:  "error",
			Message: "Invalid JSON body",
		})
		return
	}

	office, err := domain.ResolveOffice(req.Office)
	if err != nil {
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}
	policy := domain.NewSchedulingPolicy(office)

	insuranceMode := domain.InsuranceModeForCoverage(req.CoverageType)
	if domain.IsSelfPayInsurance(req.Insurance) && strings.TrimSpace(req.SubscriberNum) == "" {
		req.SubscriberNum = "self pay"
	}

	log.Printf("add-patient: received request office=%s coverageMode=%s", office.ID, insuranceMode)

	// Validate required fields (aptSuite and email are optional)
	missing := addPatientMissingFields(req)
	if len(missing) > 0 {
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:  "error",
			Message: fmt.Sprintf("Missing required fields: %s", strings.Join(missing, ", ")),
		})
		return
	}
	if insuranceMode == domain.InsuranceModeVision && !policy.SupportsRouting(domain.RoutingOpticalOnly) {
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:  "error",
			Message: fmt.Sprintf("Routine vision coverage is not supported at %s. Route the patient to Spring Hill routine vision scheduling.", office.DisplayName),
		})
		return
	}
	if insuranceMode == domain.InsuranceModeMedical && !policy.SupportsMedical() {
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:  "error",
			Message: fmt.Sprintf("Medical coverage is not supported at %s. Use routine vision coverage for this office or route medical visits to a medical office.", office.DisplayName),
		})
		return
	}

	// Normalize inputs
	normalizedDOB := domain.NormalizeDOB(req.DOB)
	formattedPhone := domain.FormatPhone(req.Phone)
	normalizedSex := domain.NormalizeSex(req.Sex)
	normalizedFirstName := domain.StripDiacritics(req.FirstName)
	normalizedLastName := domain.StripDiacritics(req.LastName)
	normalizedEmail := strings.TrimSpace(req.Email)

	// Get auth token
	tokenData, err := h.session.Get(r.Context())
	if err != nil {
		log.Printf("add-patient: authentication failed category=%s", safeerrors.Classify(err))
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:  "error",
			Message: "Service authentication is temporarily unavailable. Please try again.",
		})
		return
	}

	// Create patient in AMD
	rawPatientID, respPartyID, patientName, err := h.amdClient.AddPatient(r.Context(), tokenData, clients.AddPatientParams{
		FirstName: normalizedFirstName,
		LastName:  normalizedLastName,
		DOB:       normalizedDOB,
		Phone:     formattedPhone,
		Email:     normalizedEmail,
		Street:    req.Street,
		AptSuite:  req.AptSuite,
		City:      req.City,
		State:     strings.ToUpper(req.State),
		Zip:       req.Zip,
		Sex:       normalizedSex,
		SSN:       strings.TrimSpace(req.SSN),
		ProfileID: office.DefaultProfileID,
	})
	if err != nil {
		log.Printf("add-patient: provider request failed category=%s", safeerrors.Classify(err))
		if strings.Contains(err.Error(), "Duplicate name/DOB") {
			json.NewEncoder(w).Encode(AddPatientResponse{
				Status:  "error",
				Message: "A patient with this name and date of birth already exists in the system. Please try verifying the patient again instead of registering.",
			})
			return
		}
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:  "error",
			Message: "Failed to create patient in AdvancedMD. Please try again or contact the office.",
		})
		return
	}

	strippedID := domain.StripPatientPrefix(rawPatientID)

	// Look up insurance entry from name
	insEntry, ok := domain.LookupInsuranceForCoverageAtOffice(req.Insurance, insuranceMode, office)
	if !ok {
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:    "partial",
			PatientID: strippedID,
			Name:      patientName,
			DOB:       normalizedDOB,
			Message:   fmt.Sprintf("Patient created but insurance not recognized: %q. Please use an insurance name from the accepted list.", req.Insurance),
		})
		return
	}

	// Reject insurance not accepted at this office
	if insEntry.Routing == domain.RoutingNotAccepted {
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:    "partial",
			PatientID: strippedID,
			Name:      patientName,
			DOB:       normalizedDOB,
			Message:   fmt.Sprintf("%s is not accepted at %s. The patient may self-pay or contact the office for options.", req.Insurance, office.DisplayName),
		})
		return
	}

	// Attach insurance
	if err := h.amdClient.AddInsurance(r.Context(), tokenData, rawPatientID, respPartyID, insEntry.CarrierID, req.SubscriberNum); err != nil {
		log.Printf("add-patient: add insurance failed category=%s", safeerrors.Classify(err))
		json.NewEncoder(w).Encode(AddPatientResponse{
			Status:    "partial",
			PatientID: strippedID,
			Name:      patientName,
			DOB:       normalizedDOB,
			Message:   "Patient created but insurance could not be attached. Please contact the office.",
		})
		return
	}

	routing := insEntry.Routing
	if insuranceMode == domain.InsuranceModeMedical {
		routing = policy.SchedulingRouting(routing, normalizedDOB)
	}

	json.NewEncoder(w).Encode(AddPatientResponse{
		Status:           "created",
		PatientID:        strippedID,
		Name:             patientName,
		DOB:              normalizedDOB,
		Routing:          string(routing),
		AllowedProviders: policy.ProviderNames(routing, normalizedDOB),
		PreauthRequired:  insEntry.PreauthRequired,
		Message:          "Patient created and insurance attached successfully",
	})
}

func addPatientMissingFields(req AddPatientRequest) []string {
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
	return missing
}

// PatientApptDetail is a single appointment formatted for LLM consumption.
type PatientApptDetail struct {
	ID                int    `json:"id"`                          // AMD appointment ID — for cancel_appt
	Date              string `json:"date"`                        // Human-readable (e.g., "Wednesday, March 18, 2026")
	Time              string `json:"time"`                        // e.g., "12:00 PM"
	Provider          string `json:"provider,omitempty"`          // e.g., "Dr. Austin Bach"
	Type              string `json:"type,omitempty"`              // e.g., "New Adult Medical"
	AppointmentTypeID int    `json:"appointmentTypeId,omitempty"` // AMD appointment type ID
	Facility          string `json:"facility,omitempty"`          // e.g., "Abita Eye Group Spring Hill"
	OfficeID          string `json:"officeId,omitempty"`          // Stable office ID that owns the appointment column
	Office            string `json:"office,omitempty"`            // Display name for the owning office
}

// HandlePatientResolve resolves a patient and, by default, loads appointments.
func (h *Handlers) HandlePatientResolve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req PatientResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(PatientResolveResponse{
			Status:  "error",
			Message: "Invalid JSON body",
		})
		return
	}

	office, err := domain.ResolveOffice(req.Office)
	if err != nil {
		json.NewEncoder(w).Encode(PatientResolveResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	if msg := validatePatientResolveRequest(req); msg != "" {
		json.NewEncoder(w).Encode(PatientResolveResponse{
			Status:  "error",
			Message: msg,
		})
		return
	}

	if h.patient == nil {
		log.Printf("patient-resolve: module unavailable category=%s", safeerrors.CategoryInternal)
		json.NewEncoder(w).Encode(PatientResolveResponse{
			Status:  "error",
			Message: "Failed to look up patient in AdvancedMD. Please try again.",
		})
		return
	}

	result, err := h.patient.Resolve(r.Context(), patientmodule.ResolveCommand{
		PatientID: req.PatientID,
		LastName:  req.LastName,
		DOB:       req.DOB,
		FirstName: req.FirstName,
		Phone:     req.Phone,
		OfficeID:  office.ID,
	})
	if err != nil {
		category := advancedmd.CategoryOf(err)
		log.Printf("patient-resolve: failed category=%s", category)
		message := "Failed to look up patient in AdvancedMD. Please try again."
		if category == safeerrors.CategoryAuthentication || category == safeerrors.CategoryUnavailable {
			message = "Service authentication is temporarily unavailable. Please try again."
		}
		json.NewEncoder(w).Encode(PatientResolveResponse{
			Status:  "error",
			Message: message,
		})
		return
	}

	json.NewEncoder(w).Encode(patientResolveResponse(result))
}

func patientResolveResponse(result patientmodule.ResolveResult) PatientResolveResponse {
	appointments := make([]PatientApptDetail, len(result.Appointments))
	for i, appointment := range result.Appointments {
		appointments[i] = PatientApptDetail{
			ID:                appointment.ID,
			Date:              appointment.Date,
			Time:              appointment.Time,
			Provider:          appointment.Provider,
			Type:              appointment.Type,
			AppointmentTypeID: appointment.AppointmentTypeID,
			Facility:          appointment.Facility,
			OfficeID:          appointment.OfficeID,
			Office:            appointment.Office,
		}
	}
	matches := make([]PatientResolveResponse, len(result.Matches))
	for i, match := range result.Matches {
		matches[i] = patientResolveResponse(match)
	}
	return PatientResolveResponse{
		Status:              string(result.Status),
		PatientID:           result.PatientID,
		Name:                result.Name,
		DOB:                 result.DOB,
		Phone:               result.Phone,
		InsuranceCarrier:    result.InsuranceCarrier,
		InsuranceCarrierID:  result.InsuranceCarrierID,
		InsPlanID:           result.InsPlanID,
		RespPartyID:         result.RespPartyID,
		Routing:             string(result.Routing),
		AllowedProviders:    result.AllowedProviders,
		RoutingAmbiguous:    result.RoutingAmbiguous,
		PreauthRequired:     result.PreauthRequired,
		AppointmentsStatus:  string(result.AppointmentsStatus),
		Appointments:        appointments,
		AppointmentsMessage: result.AppointmentsMessage,
		Message:             result.Message,
		Matches:             matches,
	}
}

func validatePatientResolveRequest(req PatientResolveRequest) string {
	hasPatientID := req.PatientID != ""
	hasLookupFields := req.Phone != "" || req.FirstName != "" || req.LastName != "" || req.DOB != ""
	if hasPatientID {
		if _, err := strconv.Atoi(req.PatientID); err != nil {
			return "patientId must be numeric"
		}
		if hasLookupFields {
			return "Provide either patientId or lookup fields, not both"
		}
		return ""
	}
	if req.Phone != "" {
		if domain.NormalizePhoneDigits(req.Phone) == "" {
			return "phone must contain at least one digit"
		}
		return ""
	}
	if req.LastName != "" && req.DOB != "" {
		return ""
	}
	return "Provide patientId, phone, phone + firstName, phone + dob, or lastName + dob"
}

// fetchUpcomingAppointments retrieves future appointments for a patient ID
// (current month + 5 months forward).
func (h *Handlers) fetchUpcomingAppointments(ctx context.Context, tokenData *domain.TokenData, patientID string, office *domain.OfficeConfig) ([]PatientApptDetail, error) {
	lookupOffices := domain.AppointmentLookupOffices(office)
	details := make([]PatientApptDetail, 0)
	for _, lookupOffice := range lookupOffices {
		officeDetails, err := h.fetchUpcomingAppointmentsForOffice(ctx, tokenData, patientID, lookupOffice)
		if err != nil {
			return nil, err
		}
		details = append(details, officeDetails...)
	}
	return details, nil
}

func (h *Handlers) fetchUpcomingAppointmentsForOffice(ctx context.Context, tokenData *domain.TokenData, patientID string, office *domain.OfficeConfig) ([]PatientApptDetail, error) {
	patientIDInt, err := strconv.Atoi(patientID)
	if err != nil {
		return nil, fmt.Errorf("patientId must be numeric: %w", err)
	}

	columnIDStr := strings.Join(office.AllowedColumnIDs(), "-")

	now := time.Now().In(eastern)
	cutoff := appointmentLookupCutoff(now)
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, eastern)

	months := make([]time.Time, 6)
	for i := range months {
		months[i] = thisMonth.AddDate(0, i, 0)
	}

	type monthResult struct {
		appts []clients.AMDAppointmentResponse
		err   error
	}
	ch := make(chan monthResult, len(months))

	for _, m := range months {
		m := m
		go func() {
			appts, err := h.amdRestClient.GetAppointmentsByMonth(ctx, tokenData, columnIDStr, m.Format("2006-01-02"))
			ch <- monthResult{appts, err}
		}()
	}

	var allAppts []clients.AMDAppointmentResponse
	for range months {
		r := <-ch
		if r.err != nil {
			return nil, r.err
		}
		allAppts = append(allAppts, r.appts...)
	}
	var details []PatientApptDetail
	for _, a := range allAppts {
		if a.PatientID != patientIDInt {
			continue
		}

		startTime, err := clients.ParseDateTime(a.StartDateTime)
		if err != nil {
			continue
		}
		if !startTime.After(cutoff) {
			continue
		}

		appointmentTypeID := 0
		typeName := ""
		if len(a.AppointmentTypes) > 0 {
			appointmentTypeID = a.AppointmentTypes[0]
			if canonicalTypeID, ok := domain.CanonicalAppointmentTypeID(appointmentTypeID); ok {
				appointmentTypeID = canonicalTypeID
			}
			if name, ok := office.AppointmentTypeName(appointmentTypeID); ok {
				typeName = name
			}
		}

		detail := PatientApptDetail{
			ID:                a.ID,
			Date:              startTime.Format("Monday, January 2, 2006"),
			Time:              startTime.Format("3:04 PM"),
			Provider:          office.FriendlyProviderName(a.Provider),
			Type:              typeName,
			AppointmentTypeID: appointmentTypeID,
			Facility:          appointmentFacilityName(a.Facility, office),
			OfficeID:          office.ID,
			Office:            office.DisplayName,
		}
		details = append(details, detail)
	}

	return details, nil
}

func appointmentLookupCutoff(now time.Time) time.Time {
	local := now.In(eastern)
	return time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), local.Minute(), local.Second(), 0, time.UTC)
}

// friendlyFacilityName cleans up AMD facility names to title case.
func friendlyFacilityName(amdName string) string {
	if amdName == "" {
		return ""
	}
	return cases.Title(language.English).String(strings.ToLower(amdName))
}

func appointmentFacilityName(amdName string, office *domain.OfficeConfig) string {
	facility := friendlyFacilityName(amdName)
	if facility != "" {
		return facility
	}
	return office.DisplayName
}

// CancelAppointmentRequest is the expected JSON body for cancelling an appointment.
type CancelAppointmentRequest struct {
	AppointmentID int    `json:"appointmentId"`
	PatientID     string `json:"patientId,omitempty"`
	Office        string `json:"office,omitempty"`
}

// CancelAppointmentResponse is returned after cancelling an appointment.
type CancelAppointmentResponse struct {
	Status        string `json:"status"`
	AppointmentID int    `json:"appointmentId,omitempty"`
	Message       string `json:"message"`
}

// HandleCancelAppointment cancels an appointment in AdvancedMD.
func (h *Handlers) HandleCancelAppointment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req CancelAppointmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(CancelAppointmentResponse{
			Status:  "error",
			Message: "Invalid JSON body",
		})
		return
	}

	if req.AppointmentID == 0 {
		json.NewEncoder(w).Encode(CancelAppointmentResponse{
			Status:  "error",
			Message: "appointmentId is required",
		})
		return
	}
	req.PatientID = domain.StripPatientPrefix(strings.TrimSpace(req.PatientID))
	if req.PatientID == "" {
		json.NewEncoder(w).Encode(CancelAppointmentResponse{
			Status:  "error",
			Message: "patientId is required",
		})
		return
	}
	if _, err := strconv.Atoi(req.PatientID); err != nil {
		json.NewEncoder(w).Encode(CancelAppointmentResponse{
			Status:  "error",
			Message: "patientId must be numeric",
		})
		return
	}
	office, err := domain.ResolveOffice(req.Office)
	if err != nil {
		json.NewEncoder(w).Encode(CancelAppointmentResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	// Get auth token
	tokenData, err := h.session.Get(r.Context())
	if err != nil {
		log.Printf("cancel-appointment: authentication failed category=%s", safeerrors.Classify(err))
		json.NewEncoder(w).Encode(CancelAppointmentResponse{
			Status:  "error",
			Message: "Service authentication is temporarily unavailable. Please try again.",
		})
		return
	}

	owningOffice, err := h.cancelableAppointmentOffice(r.Context(), tokenData, req.PatientID, req.AppointmentID, office)
	if err != nil {
		log.Printf("cancel-appointment: appointment ownership check failed category=%s", safeerrors.Classify(err))
		json.NewEncoder(w).Encode(CancelAppointmentResponse{
			Status:  "error",
			Message: "Unable to verify appointment before cancellation. Please load appointments again and choose the appointment to cancel.",
		})
		return
	}
	if owningOffice == nil {
		json.NewEncoder(w).Encode(CancelAppointmentResponse{
			Status:  "error",
			Message: "No upcoming appointment matches that patient and appointment ID. Please load appointments again and choose the appointment to cancel.",
		})
		return
	}

	log.Printf("cancel-appointment: request office=%s", owningOffice.ID)

	// Cancel via AMD REST API
	if err := h.amdRestClient.CancelAppointment(r.Context(), tokenData, req.AppointmentID); err != nil {
		log.Printf("cancel-appointment: provider request failed category=%s", safeerrors.Classify(err))
		json.NewEncoder(w).Encode(CancelAppointmentResponse{
			Status:  "error",
			Message: "Failed to cancel appointment in AdvancedMD. Please try again or contact the office.",
		})
		return
	}

	json.NewEncoder(w).Encode(CancelAppointmentResponse{
		Status:        "cancelled",
		AppointmentID: req.AppointmentID,
		Message:       "Appointment cancelled successfully",
	})
}

func (h *Handlers) cancelableAppointmentOffice(ctx context.Context, tokenData *domain.TokenData, patientID string, appointmentID int, office *domain.OfficeConfig) (*domain.OfficeConfig, error) {
	if h.amdRestClient == nil {
		return nil, fmt.Errorf("AdvancedMD appointment client is not configured")
	}
	appointments, err := h.fetchUpcomingAppointments(ctx, tokenData, patientID, office)
	if err != nil {
		return nil, err
	}
	for _, appointment := range appointments {
		if appointment.ID != appointmentID {
			continue
		}
		if appointment.OfficeID == "" {
			return office, nil
		}
		owningOffice, ok := lookupOfficeByID(appointment.OfficeID)
		if !ok {
			return nil, fmt.Errorf("unknown appointment office ID %q", appointment.OfficeID)
		}
		return owningOffice, nil
	}
	return nil, nil
}

// BookAppointmentRequest is the expected JSON body for booking an appointment.
type BookAppointmentRequest struct {
	PatientID         string `json:"patientId"`
	PatientName       string `json:"patientName,omitempty"`
	DOB               string `json:"dob,omitempty"`
	BookingToken      string `json:"bookingToken,omitempty"`
	ColumnID          int    `json:"columnId"`
	ProfileID         int    `json:"profileId"`
	StartDatetime     string `json:"startDatetime"`
	Duration          int    `json:"duration"`
	AppointmentTypeID int    `json:"appointmentTypeId"`
	Routing           string `json:"routing,omitempty"`
	Office            string `json:"office,omitempty"`
	VisitCategory     string `json:"visitCategory,omitempty"`
	VisitKind         string `json:"visitKind,omitempty"`
	PatientStatus     string `json:"patientStatus,omitempty"`
	AgeBand           string `json:"ageBand,omitempty"`
	IsPostOp          bool   `json:"isPostOp,omitempty"`
	VisitReason       string `json:"visitReason,omitempty"`
	AppointmentReason string `json:"appointmentReason,omitempty"`
	ReferringDoctor   string `json:"referringDoctor,omitempty"`

	bookingRequiresForce      bool
	bookingAppointmentTypeIDs []int
}

// BookAppointmentResponse is returned after booking an appointment.
type BookAppointmentResponse struct {
	Status              string   `json:"status"`
	Outcome             string   `json:"outcome,omitempty"`
	AppointmentID       int      `json:"appointmentId,omitempty"`
	PatientID           string   `json:"patientId,omitempty"`
	PatientName         string   `json:"patientName,omitempty"`
	ProviderName        string   `json:"providerName,omitempty"`
	LocationName        string   `json:"locationName,omitempty"`
	StartDatetime       string   `json:"startDatetime,omitempty"`
	Duration            int      `json:"duration,omitempty"`
	AppointmentTypeID   int      `json:"appointmentTypeId,omitempty"`
	AppointmentTypeName string   `json:"appointmentTypeName,omitempty"`
	Message             string   `json:"message"`
	Missing             []string `json:"missing,omitempty"`
}

// HandleBookAppointment books an appointment through the Scheduling Workflow.
func (h *Handlers) HandleBookAppointment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req BookAppointmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(BookAppointmentResponse{Status: "error", Message: "Invalid JSON body"})
		return
	}

	response, workflowErr := h.workflow().Book(r.Context(), req, time.Now())
	if workflowErr != nil {
		json.NewEncoder(w).Encode(BookAppointmentResponse{
			Status:  "error",
			Outcome: workflowErr.outcome,
			Message: workflowErr.message,
			Missing: workflowErr.missing,
		})
		return
	}
	json.NewEncoder(w).Encode(response)
}

// AvailabilityRequest preserves the authenticated HTTP request shape while the
// Scheduling module owns its behavior.
type AvailabilityRequest = schedulingmodule.SearchCommand

// HandleGetAvailability searches through the Scheduling module.
func (h *Handlers) HandleGetAvailability(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req AvailabilityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(ErrorResponse{Status: "error", Message: "Invalid JSON body"})
		return
	}

	if h.scheduling == nil {
		log.Printf("availability: module unavailable category=%s", safeerrors.CategoryInternal)
		json.NewEncoder(w).Encode(ErrorResponse{
			Status:  "error",
			Message: "Appointment scheduling is temporarily unavailable. Please try again.",
		})
		return
	}
	response, err := h.scheduling.Search(r.Context(), req)
	if err != nil {
		json.NewEncoder(w).Encode(ErrorResponse{Status: "error", Message: err.Error()})
		return
	}
	json.NewEncoder(w).Encode(response)
}

// UpdateInsuranceRequest is the expected JSON body for insurance updates.
type UpdateInsuranceRequest struct {
	PatientID      string `json:"patientId"`
	DOB            string `json:"dob,omitempty"`
	InsPlanID      string `json:"insPlanId"`
	RespPartyID    string `json:"respPartyId"`
	OldInsurance   string `json:"oldInsurance"`
	Insurance      string `json:"insurance"`
	CoverageType   string `json:"coverageType,omitempty"`
	SubscriberName string `json:"subscriberName"`
	SubscriberNum  string `json:"subscriberNum"`
	Office         string `json:"office,omitempty"`
}

// UpdateInsuranceResponse is returned after updating insurance.
type UpdateInsuranceResponse struct {
	Status           string   `json:"status"`
	PatientID        string   `json:"patientId,omitempty"`
	OldInsurance     string   `json:"oldInsurance,omitempty"`
	NewInsurance     string   `json:"newInsurance,omitempty"`
	Routing          string   `json:"routing,omitempty"`
	AllowedProviders []string `json:"allowedProviders,omitempty"`
	RoutingAmbiguous bool     `json:"routingAmbiguous,omitempty"`
	PreauthRequired  bool     `json:"preauthRequired,omitempty"`
	Message          string   `json:"message,omitempty"`
}

// HandleUpdateInsurance swaps a patient's insurance: end-dates the old plan and attaches a new one.
func (h *Handlers) HandleUpdateInsurance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req UpdateInsuranceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(UpdateInsuranceResponse{
			Status:  "error",
			Message: "Invalid JSON body",
		})
		return
	}

	// Validate required fields
	if domain.IsSelfPayInsurance(req.Insurance) && strings.TrimSpace(req.SubscriberNum) == "" {
		req.SubscriberNum = "self pay"
	}
	if req.PatientID == "" || req.Insurance == "" || req.SubscriberNum == "" {
		json.NewEncoder(w).Encode(UpdateInsuranceResponse{
			Status:  "error",
			Message: "patientId, insurance, and subscriberNum are required",
		})
		return
	}
	if err := domain.ValidateOptionalDOB(req.DOB); err != nil {
		json.NewEncoder(w).Encode(UpdateInsuranceResponse{Status: "error", Message: err.Error()})
		return
	}

	office, err := domain.ResolveOffice(req.Office)
	if err != nil {
		json.NewEncoder(w).Encode(UpdateInsuranceResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}
	policy := domain.NewSchedulingPolicy(office)
	insuranceMode := domain.InsuranceModeForCoverage(req.CoverageType)
	if insuranceMode == domain.InsuranceModeVision && !policy.SupportsRouting(domain.RoutingOpticalOnly) {
		json.NewEncoder(w).Encode(UpdateInsuranceResponse{
			Status:  "error",
			Message: fmt.Sprintf("Routine vision coverage is not supported at %s. Route the patient to Spring Hill routine vision scheduling.", office.DisplayName),
		})
		return
	}
	if insuranceMode == domain.InsuranceModeMedical && !policy.SupportsMedical() {
		json.NewEncoder(w).Encode(UpdateInsuranceResponse{
			Status:  "error",
			Message: fmt.Sprintf("Medical coverage is not supported at %s. Use routine vision coverage for this office or route medical visits to a medical office.", office.DisplayName),
		})
		return
	}

	// Look up new insurance
	insEntry, found := domain.LookupInsuranceForCoverageAtOffice(req.Insurance, insuranceMode, office)
	if !found {
		json.NewEncoder(w).Encode(UpdateInsuranceResponse{
			Status:  "error",
			Message: fmt.Sprintf("Insurance not recognized: %q. Please use an insurance name from the accepted list.", req.Insurance),
		})
		return
	}

	if insEntry.Routing == domain.RoutingNotAccepted {
		json.NewEncoder(w).Encode(UpdateInsuranceResponse{
			Status:  "error",
			Message: fmt.Sprintf("%s is not accepted at %s.", req.Insurance, office.DisplayName),
		})
		return
	}

	// Get AMD token
	tokenData, err := h.session.Get(r.Context())
	if err != nil {
		log.Printf("update-insurance: authentication failed category=%s", safeerrors.Classify(err))
		json.NewEncoder(w).Encode(UpdateInsuranceResponse{
			Status:  "error",
			Message: "Service authentication is temporarily unavailable. Please try again.",
		})
		return
	}

	// End-date old plan if insplan ID provided
	if req.InsPlanID != "" {
		if err := h.amdClient.EndDateInsurance(r.Context(), tokenData, req.PatientID, req.InsPlanID); err != nil {
			log.Printf("update-insurance: end-date failed category=%s", safeerrors.Classify(err))
			json.NewEncoder(w).Encode(UpdateInsuranceResponse{
				Status:  "error",
				Message: "Failed to update existing insurance in AdvancedMD. Please try again or contact the office.",
			})
			return
		}
	}

	// Add new insurance plan
	if err := h.amdClient.AddInsurance(r.Context(), tokenData, req.PatientID, req.RespPartyID, insEntry.CarrierID, req.SubscriberNum); err != nil {
		log.Printf("update-insurance: add insurance failed category=%s", safeerrors.Classify(err))
		json.NewEncoder(w).Encode(UpdateInsuranceResponse{
			Status:  "error",
			Message: "Failed to attach new insurance in AdvancedMD. Please try again or contact the office.",
		})
		return
	}

	routing := insEntry.Routing
	routing = policy.SchedulingRouting(routing, req.DOB)
	_, ambiguous := domain.RoutingForDemographicInsurance(insEntry.CarrierID, req.Insurance, office)

	json.NewEncoder(w).Encode(UpdateInsuranceResponse{
		Status:           "updated",
		PatientID:        req.PatientID,
		OldInsurance:     req.OldInsurance,
		NewInsurance:     req.Insurance,
		Routing:          string(routing),
		AllowedProviders: policy.ProviderNames(routing, req.DOB),
		RoutingAmbiguous: ambiguous,
		PreauthRequired:  insEntry.PreauthRequired,
		Message:          "Insurance updated successfully",
	})
}
