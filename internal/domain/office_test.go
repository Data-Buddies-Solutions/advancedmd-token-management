package domain

import "testing"

func TestLookupOffice(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantID  string
		wantOK  bool
	}{
		{"canonical ID", "spring_hill", "spring_hill", true},
		{"alias springhill", "springhill", "spring_hill", true},
		{"alias spring hill", "spring hill", "spring_hill", true},
		{"alias spring", "spring", "spring_hill", true},
		{"alias sh", "sh", "spring_hill", true},
		{"case insensitive", "Spring Hill", "spring_hill", true},
		{"case insensitive alias", "SH", "spring_hill", true},
		{"phone E.164", "+17275919997", "spring_hill", true},
		{"phone 11 digits", "17275919997", "spring_hill", true},
		{"phone 10 digits", "7275919997", "spring_hill", true},
		{"phone formatted", "(727) 591-9997", "spring_hill", true},
		{"phone with country code formatted", "+1 (727) 591-9997", "spring_hill", true},
		{"unknown phone", "+15551234567", "", false},
		{"unknown office", "unknown", "", false},
		{"empty string", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			office, ok := LookupOffice(tt.input)
			if ok != tt.wantOK {
				t.Errorf("LookupOffice(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
				return
			}
			if ok && office.ID != tt.wantID {
				t.Errorf("LookupOffice(%q).ID = %q, want %q", tt.input, office.ID, tt.wantID)
			}
		})
	}
}

func TestDefaultOffice(t *testing.T) {
	office := DefaultOffice()
	if office == nil {
		t.Fatal("DefaultOffice() returned nil")
	}
	if office.ID != "spring_hill" {
		t.Errorf("DefaultOffice().ID = %q, want %q", office.ID, "spring_hill")
	}
	if office.FacilityID != "1568" {
		t.Errorf("DefaultOffice().FacilityID = %q, want %q", office.FacilityID, "1568")
	}
}

func TestOfficeConfig_IsAllowedColumn(t *testing.T) {
	office := DefaultOffice()

	tests := []struct {
		columnID string
		want     bool
	}{
		{"1513", true},
		{"1551", true},
		{"1550", true},
		{"9999", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.columnID, func(t *testing.T) {
			got := office.IsAllowedColumn(tt.columnID)
			if got != tt.want {
				t.Errorf("IsAllowedColumn(%q) = %v, want %v", tt.columnID, got, tt.want)
			}
		})
	}
}

func TestOfficeConfig_AllowedColumnIDs(t *testing.T) {
	office := DefaultOffice()
	ids := office.AllowedColumnIDs()

	if len(ids) != 3 {
		t.Fatalf("AllowedColumnIDs() len = %d, want 3", len(ids))
	}

	// Check all expected IDs are present
	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}
	for _, want := range []string{"1513", "1551", "1550"} {
		if !idSet[want] {
			t.Errorf("AllowedColumnIDs() missing %q", want)
		}
	}
}

func TestOfficeConfig_ProviderDisplayName(t *testing.T) {
	office := DefaultOffice()

	tests := []struct {
		profileID string
		want      string
	}{
		{"620", "Dr. Austin Bach"},
		{"2064", "Dr. J. Licht"},
		{"2076", "Dr. D. Noel"},
		{"9999", ""},
	}

	for _, tt := range tests {
		t.Run(tt.profileID, func(t *testing.T) {
			got := office.ProviderDisplayName(tt.profileID)
			if got != tt.want {
				t.Errorf("ProviderDisplayName(%q) = %q, want %q", tt.profileID, got, tt.want)
			}
		})
	}
}

func TestOfficeConfig_FriendlyProviderName(t *testing.T) {
	office := DefaultOffice()

	tests := []struct {
		input string
		want  string
	}{
		{"BACH, AUSTIN", "Dr. Austin Bach"},
		{"LICHT, JONATHAN", "Dr. J. Licht"},
		{"NOEL, DON HERSHELSON", "Dr. D. Noel"},
		{"UNKNOWN", "UNKNOWN"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := office.FriendlyProviderName(tt.input)
			if got != tt.want {
				t.Errorf("FriendlyProviderName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestOfficeConfig_AppointmentColor(t *testing.T) {
	office := DefaultOffice()

	color, ok := office.AppointmentColor(1006)
	if !ok || color != "RED" {
		t.Errorf("AppointmentColor(1006) = (%q, %v), want (RED, true)", color, ok)
	}

	_, ok = office.AppointmentColor(9999)
	if ok {
		t.Error("AppointmentColor(9999) should return false")
	}
}

func TestLookupOfficeByColumnID(t *testing.T) {
	tests := []struct {
		columnID string
		wantID   string
		wantOK   bool
	}{
		{"1513", "spring_hill", true},
		{"1551", "spring_hill", true},
		{"1550", "spring_hill", true},
		{"9999", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.columnID, func(t *testing.T) {
			office, ok := LookupOfficeByColumnID(tt.columnID)
			if ok != tt.wantOK {
				t.Errorf("LookupOfficeByColumnID(%q) ok = %v, want %v", tt.columnID, ok, tt.wantOK)
				return
			}
			if ok && office.ID != tt.wantID {
				t.Errorf("LookupOfficeByColumnID(%q).ID = %q, want %q", tt.columnID, office.ID, tt.wantID)
			}
		})
	}
}

func TestValidOfficeNames(t *testing.T) {
	names := ValidOfficeNames()
	if len(names) == 0 {
		t.Fatal("ValidOfficeNames() returned empty list")
	}

	found := false
	for _, n := range names {
		if n == "Spring Hill" {
			found = true
		}
	}
	if !found {
		t.Errorf("ValidOfficeNames() missing 'Spring Hill': %v", names)
	}
}
