package invitation

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

	err := store.EnsureIndexes(ctx)
	if err != nil {
		t.Fatalf("EnsureIndexes() error = %v", err)
	}
}

func TestStore_Create(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	input := CreateInput{
		Email:     "test@example.com",
		Role:      "user",
		InvitedBy: primitive.NewObjectID(),
	}

	inv, err := store.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if inv.ID.IsZero() {
		t.Error("ID should not be zero")
	}
	if inv.Email != input.Email {
		t.Errorf("Email = %v, want %v", inv.Email, input.Email)
	}
	if inv.Role != input.Role {
		t.Errorf("Role = %v, want %v", inv.Role, input.Role)
	}
	if inv.Token == "" {
		t.Error("Token should not be empty")
	}
	if inv.Revoked {
		t.Error("Revoked should be false")
	}
	if inv.UsedAt != nil {
		t.Error("UsedAt should be nil")
	}
	if inv.ExpiresAt.Before(time.Now()) {
		t.Error("ExpiresAt should be in the future")
	}
}

func TestStore_VerifyToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	input := CreateInput{
		Email:     "verify@example.com",
		Role:      "user",
		InvitedBy: primitive.NewObjectID(),
	}

	created, err := store.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Valid token
	inv, err := store.VerifyToken(ctx, created.Token)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	if inv.ID != created.ID {
		t.Errorf("ID = %v, want %v", inv.ID, created.ID)
	}

	// Invalid token
	_, err = store.VerifyToken(ctx, "invalid-token")
	if err == nil {
		t.Error("VerifyToken() for invalid token should fail")
	}
}

func TestStore_VerifyToken_Expired(t *testing.T) {
	db := testutil.SetupTestDB(t)
	// Create store with very short expiry
	store := New(db, 1*time.Millisecond)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	input := CreateInput{
		Email:     "expired@example.com",
		Role:      "user",
		InvitedBy: primitive.NewObjectID(),
	}

	created, err := store.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Should fail
	_, err = store.VerifyToken(ctx, created.Token)
	if err == nil {
		t.Error("VerifyToken() for expired token should fail")
	}
}

func TestStore_VerifyToken_Used(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	input := CreateInput{
		Email:     "used@example.com",
		Role:      "user",
		InvitedBy: primitive.NewObjectID(),
	}

	created, err := store.Create(ctx, input)
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
		t.Error("VerifyToken() for used token should fail")
	}
}

func TestStore_VerifyToken_Revoked(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	input := CreateInput{
		Email:     "revoked@example.com",
		Role:      "user",
		InvitedBy: primitive.NewObjectID(),
	}

	created, err := store.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Revoke
	if err := store.Revoke(ctx, created.ID); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}

	// Should fail
	_, err = store.VerifyToken(ctx, created.Token)
	if err == nil {
		t.Error("VerifyToken() for revoked token should fail")
	}
}

func TestStore_MarkUsed(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	input := CreateInput{
		Email:     "markused@example.com",
		Role:      "user",
		InvitedBy: primitive.NewObjectID(),
	}

	created, err := store.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	err = store.MarkUsed(ctx, created.ID)
	if err != nil {
		t.Fatalf("MarkUsed() error = %v", err)
	}

	// Verify it was marked
	inv, err := store.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if inv.UsedAt == nil {
		t.Error("UsedAt should not be nil after MarkUsed")
	}
}

func TestStore_Revoke(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	input := CreateInput{
		Email:     "revoke@example.com",
		Role:      "user",
		InvitedBy: primitive.NewObjectID(),
	}

	created, err := store.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	err = store.Revoke(ctx, created.ID)
	if err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}

	// Verify it was revoked
	inv, err := store.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if !inv.Revoked {
		t.Error("Revoked should be true after Revoke")
	}
}

func TestStore_ListPending(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	inviterID := primitive.NewObjectID()

	// Create pending invitations
	for i := 0; i < 3; i++ {
		input := CreateInput{
			Email:     "pending" + string(rune('a'+i)) + "@example.com",
			Role:      "user",
			InvitedBy: inviterID,
		}
		if _, err := store.Create(ctx, input); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	// Create used invitation
	usedInput := CreateInput{
		Email:     "used@example.com",
		Role:      "user",
		InvitedBy: inviterID,
	}
	used, err := store.Create(ctx, usedInput)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := store.MarkUsed(ctx, used.ID); err != nil {
		t.Fatalf("MarkUsed() error = %v", err)
	}

	// Create revoked invitation
	revokedInput := CreateInput{
		Email:     "revoked@example.com",
		Role:      "user",
		InvitedBy: inviterID,
	}
	revoked, err := store.Create(ctx, revokedInput)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := store.Revoke(ctx, revoked.ID); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}

	// List pending
	pending, err := store.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}
	if len(pending) != 3 {
		t.Errorf("Expected 3 pending invitations, got %d", len(pending))
	}

	// Verify none are used or revoked
	for _, inv := range pending {
		if inv.UsedAt != nil {
			t.Errorf("Pending invitation %s has UsedAt set", inv.ID.Hex())
		}
		if inv.Revoked {
			t.Errorf("Pending invitation %s is revoked", inv.ID.Hex())
		}
	}
}

func TestStore_GetByID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	input := CreateInput{
		Email:     "getbyid@example.com",
		Role:      "admin",
		InvitedBy: primitive.NewObjectID(),
	}

	created, err := store.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Valid ID
	inv, err := store.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if inv.ID != created.ID {
		t.Errorf("ID = %v, want %v", inv.ID, created.ID)
	}
	if inv.Email != input.Email {
		t.Errorf("Email = %v, want %v", inv.Email, input.Email)
	}

	// Invalid ID
	_, err = store.GetByID(ctx, primitive.NewObjectID())
	if err != mongo.ErrNoDocuments {
		t.Errorf("GetByID() for nonexistent ID error = %v, want %v", err, mongo.ErrNoDocuments)
	}
}

func TestStore_Create_UniqueTokens(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	inviterID := primitive.NewObjectID()
	tokens := make(map[string]bool)

	// Create multiple invitations and verify unique tokens
	for i := 0; i < 10; i++ {
		input := CreateInput{
			Email:     "unique" + string(rune('0'+i)) + "@example.com",
			Role:      "user",
			InvitedBy: inviterID,
		}
		inv, err := store.Create(ctx, input)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if tokens[inv.Token] {
			t.Errorf("Duplicate token generated: %s", inv.Token)
		}
		tokens[inv.Token] = true
	}
}

func TestGenerateToken(t *testing.T) {
	// Test that tokens are generated and unique
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, err := generateToken()
		if err != nil {
			t.Fatalf("generateToken() error = %v", err)
		}
		if token == "" {
			t.Error("generateToken() returned empty token")
		}
		if tokens[token] {
			t.Errorf("Duplicate token: %s", token)
		}
		tokens[token] = true
	}
}
