package http

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"advancedmd-token-management/internal/clients"
	"advancedmd-token-management/internal/domain"
	"advancedmd-token-management/internal/safeerrors"
	"advancedmd-token-management/internal/session"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const maxAppointmentCommentLength = 1000

// schedulingWorkflow remains the existing booking mutation owner until the
// separately scoped mutation migration. Availability no longer crosses it.
type schedulingWorkflow struct {
	session            session.Session
	appointmentClient  *clients.AdvancedMDRestClient
	bookingTokenSecret string
	allowRawBooking    bool
}

type workflowError struct {
	outcome string
	message string
	missing []string
}

func invalidBookingTokenError() *workflowError {
	return &workflowError{
		outcome: "invalid_booking_token",
		message: "Invalid or expired booking token. Please check availability again and choose a slot.",
	}
}

func newSchedulingWorkflow(
	amdSession session.Session,
	appointmentClient *clients.AdvancedMDRestClient,
	bookingTokenSecret string,
) *schedulingWorkflow {
	return &schedulingWorkflow{
		session:            amdSession,
		appointmentClient:  appointmentClient,
		bookingTokenSecret: bookingTokenSecret,
	}
}

func (w *schedulingWorkflow) Book(ctx context.Context, req BookAppointmentRequest, now time.Time) (BookAppointmentResponse, *workflowError) {
	var office *domain.OfficeConfig
	if req.Office != "" || req.BookingToken == "" {
		var err error
		office, err = domain.ResolveOffice(req.Office)
		if err != nil {
			return BookAppointmentResponse{}, &workflowError{message: err.Error()}
		}
	}
	if req.BookingToken != "" {
		tokenOffice, err := w.applyBookingToken(&req, office, now.UTC())
		if err != nil {
			return BookAppointmentResponse{}, invalidBookingTokenError()
		}
		office = tokenOffice
	}

	log.Printf("book-appointment: request office=%s routingProvided=%t bookingToken=%t legacyRaw=%t typeId=%d", office.ID, req.Routing != "", req.BookingToken != "", req.BookingToken == "", req.AppointmentTypeID)

	if req.PatientID == "" {
		return BookAppointmentResponse{}, &workflowError{message: "patientId is required"}
	}
	if req.ColumnID == 0 {
		return BookAppointmentResponse{}, &workflowError{message: "columnId is required"}
	}
	if req.ProfileID == 0 {
		return BookAppointmentResponse{}, &workflowError{message: "profileId is required"}
	}
	if req.StartDatetime == "" {
		return BookAppointmentResponse{}, &workflowError{message: "startDatetime is required"}
	}
	if req.Duration == 0 {
		return BookAppointmentResponse{}, &workflowError{message: "duration is required"}
	}
	if err := domain.ValidateOptionalDOB(req.DOB); err != nil {
		return BookAppointmentResponse{}, &workflowError{message: err.Error()}
	}
	appointmentComment := buildBookingAppointmentComment(req.AppointmentReason, req.ReferringDoctor)
	if len([]rune(appointmentComment)) > maxAppointmentCommentLength {
		return BookAppointmentResponse{}, &workflowError{message: fmt.Sprintf("appointment comments must be %d characters or fewer", maxAppointmentCommentLength)}
	}

	policy := domain.NewSchedulingPolicy(office)
	decision, policyErr := policy.PrepareBooking(domain.BookingPolicyRequest{
		ColumnID:          req.ColumnID,
		ProfileID:         req.ProfileID,
		AppointmentTypeID: req.AppointmentTypeID,
		Routing:           domain.ParseRoutingRule(req.Routing),
		DOB:               req.DOB,
		Intent: domain.AppointmentIntent{
			VisitCategory: req.VisitCategory,
			VisitKind:     req.VisitKind,
			PatientStatus: req.PatientStatus,
			AgeBand:       req.AgeBand,
			DOB:           req.DOB,
			IsPostOp:      req.IsPostOp,
			VisitReason:   req.VisitReason,
		},
	})
	if policyErr != nil {
		return BookAppointmentResponse{}, &workflowError{outcome: policyErr.Outcome, message: policyErr.Message, missing: policyErr.Missing}
	}
	if len(req.bookingAppointmentTypeIDs) > 0 && !slices.Contains(req.bookingAppointmentTypeIDs, decision.AppointmentTypeID) {
		return BookAppointmentResponse{}, invalidBookingTokenError()
	}
	req.Routing = string(decision.Routing)
	req.AppointmentTypeID = decision.AppointmentTypeID

	patientID, err := strconv.Atoi(req.PatientID)
	if err != nil {
		return BookAppointmentResponse{}, &workflowError{message: "patientId must be numeric"}
	}
	if req.BookingToken == "" && !w.allowRawBooking {
		return BookAppointmentResponse{}, &workflowError{
			outcome: "booking_token_required",
			message: "bookingToken is required. Please check availability again and choose one of the returned slots.",
		}
	}

	tokenData, err := w.getToken(ctx)
	if err != nil {
		log.Printf("book-appointment: authentication failed category=%s", safeerrors.Classify(err))
		return BookAppointmentResponse{}, &workflowError{message: "Service authentication is temporarily unavailable. Please try again."}
	}
	if w.appointmentClient == nil {
		return BookAppointmentResponse{}, &workflowError{message: "Failed to book appointment in AdvancedMD. Please try again or contact the office."}
	}

	facilityID, _ := strconv.Atoi(office.FacilityID)
	force := 0
	if req.bookingRequiresForce {
		force = 1
	}
	appointmentID, err := w.appointmentClient.BookAppointment(ctx, tokenData, clients.BookAppointmentParams{
		PatientID:     patientID,
		ColumnID:      req.ColumnID,
		ProfileID:     req.ProfileID,
		StartDatetime: req.StartDatetime,
		Duration:      req.Duration,
		AppointmentType: []struct {
			ID int `json:"id"`
		}{{ID: decision.EnvironmentTypeID}},
		EpisodeID:  1,
		FacilityID: facilityID,
		Color:      decision.Color,
		Force:      force,
		Comments:   appointmentComment,
	})
	if err != nil {
		category := safeerrors.Classify(err)
		log.Printf("book-appointment: provider request failed category=%s", category)
		if category == safeerrors.CategoryConflict {
			return BookAppointmentResponse{}, &workflowError{
				outcome: "slot_unavailable",
				message: "This time slot is no longer available. Please check availability again and choose a different slot.",
			}
		}
		return BookAppointmentResponse{}, &workflowError{message: "Failed to book appointment in AdvancedMD. Please try again or contact the office."}
	}

	log.Printf("book-appointment: success office=%s", office.ID)
	return buildBookAppointmentReceipt(req, office, appointmentID), nil
}

