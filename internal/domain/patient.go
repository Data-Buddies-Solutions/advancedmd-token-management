package domain

import (
	"fmt"
	"strings"
	"time"
)

// NormalizeForLookup normalizes input strings for fuzzy map lookups.
// Strips punctuation (periods, commas), replaces slashes with spaces,
// collapses multiple spaces, lowercases, and trims whitespace.
func NormalizeForLookup(input string) string {
	s := strings.ToLower(strings.TrimSpace(input))
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "/", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// Patient represents a patient record.
type Patient struct {
	ID        string
	FirstName string
	LastName  string
	FullName  string // "LASTNAME,FIRSTNAME" format from AMD
	DOB       string // MM/DD/YYYY
	Phone     string
}

// StripPatientPrefix removes the "pat" prefix from patient IDs.
// AMD returns IDs like "pat45" but the booking API expects just "45".
func StripPatientPrefix(id string) string {
	return strings.TrimPrefix(id, "pat")
}

// NormalizeDOB converts various date formats to MM/DD/YYYY.
func NormalizeDOB(dob string) string {
	// Already in correct format
	if len(dob) == 10 && dob[2] == '/' && dob[5] == '/' {
		return dob
	}

	formats := []string{
		"2006-01-02",
		"01-02-2006",
		"1/2/2006",
		"01/02/2006",
		"January 2 2006",
		"January 2, 2006",
		"Jan 2 2006",
		"Jan 2, 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dob); err == nil {
			return t.Format("01/02/2006")
		}
	}

	return dob
}

// FormatPhone normalizes a phone number to (XXX)XXX-XXXX format.
// Strips all non-digit characters, then formats if exactly 10 digits remain.
func FormatPhone(phone string) string {
	var digits []byte
	for _, c := range phone {
		if c >= '0' && c <= '9' {
			digits = append(digits, byte(c))
		}
	}
	if len(digits) == 10 {
		return fmt.Sprintf("(%s)%s-%s", string(digits[0:3]), string(digits[3:6]), string(digits[6:10]))
	}
	return phone
}

// CarrierMap maps normalized insurance provider names to AMD carrier IDs.
// Keys are pre-normalized (lowercase, no punctuation) for use with NormalizeForLookup.
var CarrierMap = map[string]string{
	// Cigna (car7147)
	"cigna":             "car7147",
	"cigna health":      "car7147",
	"cigna healthcare":  "car7147",
	"cigna health care": "car7147",
	"cigna insurance":   "car7147",
	// Aetna (car63046)
	"aetna":             "car63046",
	"aetna health":      "car63046",
	"aetna health care": "car63046",
	"aetna healthcare":  "car63046",
	"aetna insurance":   "car63046",
	// Medicare (car7129)
	"medicare":               "car7129",
	"medicare part a":        "car7129",
	"medicare part b":        "car7129",
	"medicare parts a and b": "car7129",
	// Medicaid (car7489)
	"medicaid": "car7489",
	// BCBS (car7077)
	"bcbs":                       "car7077",
	"bc bs":                      "car7077",
	"bcbs of":                    "car7077",
	"blue cross":                 "car7077",
	"blue shield":                "car7077",
	"blue cross blue shield":     "car7077",
	"blue cross blue shield of":  "car7077",
	"bluecross":                  "car7077",
	"blueshield":                 "car7077",
	"bluecross blueshield":       "car7077",
	"blue cross and blue shield": "car7077",
	// BCBS Regional
	"bcbs federal": "car7554",
	"bcbs ma":      "car7555",
	// United Healthcare (car7545)
	"united healthcare": "car7545",
	"uhc":               "car7545",
	"united health":     "car7545",
	"united health care": "car7545",
	// Tricare (car7524)
	"tricare":          "car7524",
	"tricare for life": "car7160",
}

// LookupCarrierID performs a normalized lookup into CarrierMap.
// Handles variations like "B.C.B.S.", "Blue Cross/Blue Shield", etc.
func LookupCarrierID(providerName string) (string, bool) {
	id, ok := CarrierMap[NormalizeForLookup(providerName)]
	return id, ok
}

// ValidCarrierNames returns the list of recognized carrier names for error messages.
func ValidCarrierNames() []string {
	return []string{"Cigna", "Aetna", "Medicare", "Medicaid", "BCBS (Blue Cross Blue Shield)", "United Healthcare", "Tricare"}
}

// NormalizeSex converts various sex inputs to AMD's expected format (M/F/U).
func NormalizeSex(sex string) string {
	switch strings.ToUpper(strings.TrimSpace(sex)) {
	case "M", "MALE":
		return "M"
	case "F", "FEMALE":
		return "F"
	default:
		return "U"
	}
}

// ParseFirstName extracts the first name from AMD's "LASTNAME,FIRSTNAME" format.
func ParseFirstName(fullName string) string {
	parts := strings.SplitN(fullName, ",", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}
	return ""
}
