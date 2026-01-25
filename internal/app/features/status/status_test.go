package status

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/testutil"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	client := db.Client()

	handler := NewHandler(
		client,
		"http://localhost:8080",
		nil, // coreCfg
		AppConfig{
			MongoDatabase: db.Name(),
		},
		logger,
	)

	return handler
}

func TestNewHandler(t *testing.T) {
	h := newTestHandler(t)
	if h == nil {
		t.Fatal("NewHandler() returned nil")
	}
}

func TestServe_AdminOnly(t *testing.T) {
	testutil.MustBootTemplates(t)
	h := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/status", nil)
	req = auth.WithTestUser(req, sessionUser)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.Serve(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleRenew_NoCertRenewer(t *testing.T) {
	h := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/status/renew", nil)
	req = auth.WithTestUser(req, sessionUser)
	rec := httptest.NewRecorder()

	h.HandleRenew(rec, req)

	// Should return error when no cert renewer is available
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
	}{
		{"minutes only", 30 * time.Minute},
		{"1 hour", 1 * time.Hour},
		{"hours and minutes", 2*time.Hour + 30*time.Minute},
		{"1 day", 24 * time.Hour},
		{"days and hours", 50 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			// Check that format is not empty
			if got == "" {
				t.Errorf("formatDuration() returned empty string")
			}
		})
	}
}

func TestFormatExpiresIn(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		contains string
	}{
		{"negative (expired)", -1 * time.Hour, "expired"},
		{"1 day", 24 * time.Hour, "day"},
		{"multiple days", 72 * time.Hour, "day"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatExpiresIn(tt.duration)
			if !containsStr(got, tt.contains) {
				t.Errorf("formatExpiresIn() = %q, want to contain %q", got, tt.contains)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    uint64
		contains string
	}{
		{500, "B"},
		{1024, "KiB"},
		{1024 * 1024, "MiB"},
		{1024 * 1024 * 1024, "GiB"},
	}

	for _, tt := range tests {
		t.Run(tt.contains, func(t *testing.T) {
			got := formatBytes(tt.bytes)
			if !containsStr(got, tt.contains) {
				t.Errorf("formatBytes(%d) = %q, want to contain %q", tt.bytes, got, tt.contains)
			}
		})
	}
}

func TestFormatPlural(t *testing.T) {
	tests := []struct {
		n        int
		unit     string
		expected string
	}{
		{1, "day", "1 day"},
		{2, "day", "2 days"},
		{0, "hour", "0 hours"},
	}

	for _, tt := range tests {
		got := formatPlural(tt.n, tt.unit)
		if got != tt.expected {
			t.Errorf("formatPlural(%d, %q) = %q, want %q", tt.n, tt.unit, got, tt.expected)
		}
	}
}

func TestConfigItem(t *testing.T) {
	item := ConfigItem{
		Name:  "test_config",
		Value: "test_value",
	}

	if item.Name != "test_config" {
		t.Errorf("Name = %q, want %q", item.Name, "test_config")
	}
	if item.Value != "test_value" {
		t.Errorf("Value = %q, want %q", item.Value, "test_value")
	}
}

func TestConfigGroup(t *testing.T) {
	group := ConfigGroup{
		Name: "Test Group",
		Items: []ConfigItem{
			{Name: "item1", Value: "value1"},
			{Name: "item2", Value: "value2"},
		},
	}

	if group.Name != "Test Group" {
		t.Errorf("Name = %q, want %q", group.Name, "Test Group")
	}
	if len(group.Items) != 2 {
		t.Errorf("len(Items) = %d, want 2", len(group.Items))
	}
}

func TestAppConfig(t *testing.T) {
	cfg := AppConfig{
		MongoURI:          "mongodb://localhost:27017",
		MongoDatabase:     "test_db",
		SessionKey:        "secret",
		RateLimitEnabled:  true,
		IdleLogoutEnabled: true,
	}

	if cfg.MongoDatabase != "test_db" {
		t.Errorf("MongoDatabase = %q, want %q", cfg.MongoDatabase, "test_db")
	}
	if !cfg.RateLimitEnabled {
		t.Error("RateLimitEnabled should be true")
	}
}

func TestBuildConfigGroups(t *testing.T) {
	h := newTestHandler(t)
	groups := h.buildConfigGroups()

	// Should return some config groups
	if len(groups) == 0 {
		t.Error("buildConfigGroups() returned empty slice")
	}
}

// containsStr checks if s contains substr
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
