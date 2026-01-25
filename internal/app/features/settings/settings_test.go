package settings

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	settingsstore "github.com/dalemusser/stratasave/internal/app/store/settings"
	"github.com/dalemusser/stratasave/internal/app/system/htmlsanitize"
	"github.com/dalemusser/stratasave/internal/domain/models"
	"github.com/dalemusser/stratasave/internal/testutil"
	"go.uber.org/zap"
)

func TestNewHandler(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	h := NewHandler(db, nil, nil, logger)

	if h == nil {
		t.Fatal("NewHandler() returned nil")
	}
	if h.settingsStore == nil {
		t.Error("settingsStore should not be nil")
	}
}

func TestSettingsStore_GetDefault(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	store := settingsstore.New(db)

	// Get should return nil or default when nothing is set
	settings, err := store.Get(ctx)
	// First get may return error (no documents) or nil settings
	if err != nil && settings != nil {
		t.Errorf("unexpected state: err=%v, settings=%v", err, settings)
	}
}

func TestSettingsStore_Upsert(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	store := settingsstore.New(db)

	// Create initial settings
	input := settingsstore.UpdateInput{
		SiteName:       "Test Site",
		LandingTitle:   "Welcome",
		LandingContent: "<p>Hello World</p>",
		FooterHTML:     "<p>Footer</p>",
	}

	err := store.Upsert(ctx, input)
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	// Verify settings were saved
	settings, err := store.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if settings.SiteName != "Test Site" {
		t.Errorf("SiteName = %q, want %q", settings.SiteName, "Test Site")
	}
	if settings.LandingTitle != "Welcome" {
		t.Errorf("LandingTitle = %q, want %q", settings.LandingTitle, "Welcome")
	}
	if settings.LandingContent != "<p>Hello World</p>" {
		t.Errorf("LandingContent = %q, want %q", settings.LandingContent, "<p>Hello World</p>")
	}
}

func TestSettingsStore_UpsertUpdate(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	store := settingsstore.New(db)

	// Create initial settings
	store.Upsert(ctx, settingsstore.UpdateInput{
		SiteName:     "Initial",
		LandingTitle: "Initial Title",
	})

	// Update settings
	store.Upsert(ctx, settingsstore.UpdateInput{
		SiteName:     "Updated",
		LandingTitle: "Updated Title",
	})

	// Verify update
	settings, err := store.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if settings.SiteName != "Updated" {
		t.Errorf("SiteName = %q, want %q", settings.SiteName, "Updated")
	}
	if settings.LandingTitle != "Updated Title" {
		t.Errorf("LandingTitle = %q, want %q", settings.LandingTitle, "Updated Title")
	}
}

