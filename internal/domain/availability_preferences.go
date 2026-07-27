package domain

import (
	"fmt"
	"strings"
	"time"
)

// AvailabilityTimePreferenceKind is the canonical time vocabulary accepted by
// Scheduling.
type AvailabilityTimePreferenceKind string

const (
	AvailabilityTimeMorning   AvailabilityTimePreferenceKind = "morning"
	AvailabilityTimeAfternoon AvailabilityTimePreferenceKind = "afternoon"
	AvailabilityTimeExact     AvailabilityTimePreferenceKind = "exact"
	AvailabilityTimeAround    AvailabilityTimePreferenceKind = "around"
	AvailabilityTimeBefore    AvailabilityTimePreferenceKind = "before"
	AvailabilityTimeAfter     AvailabilityTimePreferenceKind = "after"
)

// AvailabilityPreference is one canonical AND branch. Separate branches are
// alternatives.
type AvailabilityPreference struct {
	Date    string                      `json:"date,omitempty"`
	Weekday string                      `json:"weekday,omitempty"`
	Time    *AvailabilityTimePreference `json:"time,omitempty"`
}

// AvailabilityTimePreference is one canonical time-of-day fact.
type AvailabilityTimePreference struct {
	Kind        AvailabilityTimePreferenceKind `json:"kind"`
	MinuteOfDay *int                           `json:"minuteOfDay,omitempty"`
}

type AvailabilityPreferenceMatch string

const (
	AvailabilityPreferenceExact    AvailabilityPreferenceMatch = "exact"
	AvailabilityPreferenceFallback AvailabilityPreferenceMatch = "fallback"
)

type AvailabilityPreferenceDifference string

const (
	AvailabilityPreferenceDateDifference    AvailabilityPreferenceDifference = "date"
	AvailabilityPreferenceWeekdayDifference AvailabilityPreferenceDifference = "weekday"
	AvailabilityPreferenceTimeDifference    AvailabilityPreferenceDifference = "time"
)

// AvailabilityPreferenceEvaluation is the closest branch comparison for one
// real slot.
type AvailabilityPreferenceEvaluation struct {
	Differences       []AvailabilityPreferenceDifference
	distanceMinutes   int
	hasDayConstraint  bool
	hasTimeConstraint bool
}

func ValidateAvailabilityPreferences(preferences []AvailabilityPreference) error {
	for index, preference := range preferences {
		if preference.Date != "" {
			if _, err := time.Parse("2006-01-02", preference.Date); err != nil {
				return fmt.Errorf("preferences[%d].date must use YYYY-MM-DD format", index)
			}
		}
		if preference.Weekday != "" {
			if _, ok := availabilityWeekday(preference.Weekday); !ok {
				return fmt.Errorf(
					"preferences[%d].weekday must be a lowercase full weekday name",
					index,
				)
			}
		}
		if preference.Time == nil {
			continue
		}
		switch preference.Time.Kind {
		case AvailabilityTimeMorning, AvailabilityTimeAfternoon:
		case AvailabilityTimeExact, AvailabilityTimeAround, AvailabilityTimeBefore, AvailabilityTimeAfter:
			if preference.Time.MinuteOfDay == nil {
				return fmt.Errorf("preferences[%d].time.minuteOfDay is required", index)
			}
			if *preference.Time.MinuteOfDay < 0 || *preference.Time.MinuteOfDay >= 24*60 {
				return fmt.Errorf(
					"preferences[%d].time.minuteOfDay must be between 0 and 1439",
					index,
				)
			}
		default:
			return fmt.Errorf("preferences[%d].time.kind is invalid", index)
		}
	}
	return nil
}

// AvailabilityPreferencesAreBroad reports whether an OR request contains an
// unconstrained branch.
func AvailabilityPreferencesAreBroad(preferences []AvailabilityPreference) bool {
	if len(preferences) == 0 {
		return true
	}
	for _, preference := range preferences {
		if preference.Date == "" && preference.Weekday == "" && preference.Time == nil {
			return true
		}
	}
	return false
}

// EvaluateAvailabilityPreferences returns the closest branch comparison for a
// real slot. The request must already be validated and non-broad.
func EvaluateAvailabilityPreferences(
	start time.Time,
	preferences []AvailabilityPreference,
) AvailabilityPreferenceEvaluation {
	if AvailabilityPreferencesAreBroad(preferences) ||
		ValidateAvailabilityPreferences(preferences) != nil {
		return AvailabilityPreferenceEvaluation{
			Differences: []AvailabilityPreferenceDifference{
				AvailabilityPreferenceDateDifference,
				AvailabilityPreferenceWeekdayDifference,
				AvailabilityPreferenceTimeDifference,
			},
			distanceMinutes: int(^uint(0) >> 1),
		}
	}
	best := evaluateAvailabilityPreference(start, preferences[0])
	for _, preference := range preferences[1:] {
		evaluation := evaluateAvailabilityPreference(start, preference)
		if evaluation.Less(best) {
			best = evaluation
		}
	}
	return best
}