func (w *schedulingWorkflow) getToken(ctx context.Context) (*domain.TokenData, error) {
	if w.session == nil {
		return nil, fmt.Errorf("session is not configured")
	}
	return w.session.Get(ctx)
}

func buildBookAppointmentReceipt(req BookAppointmentRequest, office *domain.OfficeConfig, appointmentID int) BookAppointmentResponse {
	colIDStr := strconv.Itoa(req.ColumnID)
	providerName := ""
	if col, ok := office.Columns[colIDStr]; ok {
		providerName = col.DisplayName
	}
	if providerName == "" {
		providerName = office.ProviderDisplayName(strconv.Itoa(req.ProfileID))
	}
	appointmentTypeName, _ := office.AppointmentTypeName(req.AppointmentTypeID)

	return BookAppointmentResponse{
		Status:              "booked",
		AppointmentID:       appointmentID,
		PatientID:           req.PatientID,
		PatientName:         normalizeBookingPatientName(req.PatientName),
		ProviderName:        providerName,
		LocationName:        office.DisplayName,
		StartDatetime:       req.StartDatetime,
		Duration:            req.Duration,
		AppointmentTypeID:   req.AppointmentTypeID,
		AppointmentTypeName: appointmentTypeName,
		Message:             "Appointment booked successfully",
	}
}

func normalizeBookingPatientName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	if parts := strings.SplitN(name, ",", 2); len(parts) == 2 {
		first := strings.TrimSpace(parts[1])
		last := strings.TrimSpace(parts[0])
		name = strings.TrimSpace(strings.Join([]string{first, last}, " "))
	}

	if name == strings.ToUpper(name) || name == strings.ToLower(name) {
		return cases.Title(language.English).String(strings.ToLower(name))
	}
	return name
}

func buildBookingAppointmentComment(appointmentReason string, referringDoctor string) string {
	appointmentReason = normalizeAppointmentCommentPart(appointmentReason)
	referringDoctor = normalizeAppointmentCommentPart(referringDoctor)
	if appointmentReason == "" && referringDoctor == "" {
		return ""
	}
	if appointmentReason == "" {
		appointmentReason = "none"
	}
	if referringDoctor == "" {
		referringDoctor = "none"
	}
	lines := []string{
		"Appointment reason: " + appointmentReason,
		"Referring doctor: " + referringDoctor,
		"- AI",
	}

	return strings.Join(lines, "\n")
}

func normalizeAppointmentCommentPart(value string) string {
	return strings.TrimSpace(value)
}
