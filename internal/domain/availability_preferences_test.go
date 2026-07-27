package domain_test

import (
	"slices"
	"testing"
	"time"

	"advancedmd-token-management/internal/domain"
)

func TestEvaluateAvailabilityPreferencesIsConservativeForUnusableInput(t *testing.T) {
	start := time.Date(2026, 6, 4, 9, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name        string
		preferences []domain.AvailabilityPreference
	}{
		{name: "empty"},
		{name: "broad", preferences: []domain.AvailabilityPreference{{}}},
		{
			name: "malformed",
			preferences: []domain.AvailabilityPreference{{
				Time: &domain.AvailabilityTimePreference{Kind: domain.AvailabilityTimeExact},
			}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			evaluation := domain.EvaluateAvailabilityPreferences(start, test.preferences)
			if evaluation.Exact() {
				t.Fatal("unusable preferences evaluated as an exact match")
			}
			if !slices.Equal(evaluation.Differences, []domain.AvailabilityPreferenceDifference{
				domain.AvailabilityPreferenceDateDifference,
				domain.AvailabilityPreferenceWeekdayDifference,
				domain.AvailabilityPreferenceTimeDifference,
			}) {
				t.Fatalf("differences = %v, want conservative fallback", evaluation.Differences)
			}
		})
	}
}
