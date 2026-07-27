package scheduling_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"advancedmd-token-management/internal/advancedmd"
	"advancedmd-token-management/internal/advancedmd/advancedmdtest"
	"advancedmd-token-management/internal/domain"
	"advancedmd-token-management/internal/safeerrors"
	"advancedmd-token-management/internal/scheduling"
)

func TestSearchReturnsFoundSlotWithSignedSameStartCapacityPolicy(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	searchDate := "2026-06-03"
	records := advancedmdtest.NewAdapter()
	records.SchedulerSetup = domain.SchedulerSetup{
		Columns: []domain.SchedulerColumn{{
			ID:         "1513",
			Name:       "DR. BACH - SH",
			ProfileID:  "620",
			FacilityID: "1568",
			StartTime:  "09:00",
			EndTime:    "09:15",
			Interval:   15,
			Workweek:   127,
		}},
		Profiles:   []domain.SchedulerProfile{{ID: "620", Name: "BACH, AUSTIN"}},
		Facilities: []domain.SchedulerFacility{{ID: "1568", Name: "ABITA EYE GROUP SPRING HILL"}},
	}
	records.ScheduleReads[searchDate] = domain.ScheduleReadResult{
		Columns: map[string]domain.ColumnSchedule{
			"1513": {
				Appointments: []domain.Appointment{{
					StartDateTime: time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC),
					Duration:      15,
				}},
				AppointmentsComplete: true,
				BlockHoldsComplete:   true,
			},
		},
	}

	scheduler := scheduling.New(records, "test-booking-secret", func() time.Time { return now })
	result, err := scheduler.Search(context.Background(), scheduling.SearchCommand{
		Date:    searchDate,
		Office:  "Spring Hill",
		Routing: string(domain.RoutingBachOnly),
		DOB:     "01/15/1980",
	})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if result.Outcome != domain.AvailabilityOutcomeFound || len(result.Slots) != 1 {
		t.Fatalf("result = %#v, want one found slot", result)
	}

	slot := result.Slots[0]
	if !slot.RequiresForce || slot.SameStartBooked != 1 || slot.SameStartCapacity != 2 {
		t.Fatalf("slot = %#v, want same-start capacity policy", slot)
	}
	policy, err := scheduling.VerifySlotToken("test-booking-secret", slot.BookingToken, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("VerifySlotToken error = %v", err)
	}
	if policy.OfficeID != "spring_hill" ||
		policy.Routing != string(domain.RoutingBachOnly) ||
		policy.DOB != "01/15/1980" ||
		policy.ColumnID != 1513 ||
		policy.ProfileID != 620 ||
		policy.Provider != "Dr. Austin Bach" ||
		policy.SameStartBooked != 1 ||
		policy.SameStartCapacity != 2 ||
		!policy.RequiresForce ||
		!slices.Contains(policy.AppointmentTypeIDs, 1007) {
		t.Fatalf("signed policy = %#v", policy)
	}
}

func TestSearchSignsCleanSlotCapacityWithoutChangingResponse(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	records := recordsWithSetup(testColumn("1513", "620", "1568", "09:00", "09:15", 15))
	records.ScheduleReads["2026-06-03"] = completeRead("1513", nil, nil)

	result, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
		Search(context.Background(), scheduling.SearchCommand{
			Date: "2026-06-03", Office: "Spring Hill", Routing: string(domain.RoutingBachOnly),
		})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(result.Slots) != 1 || result.Slots[0].SameStartCapacity != 0 {
		t.Fatalf("slots = %#v, want clean-slot response contract unchanged", result.Slots)
	}
	policy, err := scheduling.VerifySlotToken(
		"test-booking-secret",
		result.Slots[0].BookingToken,
		now.Add(time.Minute),
	)
	if err != nil || policy.SameStartCapacity != 2 || policy.SameStartBooked != 0 || policy.RequiresForce {
		t.Fatalf("signed capacity policy = %#v, err = %v", policy, err)
	}
}

