package domain

import (
	"log"
	"strings"
)

// OfficeConfig defines the configuration for a single office location.
type OfficeConfig struct {
	ID               string                   // "spring_hill"
	DisplayName      string                   // "Spring Hill"
	FacilityID       string                   // "1568"
	DefaultProfileID string                   // "620" (for addpatient XMLRPC)
	Columns          map[string]OfficeColumn  // column ID → config
	RoutingTiers     map[RoutingRule][]string  // routing rule → column IDs
	PediatricRouting RoutingRule              // routing override for under-18
}

// OfficeColumn defines a provider column within an office.
type OfficeColumn struct {
	ProfileID   string // "620"
	DisplayName string // "Dr. Austin Bach"
	ShortName   string // "Dr. Bach"
	MatchKey    string // "BACH" — uppercase fragment for matching AMD names
}

// IsAllowedColumn checks if a column ID belongs to this office.
func (o *OfficeConfig) IsAllowedColumn(columnID string) bool {
	_, ok := o.Columns[columnID]
	return ok
}

// AllowedColumnIDs returns all column IDs for this office.
func (o *OfficeConfig) AllowedColumnIDs() []string {
	ids := make([]string, 0, len(o.Columns))
	for id := range o.Columns {
		ids = append(ids, id)
	}
	return ids
}

// ColumnsForRouting returns the allowed column IDs for a routing rule at this office.
func (o *OfficeConfig) ColumnsForRouting(rule RoutingRule) map[string]bool {
	if rule == RoutingNotAccepted {
		return nil
	}

	colIDs, ok := o.RoutingTiers[rule]
	if !ok {
		// Fall back to all columns for this office
		colIDs = o.RoutingTiers[RoutingAll]
	}

	result := make(map[string]bool, len(colIDs))
	for _, id := range colIDs {
		result[id] = true
	}
	return result
}

// ProvidersForRouting returns the display names for a routing rule at this office.
func (o *OfficeConfig) ProvidersForRouting(rule RoutingRule) []string {
	if rule == RoutingNotAccepted {
		return nil
	}

	colIDs, ok := o.RoutingTiers[rule]
	if !ok {
		colIDs = o.RoutingTiers[RoutingAll]
	}

	names := make([]string, 0, len(colIDs))
	for _, id := range colIDs {
		if col, ok := o.Columns[id]; ok {
			names = append(names, col.ShortName)
		}
	}
	return names
}

// ValidProviderNames returns all provider short names for this office.
func (o *OfficeConfig) ValidProviderNames() []string {
	names := make([]string, 0, len(o.Columns))
	for _, col := range o.Columns {
		names = append(names, col.ShortName)
	}
	return names
}

// ProviderDisplayName returns the display name for a profile ID.
func (o *OfficeConfig) ProviderDisplayName(profileID string) string {
	for _, col := range o.Columns {
		if col.ProfileID == profileID {
			return col.DisplayName
		}
	}
	return ""
}

// FriendlyProviderName maps an AMD provider name to a friendly display name.
func (o *OfficeConfig) FriendlyProviderName(amdName string) string {
	upper := strings.ToUpper(amdName)
	for _, col := range o.Columns {
		if col.MatchKey != "" && strings.Contains(upper, col.MatchKey) {
			return col.DisplayName
		}
	}
	return amdName
}

// AppointmentColor returns the booking color for an appointment type ID.
func (o *OfficeConfig) AppointmentColor(typeID int) (string, bool) {
	color, ok := DefaultAppointmentTypeColors[typeID]
	return color, ok
}

// AppointmentTypeName returns the friendly name for an appointment type ID.
func (o *OfficeConfig) AppointmentTypeName(typeID int) (string, bool) {
	name, ok := DefaultAppointmentTypeNames[typeID]
	return name, ok
}

