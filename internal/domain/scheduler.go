package domain

import (
	"fmt"
	"time"
)

// SchedulerColumn represents a provider's scheduling column from getschedulersetup.
// A column is a provider + location combination with specific work hours.
type SchedulerColumn struct {
	ID              string // Column ID (e.g., "1716")
	Name            string // Display name (e.g., "DR. BACH - BP")
	ProfileID       string // Provider profile ID (e.g., "1135")
	FacilityID      string // Facility ID (e.g., "fac1032")
	StartTime       string // Work start time (e.g., "08:00")
	EndTime         string // Work end time (e.g., "17:00")
	Interval        int    // Slot interval in minutes (e.g., 15)
	MaxApptsPerSlot int    // Max appointments per slot (0 = unlimited)
	Workweek        int    // Bitmask for working days (1=Sun, 2=Mon, 4=Tue, etc.)
}

// SchedulerProfile represents a provider profile from getschedulersetup.
type SchedulerProfile struct {
	ID   string // Profile ID (e.g., "1135")
	Code string // Provider code (e.g., "ABCH")
	Name string // Provider name (e.g., "BACH, AUSTIN")
}

// SchedulerFacility represents a facility/location from getschedulersetup.
type SchedulerFacility struct {
	ID   string // Facility ID (e.g., "fac1032")
	Code string // Facility code (e.g., "ABSPR")
	Name string // Facility name (e.g., "ABITA EYE GROUP SPRING HILL")
}

// SchedulerSetup holds the complete scheduler configuration.
type SchedulerSetup struct {
	Columns    []SchedulerColumn
	Profiles   []SchedulerProfile
	Facilities []SchedulerFacility
}

// Appointment represents a booked appointment from the REST API.
type Appointment struct {
	ID            int       // Appointment ID
	StartDateTime time.Time // Appointment start time
	Duration      int       // Duration in minutes
	ColumnID      int       // Column ID
	ProfileID     int       // Profile ID
	PatientID     int       // Patient ID
}

// BlockHold represents a blocked time period from the REST API.
type BlockHold struct {
	ID            int       // Block hold ID
	StartDateTime time.Time // Block start time
	EndDateTime   time.Time // Block end time (from AMD enddatetime)
	ColumnID      int       // Column ID
	Note          string    // Optional note (e.g., "Lunch")
}

// AvailableSlot represents a single available time slot.
type AvailableSlot struct {
	Time     string `json:"time"`     // Human-readable time (e.g., "9:00 AM")
	DateTime string `json:"datetime"` // ISO format for booking (e.g., "2026-02-03T09:00")
}

// ProviderAvailability represents a provider's availability response.
type ProviderAvailability struct {
	Name           string          `json:"name"`
	ColumnID       int             `json:"columnId"`
	ProfileID      int             `json:"profileId"`
	Facility       string          `json:"facility"`
	SlotDuration   int             `json:"slotDuration"`
	TotalAvailable int             `json:"totalAvailable"`
	FirstAvailable string          `json:"firstAvailable,omitempty"`
	LastAvailable  string          `json:"lastAvailable,omitempty"`
	Slots          []AvailableSlot `json:"slots"`
}

// AvailabilityResponse is the response structure for the availability endpoint.
type AvailabilityResponse struct {
	SearchedDate string                 `json:"searchedDate"`
	Date         string                 `json:"date"`
	Location     string                 `json:"location"`
	Providers    []ProviderAvailability `json:"providers"`
}

// WorksOnDay checks if the column works on a given weekday.
// Weekday: 0=Sunday, 1=Monday, ..., 6=Saturday
// Workweek bitmask: 1=Sun, 2=Mon, 4=Tue, 8=Wed, 16=Thu, 32=Fri, 64=Sat
func (c *SchedulerColumn) WorksOnDay(weekday time.Weekday) bool {
	bit := 1 << weekday
	return c.Workweek&bit != 0
}

