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
		StartDateTime: time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC),
		Duration:      60,
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
	if !slices.Equal(gotTimes, []string{"8:30 AM", "9:00 AM", "10:00 AM"}) {
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
		len(result.Slots) != 18 {
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
	if !slices.Equal(gotTimes, []string{
		"8:00 AM",
		"8:30 AM",
		"9:00 AM",
		"9:30 AM",
		"10:00 AM",
		"10:30 AM",
		"11:00 AM",
		"11:30 AM",
		"12:00 PM",
		"12:30 PM",
		"1:00 PM",
		"1:30 PM",
		"2:00 PM",
		"2:30 PM",
		"3:00 PM",
		"3:30 PM",
		"4:00 PM",
		"4:30 PM",
	}) {
		t.Fatalf("slot times = %v, want every bookable slot on first available day", gotTimes)
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
