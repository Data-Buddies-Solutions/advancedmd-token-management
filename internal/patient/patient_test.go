package patient_test

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
	"time"

	"advancedmd-token-management/internal/advancedmd"
	"advancedmd-token-management/internal/advancedmd/advancedmdtest"
	"advancedmd-token-management/internal/domain"
	"advancedmd-token-management/internal/patient"
	"advancedmd-token-management/internal/safeerrors"
)

func TestResolveReturnsCompletePatientForPhoneLookup(t *testing.T) {
	domain.InitRegistry("")
	office, ok := domain.LookupOffice("Spring Hill")
	if !ok {
		t.Fatal("Spring Hill office is not configured")
	}

	search := domain.PatientSearch{Phone: "9542872010"}
	amd := advancedmdtest.NewAdapter()
	amd.PatientSearches[search] = []domain.Patient{{
		ID:        "123",
		FirstName: "JANE",
		LastName:  "DOE",
		FullName:  "DOE,JANE",
		DOB:       "01/15/1980",
		Phone:     "850-373-3869",
	}}
	amd.Demographics["123"] = domain.PatientDemographics{
		CarrierName: "HUMANA MEDICARE",
		CarrierID:   "car40906",
		InsPlanID:   "ins789",
		RespPartyID: "resp456",
		DOB:         "01/15/1980",
	}
	amd.Appointments["123"] = []domain.PatientAppointment{{
		ID:                9570263,
		Start:             time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC),
		Provider:          "Dr. Austin Bach",
		Type:              "Established Adult Medical (Follow Up)",
		AppointmentTypeID: 1007,
		Facility:          "Abita Eye Group Spring Hill",
		OfficeID:          "spring_hill",
		Office:            "Spring Hill",
	}}

	resolver := patient.New(amd)
	got, err := resolver.Resolve(context.Background(), patient.ResolveCommand{
		Phone:    "(954) 287-2010",
		OfficeID: office.ID,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	want := patient.ResolveResult{
		Status:             patient.StatusVerified,
		PatientID:          "123",
		Name:               "DOE,JANE",
		DOB:                "01/15/1980",
		Phone:              "850-373-3869",
		InsuranceCarrier:   "HUMANA MEDICARE",
		InsuranceCarrierID: "car40906",
		InsPlanID:          "ins789",
		RespPartyID:        "resp456",
		Routing:            domain.RoutingBachOnly,
		AllowedProviders:   []string{"Dr. Bach"},
		AppointmentsStatus: patient.AppointmentsFound,
		Appointments: []patient.Appointment{{
			ID:                9570263,
			Date:              "Wednesday, March 18, 2026",
			Time:              "12:00 PM",
			Provider:          "Dr. Austin Bach",
			Type:              "Established Adult Medical (Follow Up)",
			AppointmentTypeID: 1007,
			Facility:          "Abita Eye Group Spring Hill",
			OfficeID:          "spring_hill",
			Office:            "Spring Hill",
		}},
		Message: "Patient verified with 1 appointment(s)",
	}
	assertResolveResult(t, got, want)

	var _ advancedmd.PatientRecords = amd
}

func TestResolveReturnsCompleteResultsForMultipleMatches(t *testing.T) {
	domain.InitRegistry("")
	office, _ := domain.LookupOffice("Spring Hill")

	search := domain.PatientSearch{Phone: "5552223333"}
	amd := advancedmdtest.NewAdapter()
	amd.PatientSearches[search] = []domain.Patient{
		{ID: "123", FirstName: "JANE", FullName: "DOE,JANE", DOB: "01/15/1980", Phone: "5552223333"},
		{ID: "456", FirstName: "JOHN", FullName: "DOE,JOHN", DOB: "03/20/1982", Phone: "5552223333"},
	}
	amd.Demographics["123"] = domain.PatientDemographics{CarrierName: "HUMANA MEDICARE", CarrierID: "car40906"}
	amd.Demographics["456"] = domain.PatientDemographics{CarrierName: "AETNA", CarrierID: "car40887"}
	amd.Appointments["123"] = []domain.PatientAppointment{{
		ID:       9570263,
		Start:    time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC),
		OfficeID: "spring_hill",
		Office:   "Spring Hill",
	}}

	got, err := patient.New(amd).Resolve(context.Background(), patient.ResolveCommand{
		Phone:    "5552223333",
		OfficeID: office.ID,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got.Status != patient.StatusMultipleMatches {
		t.Fatalf("Status = %q, want %q", got.Status, patient.StatusMultipleMatches)
	}
	if got.Message != "Found 2 patients for this phone number. Ask the caller to confirm their name." {
		t.Fatalf("Message = %q", got.Message)
	}
	if got.Appointments == nil || len(got.Appointments) != 0 {
		t.Fatalf("top-level Appointments = %#v, want non-nil empty slice", got.Appointments)
	}
	if len(got.Matches) != 2 {
		t.Fatalf("Matches = %+v, want two complete results", got.Matches)
	}
	if got.Matches[0].PatientID != "123" || got.Matches[0].AppointmentsStatus != patient.AppointmentsFound {
		t.Fatalf("first match = %+v", got.Matches[0])
	}
	if got.Matches[1].PatientID != "456" || got.Matches[1].AppointmentsStatus != patient.AppointmentsNone {
		t.Fatalf("second match = %+v", got.Matches[1])
	}
}

func TestResolveSelectsPatientByVerifiedDemographics(t *testing.T) {
	domain.InitRegistry("")
	office, _ := domain.LookupOffice("Hollywood")
	candidates := []domain.Patient{
		{ID: "123", FirstName: "JANE", LastName: "DOE", FullName: "DOE,JANE", DOB: "01/15/1980"},
		{ID: "456", FirstName: "JANET", LastName: "DOE", FullName: "DOE,JANET", DOB: "03/20/1982"},
	}

	tests := []struct {
		name    string
		command patient.ResolveCommand
		search  domain.PatientSearch
		wantID  string
	}{
		{
			name:    "phone and first name",
			command: patient.ResolveCommand{Phone: "555-222-3333", FirstName: "Janet", OfficeID: office.ID},
			search:  domain.PatientSearch{Phone: "5552223333"},
			wantID:  "456",
		},
		{
			name:    "phone and DOB",
			command: patient.ResolveCommand{Phone: "555-222-3333", DOB: "1/15/1980", OfficeID: office.ID},
			search:  domain.PatientSearch{Phone: "5552223333"},
			wantID:  "123",
		},
		{
			name:    "phone first name and DOB",
			command: patient.ResolveCommand{Phone: "555-222-3333", FirstName: "Jane", DOB: "01-15-1980", OfficeID: office.ID},
			search:  domain.PatientSearch{Phone: "5552223333"},
			wantID:  "123",
		},
		{
			name:    "last name and DOB",
			command: patient.ResolveCommand{LastName: "Döe", DOB: "03/20/1982", OfficeID: office.ID},
			search:  domain.PatientSearch{LastName: "Doe"},
			wantID:  "456",
		},
		{
			name:    "last name first name and DOB",
			command: patient.ResolveCommand{LastName: "Döe", FirstName: "Ja", DOB: "01/15/1980", OfficeID: office.ID},
			search:  domain.PatientSearch{LastName: "Doe", FirstName: "Ja"},
			wantID:  "123",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			amd := advancedmdtest.NewAdapter()
			amd.PatientSearches[test.search] = candidates

			got, err := patient.New(amd).Resolve(context.Background(), test.command)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if got.Status != patient.StatusVerified || got.PatientID != test.wantID {
				t.Fatalf("Resolve() = %+v, want verified patient %s", got, test.wantID)
			}
		})
	}
}

func TestResolveReturnsNoMatchWithoutHydratingPatientData(t *testing.T) {
	domain.InitRegistry("")
	office, _ := domain.LookupOffice("Spring Hill")
	amd := advancedmdtest.NewAdapter()

	got, err := patient.New(amd).Resolve(context.Background(), patient.ResolveCommand{
		Phone:    "9542872010",
		OfficeID: office.ID,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Status != patient.StatusNotFound {
		t.Fatalf("Status = %q, want not_found", got.Status)
	}
	if got.Message != "No patient found for that phone number" {
		t.Fatalf("Message = %q", got.Message)
	}
	if got.Appointments == nil || len(got.Appointments) != 0 {
		t.Fatalf("Appointments = %#v, want non-nil empty slice", got.Appointments)
	}
}

func TestResolveRefreshesKnownPatientByID(t *testing.T) {
	domain.InitRegistry("")
	office, _ := domain.LookupOffice("Spring Hill")
	amd := advancedmdtest.NewAdapter()
	amd.Demographics["123"] = domain.PatientDemographics{
		CarrierName: "HUMANA MEDICARE",
		CarrierID:   "car40906",
		DOB:         "01/15/1980",
	}
	amd.Appointments["123"] = []domain.PatientAppointment{{
		ID:       9570263,
		Start:    time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC),
		OfficeID: "spring_hill",
		Office:   "Spring Hill",
	}}

	got, err := patient.New(amd).Resolve(context.Background(), patient.ResolveCommand{
		PatientID: "123",
		OfficeID:  office.ID,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Status != patient.StatusVerified || got.PatientID != "123" {
		t.Fatalf("Resolve() = %+v, want verified patient 123", got)
	}
	if got.DOB != "01/15/1980" || got.Routing != domain.RoutingBachOnly {
		t.Fatalf("demographics = DOB %q routing %q", got.DOB, got.Routing)
	}
	if got.AppointmentsStatus != patient.AppointmentsFound || len(got.Appointments) != 1 {
		t.Fatalf("appointments = %q %+v", got.AppointmentsStatus, got.Appointments)
	}
}

func TestResolveKeepsVerifiedPatientWhenAppointmentsFail(t *testing.T) {
	domain.InitRegistry("")
	office, _ := domain.LookupOffice("Spring Hill")
	amd := advancedmdtest.NewAdapter()
	amd.PatientSearches[domain.PatientSearch{Phone: "9542872010"}] = []domain.Patient{{
		ID: "123", FullName: "DOE,JANE", DOB: "01/15/1980",
	}}
	amd.AppointmentErrors["123"] = advancedmd.NewError(safeerrors.CategoryUpstreamStatus)

	var logs bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousWriter) })

	got, err := patient.New(amd).Resolve(context.Background(), patient.ResolveCommand{
		Phone:    "9542872010",
		OfficeID: office.ID,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Status != patient.StatusVerified {
		t.Fatalf("Status = %q, want verified", got.Status)
	}
	if got.AppointmentsStatus != patient.AppointmentsError {
		t.Fatalf("AppointmentsStatus = %q, want error", got.AppointmentsStatus)
	}
	if got.AppointmentsMessage != "Failed to retrieve appointments from AdvancedMD. Please try again." {
		t.Fatalf("AppointmentsMessage = %q", got.AppointmentsMessage)
	}
	if got.Message != "Patient verified, appointment lookup unavailable" {
		t.Fatalf("Message = %q", got.Message)
	}
	if got.Appointments == nil || len(got.Appointments) != 0 {
		t.Fatalf("Appointments = %#v, want non-nil empty slice", got.Appointments)
	}
	for _, forbidden := range []string{"patientId=123", "token=secret", "status 500"} {
		if strings.Contains(logs.String(), forbidden) {
			t.Fatalf("logs exposed %q: %s", forbidden, logs.String())
		}
	}
	if !strings.Contains(logs.String(), "patient-resolve: failed to get appointments category=upstream_status") {
		t.Fatalf("logs = %q, want safe classified failure", logs.String())
	}
}

func TestResolveKnownPatientReturnsUnavailableWhenAdvancedMDCannotAuthenticate(t *testing.T) {
	domain.InitRegistry("")
	office, _ := domain.LookupOffice("Spring Hill")
	amd := advancedmdtest.NewAdapter()
	amd.DemographicErrors["123"] = advancedmd.NewError(safeerrors.CategoryUnavailable)

	_, err := patient.New(amd).Resolve(context.Background(), patient.ResolveCommand{
		PatientID: "123",
		OfficeID:  office.ID,
	})
	if err == nil {
		t.Fatal("Resolve() error = nil, want unavailable")
	}
	if advancedmd.CategoryOf(err) != safeerrors.CategoryUnavailable {
		t.Fatalf("Resolve() error category = %q", advancedmd.CategoryOf(err))
	}
	if err.Error() != "unavailable" {
		t.Fatalf("Resolve() error = %q, want redacted category", err)
	}
}

func TestResolveKeepsVerifiedPatientWhenDemographicsProviderReadFails(t *testing.T) {
	domain.InitRegistry("")
	office, _ := domain.LookupOffice("Spring Hill")
	amd := advancedmdtest.NewAdapter()
	amd.DemographicErrors["123"] = advancedmd.NewError(safeerrors.CategoryAuthentication)

	got, err := patient.New(amd).Resolve(context.Background(), patient.ResolveCommand{
		PatientID: "123",
		OfficeID:  office.ID,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Status != patient.StatusVerified || got.PatientID != "123" {
		t.Fatalf("Resolve() = %+v, want verified patient 123", got)
	}
	if got.AppointmentsStatus != patient.AppointmentsNone {
		t.Fatalf("AppointmentsStatus = %q, want none", got.AppointmentsStatus)
	}
}

func TestResolveAppliesPreauthorizationAndPediatricProviderPolicy(t *testing.T) {
	domain.InitRegistry("")
	office, _ := domain.LookupOffice("Spring Hill")

	tests := []struct {
		name             string
		demographics     domain.PatientDemographics
		patientDOB       string
		wantRouting      domain.RoutingRule
		wantPreauth      bool
		wantAmbiguous    bool
		wantProviderList bool
	}{
		{
			name: "preauthorization carrier",
			demographics: domain.PatientDemographics{
				CarrierName: "AETNA HMO",
				CarrierID:   "car40907",
			},
			patientDOB:       "01/01/1980",
			wantRouting:      domain.RoutingAll,
			wantPreauth:      true,
			wantAmbiguous:    false,
			wantProviderList: true,
		},
		{
			name: "minor uses pediatric routing",
			demographics: domain.PatientDemographics{
				CarrierName: "AETNA",
				CarrierID:   "car40887",
			},
			patientDOB:       "01/01/2015",
			wantRouting:      office.PediatricRouting,
			wantPreauth:      false,
			wantAmbiguous:    false,
			wantProviderList: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			amd := advancedmdtest.NewAdapter()
			amd.PatientSearches[domain.PatientSearch{Phone: "9542872010"}] = []domain.Patient{{
				ID: "123", FullName: "DOE,JANE", DOB: test.patientDOB,
			}}
			amd.Demographics["123"] = test.demographics

			got, err := patient.New(amd).Resolve(context.Background(), patient.ResolveCommand{
				Phone:    "9542872010",
				OfficeID: office.ID,
			})
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if got.Routing != test.wantRouting ||
				got.PreauthRequired != test.wantPreauth ||
				got.RoutingAmbiguous != test.wantAmbiguous {
				t.Fatalf("policy result = %+v", got)
			}
			if (len(got.AllowedProviders) > 0) != test.wantProviderList {
				t.Fatalf("AllowedProviders = %v", got.AllowedProviders)
			}
		})
	}
}

func assertResolveResult(t *testing.T, got, want patient.ResolveResult) {
	t.Helper()
	if got.Status != want.Status ||
		got.PatientID != want.PatientID ||
		got.Name != want.Name ||
		got.DOB != want.DOB ||
		got.Phone != want.Phone ||
		got.InsuranceCarrier != want.InsuranceCarrier ||
		got.InsuranceCarrierID != want.InsuranceCarrierID ||
		got.InsPlanID != want.InsPlanID ||
		got.RespPartyID != want.RespPartyID ||
		got.Routing != want.Routing ||
		got.RoutingAmbiguous != want.RoutingAmbiguous ||
		got.PreauthRequired != want.PreauthRequired ||
		got.AppointmentsStatus != want.AppointmentsStatus ||
		got.AppointmentsMessage != want.AppointmentsMessage ||
		got.Message != want.Message {
		t.Fatalf("Resolve() = %+v, want %+v", got, want)
	}
	if len(got.AllowedProviders) != len(want.AllowedProviders) {
		t.Fatalf("AllowedProviders = %v, want %v", got.AllowedProviders, want.AllowedProviders)
	}
	for i := range want.AllowedProviders {
		if got.AllowedProviders[i] != want.AllowedProviders[i] {
			t.Fatalf("AllowedProviders = %v, want %v", got.AllowedProviders, want.AllowedProviders)
		}
	}
	if len(got.Appointments) != len(want.Appointments) {
		t.Fatalf("Appointments = %+v, want %+v", got.Appointments, want.Appointments)
	}
	for i := range want.Appointments {
		if got.Appointments[i] != want.Appointments[i] {
			t.Fatalf("Appointments = %+v, want %+v", got.Appointments, want.Appointments)
		}
	}
}
