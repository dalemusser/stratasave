package authz

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// withTestUser creates a request with a user in context.
func withTestUser(id, name, role, theme string) *http.Request {
	req := httptest.NewRequest("GET", "/", nil)
	user := &auth.SessionUser{
		ID:              id,
		Name:            name,
		Role:            role,
		ThemePreference: theme,
	}
	return auth.WithTestUser(req, user)
}

func TestUserCtx(t *testing.T) {
	validID := primitive.NewObjectID().Hex()

	tests := []struct {
		name       string
		userID     string
		userName   string
		userRole   string
		wantRole   string
		wantName   string
		wantOK     bool
		wantNilID  bool
	}{
		{
			name:      "admin user",
			userID:    validID,
			userName:  "Admin User",
			userRole:  "admin",
			wantRole:  "admin",
			wantName:  "Admin User",
			wantOK:    true,
			wantNilID: false,
		},
		{
			name:      "regular user",
			userID:    validID,
			userName:  "Regular User",
			userRole:  "user",
			wantRole:  "user",
			wantName:  "Regular User",
			wantOK:    true,
			wantNilID: false,
		},
		{
			name:      "uppercase role normalized",
			userID:    validID,
			userName:  "User",
			userRole:  "ADMIN",
			wantRole:  "admin",
			wantName:  "User",
			wantOK:    true,
			wantNilID: false,
		},
		{
			name:      "mixed case role normalized",
			userID:    validID,
			userName:  "User",
			userRole:  "Admin",
			wantRole:  "admin",
			wantName:  "User",
			wantOK:    true,
			wantNilID: false,
		},
		{
			name:      "invalid user id",
			userID:    "invalid-id",
			userName:  "User",
			userRole:  "user",
			wantRole:  "visitor",
			wantName:  "",
			wantOK:    false,
			wantNilID: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := withTestUser(tt.userID, tt.userName, tt.userRole, "")

			role, name, userID, ok := UserCtx(req)

			if role != tt.wantRole {
				t.Errorf("role = %v, want %v", role, tt.wantRole)
			}
			if name != tt.wantName {
				t.Errorf("name = %v, want %v", name, tt.wantName)
			}
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantNilID && !userID.IsZero() {
				t.Error("expected nil ObjectID")
			}
			if !tt.wantNilID && userID.IsZero() {
				t.Error("expected non-nil ObjectID")
			}
		})
	}
}

func TestUserCtx_NoUser(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)

	role, name, userID, ok := UserCtx(req)

	if role != "visitor" {
		t.Errorf("role = %v, want visitor", role)
	}
	if name != "" {
		t.Errorf("name = %v, want empty", name)
	}
	if ok {
		t.Error("ok = true, want false")
	}
	if !userID.IsZero() {
		t.Error("expected nil ObjectID")
	}
}

func TestIsAdmin(t *testing.T) {
	validID := primitive.NewObjectID().Hex()

	tests := []struct {
		name     string
		userID   string
		userRole string
		want     bool
	}{
		{"admin user", validID, "admin", true},
		{"admin uppercase", validID, "ADMIN", true},
		{"admin mixed case", validID, "Admin", true},
		{"regular user", validID, "user", false},
		{"moderator", validID, "moderator", false},
		{"empty role", validID, "", false},
		{"invalid id", "invalid", "admin", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := withTestUser(tt.userID, "User", tt.userRole, "")

			if got := IsAdmin(req); got != tt.want {
				t.Errorf("IsAdmin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAdmin_NoUser(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)

	if IsAdmin(req) {
		t.Error("IsAdmin() = true for no user, want false")
	}
}

func TestIsLoggedIn(t *testing.T) {
	validID := primitive.NewObjectID().Hex()

	tests := []struct {
		name    string
		hasUser bool
		want    bool
	}{
		{"with user", true, true},
		{"no user", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.hasUser {
				req = withTestUser(validID, "User", "user", "")
			} else {
				req = httptest.NewRequest("GET", "/", nil)
			}

			if got := IsLoggedIn(req); got != tt.want {
				t.Errorf("IsLoggedIn() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasRole(t *testing.T) {
	validID := primitive.NewObjectID().Hex()

	tests := []struct {
		name     string
		userID   string
		userRole string
		roles    []string
		want     bool
	}{
		{"single role match", validID, "admin", []string{"admin"}, true},
		{"multiple roles match first", validID, "admin", []string{"admin", "user"}, true},
		{"multiple roles match second", validID, "user", []string{"admin", "user"}, true},
		{"case insensitive role", validID, "ADMIN", []string{"admin"}, true},
		{"case insensitive allowed", validID, "admin", []string{"ADMIN"}, true},
		{"no match", validID, "user", []string{"admin", "moderator"}, false},
		{"empty allowed roles", validID, "user", []string{}, false},
		{"invalid user id", "invalid", "admin", []string{"admin"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := withTestUser(tt.userID, "User", tt.userRole, "")

			if got := HasRole(req, tt.roles...); got != tt.want {
				t.Errorf("HasRole(%v) = %v, want %v", tt.roles, got, tt.want)
			}
		})
	}
}

func TestHasRole_NoUser(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)

	if HasRole(req, "admin", "user") {
		t.Error("HasRole() = true for no user, want false")
	}
}

func TestThemePreference(t *testing.T) {
	validID := primitive.NewObjectID().Hex()

	tests := []struct {
		name  string
		theme string
		want  string
	}{
		{"light theme", "light", "light"},
		{"dark theme", "dark", "dark"},
		{"system theme", "system", "system"},
		{"empty theme", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := withTestUser(validID, "User", "user", tt.theme)

			if got := ThemePreference(req); got != tt.want {
				t.Errorf("ThemePreference() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestThemePreference_NoUser(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)

	if got := ThemePreference(req); got != "" {
		t.Errorf("ThemePreference() = %v for no user, want empty", got)
	}
}