func TestSearchRanksCompoundPreferencesAcrossTheExistingWindow(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	records := recordsWithSetup(testColumn("1513", "620", "1568", "09:00", "10:00", 30))
	records.ScheduleReads["2026-06-02"] = completeRead("1513", nil, nil)
	records.ScheduleReads["2026-06-03"] = completeRead("1513", nil, []domain.BlockHold{{
		StartDateTime: time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC),
		EndDateTime:   time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC),
	}})
	records.ScheduleReads["2026-06-04"] = completeRead("1513", nil, nil)

	result, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
		Search(context.Background(), scheduling.SearchCommand{
			Date:    "2026-06-02",
			Office:  "Spring Hill",
			Routing: string(domain.RoutingBachOnly),
			Preferences: []domain.AvailabilityPreference{
				{
					Weekday: "monday",
					Time: &domain.AvailabilityTimePreference{
						Kind:        "after",
						MinuteOfDay: minuteOfDay(13*60 + 30),
					},
				},
				{Weekday: "thursday"},
			},
		})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}

	if got := slotDateTimes(result.Slots); !slices.Equal(got, []string{
		"2026-06-04T09:00",
		"2026-06-04T09:30",
	}) {
		t.Fatalf("slot datetimes = %v, want two earliest exact compound-preference matches", got)
	}
	for _, slot := range result.Slots {
		if slot.PreferenceMatch != "exact" ||
			len(slot.PreferenceDifferences) != 0 ||
			slot.BookingToken == "" {
			t.Fatalf("slot = %#v, want exact match with opaque booking token", slot)
		}
	}
	if result.SearchedFrom != "2026-06-02" ||
		result.SearchedThrough != "2026-06-04" ||
		len(records.ScheduleReadQueries) != 3 {
		t.Fatalf("result = %#v, reads = %#v", result, records.ScheduleReadQueries)
	}
}

func TestSearchReturnsUsefulFallbackContrastWhenNoSlotMatchesExactly(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	searchDate := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	records := recordsWithWindowInventory(
		searchDate,
		testColumn("1513", "620", "1568", "09:00", "15:00", 30),
		map[string][]string{
			"2026-06-03": {"09:00"},
			"2026-06-04": {"09:00"},
			"2026-06-05": {"14:00"},
		},
	)

	result, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
		Search(context.Background(), scheduling.SearchCommand{
			Date:    searchDate.Format("2006-01-02"),
			Office:  "Spring Hill",
			Routing: string(domain.RoutingBachOnly),
			Preferences: []domain.AvailabilityPreference{{
				Weekday: "thursday",
				Time: &domain.AvailabilityTimePreference{
					Kind:        "after",
					MinuteOfDay: minuteOfDay(13*60 + 30),
				},
			}},
		})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}

	if got := slotDateTimes(result.Slots); !slices.Equal(got, []string{
		"2026-06-04T09:00",
		"2026-06-05T14:00",
	}) {
		t.Fatalf("slot datetimes = %v, want weekday-preserving and time-preserving fallbacks", got)
	}
	if result.Slots[0].PreferenceMatch != "fallback" ||
		!slices.Equal(result.Slots[0].PreferenceDifferences, []domain.AvailabilityPreferenceDifference{"time"}) ||
		result.Slots[1].PreferenceMatch != "fallback" ||
		!slices.Equal(result.Slots[1].PreferenceDifferences, []domain.AvailabilityPreferenceDifference{"weekday"}) {
		t.Fatalf("slots = %#v, want authoritative fallback tradeoffs", result.Slots)
	}
	if len(records.ScheduleReadQueries) != 15 || result.SearchedThrough != "2026-06-16" {
		t.Fatalf("result = %#v, reads = %d, want complete 15-date fallback scan", result, len(records.ScheduleReadQueries))
	}
}

func TestSearchTreatsOmittedOrEmptyPreferencesAsBroadEarliestAvailability(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name        string
		preferences []domain.AvailabilityPreference
	}{
		{name: "omitted"},
		{name: "empty", preferences: []domain.AvailabilityPreference{}},
		{name: "empty branch", preferences: []domain.AvailabilityPreference{{}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			records := recordsWithSetup(testColumn("1513", "620", "1568", "09:00", "15:00", 30))
			records.ScheduleReads["2026-06-02"] = completeRead("1513", nil, nil)

			result, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
				Search(context.Background(), scheduling.SearchCommand{
					Date:        "2026-06-02",
					Office:      "Spring Hill",
					Routing:     string(domain.RoutingBachOnly),
					Preferences: test.preferences,
				})
			if err != nil {
				t.Fatalf("Search error = %v", err)
			}

			if got := slotDateTimes(result.Slots); !slices.Equal(got, []string{
				"2026-06-02T09:00",
				"2026-06-02T12:00",
			}) {
				t.Fatalf("slot datetimes = %v, want earliest slot with useful time-of-day contrast", got)
			}
			for _, slot := range result.Slots {
				if slot.PreferenceMatch != "" || len(slot.PreferenceDifferences) != 0 {
					t.Fatalf("slot = %#v, want preference metadata omitted for broad search", slot)
				}
			}
			if len(records.ScheduleReadQueries) != 1 {
				t.Fatalf("schedule reads = %#v, want first available day only", records.ScheduleReadQueries)
			}
		})
	}
}

