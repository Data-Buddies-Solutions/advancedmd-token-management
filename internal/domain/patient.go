package domain

import (
	"fmt"
	"strings"
	"time"
)

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

// CarrierMap maps insurance provider names (lowercase) to AMD carrier IDs.
var CarrierMap = map[string]string{
	"blue cross blue shield": "car7077",
	"bcbs":                   "car7077",
	"bcbs federal":           "car7554",
	"bcbs ma":                "car7555",
	"aetna":                  "car63046",
	"cigna":                  "car7147",
	"united healthcare":      "car7545",
	"uhc":                    "car7545",
	"medicare":               "car7129",
	"medicaid":               "car7489",
	"tricare":                "car7524",
	"tricare for life":       "car7160",
}

// LookupCarrierID performs a case-insensitive lookup into CarrierMap.
func LookupCarrierID(providerName string) (string, bool) {
	id, ok := CarrierMap[strings.ToLower(strings.TrimSpace(providerName))]
	return id, ok
}

// ParseFirstName extracts the first name from AMD's "LASTNAME,FIRSTNAME" format.
func ParseFirstName(fullName string) string {
	parts := strings.SplitN(fullName, ",", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}
	return ""
}
