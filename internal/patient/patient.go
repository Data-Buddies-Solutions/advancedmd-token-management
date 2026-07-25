// Package patient owns patient resolution and construction of complete Acuity
// patient results.
package patient

import (
	"context"
	"fmt"
	"log"
	"strings"

	"advancedmd-token-management/internal/advancedmd"
	"advancedmd-token-management/internal/domain"
	"advancedmd-token-management/internal/safeerrors"
)

type Status string

const (
	StatusVerified        Status = "verified"
	StatusMultipleMatches Status = "multiple_matches"
	StatusNotFound        Status = "not_found"
)

type AppointmentsStatus string

const (
	AppointmentsFound AppointmentsStatus = "found"
	AppointmentsNone  AppointmentsStatus = "none"
	AppointmentsError AppointmentsStatus = "error"
)

// ResolveCommand is the complete patient-resolution intent accepted from the
// HTTP adapter after transport validation.
type ResolveCommand struct {
	PatientID string
	LastName  string
	DOB       string
	FirstName string
	Phone     string
	OfficeID  string
}

type Appointment struct {
	ID                int
	Date              string
	Time              string
	Provider          string
	Type              string
	AppointmentTypeID int
	Facility          string
	OfficeID          string
	Office            string
}

// ResolveResult is one complete Acuity patient resolution outcome.
type ResolveResult struct {
	Status              Status
	PatientID           string
	Name                string
	DOB                 string
	Phone               string
	InsuranceCarrier    string
	InsuranceCarrierID  string
	InsPlanID           string
	RespPartyID         string
	Routing             domain.RoutingRule
	AllowedProviders    []string
	RoutingAmbiguous    bool
	PreauthRequired     bool
	AppointmentsStatus  AppointmentsStatus
	Appointments        []Appointment
	AppointmentsMessage string
	Message             string
	Matches             []ResolveResult
}

// Patient is the single interface used by patient-facing HTTP routes.
type Patient interface {
	Resolve(context.Context, ResolveCommand) (ResolveResult, error)
}

type patient struct {
	advancedMD advancedmd.PatientRecords
}

func New(advancedMD advancedmd.PatientRecords) Patient {
	return &patient{advancedMD: advancedMD}
}

func (p *patient) Resolve(ctx context.Context, command ResolveCommand) (ResolveResult, error) {
	office, ok := domain.LookupOfficeByID(command.OfficeID)
	if !ok {
		return ResolveResult{}, advancedmd.NewError(safeerrors.CategoryInternal)
	}
	if command.PatientID != "" {
		return p.resolvePatient(ctx, domain.Patient{ID: command.PatientID}, "", office)
	}

	search := patientSearch(command)
	patients, err := p.advancedMD.SearchPatients(ctx, search)
	if err != nil {
		return ResolveResult{}, err
	}
	matches := selectPatients(patients, command)
	if len(matches) == 0 {
		return ResolveResult{
			Status:       StatusNotFound,
			Appointments: []Appointment{},
			Message:      notFoundMessage(command),
		}, nil
	}
	if len(matches) > 1 {
		results := make([]ResolveResult, 0, len(matches))
		for _, candidate := range matches {
			match, err := p.resolvePatient(ctx, candidate, command.Phone, office)
			if err != nil {
				return ResolveResult{}, err
			}
			results = append(results, match)
		}
		return ResolveResult{
			Status:       StatusMultipleMatches,
			Appointments: []Appointment{},
			Matches:      results,
			Message:      multipleMatchesMessage(command, len(results)),
		}, nil
	}

	return p.resolvePatient(ctx, matches[0], command.Phone, office)
}

func patientSearch(command ResolveCommand) domain.PatientSearch {
	if command.Phone != "" {
		return domain.PatientSearch{Phone: domain.NormalizePhoneDigits(command.Phone)}
	}
	return domain.PatientSearch{
		FirstName: domain.StripDiacritics(command.FirstName),
		LastName:  domain.StripDiacritics(command.LastName),
	}
}

func selectPatients(patients []domain.Patient, command ResolveCommand) []domain.Patient {
	matches := patients
	if command.DOB != "" {
		normalizedDOB := domain.NormalizeDOB(command.DOB)
		matches = nil
		for _, candidate := range patients {
			if domain.NormalizeDOB(candidate.DOB) == normalizedDOB {
				matches = append(matches, candidate)
			}
		}
	}

	if command.FirstName == "" {
		return matches
	}
	firstNameMatches := make([]domain.Patient, 0, len(matches))
	for _, candidate := range matches {
		candidateFirstName := strings.ToUpper(domain.StripDiacritics(candidate.FirstName))
		requestFirstName := strings.ToUpper(domain.StripDiacritics(command.FirstName))
		if strings.HasPrefix(candidateFirstName, requestFirstName) {
			firstNameMatches = append(firstNameMatches, candidate)
		}
	}
	return firstNameMatches
}