func TestSearchRejectsNoncanonicalPreferenceFactsBeforeReadingTheProvider(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		preference domain.AvailabilityPreference
		want       string
	}{
		{
			name:       "date",
			preference: domain.AvailabilityPreference{Date: "June 4"},
			want:       "preferences[0].date must use YYYY-MM-DD format",
		},
		{
			name:       "weekday",
			preference: domain.AvailabilityPreference{Weekday: "Monday"},
			want:       "preferences[0].weekday must be a lowercase full weekday name",
		},
		{
			name: "time kind",
			preference: domain.AvailabilityPreference{
				Time: &domain.AvailabilityTimePreference{Kind: "near", MinuteOfDay: minuteOfDay(13 * 60)},
			},
			want: "preferences[0].time.kind is invalid",
		},
		{
			name: "missing minute of day",
			preference: domain.AvailabilityPreference{
				Time: &domain.AvailabilityTimePreference{Kind: "exact"},
			},
			want: "preferences[0].time.minuteOfDay is required",
		},
		{
			name: "minute of day",
			preference: domain.AvailabilityPreference{
				Time: &domain.AvailabilityTimePreference{Kind: "after", MinuteOfDay: minuteOfDay(24 * 60)},
			},
			want: "preferences[0].time.minuteOfDay must be between 0 and 1439",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			records := recordsWithSetup(testColumn("1513", "620", "1568", "09:00", "15:00", 30))
			_, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
				Search(context.Background(), scheduling.SearchCommand{
					Date:        "2026-06-02",
					Office:      "Spring Hill",
					Routing:     string(domain.RoutingBachOnly),
					Preferences: []domain.AvailabilityPreference{test.preference},
				})
			if err == nil || err.Error() != test.want {
				t.Fatalf("Search error = %v, want %q", err, test.want)
			}
			if records.SchedulerSetupCalls != 0 || len(records.ScheduleReadQueries) != 0 {
				t.Fatalf("provider reads = setup %d, schedule %#v", records.SchedulerSetupCalls, records.ScheduleReadQueries)
			}
		})
	}
}

func TestSearchBreaksEqualFallbackDistanceTiesByEarlierInventory(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	searchDate := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	records := recordsWithWindowInventory(
		searchDate,
		testColumn("1513", "620", "1568", "09:00", "09:30", 30),
		map[string][]string{
			"2026-06-02": {"09:00"},
			"2026-06-04": {"09:00"},
		},
	)

	result, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
		Search(context.Background(), scheduling.SearchCommand{
			Date:    searchDate.Format("2006-01-02"),
			Office:  "Spring Hill",
			Routing: string(domain.RoutingBachOnly),
			Preferences: []domain.AvailabilityPreference{
				{
					Weekday: "friday",
					Time:    &domain.AvailabilityTimePreference{Kind: "exact", MinuteOfDay: minuteOfDay(9 * 60)},
				},
				{
					Weekday: "wednesday",
					Time:    &domain.AvailabilityTimePreference{Kind: "exact", MinuteOfDay: minuteOfDay(9 * 60)},
				},
			},
		})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}

	if got := slotDateTimes(result.Slots); !slices.Equal(got, []string{
		"2026-06-02T09:00",
		"2026-06-04T09:00",
	}) {
		t.Fatalf("slot datetimes = %v, want equal-distance fallbacks ordered chronologically", got)
	}
}

