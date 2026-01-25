package settingsstore

import (
	"testing"

	"github.com/dalemusser/stratasave/internal/domain/models"
	"github.com/dalemusser/stratasave/internal/testutil"
)

func TestStore_Get_DefaultSettings(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Get settings when none exist - should return defaults
	settings, err := store.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if settings.SiteName != models.DefaultSiteName {
		t.Errorf("Get() default SiteName = %q, want %q", settings.SiteName, models.DefaultSiteName)
	}
	if settings.LandingTitle != models.DefaultLandingTitle {
		t.Errorf("Get() default LandingTitle = %q, want %q", settings.LandingTitle, models.DefaultLandingTitle)
	}
}

func TestStore_Save_And_Get(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Save settings
	settings := models.SiteSettings{
		SiteName:       "Test Site",
		LandingTitle:   "Welcome to Test",
		LandingContent: "This is test content",
		FooterHTML:     "<p>Test Footer</p>",
	}

	err := store.Save(ctx, settings)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Get and verify
	retrieved, err := store.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if retrieved.SiteName != settings.SiteName {
		t.Errorf("Get() SiteName = %q, want %q", retrieved.SiteName, settings.SiteName)
	}
	if retrieved.LandingTitle != settings.LandingTitle {
		t.Errorf("Get() LandingTitle = %q, want %q", retrieved.LandingTitle, settings.LandingTitle)
	}
	if retrieved.LandingContent != settings.LandingContent {
		t.Errorf("Get() LandingContent = %q, want %q", retrieved.LandingContent, settings.LandingContent)
	}
	if retrieved.FooterHTML != settings.FooterHTML {
		t.Errorf("Get() FooterHTML = %q, want %q", retrieved.FooterHTML, settings.FooterHTML)
	}
	if retrieved.UpdatedAt == nil {
		t.Error("Get() UpdatedAt should be set after Save()")
	}
}

func TestStore_Save_Update(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Save initial settings
	settings := models.SiteSettings{
		SiteName:     "Initial Site",
		LandingTitle: "Initial Title",
	}

	err := store.Save(ctx, settings)
	if err != nil {
		t.Fatalf("Save() initial error = %v", err)
	}

	// Update settings
	settings.SiteName = "Updated Site"
	settings.LandingTitle = "Updated Title"

	err = store.Save(ctx, settings)
	if err != nil {
		t.Fatalf("Save() update error = %v", err)
	}

	// Verify update
	retrieved, err := store.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if retrieved.SiteName != "Updated Site" {
		t.Errorf("Get() after update SiteName = %q, want %q", retrieved.SiteName, "Updated Site")
	}
	if retrieved.LandingTitle != "Updated Title" {
		t.Errorf("Get() after update LandingTitle = %q, want %q", retrieved.LandingTitle, "Updated Title")
	}
}

func TestStore_Exists(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Should not exist initially
	exists, err := store.Exists(ctx)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Exists() should return false when no settings saved")
	}

	// Save settings
	err = store.Save(ctx, models.SiteSettings{SiteName: "Test"})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Should exist now
	exists, err = store.Exists(ctx)
	if err != nil {
		t.Fatalf("Exists() after save error = %v", err)
	}
	if !exists {
		t.Error("Exists() should return true after Save()")
	}
}

func TestStore_Upsert(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Upsert when no settings exist
	input := UpdateInput{
		SiteName:       "Upsert Site",
		LandingTitle:   "Upsert Title",
		LandingContent: "Upsert Content",
		FooterHTML:     "<p>Upsert Footer</p>",
		LogoPath:       "logos/test.png",
		LogoName:       "test.png",
	}

	err := store.Upsert(ctx, input)
	if err != nil {
		t.Fatalf("Upsert() insert error = %v", err)
	}

	// Verify
	settings, err := store.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if settings.SiteName != input.SiteName {
		t.Errorf("Get() SiteName = %q, want %q", settings.SiteName, input.SiteName)
	}
	if settings.LogoPath != input.LogoPath {
		t.Errorf("Get() LogoPath = %q, want %q", settings.LogoPath, input.LogoPath)
	}
	if settings.LogoName != input.LogoName {
		t.Errorf("Get() LogoName = %q, want %q", settings.LogoName, input.LogoName)
	}

	// Upsert to update
	input.SiteName = "Updated Upsert Site"
	input.LogoPath = ""
	input.LogoName = ""

	err = store.Upsert(ctx, input)
	if err != nil {
		t.Fatalf("Upsert() update error = %v", err)
	}

	// Verify update
	settings, err = store.Get(ctx)
	if err != nil {
		t.Fatalf("Get() after update error = %v", err)
	}

	if settings.SiteName != "Updated Upsert Site" {
		t.Errorf("Get() updated SiteName = %q, want %q", settings.SiteName, "Updated Upsert Site")
	}
	// Logo should be cleared
	if settings.LogoPath != "" {
		t.Errorf("Get() LogoPath should be empty, got %q", settings.LogoPath)
	}
}

func TestStore_Singleton(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Save multiple times - should always update the same document
	for i := 0; i < 3; i++ {
		err := store.Save(ctx, models.SiteSettings{
			SiteName: "Site " + string(rune('A'+i)),
		})
		if err != nil {
			t.Fatalf("Save() iteration %d error = %v", i, err)
		}
	}

	// Should still only have one document
	settings, err := store.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// Should have the last saved value
	if settings.SiteName != "Site C" {
		t.Errorf("Get() SiteName = %q, want %q", settings.SiteName, "Site C")
	}

	// Verify only one document exists by checking Exists
	exists, err := store.Exists(ctx)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() should return true")
	}
}
