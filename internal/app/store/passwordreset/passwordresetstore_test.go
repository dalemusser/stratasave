package passwordreset

import (
	"testing"
	"time"

	"github.com/dalemusser/stratasave/internal/testutil"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const testExpiry = 24 * time.Hour

func TestNew(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	if store == nil {
		t.Fatal("New() returned nil")
	}
}

func TestStore_EnsureIndexes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
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

func TestStore_Create(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	email := "test@example.com"

	reset, err := store.Create(ctx, userID, email)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if reset.ID.IsZero() {
		t.Error("ID should not be zero")
	}
	if reset.UserID != userID {
		t.Errorf("UserID = %v, want %v", reset.UserID, userID)
	}
	if reset.Email != email {
		t.Errorf("Email = %v, want %v", reset.Email, email)
	}
	if reset.Token == "" {
		t.Error("Token should not be empty")
	}
	if reset.Used {
		t.Error("Used should be false")
	}
	if reset.ExpiresAt.Before(time.Now()) {
		t.Error("ExpiresAt should be in the future")
	}
}

func TestStore_Create_UniqueTokens(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	tokens := make(map[string]bool)

	// Create multiple resets and verify unique tokens
	for i := 0; i < 10; i++ {
		userID := primitive.NewObjectID()
		reset, err := store.Create(ctx, userID, "user"+string(rune('0'+i))+"@example.com")
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if tokens[reset.Token] {
			t.Errorf("Duplicate token generated: %s", reset.Token)
		}
		tokens[reset.Token] = true
	}
}

func TestStore_Create_InvalidatesPreviousTokens(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	email := "multi@example.com"

	// Create first reset
	first, err := store.Create(ctx, userID, email)
	if err != nil {
		t.Fatalf("Create() first error = %v", err)
	}

	// Create second reset for same user
	second, err := store.Create(ctx, userID, email)
	if err != nil {
		t.Fatalf("Create() second error = %v", err)
	}

	// First token should now be invalid (marked as used)
	_, err = store.VerifyToken(ctx, first.Token)
	if err == nil {
		t.Error("VerifyToken() should fail for invalidated first token")
	}

	// Second token should still be valid
	verified, err := store.VerifyToken(ctx, second.Token)
	if err != nil {
		t.Fatalf("VerifyToken() second token error = %v", err)
	}
	if verified.ID != second.ID {
		t.Errorf("VerifyToken() ID = %v, want %v", verified.ID, second.ID)
	}
}

func TestStore_VerifyToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	email := "verify@example.com"

	created, err := store.Create(ctx, userID, email)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Valid token
	reset, err := store.VerifyToken(ctx, created.Token)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	if reset.ID != created.ID {
		t.Errorf("VerifyToken() ID = %v, want %v", reset.ID, created.ID)
	}
	if reset.UserID != userID {
		t.Errorf("VerifyToken() UserID = %v, want %v", reset.UserID, userID)
	}

	// Invalid token
	_, err = store.VerifyToken(ctx, "invalid-token")
	if err == nil {
		t.Error("VerifyToken() should fail for invalid token")
	}
}

func TestStore_VerifyToken_Expired(t *testing.T) {
	db := testutil.SetupTestDB(t)
	// Very short expiry
	store := New(db, 1*time.Millisecond)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	created, err := store.Create(ctx, userID, "expired@example.com")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Wait for expiry
	time.Sleep(10 * time.Millisecond)

	// Should fail
	_, err = store.VerifyToken(ctx, created.Token)
	if err == nil {
		t.Error("VerifyToken() should fail for expired token")
	}
}

func TestStore_VerifyToken_Used(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	created, err := store.Create(ctx, userID, "used@example.com")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Mark as used
	if err := store.MarkUsed(ctx, created.ID); err != nil {
		t.Fatalf("MarkUsed() error = %v", err)
	}

	// Should fail
	_, err = store.VerifyToken(ctx, created.Token)
	if err == nil {
		t.Error("VerifyToken() should fail for used token")
	}
}

func TestStore_MarkUsed(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	created, err := store.Create(ctx, userID, "markused@example.com")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Mark used
	if err := store.MarkUsed(ctx, created.ID); err != nil {
		t.Fatalf("MarkUsed() error = %v", err)
	}

	// Verify the token is no longer valid
	_, err = store.VerifyToken(ctx, created.Token)
	if err == nil {
		t.Error("Token should be invalid after MarkUsed")
	}
}

func TestStore_MarkUsed_NonexistentID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Should not error, just no-op
	err := store.MarkUsed(ctx, primitive.NewObjectID())
	if err != nil {
		t.Errorf("MarkUsed() for nonexistent ID should not error, got %v", err)
	}
}

func TestGenerateToken(t *testing.T) {
	tokens := make(map[string]bool)

	for i := 0; i < 100; i++ {
		token, err := generateToken()
		if err != nil {
			t.Fatalf("generateToken() error = %v", err)
		}
		if token == "" {
			t.Error("generateToken() returned empty token")
		}
		// Token should be base64 encoded 32 bytes = ~44 characters
		if len(token) < 40 {
			t.Errorf("generateToken() token too short: %d chars", len(token))
		}
		if tokens[token] {
			t.Errorf("Duplicate token: %s", token)
		}
		tokens[token] = true
	}
}

func TestStore_Create_VerifyToken_EndToEnd(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	email := "e2e@example.com"

	// 1. Create reset token
	created, err := store.Create(ctx, userID, email)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// 2. Verify token is valid
	verified, err := store.VerifyToken(ctx, created.Token)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	if verified.UserID != userID {
		t.Error("Verified token should have correct UserID")
	}

	// 3. Mark as used
	if err := store.MarkUsed(ctx, verified.ID); err != nil {
		t.Fatalf("MarkUsed() error = %v", err)
	}

	// 4. Token should no longer be valid
	_, err = store.VerifyToken(ctx, created.Token)
	if err == nil {
		t.Error("Token should be invalid after use")
	}
	if err != nil && err.Error() != "invalid or expired token" {
		t.Errorf("Expected 'invalid or expired token' error, got: %v", err)
	}
}

func TestStore_EnsureIndexes_UniqueToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	if err := store.EnsureIndexes(ctx); err != nil {
		t.Fatalf("EnsureIndexes() error = %v", err)
	}

	// Insert directly with duplicate token to test index
	reset1 := Reset{
		ID:        primitive.NewObjectID(),
		UserID:    primitive.NewObjectID(),
		Email:     "dup1@example.com",
		Token:     "duplicate-token",
		Used:      false,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	_, err := store.c.InsertOne(ctx, reset1)
	if err != nil {
		t.Fatalf("First insert error = %v", err)
	}

	reset2 := Reset{
		ID:        primitive.NewObjectID(),
		UserID:    primitive.NewObjectID(),
		Email:     "dup2@example.com",
		Token:     "duplicate-token", // Same token
		Used:      false,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	_, err = store.c.InsertOne(ctx, reset2)
	if err == nil {
		t.Error("Duplicate token insert should fail due to unique index")
	}
	if !mongo.IsDuplicateKeyError(err) {
		t.Errorf("Expected duplicate key error, got: %v", err)
	}
}
