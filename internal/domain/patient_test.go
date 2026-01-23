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
