package domain

import "strings"

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

// OfficeRegistry maps canonical office IDs to their configurations.
var OfficeRegistry = map[string]*OfficeConfig{
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

// OfficeAliases maps normalized aliases to canonical office IDs.
var OfficeAliases = map[string]string{
	"spring_hill":  "spring_hill",
	"springhill":   "spring_hill",
	"spring hill":  "spring_hill",
	"spring":       "spring_hill",
	"sh":           "spring_hill",
}

// PhoneToOffice maps phone numbers (digits only) to canonical office IDs.
// Supports both 10-digit and 11-digit (with country code) formats.
var PhoneToOffice = map[string]string{
	"17275919997": "spring_hill", // +1 (727) 591-9997
	"7275919997":  "spring_hill",
}

// stripToDigits removes all non-digit characters from a string.
func stripToDigits(s string) string {
	var b strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
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
	digits := stripToDigits(name)
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
