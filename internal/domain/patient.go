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