// ParseWorkHours parses start and end times into time values for a given date.
func (c *SchedulerColumn) ParseWorkHours(date time.Time) (start, end time.Time, err error) {
	loc := date.Location()

	startTime, err := time.ParseInLocation("15:04", c.StartTime, loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start time: %w", err)
	}

	endTime, err := time.ParseInLocation("15:04", c.EndTime, loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end time: %w", err)
	}

	start = time.Date(date.Year(), date.Month(), date.Day(),
		startTime.Hour(), startTime.Minute(), 0, 0, loc)
	end = time.Date(date.Year(), date.Month(), date.Day(),
		endTime.Hour(), endTime.Minute(), 0, 0, loc)

	return start, end, nil
}

// FormatSlotTime formats a time for the AvailableSlot response.
func FormatSlotTime(t time.Time) string {
	return t.Format("3:04 PM")
}

// FormatSlotDateTime formats a time for ISO booking format.
func FormatSlotDateTime(t time.Time) string {
	return t.Format("2006-01-02T15:04")
}

// AllowedColumns defines the column IDs we expose for scheduling.
// Only columns for Dr. Bach, Dr. Licht, and Dr. Noel across all facilities.
var AllowedColumns = map[string]bool{
	// Dr. Bach (prof1135)
	"1707": true, // A BACH HOLLYWOOD
	"1708": true, // A BACH OVERFLOW (Sweetwater)
	"1709": true, // A BACH SWEETWATER
	"1710": true, // A BACH VISION (Hollywood)
	"1716": true, // DR. BACH - BP (Spring Hill)
	"1717": true, // DR. BACH HW OVERFLOW (Hollywood)
	// Dr. Licht (prof1141)
	"1714": true, // CR SURGERY SUITE (Crystal River)
	"1715": true, // CRYSTAL RIVER
	"1723": true, // DR. LICHT (Spring Hill)
	"1730": true, // LICHT CR (Crystal River)
	"1731": true, // LICHT WEEK 2 (Crystal River)
	"1732": true, // LICHT WEEK 2 (Crystal River)
	"1736": true, // POST OP 1 (Crystal River)
	"1737": true, // POST OP 2 (Crystal River)
	// Dr. Noel (prof1137)
	"1726": true, // DR. NOEL (Spring Hill)
}

// IsAllowedColumn checks if a column ID is in the allowed list.
func IsAllowedColumn(columnID string) bool {
	return AllowedColumns[columnID]
}

// IsBlockedByHold checks if a time slot falls within any block hold.
func IsBlockedByHold(slotTime time.Time, holds []BlockHold) bool {
	for _, hold := range holds {
		// Slot is blocked if it starts during the hold period
		if !slotTime.Before(hold.StartDateTime) && slotTime.Before(hold.EndDateTime) {
			return true
		}
	}
	return false
}

// OfficeFacilityMap maps normalized office names to AMD facility IDs.
// Keys are pre-normalized (lowercase, no punctuation) for use with NormalizeForLookup.
var OfficeFacilityMap = map[string]string{
	"springhill":    "1032", // ABITA EYE GROUP SPRING HILL
	"spring hill":   "1032",
	"spring":        "1032",
	"sh":            "1032",
	"hollywood":     "4", // ABITA EYE GROUP HOLLYWOOD
	"hw":            "4",
	"sweetwater":    "1031", // ABITA EYE GROUP SWEETWATER
	"sweet water":   "1031",
	"sw":            "1031",
	"crystalriver":  "1033", // ABITA EYE GROUP CRYSTAL RIVER
	"crystal river": "1033",
	"crystal":       "1033",
	"cr":            "1033",
	"coralsprings":  "1034", // ABITA EYE GROUP CORAL SPRINGS
	"coral springs": "1034",
	"coral":         "1034",
	"cs":            "1034",
}

// LookupFacilityID returns facility ID for an office name.
// Uses NormalizeForLookup for tolerance of punctuation, casing, and spacing variations.
func LookupFacilityID(office string) (string, bool) {
	id, ok := OfficeFacilityMap[NormalizeForLookup(office)]
	return id, ok
}

// ValidOfficeNames returns the list of recognized office names for error messages.
func ValidOfficeNames() []string {
	return []string{"Spring Hill", "Hollywood", "Sweetwater", "Crystal River", "Coral Springs"}
}

// ValidProviderNames returns the list of recognized provider names for error messages.
func ValidProviderNames() []string {
	return []string{"Dr. Bach", "Dr. Licht", "Dr. Noel"}
}
