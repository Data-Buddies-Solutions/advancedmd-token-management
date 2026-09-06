package scheduling_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"advancedmd-token-management/internal/advancedmd"
	"advancedmd-token-management/internal/clients"
	"advancedmd-token-management/internal/domain"
	"advancedmd-token-management/internal/safeerrors"
	"advancedmd-token-management/internal/scheduling"
	"advancedmd-token-management/internal/session"
)

type providerTransport func(*http.Request) (*http.Response, error)

func (f providerTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func providerResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

type providerSession struct{}

func (providerSession) Get(context.Context) (*domain.TokenData, error) {
	return &domain.TokenData{RestApiBase: "provider.test", XmlrpcURL: "provider.test/xmlrpc"}, nil
}
func (providerSession) Maintain(context.Context) error { return nil }
func (providerSession) Status() session.SessionStatus {
	return session.SessionStatus{State: session.SessionFresh}
}
func providerAdapter(transport providerTransport) *advancedmd.Adapter {
	client := &http.Client{Transport: transport}
	return advancedmd.NewAdapter(providerSession{}, clients.NewAdvancedMDClient(client), clients.NewAdvancedMDRestClient(client))
}

// Use real occupancy parsing and writes with deterministic patient/setup reads.
type providerSchedulingRecords struct{ *advancedmd.Adapter }

func (providerSchedulingRecords) GetSchedulerSetup(context.Context) (domain.SchedulerSetup, error) {
	return bookingRecords().SchedulerSetup, nil
}
func (providerSchedulingRecords) GetPatientDemographics(context.Context, string) (domain.PatientDemographics, error) {
	return domain.PatientDemographics{DOB: "01/15/1980"}, nil
}

func TestMalformedOccupancyCannotOfferOrBook(t *testing.T) {
	for _, operation := range []string{"appointments", "blockholds"} {
		t.Run(operation, func(t *testing.T) {
			var writes atomic.Int32
			records := providerSchedulingRecords{providerAdapter(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodGet {
					writes.Add(1)
					return providerResponse(200, `{"id":98765}`), nil
				}
				if strings.HasSuffix(r.URL.Path, operation) {
					return providerResponse(200, `[{"id":1,"startdatetime":"malformed","duration":15,"columnid":1513}]`), nil
				}
				return providerResponse(200, `[]`), nil
			})}
			scheduler := scheduling.New(records, "test-booking-secret", mutationTestNow)
			result, err := scheduler.List(context.Background(), scheduling.ListCommand{Office: "Spring Hill", Routing: "bach_only"})
			if err == nil || len(result.Slots) != 0 || scheduling.ProviderFailureOf(err) != safeerrors.CategoryInvalidResponse {
				t.Errorf("List: err=%v slots=%d provider=%s", err, len(result.Slots), scheduling.ProviderFailureOf(err))
			}
			_, err = scheduler.Book(context.Background(), signedBookCommand(t, mutationTestNow()))
			if err == nil || writes.Load() != 0 {
				t.Fatalf("Book: err=%v writes=%d", err, writes.Load())
			}
		})
	}
}

func TestInventoryStopsAfterProviderFailure(t *testing.T) {
	for _, status := range []int{401, 429, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			records := providerSchedulingRecords{providerAdapter(func(r *http.Request) (*http.Response, error) {
				calls.Add(1)
				response := providerResponse(status, `{"error":"unavailable"}`)
				response.Header.Set("Retry-After", "60")
				return response, nil
			})}
			result, err := scheduling.New(records, "test-booking-secret", mutationTestNow).List(context.Background(), scheduling.ListCommand{Office: "Spring Hill", Routing: "bach_only", RangeDays: 90})
			// Four day workers, two reads per column. No new date once a day fails.
			if err == nil || len(result.Slots) != 0 || calls.Load() > 8 {
				t.Fatalf("err=%v slots=%d provider calls=%d, want failure within first wave", err, len(result.Slots), calls.Load())
			}
			want := map[int]safeerrors.Category{401: safeerrors.CategoryAuthentication, 429: safeerrors.CategoryRateLimited, 503: safeerrors.CategoryUpstreamStatus}[status]
			if got := scheduling.ProviderFailureOf(err); got != want {
				t.Fatalf("lost original provider failure: %s", got)
			}
			t.Logf("HTTP %d: %d provider calls, category=%s", status, calls.Load(), scheduling.ProviderFailureOf(err))
		})
	}
}

func TestHealthyInventoryStillReadsAllFourteenDays(t *testing.T) {
	var reads atomic.Int32
	records := providerSchedulingRecords{providerAdapter(func(r *http.Request) (*http.Response, error) {
		reads.Add(1)
		return providerResponse(200, `[]`), nil
	})}
	result, err := scheduling.New(records, "test-booking-secret", mutationTestNow).List(context.Background(), scheduling.ListCommand{Office: "Spring Hill", Routing: "bach_only"})
	if err != nil || len(result.Slots) != 14 || reads.Load() != 28 || result.SearchedFrom != "2026-06-02" || result.SearchedThrough != "2026-06-15" {
		t.Fatalf("err=%v slots=%d reads=%d coverage=%s..%s", err, len(result.Slots), reads.Load(), result.SearchedFrom, result.SearchedThrough)
	}
}

