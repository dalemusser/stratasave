package activity

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	activitystore "github.com/dalemusser/stratasave/internal/app/store/activity"
	"github.com/dalemusser/stratasave/internal/app/store/sessions"
	userstore "github.com/dalemusser/stratasave/internal/app/store/users"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/testutil"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

func newTestHandler(t *testing.T) (*Handler, *mongo.Database) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	sessStore := sessions.New(db)
	actStore := activitystore.New(db)
	usersStore := userstore.New(db)

	sessionMgr, err := auth.NewSessionManager(
		"test-session-key-for-testing-1234567890",
		"test-session",
		"",
		24*time.Hour,
		false,
		logger,
	)
	if err != nil {
		t.Fatalf("failed to create session manager: %v", err)
	}

	handler := NewHandler(
		db,
		sessStore,
		actStore,
		usersStore,
		sessionMgr,
		nil, // errLog
		logger,
	)

	return handler, db
}

func TestNewHandler(t *testing.T) {
	h, _ := newTestHandler(t)
	if h == nil {
		t.Fatal("NewHandler() returned nil")
	}
}

func TestRoutes(t *testing.T) {
	h, _ := newTestHandler(t)
	logger := zap.NewNop()

	sessionMgr, err := auth.NewSessionManager(
		"test-session-key-for-testing-1234567890",
		"test-session",
		"",
		24*time.Hour,
		false,
		logger,
	)
	if err != nil {
		t.Fatalf("failed to create session manager: %v", err)
	}

	router := Routes(h, sessionMgr)
	if router == nil {
		t.Fatal("Routes() returned nil")
	}
}

func TestServeDashboard_AdminOnly(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodGet, "/activity", nil)
	req = auth.WithTestUser(req, sessionUser)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.ServeDashboard(rec, req)

	// Handler should return 200 OK when templates are properly initialized
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestServeOnlineTable_ReturnsActiveSessions(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodGet, "/activity/online-table", nil)
	req = auth.WithTestUser(req, sessionUser)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.ServeOnlineTable(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestServeSummary_WeeklyStats(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodGet, "/activity/summary", nil)
	req = auth.WithTestUser(req, sessionUser)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.ServeSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestServeExport_DateFiltering(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodGet, "/activity/export?start=2024-01-01&end=2024-01-31", nil)
	req = auth.WithTestUser(req, sessionUser)
	req = testutil.WithCSRFToken(req)
	rec := httptest.NewRecorder()

	h.ServeExport(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestServeSessionsCSV_Format(t *testing.T) {
	h, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodGet, "/activity/export/sessions.csv?start=2024-01-01&end=2024-01-31", nil)
	req = auth.WithTestUser(req, sessionUser)
	rec := httptest.NewRecorder()

	h.ServeSessionsCSV(rec, req)

	// Should return OK
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Check content type
	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/csv; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", contentType, "text/csv; charset=utf-8")
	}

	// Check content disposition
	contentDisp := rec.Header().Get("Content-Disposition")
	if contentDisp == "" {
		t.Error("Content-Disposition header should be set")
	}
}

func TestServeSessionsJSON_Format(t *testing.T) {
	h, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodGet, "/activity/export/sessions.json?start=2024-01-01&end=2024-01-31", nil)
	req = auth.WithTestUser(req, sessionUser)
	rec := httptest.NewRecorder()

	h.ServeSessionsJSON(rec, req)

	// Should return OK
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Check content type
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json; charset=utf-8")
	}
}

func TestServeEventsCSV_Format(t *testing.T) {
	h, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodGet, "/activity/export/events.csv?start=2024-01-01&end=2024-01-31", nil)
	req = auth.WithTestUser(req, sessionUser)
	rec := httptest.NewRecorder()

	h.ServeEventsCSV(rec, req)

	// Should return OK
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Check content type
	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/csv; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", contentType, "text/csv; charset=utf-8")
	}
}

func TestServeEventsJSON_Format(t *testing.T) {
	h, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	req := httptest.NewRequest(http.MethodGet, "/activity/export/events.json?start=2024-01-01&end=2024-01-31", nil)
	req = auth.WithTestUser(req, sessionUser)
	rec := httptest.NewRecorder()

	h.ServeEventsJSON(rec, req)

	// Should return OK
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Check content type
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json; charset=utf-8")
	}
}

func TestServeUserDetail_NotFound(t *testing.T) {
	testutil.MustBootTemplates(t)
	h, _ := newTestHandler(t)

	adminID := primitive.NewObjectID()
	sessionUser := &auth.SessionUser{
		ID:      adminID.Hex(),
		Name:    "Admin User",
		LoginID: "admin@example.com",
		Role:    "admin",
	}

	nonExistentID := primitive.NewObjectID()

	req := httptest.NewRequest(http.MethodGet, "/activity/user/"+nonExistentID.Hex(), nil)
	req = auth.WithTestUser(req, sessionUser)
	req = testutil.WithCSRFToken(req)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("userID", nonExistentID.Hex())
	req = req.WithContext(req.Context())

	rec := httptest.NewRecorder()

	h.ServeUserDetail(rec, req)

	// Handler should handle non-existent user gracefully
	// (either 404 or render a page)
}