func (p *patient) resolvePatient(ctx context.Context, candidate domain.Patient, lookupPhone string, office *domain.OfficeConfig) (ResolveResult, error) {
	result := ResolveResult{
		Status:       StatusVerified,
		PatientID:    candidate.ID,
		Name:         candidate.FullName,
		DOB:          candidate.DOB,
		Phone:        firstNonEmpty(candidate.Phone, lookupPhone),
		Appointments: []Appointment{},
	}

	demographics, err := p.advancedMD.GetPatientDemographics(ctx, candidate.ID)
	if err != nil {
		category := advancedmd.CategoryOf(err)
		log.Printf("patient-resolve: failed to get demographics category=%s", category)
		if category == safeerrors.CategoryUnavailable {
			return ResolveResult{}, err
		}
	} else {
		if result.DOB == "" {
			result.DOB = demographics.DOB
		}
		applyDemographics(&result, demographics, office, result.DOB)
	}

	appointments, err := p.advancedMD.GetUpcomingAppointments(ctx, domain.PatientAppointmentsQuery{
		PatientID: candidate.ID,
		OfficeIDs: appointmentOfficeIDs(office),
	})
	if err != nil {
		log.Printf("patient-resolve: failed to get appointments category=%s", advancedmd.CategoryOf(err))
		result.AppointmentsStatus = AppointmentsError
		result.AppointmentsMessage = "Failed to retrieve appointments from AdvancedMD. Please try again."
		result.Message = "Patient verified, appointment lookup unavailable"
		return result, nil
	}

	for _, appointment := range appointments {
		result.Appointments = append(result.Appointments, Appointment{
			ID:                appointment.ID,
			Date:              appointment.Start.Format("Monday, January 2, 2006"),
			Time:              appointment.Start.Format("3:04 PM"),
			Provider:          appointment.Provider,
			Type:              appointment.Type,
			AppointmentTypeID: appointment.AppointmentTypeID,
			Facility:          appointment.Facility,
			OfficeID:          appointment.OfficeID,
			Office:            appointment.Office,
		})
	}
	if len(result.Appointments) == 0 {
		result.AppointmentsStatus = AppointmentsNone
		result.Message = "Patient verified, no appointments found"
		return result, nil
	}

	result.AppointmentsStatus = AppointmentsFound
	result.Message = fmt.Sprintf("Patient verified with %d appointment(s)", len(result.Appointments))
	return result, nil
}

func appointmentOfficeIDs(office *domain.OfficeConfig) []string {
	offices := domain.AppointmentLookupOffices(office)
	officeIDs := make([]string, len(offices))
	for i, lookupOffice := range offices {
		officeIDs[i] = lookupOffice.ID
	}
	return officeIDs
}

func applyDemographics(result *ResolveResult, demographics domain.PatientDemographics, office *domain.OfficeConfig, patientDOB string) {
	policy := domain.NewSchedulingPolicy(office)
	result.InsuranceCarrier = demographics.CarrierName
	result.InsPlanID = demographics.InsPlanID
	result.RespPartyID = demographics.RespPartyID

	if demographics.CarrierID == "" {
		return
	}

	result.InsuranceCarrierID = demographics.CarrierID
	routing, ambiguous := domain.RoutingForDemographicInsurance(demographics.CarrierID, demographics.CarrierName, office)
	routing = policy.PatientRouting(routing, patientDOB)
	result.Routing = routing
	result.AllowedProviders = policy.ProviderNames(routing, patientDOB)
	result.RoutingAmbiguous = ambiguous
	if entry, ok := domain.LookupInsuranceForCoverageAtOffice(demographics.CarrierName, domain.InsuranceModeMedical, office); ok {
		result.PreauthRequired = entry.PreauthRequired
	}
	if domain.IsMinor(patientDOB) && routing != domain.RoutingNotAccepted {
		result.RoutingAmbiguous = false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func notFoundMessage(command ResolveCommand) string {
	if command.Phone != "" && command.FirstName == "" && command.DOB == "" {
		return "No patient found for that phone number"
	}
	if command.FirstName != "" {
		return "No patient found matching that first name"
	}
	return "No patient found matching the provided information"
}

func multipleMatchesMessage(command ResolveCommand, count int) string {
	if command.Phone != "" && command.FirstName != "" && command.DOB == "" {
		return fmt.Sprintf("Found %d patients with that name and phone number. Please provide date of birth.", count)
	}
	if command.Phone != "" {
		return fmt.Sprintf("Found %d patients for this phone number. Ask the caller to confirm their name.", count)
	}
	return fmt.Sprintf("Found %d patients with that last name and DOB. Please provide first name.", count)
}