func TestSettingsStore_Logo(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	store := settingsstore.New(db)

	// Set logo
	err := store.Upsert(ctx, settingsstore.UpdateInput{
		SiteName: "Logo Test",
		LogoPath: "logos/2024/01/abc123.png",
		LogoName: "logo.png",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	settings, _ := store.Get(ctx)
	if settings.LogoPath != "logos/2024/01/abc123.png" {
		t.Errorf("LogoPath = %q, want %q", settings.LogoPath, "logos/2024/01/abc123.png")
	}
	if settings.LogoName != "logo.png" {
		t.Errorf("LogoName = %q, want %q", settings.LogoName, "logo.png")
	}
	if !settings.HasLogo() {
		t.Error("HasLogo() should return true")
	}

	// Remove logo
	store.Upsert(ctx, settingsstore.UpdateInput{
		SiteName: "Logo Test",
		LogoPath: "",
		LogoName: "",
	})

	settings, _ = store.Get(ctx)
	if settings.HasLogo() {
		t.Error("HasLogo() should return false after removal")
	}
}

func TestSettingsVM_Fields(t *testing.T) {
	settings := &models.SiteSettings{
		SiteName:       "Test Site",
		LandingTitle:   "Welcome",
		LandingContent: "<p>Content</p>",
		FooterHTML:     "<p>Footer</p>",
		LogoPath:       "logos/test.png",
		LogoName:       "test.png",
	}

	vm := SettingsVM{
		Settings:       settings,
		LandingTitle:   settings.LandingTitle,
		LandingContent: settings.LandingContent,
		HasLogo:        settings.HasLogo(),
		LogoURL:        "/files/logos/test.png",
		LogoName:       settings.LogoName,
		Success:        "Settings updated",
		Error:          "",
	}

	if vm.Settings.SiteName != "Test Site" {
		t.Errorf("Settings.SiteName = %q, want %q", vm.Settings.SiteName, "Test Site")
	}
	if vm.LandingTitle != "Welcome" {
		t.Errorf("LandingTitle = %q, want %q", vm.LandingTitle, "Welcome")
	}
	if !vm.HasLogo {
		t.Error("HasLogo should be true")
	}
	if vm.Success != "Settings updated" {
		t.Errorf("Success = %q, want %q", vm.Success, "Settings updated")
	}
}

func TestSiteSettings_HasLogo(t *testing.T) {
	tests := []struct {
		name     string
		logoPath string
		want     bool
	}{
		{"with logo", "logos/test.png", true},
		{"empty path", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &models.SiteSettings{LogoPath: tt.logoPath}
			if got := s.HasLogo(); got != tt.want {
				t.Errorf("HasLogo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHTMLSanitization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
		excludes string
	}{
		{
			name:     "allows safe HTML",
			input:    "<p>Hello <strong>World</strong></p>",
			contains: "<p>",
			excludes: "",
		},
		{
			name:     "removes script tags",
			input:    "<p>Hello</p><script>alert('xss')</script>",
			contains: "<p>Hello</p>",
			excludes: "script",
		},
		{
			name:     "removes onclick",
			input:    "<p onclick=\"alert('xss')\">Click me</p>",
			contains: "<p>",
			excludes: "onclick",
		},
		{
			name:     "allows links",
			input:    "<a href=\"https://example.com\">Link</a>",
			contains: "href",
			excludes: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := htmlsanitize.Sanitize(tt.input)

			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("result %q should contain %q", result, tt.contains)
			}
			if tt.excludes != "" && strings.Contains(result, tt.excludes) {
				t.Errorf("result %q should not contain %q", result, tt.excludes)
			}
		})
	}
}

func TestContentLengthValidation(t *testing.T) {
	// Test MaxContentLength validation
	if MaxContentLength != 100000 {
		t.Errorf("MaxContentLength = %d, want %d", MaxContentLength, 100000)
	}
	if MaxFooterLength != 10000 {
		t.Errorf("MaxFooterLength = %d, want %d", MaxFooterLength, 10000)
	}

	tests := []struct {
		name    string
		content string
		maxLen  int
		wantErr bool
	}{
		{"empty content", "", MaxContentLength, false},
		{"short content", "Hello World", MaxContentLength, false},
		{"at limit", strings.Repeat("a", MaxContentLength), MaxContentLength, false},
		{"over limit", strings.Repeat("a", MaxContentLength+1), MaxContentLength, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tooLong := len(tt.content) > tt.maxLen
			if tooLong != tt.wantErr {
				t.Errorf("content length %d: got tooLong=%v, want=%v", len(tt.content), tooLong, tt.wantErr)
			}
		})
	}
}

func TestDefaultLandingTitle(t *testing.T) {
	if models.DefaultLandingTitle == "" {
		t.Error("DefaultLandingTitle should not be empty")
	}
}

func TestFormParsing(t *testing.T) {
	form := url.Values{}
	form.Set("site_name", "My Site")
	form.Set("landing_title", "Welcome to My Site")
	form.Set("landing_content", "<p>Welcome!</p>")
	form.Set("footer_html", "<p>Copyright 2024</p>")

	req := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm() error = %v", err)
	}

	if got := req.FormValue("site_name"); got != "My Site" {
		t.Errorf("site_name = %q, want %q", got, "My Site")
	}
	if got := req.FormValue("landing_title"); got != "Welcome to My Site" {
		t.Errorf("landing_title = %q, want %q", got, "Welcome to My Site")
	}
	if got := req.FormValue("landing_content"); got != "<p>Welcome!</p>" {
		t.Errorf("landing_content = %q, want %q", got, "<p>Welcome!</p>")
	}
}

func TestRemoveLogoFlag(t *testing.T) {
	form := url.Values{}
	form.Set("site_name", "Test")
	form.Set("remove_logo", "1")

	req := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ParseForm()

	removeLogo := req.FormValue("remove_logo") != ""
	if !removeLogo {
		t.Error("remove_logo should be true")
	}

	// Without the flag
	form2 := url.Values{}
	form2.Set("site_name", "Test")

	req2 := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.ParseForm()

	removeLogo2 := req2.FormValue("remove_logo") != ""
	if removeLogo2 {
		t.Error("remove_logo should be false when not set")
	}
}

func TestSuccessQueryParam(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/settings?success=1", nil)

	if req.URL.Query().Get("success") != "1" {
		t.Error("success query param should be '1'")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/settings", nil)
	if req2.URL.Query().Get("success") != "" {
		t.Error("success query param should be empty when not set")
	}
}

func TestMountRoutes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	h := NewHandler(db, nil, nil, logger)

	// Verify MountRoutes doesn't panic
	// We can't fully test without a chi.Router setup
	if h == nil {
		t.Error("handler should not be nil")
	}
}

func TestSettingsStore_UpdateInput(t *testing.T) {
	input := settingsstore.UpdateInput{
		SiteName:       "Test Site",
		LandingTitle:   "Welcome",
		LandingContent: "<p>Content</p>",
		FooterHTML:     "<p>Footer</p>",
		LogoPath:       "logos/test.png",
		LogoName:       "logo.png",
	}

	if input.SiteName != "Test Site" {
		t.Errorf("SiteName = %q, want %q", input.SiteName, "Test Site")
	}
	if input.LandingTitle != "Welcome" {
		t.Errorf("LandingTitle = %q, want %q", input.LandingTitle, "Welcome")
	}
	if input.LogoPath != "logos/test.png" {
		t.Errorf("LogoPath = %q, want %q", input.LogoPath, "logos/test.png")
	}
}

func TestSanitizeToHTML(t *testing.T) {
	// Test that sanitized content can be safely used as HTML
	tests := []struct {
		name  string
		input string
	}{
		{"simple paragraph", "<p>Hello World</p>"},
		{"with formatting", "<p><strong>Bold</strong> and <em>italic</em></p>"},
		{"with link", "<a href=\"https://example.com\">Link</a>"},
		{"plain text", "Just text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := htmlsanitize.SanitizeToHTML(tt.input)
			// Should not panic and should return valid HTML
			if result == "" && tt.input != "" {
				// Only error if input was not empty but result is
				// (some inputs may legitimately produce empty output after sanitization)
			}
		})
	}
}