func TestSearchSupportsCanonicalTimePreferences(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	searchDate := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		time      domain.AvailabilityTimePreference
		wantSlots []string
		wantMatch []domain.AvailabilityPreferenceMatch
	}{
		{
			name:      "morning",
			time:      domain.AvailabilityTimePreference{Kind: "morning"},
			wantSlots: []string{"2026-06-02T09:00", "2026-06-02T09:30"},
			wantMatch: []domain.AvailabilityPreferenceMatch{"exact", "exact"},
		},
		{
			name:      "afternoon",
			time:      domain.AvailabilityTimePreference{Kind: "afternoon"},
			wantSlots: []string{"2026-06-02T12:00", "2026-06-02T12:30"},
			wantMatch: []domain.AvailabilityPreferenceMatch{"exact", "exact"},
		},
		{
			name:      "exact",
			time:      domain.AvailabilityTimePreference{Kind: "exact", MinuteOfDay: minuteOfDay(14 * 60)},
			wantSlots: []string{"2026-06-02T14:00", "2026-06-02T13:30"},
			wantMatch: []domain.AvailabilityPreferenceMatch{"exact", "fallback"},
		},
		{
			name:      "around",
			time:      domain.AvailabilityTimePreference{Kind: "around", MinuteOfDay: minuteOfDay(13*60 + 15)},
			wantSlots: []string{"2026-06-02T13:00", "2026-06-02T13:30"},
			wantMatch: []domain.AvailabilityPreferenceMatch{"fallback", "fallback"},
		},
		{
			name:      "before",
			time:      domain.AvailabilityTimePreference{Kind: "before", MinuteOfDay: minuteOfDay(10 * 60)},
			wantSlots: []string{"2026-06-02T09:00", "2026-06-02T09:30"},
			wantMatch: []domain.AvailabilityPreferenceMatch{"exact", "exact"},
		},
		{
			name:      "after",
			time:      domain.AvailabilityTimePreference{Kind: "after", MinuteOfDay: minuteOfDay(13*60 + 30)},
			wantSlots: []string{"2026-06-02T14:00", "2026-06-02T14:30"},
			wantMatch: []domain.AvailabilityPreferenceMatch{"exact", "exact"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			records := recordsWithWindowInventory(
				searchDate,
				testColumn("1513", "620", "1568", "09:00", "15:00", 30),
				map[string][]string{
					"2026-06-02": {
						"09:00", "09:30", "10:00", "10:30", "11:00", "11:30",
						"12:00", "12:30", "13:00", "13:30", "14:00", "14:30",
					},
				},
			)

			result, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
				Search(context.Background(), scheduling.SearchCommand{
					Date:        searchDate.Format("2006-01-02"),
					Office:      "Spring Hill",
					Routing:     string(domain.RoutingBachOnly),
					Preferences: []domain.AvailabilityPreference{{Time: &test.time}},
				})
			if err != nil {
				t.Fatalf("Search error = %v", err)
			}

			if got := slotDateTimes(result.Slots); !slices.Equal(got, test.wantSlots) {
				t.Fatalf("slot datetimes = %v, want %v", got, test.wantSlots)
			}
			gotMatches := make([]domain.AvailabilityPreferenceMatch, 0, len(result.Slots))
			for _, slot := range result.Slots {
				gotMatches = append(gotMatches, slot.PreferenceMatch)
				if slot.PreferenceMatch == "fallback" &&
					!slices.Equal(
						slot.PreferenceDifferences,
						[]domain.AvailabilityPreferenceDifference{"time"},
					) {
					t.Fatalf("slot = %#v, want time-only fallback metadata", slot)
				}
			}
			if !slices.Equal(gotMatches, test.wantMatch) {
				t.Fatalf("preference matches = %v, want %v", gotMatches, test.wantMatch)
			}
		})
	}
}

func TestSearchReturnsNearestRealDatesWhenThePreferredDateHasNoInventory(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	searchDate := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	records := recordsWithWindowInventory(
		searchDate,
		testColumn("1513", "620", "1568", "09:00", "09:30", 30),
		map[string][]string{
			"2026-06-03": {"09:00"},
			"2026-06-05": {"09:00"},
		},
	)

	result, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
		Search(context.Background(), scheduling.SearchCommand{
			Date:    searchDate.Format("2006-01-02"),
			Office:  "Spring Hill",
			Routing: string(domain.RoutingBachOnly),
			Preferences: []domain.AvailabilityPreference{{
				Date: "2026-06-04",
			}},
		})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}

	if got := slotDateTimes(result.Slots); !slices.Equal(got, []string{
		"2026-06-03T09:00",
		"2026-06-05T09:00",
	}) {
		t.Fatalf("slot datetimes = %v, want equally near dates ordered chronologically", got)
	}
	for _, slot := range result.Slots {
		if slot.PreferenceMatch != "fallback" ||
			!slices.Equal(
				slot.PreferenceDifferences,
				[]domain.AvailabilityPreferenceDifference{"date"},
			) {
			t.Fatalf("slot = %#v, want date-only fallback metadata", slot)
		}
	}
}

func TestBroadSearchStopsAtTheFirstAvailableDayWithOneSlot(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	searchDate := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	records := recordsWithWindowInventory(
		searchDate,
		testColumn("1513", "620", "1568", "09:00", "09:30", 30),
		map[string][]string{"2026-06-04": {"09:00"}},
	)
	records.ScheduleReadErrors["2026-06-05"] = errors.New("later read failed")

	result, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
		Search(context.Background(), scheduling.SearchCommand{
			Date:    searchDate.Format("2006-01-02"),
			Office:  "Spring Hill",
			Routing: string(domain.RoutingBachOnly),
		})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if got := slotDateTimes(result.Slots); !slices.Equal(got, []string{"2026-06-04T09:00"}) {
		t.Fatalf("slot datetimes = %v, want the only real slot", got)
	}
	if result.Slots[0].PreferenceMatch != "" ||
		result.Outcome != domain.AvailabilityOutcomeFound ||
		result.SearchedThrough != "2026-06-04" ||
		len(records.ScheduleReadQueries) != 3 {
		t.Fatalf("result = %#v, reads = %d", result, len(records.ScheduleReadQueries))
	}
}

