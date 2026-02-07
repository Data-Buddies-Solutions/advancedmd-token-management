package domain

import "testing"

func TestStripPatientPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"pat123", "123"},
		{"pat45", "45"},
		{"123", "123"},      // No prefix
		{"patient1", "ient1"}, // Only strips "pat"
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := StripPatientPrefix(tt.input)
			if got != tt.expected {
				t.Errorf("StripPatientPrefix(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalizeDOB(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"already correct format", "01/15/1980", "01/15/1980"},
		{"ISO format", "1980-01-15", "01/15/1980"},
		{"dash format", "01-15-1980", "01/15/1980"},
		{"single digit month/day", "1/5/1980", "01/05/1980"},
		{"full month name", "January 15 1980", "01/15/1980"},
		{"full month with comma", "January 15, 1980", "01/15/1980"},
		{"short month name", "Jan 15 1980", "01/15/1980"},
		{"short month with comma", "Jan 15, 1980", "01/15/1980"},
		{"unknown format returns as-is", "15.01.1980", "15.01.1980"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeDOB(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeDOB(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalizeForLookup(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"lowercase and trim", "  Cigna  ", "cigna"},
		{"strips periods", "B.C.B.S.", "bcbs"},
		{"strips commas", "Blue Cross, Blue Shield", "blue cross blue shield"},
		{"replaces slashes with space", "Blue Cross/Blue Shield", "blue cross blue shield"},
		{"collapses multiple spaces", "blue   cross", "blue cross"},
		{"combined normalizations", " B.C.B.S. / of Florida ", "bcbs of florida"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeForLookup(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeForLookup(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLookupCarrierID(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantID     string
		wantFound  bool
	}{
		{"exact match lowercase", "cigna", "car7147", true},
		{"case insensitive", "CIGNA", "car7147", true},
		{"with periods (BCBS)", "B.C.B.S.", "car7077", true},
		{"slash variation", "Blue Cross/Blue Shield", "car7077", true},
		{"with whitespace", "  Aetna  ", "car63046", true},
		{"medicare part b", "Medicare Part B", "car7129", true},
		{"blue cross blue shield", "Blue Cross Blue Shield", "car7077", true},
		{"cigna healthcare", "Cigna Healthcare", "car7147", true},
		{"aetna insurance", "Aetna Insurance", "car63046", true},
		{"unknown carrier", "unknown", "", false},
		{"empty string", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotFound := LookupCarrierID(tt.input)
			if gotFound != tt.wantFound {
				t.Errorf("LookupCarrierID(%q) found = %v, want %v", tt.input, gotFound, tt.wantFound)
			}
			if gotID != tt.wantID {
				t.Errorf("LookupCarrierID(%q) id = %q, want %q", tt.input, gotID, tt.wantID)
			}
		})
	}
}

func TestParseFirstName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"SMITH,JOHN", "JOHN"},
		{"DOE,JANE MARIE", "JANE MARIE"},
		{"SMITH, JOHN", "JOHN"}, // With space after comma
		{"SMITH", ""},           // No comma
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseFirstName(tt.input)
			if got != tt.expected {
				t.Errorf("ParseFirstName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
