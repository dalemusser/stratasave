package ratelimit

import (
	"testing"
	"time"

	"github.com/dalemusser/stratasave/internal/testutil"
)

func TestNew(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, 5, 15*time.Minute, 30*time.Minute)
	if store == nil {
		t.Fatal("New() returned nil")
	}
}

func TestStore_EnsureIndexes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, 5, 15*time.Minute, 30*time.Minute)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	if err := store.EnsureIndexes(ctx); err != nil {
		t.Fatalf("EnsureIndexes() error = %v", err)
	}

	// Should be idempotent
	if err := store.EnsureIndexes(ctx); err != nil {
		t.Fatalf("EnsureIndexes() second call error = %v", err)
	}
}

func TestStore_CheckAllowed_NoRecord(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, 5, 15*time.Minute, 30*time.Minute)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	allowed, remaining, lockedUntil := store.CheckAllowed(ctx, "newuser@example.com")

	if !allowed {
		t.Error("CheckAllowed() should return true for new login")
	}
	if remaining != 5 {
		t.Errorf("CheckAllowed() remaining = %d, want 5", remaining)
	}
	if lockedUntil != nil {
		t.Error("CheckAllowed() lockedUntil should be nil for new login")
	}
}

func TestStore_CheckAllowed_CaseInsensitive(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, 5, 15*time.Minute, 30*time.Minute)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Record failure for lowercase
	store.RecordFailure(ctx, "test@example.com")

	// Check with uppercase - should see the same record
	allowed, remaining, _ := store.CheckAllowed(ctx, "TEST@EXAMPLE.COM")

	if !allowed {
		t.Error("CheckAllowed() should return true")
	}
	if remaining != 4 {
		t.Errorf("CheckAllowed() remaining = %d, want 4 (case-insensitive)", remaining)
	}
}

func TestStore_RecordFailure_IncreasesCount(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, 5, 15*time.Minute, 30*time.Minute)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	loginID := "failuser@example.com"

	// First failure
	lockedOut, _ := store.RecordFailure(ctx, loginID)
	if lockedOut {
		t.Error("RecordFailure() should not lock out on first failure")
	}

	allowed, remaining, _ := store.CheckAllowed(ctx, loginID)
	if !allowed {
		t.Error("CheckAllowed() should return true after one failure")
	}
	if remaining != 4 {
		t.Errorf("CheckAllowed() remaining = %d, want 4", remaining)
	}

	// More failures
	store.RecordFailure(ctx, loginID)
	store.RecordFailure(ctx, loginID)

	allowed, remaining, _ = store.CheckAllowed(ctx, loginID)
	if !allowed {
		t.Error("CheckAllowed() should return true after three failures")
	}
	if remaining != 2 {
		t.Errorf("CheckAllowed() remaining = %d, want 2", remaining)
	}
}

func TestStore_RecordFailure_TriggersLockout(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, 3, 15*time.Minute, 30*time.Minute) // Only 3 attempts
	ctx, cancel := testutil.TestContext()
	defer cancel()

	loginID := "lockout@example.com"

	// First two failures - no lockout
	store.RecordFailure(ctx, loginID)
	store.RecordFailure(ctx, loginID)

	// Third failure should trigger lockout
	lockedOut, lockedUntil := store.RecordFailure(ctx, loginID)
	if !lockedOut {
		t.Error("RecordFailure() should return lockedOut=true at max attempts")
	}
	if lockedUntil == nil {
		t.Error("RecordFailure() should return lockedUntil time")
	}
	if lockedUntil != nil && lockedUntil.Before(time.Now().Add(29*time.Minute)) {
		t.Error("lockedUntil should be at least 29 minutes in the future")
	}
}

func TestStore_CheckAllowed_WhenLocked(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, 2, 15*time.Minute, 30*time.Minute) // Only 2 attempts
	ctx, cancel := testutil.TestContext()
	defer cancel()

	loginID := "locked@example.com"

	// Trigger lockout
	store.RecordFailure(ctx, loginID)
	store.RecordFailure(ctx, loginID)

	// Check - should be locked
	allowed, remaining, lockedUntil := store.CheckAllowed(ctx, loginID)
	if allowed {
		t.Error("CheckAllowed() should return false when locked")
	}
	if remaining != -1 {
		t.Errorf("CheckAllowed() remaining = %d, want -1 when locked", remaining)
	}
	if lockedUntil == nil {
		t.Error("CheckAllowed() should return lockedUntil when locked")
	}
}

func TestStore_ClearOnSuccess(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, 5, 15*time.Minute, 30*time.Minute)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	loginID := "clearuser@example.com"

	// Record some failures
	store.RecordFailure(ctx, loginID)
	store.RecordFailure(ctx, loginID)

	// Clear on success
	if err := store.ClearOnSuccess(ctx, loginID); err != nil {
		t.Fatalf("ClearOnSuccess() error = %v", err)
	}

	// Should be back to full attempts
	allowed, remaining, _ := store.CheckAllowed(ctx, loginID)
	if !allowed {
		t.Error("CheckAllowed() should return true after clear")
	}
	if remaining != 5 {
		t.Errorf("CheckAllowed() remaining = %d, want 5 after clear", remaining)
	}
}

func TestStore_GetAttempt(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, 5, 15*time.Minute, 30*time.Minute)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	loginID := "getattempt@example.com"

	// No record yet
	attempt, err := store.GetAttempt(ctx, loginID)
	if err != nil {
		t.Fatalf("GetAttempt() error = %v", err)
	}
	if attempt != nil {
		t.Error("GetAttempt() should return nil for nonexistent login")
	}

	// Record a failure
	store.RecordFailure(ctx, loginID)

	// Should have a record now
	attempt, err = store.GetAttempt(ctx, loginID)
	if err != nil {
		t.Fatalf("GetAttempt() error = %v", err)
	}
	if attempt == nil {
		t.Fatal("GetAttempt() should return attempt after failure")
	}
	if attempt.AttemptCount != 1 {
		t.Errorf("AttemptCount = %d, want 1", attempt.AttemptCount)
	}
	if attempt.LoginID != "getattempt@example.com" {
		t.Errorf("LoginID = %s, want getattempt@example.com", attempt.LoginID)
	}
}

func TestStore_WindowExpiry_ResetsCounter(t *testing.T) {
	db := testutil.SetupTestDB(t)
	// Very short window for testing
	store := New(db, 5, 1*time.Millisecond, 30*time.Minute)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	loginID := "windowtest@example.com"

	// Record some failures
	store.RecordFailure(ctx, loginID)
	store.RecordFailure(ctx, loginID)

	// Wait for window to expire
	time.Sleep(10 * time.Millisecond)

	// Should have full attempts again
	allowed, remaining, _ := store.CheckAllowed(ctx, loginID)
	if !allowed {
		t.Error("CheckAllowed() should return true after window expiry")
	}
	if remaining != 5 {
		t.Errorf("CheckAllowed() remaining = %d, want 5 after window expiry", remaining)
	}
}

func TestNormalizeLoginID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Test@Example.COM", "test@example.com"},
		{"  user@test.com  ", "user@test.com"},
		{"USER", "user"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeLoginID(tt.input)
			if got != tt.want {
				t.Errorf("normalizeLoginID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