// DefaultAppointmentTypeColors maps AMD appointment type IDs to booking colors.
var DefaultAppointmentTypeColors = map[int]string{
	1006: "RED",    // New Adult Medical
	1004: "GREEN",  // New Pediatric Medical
	1007: "ORANGE", // Established Adult Medical (Follow Up)
	1005: "PINK",   // Established Pediatric Medical (Follow Up)
	1008: "BLUE",   // Post Op
}

// DefaultAppointmentTypeNames maps AMD appointment type IDs to friendly names.
var DefaultAppointmentTypeNames = map[int]string{
	1006: "New Adult Medical",
	1004: "New Pediatric Medical",
	1007: "Established Adult Medical (Follow Up)",
	1005: "Established Pediatric Medical (Follow Up)",
	1008: "Post Op",
}

// devAppointmentTypes maps prod type IDs to dev type IDs.
// Only used when AMD_ENV=dev; in prod the IDs pass through unchanged.
var devAppointmentTypes = map[int]int{
	1006: 12,   // New Adult Medical
	1004: 20,   // New Pediatric Medical
	1007: 18,   // Established Adult Medical (Follow Up)
	1005: 8,    // Established Pediatric Medical (Follow Up)
	1008: 1627, // Post Op
}

// isDevEnv tracks whether we're running in dev mode. Set by InitRegistry.
var isDevEnv bool

// ResolveAppointmentTypeID translates a prod type ID to the env-specific ID.
// In prod, returns the ID unchanged. In dev, maps to the dev ID.
func ResolveAppointmentTypeID(typeID int) (int, bool) {
	if _, ok := DefaultAppointmentTypeColors[typeID]; !ok {
		return 0, false
	}
	if isDevEnv {
		devID, ok := devAppointmentTypes[typeID]
		return devID, ok
	}
	return typeID, true
}

// prodOffices contains office configs with production AMD IDs.
var prodOffices = map[string]*OfficeConfig{
	"spring_hill": {
		ID:               "spring_hill",
		DisplayName:      "Spring Hill",
		FacilityID:       "1568",
		DefaultProfileID: "620",
		Columns: map[string]OfficeColumn{
			"1513": {ProfileID: "620", DisplayName: "Dr. Austin Bach", ShortName: "Dr. Bach", MatchKey: "BACH"},
			"1551": {ProfileID: "2064", DisplayName: "Dr. J. Licht", ShortName: "Dr. Licht", MatchKey: "LICHT"},
			"1550": {ProfileID: "2076", DisplayName: "Dr. D. Noel", ShortName: "Dr. Noel", MatchKey: "NOEL"},
		},
		RoutingTiers: map[RoutingRule][]string{
			RoutingBachOnly:  {"1513"},
			RoutingBachLicht: {"1513", "1551"},
			RoutingAll:       {"1513", "1551", "1550"},
		},
		PediatricRouting: RoutingBachOnly,
	},
}

// devOffices contains office configs with dev AMD IDs.
var devOffices = map[string]*OfficeConfig{
	"spring_hill": {
		ID:               "spring_hill",
		DisplayName:      "Spring Hill",
		FacilityID:       "1032",
		DefaultProfileID: "1135",
		Columns: map[string]OfficeColumn{
			"1716": {ProfileID: "1135", DisplayName: "Dr. Austin Bach", ShortName: "Dr. Bach", MatchKey: "BACH"},
			"1723": {ProfileID: "1141", DisplayName: "Dr. J. Licht", ShortName: "Dr. Licht", MatchKey: "LICHT"},
			"1726": {ProfileID: "1137", DisplayName: "Dr. D. Noel", ShortName: "Dr. Noel", MatchKey: "NOEL"},
		},
		RoutingTiers: map[RoutingRule][]string{
			RoutingBachOnly:  {"1716"},
			RoutingBachLicht: {"1716", "1723"},
			RoutingAll:       {"1716", "1723", "1726"},
		},
		PediatricRouting: RoutingBachOnly,
	},
}

// OfficeRegistry maps canonical office IDs to their configurations.
// Defaults to prod; call InitRegistry to switch environments.
var OfficeRegistry = prodOffices

