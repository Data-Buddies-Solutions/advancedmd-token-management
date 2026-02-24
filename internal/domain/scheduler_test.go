package domain

import "testing"

func TestLookupFacilityID(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantID    string
		wantFound bool
	}{
		{"exact match", "spring hill", "1568", true},
		{"case insensitive", "Spring Hill", "1568", true},
		{"abbreviation sh", "SH", "1568", true},
		{"no space", "springhill", "1568", true},
		{"hollywood", "Hollywood", "4", true},
		{"abbreviation hw", "hw", "4", true},
		{"sweetwater", "Sweetwater", "1031", true},
		{"sweet water split", "Sweet Water", "1031", true},
		{"abbreviation sw", "SW", "1031", true},
		{"crystal river", "Crystal River", "1033", true},
		{"abbreviation cr", "CR", "1033", true},
		{"coral springs", "Coral Springs", "1034", true},
		{"abbreviation cs", "CS", "1034", true},
		{"with periods", "S.H.", "1568", true},
		{"unknown office", "unknown", "", false},
		{"empty string", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotFound := LookupFacilityID(tt.input)
			if gotFound != tt.wantFound {
				t.Errorf("LookupFacilityID(%q) found = %v, want %v", tt.input, gotFound, tt.wantFound)
			}
			if gotID != tt.wantID {
				t.Errorf("LookupFacilityID(%q) id = %q, want %q", tt.input, gotID, tt.wantID)
			}
		})
	}
}