func TestScheduleFailureCancelsSlowSibling(t *testing.T) {
	started := make(chan struct{})
	records := providerAdapter(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "blockholds") {
			close(started)
			<-r.Context().Done()
			return nil, r.Context().Err()
		}
		<-started
		return providerResponse(503, `{}`), nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := records.ReadSchedule(ctx, domain.ScheduleReadQuery{Date: "2026-06-03", ColumnIDs: []string{"1513"}})
	if ctx.Err() != nil || advancedmd.CategoryOf(err) != safeerrors.CategoryUpstreamStatus {
		t.Fatalf("parent=%v err=%v", ctx.Err(), err)
	}
}

func TestInventoryDeadlineStopsSlowProviderReads(t *testing.T) {
	var active atomic.Int32
	records := providerSchedulingRecords{providerAdapter(func(r *http.Request) (*http.Response, error) {
		active.Add(1)
		defer active.Add(-1)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	result, err := scheduling.New(records, "test-booking-secret", mutationTestNow).List(ctx, scheduling.ListCommand{Office: "Spring Hill", Routing: "bach_only"})
	if err == nil || len(result.Slots) != 0 || active.Load() != 0 || scheduling.ProviderFailureOf(err) != safeerrors.CategoryTimeout {
		t.Fatalf("err=%v active=%d slots=%d provider=%s", err, active.Load(), len(result.Slots), scheduling.ProviderFailureOf(err))
	}
}

func TestProviderSetupErrorIsNotCachedAsEmptyInventory(t *testing.T) {
	var calls atomic.Int32
	records := providerAdapter(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return providerResponse(200, `{"PPMDResults":{"Error":{"Fault":{"detail":{"code":"401","description":"login failed"}}}}}`), nil
	})
	scheduler := scheduling.New(records, "test-secret", mutationTestNow)
	for range 2 {
		result, err := scheduler.List(context.Background(), scheduling.ListCommand{Office: "Spring Hill", Routing: "bach_only"})
		if err == nil || result.Outcome == domain.AvailabilityOutcomeNoEligibleProviders {
			t.Fatalf("err=%v outcome=%s", err, result.Outcome)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("setup requests=%d, error must not enter cache", calls.Load())
	}
}

type waitingSetupRecords struct {
	advancedmd.SchedulingRecords
	started chan struct{}
	release chan struct{}
}

func (r waitingSetupRecords) GetSchedulerSetup(ctx context.Context) (domain.SchedulerSetup, error) {
	close(r.started)
	<-r.release
	return domain.SchedulerSetup{}, advancedmd.NewError(safeerrors.CategoryUnavailable)
}
func TestCanceledSearchDoesNotWaitForSetupRefresh(t *testing.T) {
	records := waitingSetupRecords{started: make(chan struct{}), release: make(chan struct{})}
	scheduler := scheduling.New(records, "test-secret", mutationTestNow)
	first := make(chan struct{})
	go func() {
		defer close(first)
		scheduler.List(context.Background(), scheduling.ListCommand{Office: "Spring Hill"})
	}()
	<-records.started
	defer func() { close(records.release); <-first }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := make(chan error, 1)
	go func() { _, err := scheduler.List(ctx, scheduling.ListCommand{Office: "Spring Hill"}); result <- err }()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("expected canceled request")
		}
	case <-time.After(time.Second):
		t.Fatal("canceled search stuck behind setup refresh")
	}
}

type reconciliationRecords struct {
	providerSchedulingRecords
	deadline    time.Time
	unavailable bool
	t           *testing.T
}

func (r reconciliationRecords) ReadPatientAppointmentsForMonth(ctx context.Context, query advancedmd.AppointmentMonthQuery) (advancedmd.AppointmentRead, error) {
	deadline, ok := ctx.Deadline()
	if ctx.Err() != nil || !ok || !deadline.Equal(r.deadline) {
		r.t.Errorf("reconciliation lost original live context: err=%v deadline=%v", ctx.Err(), deadline)
	}
	if r.unavailable {
		return advancedmd.AppointmentRead{}, advancedmd.NewError(safeerrors.CategoryUnavailable)
	}
	return advancedmd.AppointmentRead{Complete: true, Appointments: []domain.PatientAppointment{matchingAppointment(98765)}}, nil
}

func TestBookingTimeoutReservesReconciliationWithoutRetry(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		t.Run(map[bool]string{false: "verified", true: "indeterminate"}[unavailable], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second+100*time.Millisecond)
			defer cancel()
			deadline, _ := ctx.Deadline()
			var writes atomic.Int32
			records := reconciliationRecords{deadline: deadline, unavailable: unavailable, t: t}
			records.providerSchedulingRecords = providerSchedulingRecords{providerAdapter(func(r *http.Request) (*http.Response, error) {
				if r.Method == http.MethodGet {
					return providerResponse(200, `[]`), nil
				}
				writes.Add(1)
				mutationDeadline, ok := r.Context().Deadline()
				if !ok || deadline.Sub(mutationDeadline) != 5*time.Second {
					t.Errorf("mutation deadline=%v, workflow deadline=%v", mutationDeadline, deadline)
				}
				<-r.Context().Done()
				return nil, r.Context().Err()
			})}
			receipt, err := scheduling.New(records, "test-booking-secret", mutationTestNow).Book(ctx, signedBookCommand(t, mutationTestNow()))
			if writes.Load() != 1 {
				t.Fatalf("writes=%d, want exactly one attempt", writes.Load())
			}
			if unavailable {
				if scheduling.CategoryOf(err) != scheduling.CategoryIndeterminateWrite {
					t.Fatalf("err=%v, want indeterminate_write", err)
				}
			} else if err != nil || receipt.AppointmentID != 98765 {
				t.Fatalf("receipt=%+v err=%v", receipt, err)
			}
		})
	}
}