func (e AvailabilityPreferenceEvaluation) Exact() bool {
	return len(e.Differences) == 0
}

func (e AvailabilityPreferenceEvaluation) Less(other AvailabilityPreferenceEvaluation) bool {
	if len(e.Differences) != len(other.Differences) {
		return len(e.Differences) < len(other.Differences)
	}
	return e.distanceMinutes < other.distanceMinutes
}

func (e AvailabilityPreferenceEvaluation) PreservesDayOnly() bool {
	return e.hasDayConstraint &&
		e.hasTimeConstraint &&
		!e.differsOnDay() &&
		e.differsOn(AvailabilityPreferenceTimeDifference)
}

func (e AvailabilityPreferenceEvaluation) PreservesTimeOnly() bool {
	return e.hasDayConstraint &&
		e.hasTimeConstraint &&
		e.differsOnDay() &&
		!e.differsOn(AvailabilityPreferenceTimeDifference)
}

func evaluateAvailabilityPreference(
	start time.Time,
	preference AvailabilityPreference,
) AvailabilityPreferenceEvaluation {
	evaluation := AvailabilityPreferenceEvaluation{
		hasDayConstraint:  preference.Date != "" || preference.Weekday != "",
		hasTimeConstraint: preference.Time != nil,
	}
	if preference.Date != "" && preference.Date != start.Format("2006-01-02") {
		preferredDate, _ := time.Parse("2006-01-02", preference.Date)
		evaluation.Differences = append(
			evaluation.Differences,
			AvailabilityPreferenceDateDifference,
		)
		evaluation.distanceMinutes += availabilityDayDistance(start, preferredDate) * 24 * 60
	}
	if preference.Weekday != "" {
		preferredWeekday, _ := availabilityWeekday(preference.Weekday)
		if start.Weekday() != preferredWeekday {
			evaluation.Differences = append(
				evaluation.Differences,
				AvailabilityPreferenceWeekdayDifference,
			)
			evaluation.distanceMinutes += availabilityWeekdayDistance(
				start.Weekday(),
				preferredWeekday,
			) * 24 * 60
		}
	}
	if preference.Time != nil {
		matches, distance := evaluateAvailabilityTime(start, *preference.Time)
		if !matches {
			evaluation.Differences = append(
				evaluation.Differences,
				AvailabilityPreferenceTimeDifference,
			)
			evaluation.distanceMinutes += distance
		}
	}
	return evaluation
}

func evaluateAvailabilityTime(
	start time.Time,
	preference AvailabilityTimePreference,
) (bool, int) {
	minuteOfDay := start.Hour()*60 + start.Minute()
	switch preference.Kind {
	case AvailabilityTimeMorning:
		if minuteOfDay < 12*60 {
			return true, 0
		}
		return false, minuteOfDay - 12*60 + 1
	case AvailabilityTimeAfternoon:
		if minuteOfDay >= 12*60 {
			return true, 0
		}
		return false, 12*60 - minuteOfDay
	case AvailabilityTimeExact, AvailabilityTimeAround:
		distance := availabilityAbsolute(minuteOfDay - *preference.MinuteOfDay)
		return distance == 0, distance
	case AvailabilityTimeBefore:
		if minuteOfDay < *preference.MinuteOfDay {
			return true, 0
		}
		return false, minuteOfDay - *preference.MinuteOfDay + 1
	case AvailabilityTimeAfter:
		if minuteOfDay > *preference.MinuteOfDay {
			return true, 0
		}
		return false, *preference.MinuteOfDay - minuteOfDay + 1
	}
	return false, 24 * 60
}

func (e AvailabilityPreferenceEvaluation) differsOnDay() bool {
	return e.differsOn(AvailabilityPreferenceDateDifference) ||
		e.differsOn(AvailabilityPreferenceWeekdayDifference)
}

func (e AvailabilityPreferenceEvaluation) differsOn(
	difference AvailabilityPreferenceDifference,
) bool {
	for _, candidate := range e.Differences {
		if candidate == difference {
			return true
		}
	}
	return false
}

func availabilityWeekday(value string) (time.Weekday, bool) {
	for weekday := time.Sunday; weekday <= time.Saturday; weekday++ {
		if strings.ToLower(weekday.String()) == value {
			return weekday, true
		}
	}
	return time.Sunday, false
}

func availabilityDayDistance(left, right time.Time) int {
	left = time.Date(left.Year(), left.Month(), left.Day(), 0, 0, 0, 0, time.UTC)
	right = time.Date(right.Year(), right.Month(), right.Day(), 0, 0, 0, 0, time.UTC)
	return availabilityAbsolute(int(left.Sub(right).Hours() / 24))
}

func availabilityWeekdayDistance(left, right time.Weekday) int {
	distance := availabilityAbsolute(int(left) - int(right))
	if distance > 3 {
		return 7 - distance
	}
	return distance
}

func availabilityAbsolute(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
