package status

import "testing"

func TestIsValid(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{Active, true},
		{Disabled, true},
		{"active", true},
		{"disabled", true},
		{"ACTIVE", false},
		{"DISABLED", false},
		{"pending", false},
		{"inactive", false},
		{"", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := IsValid(tt.status)
			if got != tt.want {
				t.Errorf("IsValid(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestDefault(t *testing.T) {
	got := Default()
	if got != Active {
		t.Errorf("Default() = %q, want %q", got, Active)
	}
}

func TestConstants(t *testing.T) {
	if Active != "active" {
		t.Errorf("Active = %q, want 'active'", Active)
	}
	if Disabled != "disabled" {
		t.Errorf("Disabled = %q, want 'disabled'", Disabled)
	}
}
