package advancedmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"advancedmd-token-management/internal/clients"
	"advancedmd-token-management/internal/domain"
	"advancedmd-token-management/internal/safeerrors"
	"advancedmd-token-management/internal/session"
)

func TestAdapterSearchPatientsUsesControlledXMLRPCServer(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Cookie") != "token=test-cookie" {
			t.Fatalf("Cookie = %q", r.Header.Get("Cookie"))
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"PPMDResults": {
				"Results": {
					"patientlist": {
						"@itemcount": "1",
						"patient": {
							"@id": "pat123",
							"@name": "DOE,JANE",
							"@dob": "01/15/1980",
							"contactinfo": {"@cellphone": "850-373-3869"}
						}
					}
				}
			}
		}`))
	}))
	defer server.Close()

	adapter := NewAdapter(
		staticSession{token: &domain.TokenData{
			CookieToken: "token=test-cookie",
			XmlrpcURL:   strings.TrimPrefix(server.URL, "https://"),
		}},
		clients.NewAdvancedMDClient(server.Client()),
		nil,
	)

	got, err := adapter.SearchPatients(context.Background(), domain.PatientSearch{Phone: "9542872010"})
	if err != nil {
		t.Fatalf("SearchPatients() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != "123" || got[0].Phone != "850-373-3869" {
		t.Fatalf("SearchPatients() = %+v", got)
	}
	message, ok := requestBody["ppmdmsg"].(map[string]any)
	if !ok {
		t.Fatalf("request body = %#v", requestBody)
	}
	if message["@action"] != "lookuppatient" || message["@phone"] != "9542872010" {
		t.Fatalf("ppmdmsg = %#v", message)
	}
}

func TestAdapterSearchPatientsByNameUsesControlledXMLRPCServer(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"PPMDResults": {
				"Results": {
					"patientlist": {"@itemcount": "0"}
				}
			}
		}`))
	}))
	defer server.Close()

	adapter := NewAdapter(
		staticSession{token: &domain.TokenData{
			CookieToken: "token=test-cookie",
			XmlrpcURL:   strings.TrimPrefix(server.URL, "https://"),
		}},
		clients.NewAdvancedMDClient(server.Client()),
		nil,
	)
	got, err := adapter.SearchPatients(context.Background(), domain.PatientSearch{
		LastName:  "Doe",
		FirstName: "Jane",
	})
	if err != nil {
		t.Fatalf("SearchPatients() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("SearchPatients() = %+v, want no matches", got)
	}
	message := requestBody["ppmdmsg"].(map[string]any)
	if message["@action"] != "lookuppatient" || message["@name"] != "Doe,Jane" {
		t.Fatalf("ppmdmsg = %#v", message)
	}
}