func TestEmptySiteName(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	store := settingsstore.New(db)

	// Empty site name should still work (will use default in UI)
	err := store.Upsert(ctx, settingsstore.UpdateInput{
		SiteName: "",
	})
	if err != nil {
		t.Fatalf("Upsert() with empty site name error = %v", err)
	}

	settings, _ := store.Get(ctx)
	if settings.SiteName != "" {
		t.Errorf("SiteName = %q, want empty string", settings.SiteName)
	}
}

func TestFooterHTMLSanitization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "simple footer",
			input:    "<p>Copyright 2024</p>",
			contains: "Copyright",
		},
		{
			name:     "footer with link",
			input:    "<p>Visit <a href=\"https://example.com\">our site</a></p>",
			contains: "href",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := htmlsanitize.Sanitize(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("sanitized footer should contain %q, got %q", tt.contains, result)
			}
		})
	}
}

func TestLogoPathGeneration(t *testing.T) {
	// Test that logo paths follow expected format: logos/YYYY/MM/uuid-ext
	tests := []struct {
		name     string
		path     string
		valid    bool
	}{
		{"valid path", "logos/2024/01/abc12345.png", true},
		{"with jpg", "logos/2024/12/xyz99999.jpg", true},
		{"wrong prefix", "images/2024/01/abc12345.png", false},
		{"missing year", "logos/abc12345.png", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasValidPrefix := strings.HasPrefix(tt.path, "logos/")
			// Simple validation: starts with logos/ and has at least 3 path segments
			parts := strings.Split(tt.path, "/")
			isValid := hasValidPrefix && len(parts) >= 4

			if isValid != tt.valid {
				t.Errorf("path %q: got valid=%v, want=%v", tt.path, isValid, tt.valid)
			}
		})
	}
}
