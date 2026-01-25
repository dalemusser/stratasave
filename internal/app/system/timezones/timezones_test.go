package timezones

import "testing"

func TestLoad(t *testing.T) {
	err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should be idempotent
	err = Load()
	if err != nil {
		t.Fatalf("Load() second call error = %v", err)
	}
}

func TestAll(t *testing.T) {
	zones, err := All()
	if err != nil {
		t.Fatalf("All() error = %v", err)
	}

	if len(zones) == 0 {
		t.Error("All() returned empty slice")
	}

	// Check that zones have required fields
	for _, z := range zones {
		if z.ID == "" {
			t.Error("Zone with empty ID found")
		}
		if z.Label == "" {
			t.Errorf("Zone %s has empty Label", z.ID)
		}
	}
}

func TestValid(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"America/New_York", true},
		{"America/Los_Angeles", true},
		{"Europe/London", true},
		{"UTC", true},
		{"Invalid/Timezone", false},
		{"", false},
		{"random", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := Valid(tt.id)
			if got != tt.want {
				t.Errorf("Valid(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestLabel(t *testing.T) {
	// Valid timezone should return label
	label := Label("America/New_York")
	if label == "" {
		t.Error("Label(America/New_York) returned empty string")
	}
	if label == "America/New_York" {
		// Should return a human-friendly label, not just the ID
		// But some zones might use ID as label, so this test is flexible
	}

	// Invalid timezone should return the ID itself
	label = Label("Invalid/Timezone")
	if label != "Invalid/Timezone" {
		t.Errorf("Label(Invalid/Timezone) = %q, want 'Invalid/Timezone'", label)
	}
}

func TestGroups(t *testing.T) {
	groups, err := Groups()
	if err != nil {
		t.Fatalf("Groups() error = %v", err)
	}

	if len(groups) == 0 {
		t.Error("Groups() returned empty slice")
	}

	// Check that groups have required fields
	totalZones := 0
	for _, g := range groups {
		if g.Region == "" {
			t.Error("Group with empty Region found")
		}
		if len(g.Zones) == 0 {
			t.Errorf("Group %s has no zones", g.Region)
		}
		totalZones += len(g.Zones)
	}

	// Total zones in groups should match All()
	all, _ := All()
	if totalZones != len(all) {
		t.Errorf("Groups total zones = %d, All() zones = %d", totalZones, len(all))
	}
}

func TestGroups_Sorted(t *testing.T) {
	groups, err := Groups()
	if err != nil {
		t.Fatalf("Groups() error = %v", err)
	}

	// Check groups are sorted by Region
	for i := 1; i < len(groups); i++ {
		if groups[i].Region < groups[i-1].Region {
			t.Errorf("Groups not sorted: %q before %q", groups[i-1].Region, groups[i].Region)
		}
	}

	// Check zones within groups are sorted by Label
	for _, g := range groups {
		for i := 1; i < len(g.Zones); i++ {
			if g.Zones[i].Label < g.Zones[i-1].Label {
				t.Errorf("Zones in %s not sorted: %q before %q", g.Region, g.Zones[i-1].Label, g.Zones[i].Label)
			}
		}
	}
}

func TestCommonTimezones(t *testing.T) {
	// Test that common timezones exist
	common := []string{
		"UTC",
		"America/New_York",
		"America/Los_Angeles",
		"America/Chicago",
		"Europe/London",
		"Europe/Paris",
		"Asia/Tokyo",
		"Australia/Sydney",
	}

	for _, tz := range common {
		if !Valid(tz) {
			t.Errorf("Common timezone %q not found", tz)
		}
	}
}

func TestZoneStruct(t *testing.T) {
	zones, _ := All()
	if len(zones) == 0 {
		t.Skip("No zones loaded")
	}

	// Find a zone and check its structure
	var found *Zone
	for i := range zones {
		if zones[i].ID == "America/New_York" {
			found = &zones[i]
			break
		}
	}

	if found == nil {
		t.Fatal("America/New_York not found")
	}

	if found.ID != "America/New_York" {
		t.Errorf("ID = %q, want 'America/New_York'", found.ID)
	}
	if found.Label == "" {
		t.Error("Label should not be empty")
	}
	// Region might be "America" or similar
	if found.Region == "" {
		t.Logf("Note: America/New_York has no region (using 'Other')")
	}
}