func TestSearchReturnsNoneOnlyAfterACompleteWindow(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	searchDate := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	records := recordsWithSetup(testColumn("1513", "620", "1568", "09:00", "09:15", 15))
	for day := 0; day <= 14; day++ {
		date := searchDate.AddDate(0, 0, day)
		records.ScheduleReads[date.Format("2006-01-02")] = completeRead("1513", nil, []domain.BlockHold{{
			StartDateTime: date.Add(9 * time.Hour),
			EndDateTime:   date.Add(9*time.Hour + 15*time.Minute),
		}})
	}

	result, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
		Search(context.Background(), scheduling.SearchCommand{
			Date:    searchDate.Format("2006-01-02"),
			Office:  "Spring Hill",
			Routing: string(domain.RoutingBachOnly),
			DOB:     "01/15/1980",
		})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if result.Outcome != domain.AvailabilityOutcomeNoAvailability ||
		result.Status != domain.AvailabilityStatusSuccess ||
		result.AvailabilityFound ||
		result.ShouldRetrySameSearch ||
		result.NextAction != domain.AvailabilityNextActionAskDifferentPreferences ||
		result.SearchedThrough != "2026-06-17" ||
		len(result.Slots) != 0 {
		t.Fatalf("result = %#v, want explicit no-availability outcome", result)
	}
}

func TestSearchReturnsIncompleteWhenProviderReadsCannotProveNone(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	searchDate := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	records := recordsWithSetup(testColumn("1513", "620", "1568", "09:00", "09:15", 15))
	for day := 0; day <= 14; day++ {
		date := searchDate.AddDate(0, 0, day).Format("2006-01-02")
		records.ScheduleReads[date] = domain.ScheduleReadResult{
			Columns: map[string]domain.ColumnSchedule{
				"1513": {
					AppointmentsComplete: false,
					BlockHoldsComplete:   true,
				},
			},
		}
	}

	result, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
		Search(context.Background(), scheduling.SearchCommand{
			Date:    searchDate.Format("2006-01-02"),
			Office:  "Spring Hill",
			Routing: string(domain.RoutingBachOnly),
			DOB:     "01/15/1980",
		})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if result.Outcome != domain.AvailabilityOutcomeSearchIncomplete ||
		result.Status != domain.AvailabilityStatusError ||
		result.AvailabilityFound ||
		!result.ShouldRetrySameSearch ||
		result.NextAction != domain.AvailabilityNextActionRetryOnceThenAskPreferences ||
		len(result.Slots) != 0 {
		t.Fatalf("result = %#v, want explicit incomplete outcome", result)
	}
}

func TestSearchReturnsIncompleteAfterPartialProviderFailure(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	searchDate := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	records := recordsWithSetup(
		testColumn("1513", "620", "1568", "09:00", "09:15", 15),
		testColumn("1598", "620", "1568", "09:00", "09:15", 15),
	)
	for day := 0; day <= 14; day++ {
		date := searchDate.AddDate(0, 0, day)
		records.ScheduleReads[date.Format("2006-01-02")] = domain.ScheduleReadResult{
			Columns: map[string]domain.ColumnSchedule{
				"1513": {
					AppointmentsComplete: false,
					BlockHoldsComplete:   true,
				},
				"1598": {
					BlockHolds: []domain.BlockHold{{
						StartDateTime: date.Add(9 * time.Hour),
						EndDateTime:   date.Add(9*time.Hour + 15*time.Minute),
					}},
					AppointmentsComplete: true,
					BlockHoldsComplete:   true,
				},
			},
		}
	}

	result, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
		Search(context.Background(), scheduling.SearchCommand{
			Date:    searchDate.Format("2006-01-02"),
			Office:  "Spring Hill",
			Routing: string(domain.RoutingBachOnly),
			DOB:     "01/15/1980",
		})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if result.Outcome != domain.AvailabilityOutcomeSearchIncomplete || !result.ShouldRetrySameSearch {
		t.Fatalf("result = %#v, want partial-read incomplete outcome", result)
	}
}

