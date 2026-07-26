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
	session       session.Session
	amdClient     *clients.AdvancedMDClient
	amdRestClient *clients.AdvancedMDRestClient
	patient       patientmodule.Patient
	scheduling    schedulingmodule.Scheduling
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(
	amdSession session.Session,
	amdClient *clients.AdvancedMDClient,
	amdRestClient *clients.AdvancedMDRestClient,
	patient patientmodule.Patient,
	scheduling schedulingmodule.Scheduling,
) *Handlers {
	if patient == nil {
		patient = patientmodule.New(advancedmd.NewAdapter(amdSession, amdClient, amdRestClient))
	}
	return &Handlers{
		session:       amdSession,
		amdClient:     amdClient,
		amdRestClient: amdRestClient,
		patient:       patient,
		scheduling:    scheduling,
	}
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

// HandleMetrics exposes PHI-free patient mutation outcome counters.
func (h *Handlers) HandleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintln(w, "# HELP patient_mutation_outcomes_total Patient mutation outcomes by operation and category.")
	fmt.Fprintln(w, "# TYPE patient_mutation_outcomes_total counter")
	for _, metric := range patientmodule.MutationMetricSnapshot() {
		fmt.Fprintf(
			w,
			"patient_mutation_outcomes_total{operation=%q,outcome=%q} %d\n",
			metric.Operation,
			metric.Outcome,
			metric.Count,
		)
	}
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
	Outcome          string   `json:"outcome,omitempty"`
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

	result := h.patient.Create(r.Context(), patientmodule.CreateCommand{
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		DOB:            req.DOB,
		Phone:          req.Phone,
		Email:          req.Email,
		Street:         req.Street,
		AptSuite:       req.AptSuite,
		City:           req.City,
		State:          req.State,
		Zip:            req.Zip,
		Sex:            req.Sex,
		SSN:            req.SSN,
		Insurance:      req.Insurance,
		CoverageType:   req.CoverageType,
		SubscriberName: req.SubscriberName,
		SubscriberNum:  req.SubscriberNum,
		Office:         req.Office,
	})
	outcome := ""
	if result.Status != patientmodule.CreateStatusCreated {
		outcome = string(result.Outcome)
	}
	json.NewEncoder(w).Encode(AddPatientResponse{
		Status:           string(result.Status),
		Outcome:          outcome,
		PatientID:        result.PatientID,
		Name:             result.Name,
		DOB:              result.DOB,
		Routing:          string(result.Routing),
		AllowedProviders: result.AllowedProviders,
		PreauthRequired:  result.PreauthRequired,
		Message:          result.Message,
	})
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

type CancelAppointmentRequest = schedulingmodule.CancelCommand
type CancelAppointmentResponse = schedulingmodule.CancelReceipt

// HandleCancelAppointment delegates cancellation behavior to Scheduling.
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
	if h.scheduling == nil {
		json.NewEncoder(w).Encode(CancelAppointmentResponse{
			Status:  "error",
			Outcome: string(schedulingmodule.CategoryWriteFailed),
			Message: "Appointment scheduling is temporarily unavailable. Please try again.",
		})
		return
	}
	response, err := h.scheduling.Cancel(r.Context(), req)
	if err != nil {
		json.NewEncoder(w).Encode(CancelAppointmentResponse{
			Status:  "error",
			Outcome: schedulingOutcome(err),
			Message: err.Error(),
		})
		return
	}
	json.NewEncoder(w).Encode(response)
}

type BookAppointmentRequest = schedulingmodule.BookCommand
type BookAppointmentResponse = schedulingmodule.BookReceipt

// HandleBookAppointment delegates booking behavior to Scheduling.
func (h *Handlers) HandleBookAppointment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req BookAppointmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(BookAppointmentResponse{Status: "error", Message: "Invalid JSON body"})
		return
	}
	if h.scheduling == nil {
		json.NewEncoder(w).Encode(BookAppointmentResponse{
			Status:  "error",
			Outcome: string(schedulingmodule.CategoryWriteFailed),
			Message: "Appointment scheduling is temporarily unavailable. Please try again.",
		})
		return
	}
	response, err := h.scheduling.Book(r.Context(), req)
	if err != nil {
		json.NewEncoder(w).Encode(BookAppointmentResponse{
			Status:  "error",
			Outcome: schedulingOutcome(err),
			Message: err.Error(),
			Missing: schedulingmodule.MissingOf(err),
		})
		return
	}
	json.NewEncoder(w).Encode(response)
}

func schedulingOutcome(err error) string {
	category := schedulingmodule.CategoryOf(err)
	if category == schedulingmodule.CategoryValidation {
		return ""
	}
	return string(category)
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
	Outcome          string   `json:"outcome,omitempty"`
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

	result := h.patient.UpdateInsurance(r.Context(), patientmodule.UpdateInsuranceCommand{
		PatientID:      req.PatientID,
		DOB:            req.DOB,
		InsPlanID:      req.InsPlanID,
		RespPartyID:    req.RespPartyID,
		OldInsurance:   req.OldInsurance,
		Insurance:      req.Insurance,
		CoverageType:   req.CoverageType,
		SubscriberName: req.SubscriberName,
		SubscriberNum:  req.SubscriberNum,
		Office:         req.Office,
	})
	outcome := ""
	if result.Status != patientmodule.UpdateInsuranceStatusUpdated {
		outcome = string(result.Outcome)
	}
	json.NewEncoder(w).Encode(UpdateInsuranceResponse{
		Status:           string(result.Status),
		Outcome:          outcome,
		PatientID:        result.PatientID,
		OldInsurance:     result.OldInsurance,
		NewInsurance:     result.NewInsurance,
		Routing:          string(result.Routing),
		AllowedProviders: result.AllowedProviders,
		RoutingAmbiguous: result.RoutingAmbiguous,
		PreauthRequired:  result.PreauthRequired,
		Message:          result.Message,
	})
}