// InitRegistry sets the active office registry based on the AMD_ENV value.
// "dev" loads dev AMD IDs; anything else (including empty) loads prod.
func InitRegistry(env string) {
	switch env {
	case "dev":
		OfficeRegistry = devOffices
		isDevEnv = true
		log.Printf("Office registry: dev")
	default:
		OfficeRegistry = prodOffices
		isDevEnv = false
		log.Printf("Office registry: prod")
	}
	rebuildPhoneMap()
}

// rebuildPhoneMap reconstructs PhoneToOffice from the active OfficeRegistry.
func rebuildPhoneMap() {
	PhoneToOffice = make(map[string]string)
	for id := range OfficeRegistry {
		for phone, target := range phoneMap {
			if target == id {
				PhoneToOffice[phone] = target
			}
		}
	}
}

// phoneMap is the master phone→office mapping used to rebuild PhoneToOffice.
var phoneMap = map[string]string{
	"17275919997": "spring_hill", // prod: +1 (727) 591-9997
	"7275919997":  "spring_hill",
	"14843989071": "spring_hill", // dev: +1 (484) 398-9071
	"4843989071":  "spring_hill",
}

// OfficeAliases maps normalized aliases to canonical office IDs.
var OfficeAliases = map[string]string{
	"spring_hill":  "spring_hill",
	"springhill":   "spring_hill",
	"spring hill":  "spring_hill",
	"spring":       "spring_hill",
	"sh":           "spring_hill",
}

// PhoneToOffice maps phone numbers (digits only) to canonical office IDs.
// Rebuilt by InitRegistry from phoneMap.
var PhoneToOffice = map[string]string{
	"17275919997": "spring_hill",
	"7275919997":  "spring_hill",
	"14843989071": "spring_hill",
	"4843989071":  "spring_hill",
}

// StripToDigits removes all non-digit characters from a string.
func StripToDigits(s string) string {
	var b strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// NormalizePhoneDigits strips a phone number to digits and removes the
// leading US country code ("1") if the result is 11 digits. AMD stores
// 10-digit numbers and won't match on 11.
func NormalizePhoneDigits(s string) string {
	digits := StripToDigits(s)
	if len(digits) == 11 && digits[0] == '1' {
		return digits[1:]
	}
	return digits
}

// LookupOffice resolves an office name, alias, or phone number to its config.
func LookupOffice(name string) (*OfficeConfig, bool) {
	normalized := NormalizeForLookup(name)
	// Also try underscore variant
	underscored := strings.ReplaceAll(normalized, " ", "_")

	// Check aliases first
	for _, key := range []string{normalized, underscored} {
		if canonical, ok := OfficeAliases[key]; ok {
			if office, ok := OfficeRegistry[canonical]; ok {
				return office, true
			}
		}
	}

	// Direct registry lookup
	for _, key := range []string{normalized, underscored} {
		if office, ok := OfficeRegistry[key]; ok {
			return office, true
		}
	}

	// Phone number lookup — strip to digits and check
	digits := StripToDigits(name)
	if len(digits) >= 10 {
		if canonical, ok := PhoneToOffice[digits]; ok {
			if office, ok := OfficeRegistry[canonical]; ok {
				return office, true
			}
		}
	}

	return nil, false
}

// DefaultOffice returns the default office config (Spring Hill).
func DefaultOffice() *OfficeConfig {
	return OfficeRegistry["spring_hill"]
}

// LookupOfficeByColumnID finds the office that contains a given column ID.
func LookupOfficeByColumnID(columnID string) (*OfficeConfig, bool) {
	for _, office := range OfficeRegistry {
		if _, ok := office.Columns[columnID]; ok {
			return office, true
		}
	}
	return nil, false
}

// ValidOfficeNames returns the list of recognized office display names.
func ValidOfficeNames() []string {
	names := make([]string, 0, len(OfficeRegistry))
	for _, office := range OfficeRegistry {
		names = append(names, office.DisplayName)
	}
	return names
}