func TestAdapterDemographicsUsesControlledXMLRPCServer(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"PPMDResults": {
				"Results": {
					"patientlist": {
						"patient": {
							"@id": "pat123",
							"@respparty": "resp456",
							"@dob": "01/15/1980",
							"insplanlist": {
								"insplan": {
									"@id": "ins789",
									"@carrier": "car40906",
									"@subscriber": "resp456",
									"@enddate": "",
									"@coverage": "1"
								}
							}
						}
					},
					"carrierlist": {
						"carrier": {"@id": "car40906", "@name": "HUMANA MEDICARE"}
					}
				}
			}
		}`))
	}))
	defer server.Close()

	adapter := NewAdapter(
		staticSession{token: &domain.TokenData{
			CookieToken: "token=test-cookie",
			XmlrpcURL:   strings.TrimPrefix(server.URL, "https://"),
		}},
		clients.NewAdvancedMDClient(server.Client()),
		nil,
	)
	got, err := adapter.GetPatientDemographics(context.Background(), "123")
	if err != nil {
		t.Fatalf("GetPatientDemographics() error = %v", err)
	}
	want := domain.PatientDemographics{
		CarrierName: "HUMANA MEDICARE",
		CarrierID:   "car40906",
		InsPlanID:   "ins789",
		RespPartyID: "resp456",
		DOB:         "01/15/1980",
	}
	if got != want {
		t.Fatalf("GetPatientDemographics() = %+v, want %+v", got, want)
	}
	message := requestBody["ppmdmsg"].(map[string]any)
	if message["@action"] != "getdemographic" || message["@patientid"] != "123" {
		t.Fatalf("ppmdmsg = %#v", message)
	}
}

func TestAdapterReturnsStableRedactedErrors(t *testing.T) {
	t.Run("session unavailable", func(t *testing.T) {
		adapter := NewAdapter(
			staticSession{err: errors.New("login failed password=secret")},
			clients.NewAdvancedMDClient(http.DefaultClient),
			nil,
		)
		_, err := adapter.SearchPatients(context.Background(), domain.PatientSearch{Phone: "9542872010"})
		if CategoryOf(err) != safeerrors.CategoryUnavailable || err.Error() != "unavailable" {
			t.Fatalf("error = %v category=%q", err, CategoryOf(err))
		}
	})

	t.Run("provider status", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "patientId=123 token=secret", http.StatusServiceUnavailable)
		}))
		defer server.Close()

		adapter := NewAdapter(
			staticSession{token: &domain.TokenData{
				CookieToken: "token=test-cookie",
				XmlrpcURL:   strings.TrimPrefix(server.URL, "https://"),
			}},
			clients.NewAdvancedMDClient(server.Client()),
			nil,
		)
		_, err := adapter.SearchPatients(context.Background(), domain.PatientSearch{Phone: "9542872010"})
		if CategoryOf(err) != safeerrors.CategoryUpstreamStatus || err.Error() != "upstream_status" {
			t.Fatalf("error = %v category=%q", err, CategoryOf(err))
		}
		if strings.Contains(err.Error(), "patientId") || strings.Contains(err.Error(), "token") {
			t.Fatalf("error exposed provider details: %v", err)
		}
	})
}

func TestAdapterUpcomingAppointmentsUsesControlledRESTServer(t *testing.T) {
	domain.InitRegistry("")
	fixedNow := time.Date(2026, time.July, 25, 10, 30, 0, 0, time.FixedZone("EDT", -4*60*60))
	var requestedColumns map[string]int = make(map[string]int)
	var mu sync.Mutex

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/scheduler/appointments" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if r.URL.Query().Get("forView") != "month" || r.URL.Query().Get("isLegacy") != "true" {
			t.Fatalf("query = %q", r.URL.RawQuery)
		}
		columns := r.URL.Query().Get("columnId")
		mu.Lock()
		requestedColumns[columns]++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("startDate") == "2026-07-01" && strings.Contains(columns, "1513") {
			w.Write([]byte(`[
				{
					"id": 111,
					"startdatetime": "2026-07-20T09:00:00",
					"columnid": 1513,
					"patientid": 123
				},
				{
					"id": 222,
					"startdatetime": "2026-07-30T09:00:00",
					"columnid": 1513,
					"patientid": 999
				}
			]`))
			return
		}
		if r.URL.Query().Get("startDate") != "2026-08-01" {
			w.Write([]byte(`[]`))
			return
		}
		switch {
		case strings.Contains(columns, "1513"):
			w.Write([]byte(`[{
				"id": 9570263,
				"startdatetime": "2026-08-14T12:00:00",
				"columnid": 1513,
				"provider": "BACH, AUSTIN",
				"facility": "ABITA EYE GROUP SPRING HILL",
				"appointmenttypeids": [1007],
				"patientid": 123
			}]`))
		case strings.Contains(columns, "1593"):
			w.Write([]byte(`[{
				"id": 9570264,
				"startdatetime": "2026-08-15T09:00:00",
				"columnid": 1593,
				"provider": "BACH, AUSTIN",
				"facility": "ABITA EYE GROUP CRYSTAL RIVER",
				"appointmenttypeids": [6169],
				"patientid": 123
			}]`))
		default:
			w.Write([]byte(`[]`))
		}
	}))
	defer server.Close()

	adapter := NewAdapter(
		staticSession{token: &domain.TokenData{
			Token:       "Bearer test-token",
			RestApiBase: strings.TrimPrefix(server.URL, "https://"),
		}},
		nil,
		clients.NewAdvancedMDRestClient(server.Client()),
		func() time.Time { return fixedNow },
	)

	got, err := adapter.GetUpcomingAppointments(
		context.Background(),
		domain.PatientAppointmentsQuery{
			PatientID: "123",
			OfficeIDs: []string{"spring_hill", "crystal_river"},
		},
	)
	if err != nil {
		t.Fatalf("GetUpcomingAppointments() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetUpcomingAppointments() = %+v, want two", got)
	}
	if got[0].OfficeID != "spring_hill" || got[0].Provider != "Dr. Austin Bach" ||
		got[0].Type != "Established Adult Medical (Follow Up)" {
		t.Fatalf("Spring Hill appointment = %+v", got[0])
	}
	if got[1].OfficeID != "crystal_river" || got[1].Office != "Crystal River" ||
		got[1].Type != "Crystal River Established Patient" {
		t.Fatalf("Crystal River appointment = %+v", got[1])
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requestedColumns) != 2 {
		t.Fatalf("requested column groups = %#v, want two nearby offices", requestedColumns)
	}
	for columns, count := range requestedColumns {
		if count != 6 {
			t.Fatalf("requests for %q = %d, want six months", columns, count)
		}
	}
}

func TestAdapterCanonicalizesDevelopmentAppointmentTypeIDs(t *testing.T) {
	domain.InitRegistry("dev")
	t.Cleanup(func() { domain.InitRegistry("") })
	office := domain.DefaultOffice()
	fixedNow := time.Date(2026, time.July, 25, 10, 30, 0, 0, time.FixedZone("EDT", -4*60*60))

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("startDate") != "2026-08-01" ||
			!strings.Contains(r.URL.Query().Get("columnId"), "1716") {
			w.Write([]byte(`[]`))
			return
		}
		w.Write([]byte(`[{
			"id": 22222,
			"startdatetime": "2026-08-14T12:00:00",
			"patientid": 12345,
			"provider": "BACH, AUSTIN",
			"appointmenttypeids": [18]
		}]`))
	}))
	defer server.Close()

	adapter := NewAdapter(
		staticSession{token: &domain.TokenData{
			Token:       "Bearer test-token",
			RestApiBase: strings.TrimPrefix(server.URL, "https://"),
		}},
		nil,
		clients.NewAdvancedMDRestClient(server.Client()),
		func() time.Time { return fixedNow },
	)
	got, err := adapter.GetUpcomingAppointments(context.Background(), domain.PatientAppointmentsQuery{
		PatientID: "12345",
		OfficeIDs: []string{office.ID},
	})
	if err != nil {
		t.Fatalf("GetUpcomingAppointments() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("appointments = %+v, want one", got)
	}
	if got[0].AppointmentTypeID != 1007 || got[0].Type != "Established Adult Medical (Follow Up)" {
		t.Fatalf("appointment type = %d/%q", got[0].AppointmentTypeID, got[0].Type)
	}
}

type staticSession struct {
	token *domain.TokenData
	err   error
}

func (s staticSession) Get(context.Context) (*domain.TokenData, error) {
	return s.token, s.err
}

func (staticSession) Maintain(context.Context) error {
	return nil
}

func (staticSession) Status() session.SessionStatus {
	return session.SessionStatus{}
}