func TestSearchExcludesRestrictedProviders(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	records := recordsWithSetup(testColumn("1210", "1993", "670", "09:00", "09:15", 15))

	result, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
		Search(context.Background(), scheduling.SearchCommand{
			Date:    "2026-06-03",
			Office:  "Sweetwater",
			Routing: string(domain.RoutingOpticalOnly),
			DOB:     "06/01/2023",
		})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if result.Outcome != domain.AvailabilityOutcomeNoEligibleProviders || result.AvailabilityFound {
		t.Fatalf("result = %#v, want age-restricted provider excluded", result)
	}
}

func TestSearchBlocksSlotsOverlappedByMultiSlotAppointments(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	searchDate := "2026-06-03"
	records := recordsWithSetup(testColumn("1550", "2076", "1568", "08:30", "10:30", 30))
	records.ScheduleReads[searchDate] = completeRead("1550", []domain.Appointment{{
		StartDateTime: time.Date(2026, 6, 3, 8, 0, 0, 0, time.UTC),
		Duration:      90,
	}}, nil)

	result, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
		Search(context.Background(), scheduling.SearchCommand{
			Date:    searchDate,
			Office:  "Spring Hill",
			Routing: string(domain.RoutingAll),
			DOB:     "01/15/1980",
		})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	gotTimes := make([]string, 0, len(result.Slots))
	for _, slot := range result.Slots {
		gotTimes = append(gotTimes, slot.Time)
	}
	if !slices.Equal(gotTimes, []string{"9:30 AM", "10:00 AM"}) {
		t.Fatalf("slot times = %v, want multi-slot overlap excluded", gotTimes)
	}
}

func TestSearchOwnsRoutingPreauthAndReturnsCompleteFirstAvailableDay(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	records := recordsWithSetup(
		testColumn("1513", "620", "1568", "08:00", "17:00", 30),
		testColumn("1600", "1983", "1568", "08:00", "17:00", 30),
	)
	records.ScheduleReads["2026-06-15"] = completeRead("1600", nil, nil)

	result, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
		Search(context.Background(), scheduling.SearchCommand{
			Date:            "2026-06-03",
			Office:          "Spring Hill",
			Routing:         string(domain.RoutingOpticalOnly),
			DOB:             "01/15/1980",
			PreauthRequired: true,
		})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if result.RequestedDate != "2026-06-03" ||
		result.ActualDate != "2026-06-15" ||
		result.SearchedFrom != "2026-06-15" ||
		result.BookingTokenExpiresAt != "2026-06-01T12:15:00Z" ||
		!result.DateShifted ||
		len(result.Slots) != 2 {
		t.Fatalf("result = %#v", result)
	}
	gotTimes := make([]string, 0, len(result.Slots))
	for _, slot := range result.Slots {
		if slot.ColumnID != 1600 {
			t.Fatalf("slot = %#v, want optical routing column 1600", slot)
		}
		policy, err := scheduling.VerifySlotToken(
			"test-booking-secret",
			slot.BookingToken,
			now.Add(time.Minute),
		)
		if err != nil ||
			policy.Routing != string(domain.RoutingOpticalOnly) ||
			policy.StartDatetime != slot.DateTime ||
			policy.ExpiresAt != now.Add(15*time.Minute).Unix() {
			t.Fatalf("signed policy for %s = %#v, err = %v", slot.Time, policy, err)
		}
		gotTimes = append(gotTimes, slot.Time)
	}
	if !slices.Equal(gotTimes, []string{"8:00 AM", "12:00 PM"}) {
		t.Fatalf("slot times = %v, want earliest slot with useful time-of-day contrast", gotTimes)
	}
	if len(records.ScheduleReadQueries) != 1 ||
		records.ScheduleReadQueries[0].Date != "2026-06-15" ||
		!slices.Equal(records.ScheduleReadQueries[0].ColumnIDs, []string{"1600"}) {
		t.Fatalf("schedule read queries = %#v, want only eligible optical column", records.ScheduleReadQueries)
	}
}

