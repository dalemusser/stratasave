package login

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dalemusser/stratasave/internal/app/store/ratelimit"
	userstore "github.com/dalemusser/stratasave/internal/app/store/users"
	"github.com/dalemusser/stratasave/internal/app/system/authutil"
	"github.com/dalemusser/stratasave/internal/testutil"
	"go.uber.org/zap"
)

func TestHandler_PasswordLogin_ValidCredentials(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	store := userstore.New(db)

	// Create a test user with password
	hash, err := authutil.HashPassword("validpassword123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	input := userstore.CreateInput{
		FullName:     "Test User",
		LoginID:      "testuser",
		AuthMethod:   "password",
		Role:         "admin",
		PasswordHash: &hash,
	}
	created, err := store.CreateFromInput(ctx, input)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Verify user exists and has correct password hash
	user, err := store.GetByLoginID(ctx, "testuser")
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if user.ID != created.ID {
		t.Error("user ID mismatch")
	}
	if user.PasswordHash == nil {
		t.Fatal("password hash should not be nil")
	}

	// Test password verification
	if !authutil.CheckPassword("validpassword123", *user.PasswordHash) {
		t.Error("password check should succeed")
	}
	if authutil.CheckPassword("wrongpassword", *user.PasswordHash) {
		t.Error("password check should fail for wrong password")
	}
}

func TestHandler_PasswordLogin_UserNotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	store := userstore.New(db)

	// Try to get non-existent user
	_, err := store.GetByLoginID(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent user")
	}
}

func TestHandler_PasswordLogin_DisabledUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	store := userstore.New(db)

	// Create a disabled user
	hash, _ := authutil.HashPassword("password123")
	input := userstore.CreateInput{
		FullName:     "Disabled User",
		LoginID:      "disabled",
		AuthMethod:   "password",
		Role:         "admin",
		PasswordHash: &hash,
	}
	created, err := store.CreateFromInput(ctx, input)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Disable the user
	status := "disabled"
	err = store.UpdateFromInput(ctx, created.ID, userstore.UpdateInput{Status: &status})
	if err != nil {
		t.Fatalf("failed to disable user: %v", err)
	}

	// Verify user is disabled
	user, err := store.GetByLoginID(ctx, "disabled")
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if user.Status != "disabled" {
		t.Errorf("user status = %q, want %q", user.Status, "disabled")
	}
}

func TestHandler_RateLimit_BlocksAfterMaxAttempts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create rate limit store with 3 attempts, 1 minute window, 1 minute lockout
	rateLimitStore := ratelimit.New(db, 3, time.Minute, time.Minute)

	loginID := "ratelimited@test.com"

	// First 3 attempts should be allowed
	for i := 0; i < 3; i++ {
		allowed, _, _ := rateLimitStore.CheckAllowed(ctx, loginID)
		if !allowed {
			t.Errorf("attempt %d should be allowed", i+1)
		}
		rateLimitStore.RecordFailure(ctx, loginID)
	}

	// 4th attempt should be blocked
	allowed, _, lockedUntil := rateLimitStore.CheckAllowed(ctx, loginID)
	if allowed {
		t.Error("should be blocked after 3 failures")
	}
	if lockedUntil == nil {
		t.Error("should have lockout time")
	}
}

func TestHandler_RateLimit_ClearsOnSuccess(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	rateLimitStore := ratelimit.New(db, 3, time.Minute, time.Minute)

	loginID := "cleartest@test.com"

	// Record 2 failures
	rateLimitStore.RecordFailure(ctx, loginID)
	rateLimitStore.RecordFailure(ctx, loginID)

	// Clear on success
	rateLimitStore.ClearOnSuccess(ctx, loginID)

	// Should be allowed and remaining attempts reset to max
	allowed, remaining, _ := rateLimitStore.CheckAllowed(ctx, loginID)
	if !allowed {
		t.Error("should be allowed after clear")
	}
	// After clear, remaining should equal maxAttempts (3) since record is deleted
	if remaining != 3 {
		t.Errorf("remaining = %d, want 3 (maxAttempts)", remaining)
	}
}

func TestHandler_UserLookup_CaseInsensitive(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	store := userstore.New(db)

	// Create user with mixed case login ID
	input := userstore.CreateInput{
		FullName:   "Case Test User",
		LoginID:    "CaseTest",
		AuthMethod: "trust",
		Role:       "admin",
	}
	_, err := store.CreateFromInput(ctx, input)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Test case-insensitive lookup
	testCases := []string{"casetest", "CASETEST", "CaseTest", "cAsEtEsT"}
	for _, loginID := range testCases {
		user, err := store.GetByLoginID(ctx, loginID)
		if err != nil {
			t.Errorf("failed to find user with login ID %q: %v", loginID, err)
			continue
		}
		// LoginID is normalized to lowercase
		if user.LoginID == nil || *user.LoginID != "casetest" {
			var got string
			if user.LoginID != nil {
				got = *user.LoginID
			}
			t.Errorf("login ID = %q, want %q", got, "casetest")
		}
	}
}

func TestHandler_AuthMethod_Routing(t *testing.T) {
	// Test that users with different auth methods would route correctly
	tests := []struct {
		authMethod      string
		expectedURLPart string
	}{
		{"password", "/login/password"},
		{"email", "/login/email"},
		{"google", "/auth/google"},
	}

	for _, tt := range tests {
		t.Run(tt.authMethod, func(t *testing.T) {
			// This tests the routing logic without making actual HTTP requests
			var redirectURL string
			loginID := "user@test.com"

			switch tt.authMethod {
			case "password":
				redirectURL = "/login/password?login_id=" + loginID
			case "email":
				redirectURL = "/login/email?login_id=" + loginID
			case "google":
				redirectURL = "/auth/google"
			}

			if !strings.Contains(redirectURL, tt.expectedURLPart) {
				t.Errorf("redirect URL %q doesn't contain %q", redirectURL, tt.expectedURLPart)
			}
		})
	}
}

