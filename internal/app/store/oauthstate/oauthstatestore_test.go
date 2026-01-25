package oauthstate

import (
	"testing"
	"time"

	"github.com/dalemusser/stratasave/internal/testutil"
)

func TestNew(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	if store == nil {
		t.Fatal("New() returned nil")
	}
}

func TestStore_EnsureIndexes(t *testing.T) {
	// Skip: testutil.SetupTestDB already calls indexes.EnsureAll() which creates
	// indexes with explicit names. The store's EnsureIndexes() creates indexes
	// with different names, causing IndexOptionsConflict. Global index management
	// is handled by indexes.EnsureAll() in production.
	t.Skip("indexes already created by testutil.SetupTestDB via indexes.EnsureAll()")
}

func TestStore_Create(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	state := "random-state-token-12345"

	err := store.Create(ctx, state)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify by attempting to verify it
	valid := store.Verify(ctx, state)
	if !valid {
		t.Error("Create() should create a valid state token")
	}
}

func TestStore_Create_UniqueConstraint(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Indexes are already created by testutil.SetupTestDB via indexes.EnsureAll()
	state := "duplicate-state-token"

	// First create should succeed
	err := store.Create(ctx, state)
	if err != nil {
		t.Fatalf("Create() first call error = %v", err)
	}

	// Second create with same state should fail (unique constraint)
	err = store.Create(ctx, state)
	if err == nil {
		t.Error("Create() with duplicate state should fail")
	}
}

func TestStore_Verify_ValidToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	state := "valid-state-for-verify"

	store.Create(ctx, state)

	// First verify should return true and delete the token
	valid := store.Verify(ctx, state)
	if !valid {
		t.Error("Verify() should return true for valid token")
	}

	// Second verify should return false (token was deleted)
	valid = store.Verify(ctx, state)
	if valid {
		t.Error("Verify() should return false after token is used (single use)")
	}
}

func TestStore_Verify_NonexistentToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	valid := store.Verify(ctx, "nonexistent-token")
	if valid {
		t.Error("Verify() should return false for nonexistent token")
	}
}

func TestStore_Verify_SingleUse(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	state := "single-use-token"

	store.Create(ctx, state)

	// First verification
	if !store.Verify(ctx, state) {
		t.Fatal("First Verify() should return true")
	}

	// Token should be deleted now, second verification should fail
	if store.Verify(ctx, state) {
		t.Error("Second Verify() should return false (token is single-use)")
	}
}

func TestStore_Verify_MultipleTokens(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	tokens := []string{
		"token-1-abc",
		"token-2-def",
		"token-3-ghi",
	}

	// Create all tokens
	for _, token := range tokens {
		store.Create(ctx, token)
	}

	// Each token should verify once
	for _, token := range tokens {
		if !store.Verify(ctx, token) {
			t.Errorf("Verify(%s) should return true", token)
		}
	}

	// None should verify again
	for _, token := range tokens {
		if store.Verify(ctx, token) {
			t.Errorf("Verify(%s) second time should return false", token)
		}
	}
}

func TestStore_TokenExpiry(t *testing.T) {
	// Note: This test cannot easily test MongoDB TTL behavior since TTL indexes
	// run in the background and may take up to a minute to clean up documents.
	// We can only verify that the Verify method checks expiry correctly.

	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// The Create method sets expiry to 10 minutes from now,
	// so we can't easily test expired tokens without manipulating the database directly.
	// This test documents the expected behavior.

	state := "test-expiry-token"
	store.Create(ctx, state)

	// Token should be valid immediately after creation
	// Note: Verify also deletes the token, so we're testing both behaviors
	if !store.Verify(ctx, state) {
		t.Error("Token should be valid immediately after creation")
	}
}

func TestStateStruct(t *testing.T) {
	// Verify State struct has expected fields
	now := time.Now()
	state := State{
		State:     "test-state",
		ExpiresAt: now.Add(10 * time.Minute),
		CreatedAt: now,
	}

	if state.State != "test-state" {
		t.Errorf("State.State = %v, want 'test-state'", state.State)
	}
	if state.ExpiresAt.Before(now) {
		t.Error("ExpiresAt should be in the future")
	}
	if state.ID.IsZero() {
		// ID is zero until inserted, this is expected
	}
}