func TestSearchUsesFreshSchedulerSetupCacheAndStaleFallback(t *testing.T) {
	currentTime := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	records := recordsWithSetup(testColumn("1513", "620", "1568", "09:00", "09:15", 15))
	records.ScheduleReads["2026-06-03"] = completeRead("1513", nil, nil)
	records.ScheduleReads["2026-06-04"] = completeRead("1513", nil, nil)
	records.ScheduleReads["2026-06-05"] = completeRead("1513", nil, nil)
	scheduler := scheduling.New(records, "test-booking-secret", func() time.Time { return currentTime })

	if _, err := scheduler.Search(context.Background(), scheduling.SearchCommand{
		Date: "2026-06-03", Office: "Spring Hill", Routing: string(domain.RoutingBachOnly),
	}); err != nil {
		t.Fatalf("first Search error = %v", err)
	}
	if records.SchedulerSetupCalls != 1 {
		t.Fatalf("scheduler setup calls = %d, want 1", records.SchedulerSetupCalls)
	}

	records.SchedulerSetupError = errors.New("provider setup unavailable")
	currentTime = currentTime.Add(time.Hour)
	result, err := scheduler.Search(context.Background(), scheduling.SearchCommand{
		Date: "2026-06-04", Office: "Spring Hill", Routing: string(domain.RoutingBachOnly),
	})
	if err != nil {
		t.Fatalf("fresh-cache Search error = %v", err)
	}
	if result.Outcome != domain.AvailabilityOutcomeFound {
		t.Fatalf("fresh-cache result = %#v", result)
	}
	if records.SchedulerSetupCalls != 1 {
		t.Fatalf("fresh-cache setup calls = %d, want 1", records.SchedulerSetupCalls)
	}

	currentTime = currentTime.Add(6 * time.Hour)
	result, err = scheduler.Search(context.Background(), scheduling.SearchCommand{
		Date: "2026-06-05", Office: "Spring Hill", Routing: string(domain.RoutingBachOnly),
	})
	if err != nil {
		t.Fatalf("stale-fallback Search error = %v", err)
	}
	if result.Outcome != domain.AvailabilityOutcomeFound {
		t.Fatalf("stale-fallback result = %#v", result)
	}
	if records.SchedulerSetupCalls != 2 {
		t.Fatalf("expired-cache setup calls = %d, want 2", records.SchedulerSetupCalls)
	}
}

func TestSearchRejectsInvalidDOBAndRequestedProviderOutsideRouting(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	records := recordsWithSetup(
		testColumn("1513", "620", "1568", "09:00", "09:15", 15),
		testColumn("1551", "2064", "1568", "09:00", "09:15", 15),
	)
	scheduler := scheduling.New(records, "test-booking-secret", func() time.Time { return now })

	_, err := scheduler.Search(context.Background(), scheduling.SearchCommand{
		Date: "2026-06-03", Office: "Spring Hill", Routing: string(domain.RoutingBachOnly), DOB: "not-a-date",
	})
	if err == nil || err.Error() != "dob must be a valid date" {
		t.Fatalf("invalid DOB error = %v", err)
	}

	_, err = scheduler.Search(context.Background(), scheduling.SearchCommand{
		Date: "2026-06-03", Office: "Spring Hill", Routing: string(domain.RoutingBachOnly), Provider: "Dr. Licht",
	})
	if err == nil || !strings.Contains(err.Error(), `No provider found matching "Dr. Licht"`) {
		t.Fatalf("restricted provider error = %v", err)
	}
}

func TestSearchTreatsMissingBlockHoldsAsIncomplete(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	searchDate := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	records := recordsWithSetup(testColumn("1513", "620", "1568", "09:00", "09:15", 15))
	for day := 0; day <= 14; day++ {
		date := searchDate.AddDate(0, 0, day).Format("2006-01-02")
		records.ScheduleReads[date] = domain.ScheduleReadResult{
			Columns: map[string]domain.ColumnSchedule{
				"1513": {
					AppointmentsComplete: true,
					BlockHoldsComplete:   false,
				},
			},
		}
	}

	result, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
		Search(context.Background(), scheduling.SearchCommand{
			Date: searchDate.Format("2006-01-02"), Office: "Spring Hill", Routing: string(domain.RoutingBachOnly),
		})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if result.Outcome != domain.AvailabilityOutcomeSearchIncomplete {
		t.Fatalf("result = %#v, want incomplete block-hold read", result)
	}
}

