package scheduling_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"advancedmd-token-management/internal/domain"
	"advancedmd-token-management/internal/scheduling"
)

func TestSearchKeepsBeforeAndAfterWindowsDistinct(t *testing.T) {
	now := time.Date(2026, 6, 8, 14, 0, 0, 0, time.UTC)
	records := recordsWithSetup(testColumn("1513", "620", "1568", "09:00", "17:00", 60))
	records.ScheduleReads["2026-06-09"] = completeRead("1513", nil, nil)

	search := func(window scheduling.AvailabilityWindow) []string {
		t.Helper()
		result, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
			Search(context.Background(), scheduling.SearchCommand{
				Windows:  []scheduling.AvailabilityWindow{window},
				TimeZone: "America/New_York",
				Office:   "Spring Hill",
				Routing:  string(domain.RoutingBachOnly),
			})
		if err != nil {
			t.Fatalf("Search error = %v", err)
		}
		if result.MatchStatus != domain.AvailabilityMatchExact {
			t.Fatalf("match status = %q, want exact", result.MatchStatus)
		}
		got := make([]string, 0, len(result.Slots))
		for _, slot := range result.Slots {
			got = append(got, slot.DateTime)
		}
		return got
	}

	before := search(scheduling.AvailabilityWindow{
		Start: "2026-06-09T00:00:00-04:00",
		End:   "2026-06-09T12:00:00-04:00",
	})
	after := search(scheduling.AvailabilityWindow{
		Start: "2026-06-09T14:00:00-04:00",
		End:   "2026-06-10T00:00:00-04:00",
	})

	if !slices.Equal(before, []string{"2026-06-09T09:00", "2026-06-09T10:00"}) {
		t.Fatalf("before slots = %v", before)
	}
	if !slices.Equal(after, []string{"2026-06-09T14:00", "2026-06-09T15:00"}) {
		t.Fatalf("after slots = %v", after)
	}
}

func TestSearchCoversCompoundExactWindows(t *testing.T) {
	now := time.Date(2026, 6, 8, 14, 0, 0, 0, time.UTC)
	records := recordsWithSetup(testColumn("1513", "620", "1568", "09:00", "11:00", 60))
	for day := 0; day <= 2; day++ {
		date := time.Date(2026, 6, 9+day, 0, 0, 0, 0, time.UTC)
		var holds []domain.BlockHold
		if day == 1 {
			holds = []domain.BlockHold{{StartDateTime: date.Add(9 * time.Hour), EndDateTime: date.Add(11 * time.Hour)}}
		}
		records.ScheduleReads[date.Format("2006-01-02")] = completeRead("1513", nil, holds)
	}

	result, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
		Search(context.Background(), scheduling.SearchCommand{
			Windows: []scheduling.AvailabilityWindow{
				{Start: "2026-06-09T00:00:00-04:00", End: "2026-06-10T00:00:00-04:00"},
				{Start: "2026-06-11T00:00:00-04:00", End: "2026-06-12T00:00:00-04:00"},
			},
			TimeZone: "America/New_York",
			Office:   "Spring Hill",
			Routing:  string(domain.RoutingBachOnly),
		})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	got := []string{result.Slots[0].DateTime, result.Slots[1].DateTime}
	if !slices.Equal(got, []string{"2026-06-09T09:00", "2026-06-11T09:00"}) {
		t.Fatalf("exact slots = %v, want one from each requested date", got)
	}
}

func TestSearchLabelsDateAndTimePreservingAlternatives(t *testing.T) {
	now := time.Date(2026, 6, 8, 14, 0, 0, 0, time.UTC)
	start := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)
	records := recordsWithSetup(testColumn("1513", "620", "1568", "09:00", "17:00", 60))
	for day := 0; day <= 14; day++ {
		date := start.AddDate(0, 0, day)
		holds := []domain.BlockHold{{StartDateTime: date.Add(9 * time.Hour), EndDateTime: date.Add(17 * time.Hour)}}
		if day == 0 {
			holds = []domain.BlockHold{
				{StartDateTime: date.Add(9 * time.Hour), EndDateTime: date.Add(14 * time.Hour)},
				{StartDateTime: date.Add(16 * time.Hour), EndDateTime: date.Add(17 * time.Hour)},
			}
		}
		if day == 1 {
			holds = []domain.BlockHold{{StartDateTime: date.Add(9 * time.Hour), EndDateTime: date.Add(16 * time.Hour)}}
		}
		records.ScheduleReads[date.Format("2006-01-02")] = completeRead("1513", nil, holds)
	}

	result, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
		Search(context.Background(), scheduling.SearchCommand{
			Windows: []scheduling.AvailabilityWindow{
				{Start: "2026-06-09T00:00:00-04:00", End: "2026-06-09T10:00:00-04:00"},
				{Start: "2026-06-11T15:00:00-04:00", End: "2026-06-12T00:00:00-04:00"},
			},
			TimeZone: "America/New_York",
			Office:   "Spring Hill",
			Routing:  string(domain.RoutingBachOnly),
		})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if result.MatchStatus != domain.AvailabilityMatchAlternatives || len(result.Slots) != 2 {
		t.Fatalf("result = %#v, want two labeled alternatives", result)
	}
	got := map[string][]string{}
	for _, slot := range result.Slots {
		got[slot.DateTime] = slot.UnmetConstraints
	}
	if !slices.Equal(got["2026-06-09T14:00"], []string{"time"}) ||
		!slices.Equal(got["2026-06-10T16:00"], []string{"date"}) {
		t.Fatalf("alternatives = %#v", got)
	}
}

func TestSearchDoesNotLabelAlternativesWhenExactInventoryIsIncomplete(t *testing.T) {
	now := time.Date(2026, 6, 8, 14, 0, 0, 0, time.UTC)
	start := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)
	records := recordsWithSetup(
		testColumn("1513", "620", "1568", "09:00", "10:00", 60),
		testColumn("1551", "2064", "1568", "09:00", "10:00", 60),
	)
	for day := 0; day <= 14; day++ {
		date := start.AddDate(0, 0, day)
		records.ScheduleReads[date.Format("2006-01-02")] = domain.ScheduleReadResult{
			Columns: map[string]domain.ColumnSchedule{
				"1513": {AppointmentsComplete: true, BlockHoldsComplete: true},
				"1551": {AppointmentsComplete: false, BlockHoldsComplete: true},
			},
		}
	}

	result, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
		Search(context.Background(), scheduling.SearchCommand{
			Windows: []scheduling.AvailabilityWindow{{
				Start: "2026-06-09T15:00:00-04:00",
				End:   "2026-06-09T16:00:00-04:00",
			}},
			TimeZone: "America/New_York",
			Office:   "Spring Hill",
			Routing:  string(domain.RoutingBachLicht),
		})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if result.Outcome != domain.AvailabilityOutcomeSearchIncomplete || len(result.Slots) != 0 {
		t.Fatalf("result = %#v, want incomplete without labeled alternatives", result)
	}
}