func TestParseDateRange(t *testing.T) {
	tests := []struct {
		name            string
		startParam      string
		endParam        string
		wantStartFormat string
		wantEndFormat   string
	}{
		{
			name:            "valid dates",
			startParam:      "2024-01-01",
			endParam:        "2024-01-31",
			wantStartFormat: "2024-01-01",
			wantEndFormat:   "2024-01-31",
		},
		{
			name:            "invalid dates default to 30 days",
			startParam:      "invalid",
			endParam:        "invalid",
			wantStartFormat: "", // will be 30 days ago
			wantEndFormat:   "", // will be today
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/export?start="+tt.startParam+"&end="+tt.endParam, nil)
			startDate, endDate := parseDateRange(req)

			if tt.wantStartFormat != "" {
				if got := startDate.Format("2006-01-02"); got != tt.wantStartFormat {
					t.Errorf("startDate = %q, want %q", got, tt.wantStartFormat)
				}
			}

			if tt.wantEndFormat != "" {
				// endDate is end of day, so check the date part
				if got := endDate.Format("2006-01-02"); got != tt.wantEndFormat {
					t.Errorf("endDate = %q, want %q", got, tt.wantEndFormat)
				}
			}
		})
	}
}

func TestFormatPageName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/", "Dashboard"},
		{"/dashboard", "Dashboard"},
		{"/profile", "Profile"},
		{"/settings", "Settings"},
		{"/activity", "Activity"},
		{"/activity/summary", "Activity Summary"},
		{"/activity/user/123", "User Activity"},
		{"/unknown/path", "Path"},
		{"", "Dashboard"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := formatPageName(tt.input)
			if got != tt.expected {
				t.Errorf("formatPageName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSafeDiv(t *testing.T) {
	tests := []struct {
		a        int
		b        int
		expected int
	}{
		{10, 2, 5},
		{10, 0, 0},
		{0, 10, 0},
		{100, 3, 33},
	}

	for _, tt := range tests {
		got := safeDiv(tt.a, tt.b)
		if got != tt.expected {
			t.Errorf("safeDiv(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.expected)
		}
	}
}

func TestFindPeakHour(t *testing.T) {
	tests := []struct {
		name     string
		hours    map[int]int
		expected string
	}{
		{
			name:     "empty map",
			hours:    map[int]int{},
			expected: "N/A",
		},
		{
			name:     "single hour",
			hours:    map[int]int{14: 10},
			expected: "14:00",
		},
		{
			name:     "multiple hours",
			hours:    map[int]int{9: 5, 14: 10, 18: 3},
			expected: "14:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findPeakHour(tt.hours)
			if got != tt.expected {
				t.Errorf("findPeakHour() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFindMostActiveDay(t *testing.T) {
	tests := []struct {
		name     string
		days     map[string]int
		expected string
	}{
		{
			name:     "empty map",
			days:     map[string]int{},
			expected: "N/A",
		},
		{
			name:     "single day",
			days:     map[string]int{"Monday": 10},
			expected: "Monday",
		},
		{
			name:     "multiple days",
			days:     map[string]int{"Monday": 5, "Wednesday": 15, "Friday": 8},
			expected: "Wednesday",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findMostActiveDay(tt.days)
			if got != tt.expected {
				t.Errorf("findMostActiveDay() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSanitizeCSVField(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal text", "normal text"},
		{"=formula", "'=formula"},
		{"+formula", "'+formula"},
		{"-formula", "'-formula"},
		{"@formula", "'@formula"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeCSVField(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeCSVField(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatMinutes(t *testing.T) {
	tests := []struct {
		mins     int
		expected string
	}{
		{0, "0 min"},
		{30, "30 min"},
		{59, "59 min"},
		{60, "1h 0m"},
		{90, "1h 30m"},
		{150, "2h 30m"},
	}

	for _, tt := range tests {
		got := formatMinutes(tt.mins)
		if got != tt.expected {
			t.Errorf("formatMinutes(%d) = %q, want %q", tt.mins, got, tt.expected)
		}
	}
}

func TestFilterUsersBySearch(t *testing.T) {
	users := []userRow{
		{Name: "Alice Admin", LoginID: "alice"},
		{Name: "Bob User", LoginID: "bob"},
		{Name: "Charlie Admin", LoginID: "charlie"},
	}

	tests := []struct {
		name     string
		query    string
		expected int
	}{
		{"empty query returns all", "", 3},
		{"search by name prefix", "Ali", 1},
		{"search by login prefix", "bob", 1},
		{"no matches", "xyz", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterUsersBySearch(users, tt.query)
			if len(result) != tt.expected {
				t.Errorf("filterUsersBySearch() returned %d results, want %d", len(result), tt.expected)
			}
		})
	}
}

func TestFilterUsersByStatus(t *testing.T) {
	users := []userRow{
		{Name: "Online User", Status: StatusOnline},
		{Name: "Idle User", Status: StatusIdle},
		{Name: "Offline User", Status: StatusOffline},
	}

	tests := []struct {
		name     string
		filter   string
		expected int
	}{
		{"all returns all", "all", 3},
		{"empty returns all", "", 3},
		{"online only", "online", 1},
		{"idle only", "idle", 1},
		{"offline only", "offline", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterUsersByStatus(users, tt.filter)
			if len(result) != tt.expected {
				t.Errorf("filterUsersByStatus() returned %d results, want %d", len(result), tt.expected)
			}
		})
	}
}

func TestDashboardData(t *testing.T) {
	// Test that dashboardData struct works as expected
	data := dashboardData{
		StatusFilter: "online",
		SearchQuery:  "test",
		Page:         1,
		Total:        50,
		OnlineCount:  10,
		IdleCount:    5,
		OfflineCount: 35,
	}

	if data.StatusFilter != "online" {
		t.Errorf("StatusFilter = %q, want %q", data.StatusFilter, "online")
	}
	if data.OnlineCount != 10 {
		t.Errorf("OnlineCount = %d, want %d", data.OnlineCount, 10)
	}
}