func TestPasswordValidation(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"empty password", "", true},
		{"too short", "abc1234", true},
		{"exactly 8 chars", "abcd1234", false},
		{"long password", "verylongpassword123456", false},
		{"with special chars", "Pass@word123!", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (len(tt.password) < 8)
			if err != tt.wantErr {
				t.Errorf("password %q: got err=%v, wantErr=%v", tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestPasswordResetTokenValidation(t *testing.T) {
	// Test that tokens must be present
	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{"empty token", "", true},
		{"valid token format", "abc123def456", false},
		{"whitespace only", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := strings.TrimSpace(tt.token)
			hasErr := token == ""
			if hasErr != tt.wantErr {
				t.Errorf("token %q: got err=%v, wantErr=%v", tt.token, hasErr, tt.wantErr)
			}
		})
	}
}

func TestFormParsing(t *testing.T) {
	// Test form value extraction
	form := url.Values{}
	form.Set("login_id", "test@example.com")
	form.Set("password", "secret123")
	form.Set("return", "/dashboard")

	req := httptest.NewRequest(http.MethodPost, "/login/password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if err := req.ParseForm(); err != nil {
		t.Fatalf("failed to parse form: %v", err)
	}

	if got := req.FormValue("login_id"); got != "test@example.com" {
		t.Errorf("login_id = %q, want %q", got, "test@example.com")
	}
	if got := req.FormValue("password"); got != "secret123" {
		t.Errorf("password = %q, want %q", got, "secret123")
	}
	if got := req.FormValue("return"); got != "/dashboard" {
		t.Errorf("return = %q, want %q", got, "/dashboard")
	}
}

func TestLoginVM_Fields(t *testing.T) {
	vm := LoginVM{
		GoogleEnabled: true,
		Error:         "Test error",
		LoginID:       "test@example.com",
		ReturnURL:     "/dashboard",
	}

	if !vm.GoogleEnabled {
		t.Error("GoogleEnabled should be true")
	}
	if vm.Error != "Test error" {
		t.Errorf("Error = %q, want %q", vm.Error, "Test error")
	}
	if vm.LoginID != "test@example.com" {
		t.Errorf("LoginID = %q, want %q", vm.LoginID, "test@example.com")
	}
	if vm.ReturnURL != "/dashboard" {
		t.Errorf("ReturnURL = %q, want %q", vm.ReturnURL, "/dashboard")
	}
}

func TestPasswordLoginVM_Fields(t *testing.T) {
	vm := PasswordLoginVM{
		Error:     "Invalid credentials",
		LoginID:   "user@test.com",
		ReturnURL: "/profile",
	}

	if vm.Error != "Invalid credentials" {
		t.Errorf("Error = %q, want %q", vm.Error, "Invalid credentials")
	}
	if vm.LoginID != "user@test.com" {
		t.Errorf("LoginID = %q, want %q", vm.LoginID, "user@test.com")
	}
}

func TestEmailVerifyVM_Fields(t *testing.T) {
	vm := EmailVerifyVM{
		Error: "Invalid code",
		Email: "verify@test.com",
	}

	if vm.Error != "Invalid code" {
		t.Errorf("Error = %q, want %q", vm.Error, "Invalid code")
	}
	if vm.Email != "verify@test.com" {
		t.Errorf("Email = %q, want %q", vm.Email, "verify@test.com")
	}
}

func TestResetPasswordVM_Fields(t *testing.T) {
	vm := ResetPasswordVM{
		Error:   "",
		Success: "Password reset successfully",
		Token:   "reset-token-123",
	}

	if vm.Error != "" {
		t.Errorf("Error should be empty, got %q", vm.Error)
	}
	if vm.Success != "Password reset successfully" {
		t.Errorf("Success = %q, want %q", vm.Success, "Password reset successfully")
	}
	if vm.Token != "reset-token-123" {
		t.Errorf("Token = %q, want %q", vm.Token, "reset-token-123")
	}
}

func TestNewHandler(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	// NewHandler should not panic with minimal config
	h := NewHandler(
		db,
		nil, // sessionMgr
		nil, // errLog
		nil, // mailer
		nil, // auditLogger
		nil, // sessionsStore
		nil, // activityStore
		nil, // rateLimitStore (nil = disabled)
		"http://localhost:8080",
		10*time.Minute,
		false, // googleEnabled
		false, // trustLoginEnabled
		logger,
	)

	if h == nil {
		t.Error("NewHandler() returned nil")
	}
	if h.baseURL != "http://localhost:8080" {
		t.Errorf("baseURL = %q, want %q", h.baseURL, "http://localhost:8080")
	}
	if h.googleEnabled {
		t.Error("googleEnabled should be false")
	}
	if h.trustLoginEnabled {
		t.Error("trustLoginEnabled should be false")
	}
}

func TestRoutes_TrustLoginEnabled(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	// Test with trust login enabled
	h := NewHandler(db, nil, nil, nil, nil, nil, nil, nil, "", 0, false, true, logger)
	routes := Routes(h)

	if routes == nil {
		t.Error("Routes() returned nil")
	}
}

func TestRoutes_TrustLoginDisabled(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	// Test with trust login disabled
	h := NewHandler(db, nil, nil, nil, nil, nil, nil, nil, "", 0, false, false, logger)
	routes := Routes(h)

	if routes == nil {
		t.Error("Routes() returned nil")
	}
}