func TestSearchPreservesAuthenticationFailureContract(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	want := "Service authentication is temporarily unavailable. Please try again."

	t.Run("scheduler setup", func(t *testing.T) {
		records := recordsWithSetup(testColumn("1513", "620", "1568", "09:00", "09:15", 15))
		records.SchedulerSetupError = advancedmd.NewError(safeerrors.CategoryUnavailable)
		_, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
			Search(context.Background(), scheduling.SearchCommand{
				Date: "2026-06-03", Office: "Spring Hill", Routing: string(domain.RoutingBachOnly),
			})
		if err == nil || err.Error() != want {
			t.Fatalf("Search error = %v, want %q", err, want)
		}
		if got := scheduling.ProviderFailureOf(err); got != safeerrors.CategoryUnavailable {
			t.Fatalf("ProviderFailureOf(error) = %q, want unavailable", got)
		}
	})

	t.Run("schedule read", func(t *testing.T) {
		records := recordsWithSetup(testColumn("1513", "620", "1568", "09:00", "09:15", 15))
		records.ScheduleReadErrors["2026-06-03"] = advancedmd.NewError(safeerrors.CategoryAuthentication)
		_, err := scheduling.New(records, "test-booking-secret", func() time.Time { return now }).
			Search(context.Background(), scheduling.SearchCommand{
				Date: "2026-06-03", Office: "Spring Hill", Routing: string(domain.RoutingBachOnly),
			})
		if err == nil || err.Error() != want {
			t.Fatalf("Search error = %v, want %q", err, want)
		}
		if got := scheduling.ProviderFailureOf(err); got != safeerrors.CategoryAuthentication {
			t.Fatalf("ProviderFailureOf(error) = %q, want authentication", got)
		}
	})
}

func recordsWithSetup(columns ...domain.SchedulerColumn) *advancedmdtest.Adapter {
	records := advancedmdtest.NewAdapter()
	records.SchedulerSetup = domain.SchedulerSetup{
		Columns: columns,
		Profiles: []domain.SchedulerProfile{
			{ID: "620", Name: "BACH, AUSTIN"},
			{ID: "1993", Name: "CALERO, GISSELLE"},
			{ID: "1983", Name: "OTERO, MELISSA"},
			{ID: "2064", Name: "LICHT, JOSEPH"},
			{ID: "2076", Name: "NOEL"},
		},
		Facilities: []domain.SchedulerFacility{
			{ID: "1568", Name: "ABITA EYE GROUP SPRING HILL"},
			{ID: "670", Name: "ABITA EYE GROUP SWEETWATER"},
		},
	}
	return records
}

func testColumn(id, profileID, facilityID, start, end string, interval int) domain.SchedulerColumn {
	return domain.SchedulerColumn{
		ID:         id,
		Name:       "TEST COLUMN",
		ProfileID:  profileID,
		FacilityID: facilityID,
		StartTime:  start,
		EndTime:    end,
		Interval:   interval,
		Workweek:   127,
	}
}

func completeRead(columnID string, appointments []domain.Appointment, blockHolds []domain.BlockHold) domain.ScheduleReadResult {
	return domain.ScheduleReadResult{
		Columns: map[string]domain.ColumnSchedule{
			columnID: {
				Appointments:         appointments,
				BlockHolds:           blockHolds,
				AppointmentsComplete: true,
				BlockHoldsComplete:   true,
			},
		},
	}
}

func recordsWithWindowInventory(
	searchDate time.Time,
	column domain.SchedulerColumn,
	availableByDate map[string][]string,
) *advancedmdtest.Adapter {
	records := recordsWithSetup(column)
	for day := 0; day <= 14; day++ {
		date := searchDate.AddDate(0, 0, day)
		records.ScheduleReads[date.Format("2006-01-02")] = completeReadWithOnlyAvailableSlots(
			column.ID,
			date,
			column.StartTime,
			column.EndTime,
			availableByDate[date.Format("2006-01-02")]...,
		)
	}
	return records
}

func completeReadWithOnlyAvailableSlots(
	columnID string,
	date time.Time,
	start string,
	end string,
	available ...string,
) domain.ScheduleReadResult {
	availableSet := make(map[string]bool, len(available))
	for _, value := range available {
		availableSet[value] = true
	}
	startTime, _ := time.Parse("15:04", start)
	endTime, _ := time.Parse("15:04", end)
	cursor := time.Date(date.Year(), date.Month(), date.Day(), startTime.Hour(), startTime.Minute(), 0, 0, date.Location())
	until := time.Date(date.Year(), date.Month(), date.Day(), endTime.Hour(), endTime.Minute(), 0, 0, date.Location())
	holds := make([]domain.BlockHold, 0)
	for cursor.Before(until) {
		if !availableSet[cursor.Format("15:04")] {
			holds = append(holds, domain.BlockHold{
				StartDateTime: cursor,
				EndDateTime:   cursor.Add(30 * time.Minute),
			})
		}
		cursor = cursor.Add(30 * time.Minute)
	}
	return completeRead(columnID, nil, holds)
}

func slotDateTimes(slots []domain.AvailabilitySlotOption) []string {
	values := make([]string, 0, len(slots))
	for _, slot := range slots {
		values = append(values, slot.DateTime)
	}
	return values
}

func minuteOfDay(value int) *int {
	return &value
}
