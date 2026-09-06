package clients

import (
	"context"
	"net/http"
	"testing"

	"advancedmd-token-management/internal/safeerrors"
)

func TestSchedulerSetupRejectsMissingAndMalformedResults(t *testing.T) {
	for name, body := range map[string]string{
		"missing envelope": `{}`,
		"null results":     `{"PPMDResults":{"Results":null}}`,
		"missing lists":    `{"PPMDResults":{"Results":{}}}`,
		"invalid list":     `{"PPMDResults":{"Results":{"columnlist":{"column":false},"profilelist":{},"facilitylist":{}}}}`,
		"invalid row":      `{"PPMDResults":{"Results":{"columnlist":{"column":[{}]},"profilelist":{},"facilitylist":{}}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			client, token, cleanup := newTestXMLRPCClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(body)) }))
			defer cleanup()
			setup, err := client.GetSchedulerSetup(context.Background(), token)
			if err == nil || setup != nil || safeerrors.Classify(err) != safeerrors.CategoryInvalidResponse {
				t.Fatalf("setup=%v err=%v", setup, err)
			}
		})
	}
}

func TestSchedulerSetupAcceptsExplicitEmptyLists(t *testing.T) {
	client, token, cleanup := newTestXMLRPCClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"PPMDResults":{"Results":{"columnlist":{"column":[]},"profilelist":{"profile":[]},"facilitylist":{"facility":[]}},"Error":{}}}`))
	}))
	defer cleanup()
	setup, err := client.GetSchedulerSetup(context.Background(), token)
	if err != nil || setup == nil || len(setup.Columns) != 0 {
		t.Fatalf("setup=%v err=%v", setup, err)
	}
}

func TestRESTNullCannotProveEmptySchedule(t *testing.T) {
	client, token, cleanup := newTestRestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(`null`)) }))
	defer cleanup()
	reads := map[string]func() error{
		"appointments": func() error {
			_, err := client.GetAppointments(context.Background(), token, "1513", "2026-06-03")
			return err
		},
		"holds": func() error {
			_, err := client.GetBlockHolds(context.Background(), token, "1513", "2026-06-03")
			return err
		},
		"monthly": func() error {
			_, err := client.GetAppointmentsByMonth(context.Background(), token, "1513", "2026-06-01")
			return err
		},
	}
	for name, read := range reads {
		t.Run(name, func(t *testing.T) {
			if err := read(); safeerrors.Classify(err) != safeerrors.CategoryInvalidResponse {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestMalformedOccupancyDurationIsRejected(t *testing.T) {
	for name, body := range map[string]string{
		"zero appointment duration": `[{"id":1,"startdatetime":"2026-06-03T09:00:00","duration":0}]`,
		"backward hold":             `[{"id":1,"startdatetime":"2026-06-03T09:00:00","enddatetime":"2026-06-03T08:00:00"}]`,
	} {
		t.Run(name, func(t *testing.T) {
			client, token, cleanup := newTestRestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(body)) }))
			defer cleanup()
			var err error
			if name == "backward hold" {
				_, err = client.GetBlockHolds(context.Background(), token, "1513", "2026-06-03")
			} else {
				_, err = client.GetAppointments(context.Background(), token, "1513", "2026-06-03")
			}
			if safeerrors.Classify(err) != safeerrors.CategoryInvalidResponse {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
