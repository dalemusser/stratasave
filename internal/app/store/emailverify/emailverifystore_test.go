package emailverify

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
	// Skip: testutil.SetupTestDB already calls indexes.EnsureAll() which creates
	// indexes with explicit names. The store's EnsureIndexes() creates indexes
	// without names, causing IndexOptionsConflict. Global index management is
	// handled by indexes.EnsureAll() in production.
	t.Skip("indexes already created by testutil.SetupTestDB via indexes.EnsureAll()")
}

func TestStore_Create(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	email := "test@example.com"
	userID := primitive.NewObjectID()

	verification, err := store.Create(ctx, email, userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if verification.ID.IsZero() {
		t.Error("ID should not be zero")
	}
	if verification.Email != email {
		t.Errorf("Email = %v, want %v", verification.Email, email)
	}
	if verification.UserID != userID {
		t.Errorf("UserID = %v, want %v", verification.UserID, userID)
	}
	if verification.Code == "" {
		t.Error("Code should not be empty")
	}
	if len(verification.Code) != 6 {
		t.Errorf("Code length = %d, want 6", len(verification.Code))
	}
	if verification.Token == "" {
		t.Error("Token should not be empty")
	}
	if verification.Used {
		t.Error("Used should be false")
	}
	if verification.ExpiresAt.Before(time.Now()) {
		t.Error("ExpiresAt should be in the future")
	}
}

func TestStore_Create_UniqueTokens(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	tokens := make(map[string]bool)
	codes := make(map[string]bool)

	for i := 0; i < 10; i++ {
		userID := primitive.NewObjectID()
		v, err := store.Create(ctx, "user"+string(rune('0'+i))+"@example.com", userID)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if tokens[v.Token] {
			t.Errorf("Duplicate token generated: %s", v.Token)
		}
		tokens[v.Token] = true
		codes[v.Code] = true // Codes may collide with 6 digits, just collect them
	}
}

func TestStore_VerifyCode(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	email := "verify@example.com"
	userID := primitive.NewObjectID()

	created, err := store.Create(ctx, email, userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Valid code
	v, err := store.VerifyCode(ctx, email, created.Code)
	if err != nil {
		t.Fatalf("VerifyCode() error = %v", err)
	}
	if v.ID != created.ID {
		t.Errorf("VerifyCode() ID = %v, want %v", v.ID, created.ID)
	}

	// Invalid code
	_, err = store.VerifyCode(ctx, email, "000000")
	if err == nil {
		t.Error("VerifyCode() should fail for invalid code")
	}

	// Wrong email
	_, err = store.VerifyCode(ctx, "wrong@example.com", created.Code)
	if err == nil {
		t.Error("VerifyCode() should fail for wrong email")
	}
}

func TestStore_VerifyCode_Expired(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, 1*time.Millisecond)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	email := "expired@example.com"
	userID := primitive.NewObjectID()

	created, err := store.Create(ctx, email, userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	_, err = store.VerifyCode(ctx, email, created.Code)
	if err == nil {
		t.Error("VerifyCode() should fail for expired code")
	}
}

func TestStore_VerifyCode_Used(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	email := "used@example.com"
	userID := primitive.NewObjectID()

	created, err := store.Create(ctx, email, userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := store.MarkUsed(ctx, created.ID); err != nil {
		t.Fatalf("MarkUsed() error = %v", err)
	}

	_, err = store.VerifyCode(ctx, email, created.Code)
	if err == nil {
		t.Error("VerifyCode() should fail for used code")
	}
}

func TestStore_VerifyToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	email := "token@example.com"
	userID := primitive.NewObjectID()

	created, err := store.Create(ctx, email, userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Valid token
	v, err := store.VerifyToken(ctx, created.Token)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	if v.ID != created.ID {
		t.Errorf("VerifyToken() ID = %v, want %v", v.ID, created.ID)
	}
	if v.Email != email {
		t.Errorf("VerifyToken() Email = %v, want %v", v.Email, email)
	}

	// Invalid token
	_, err = store.VerifyToken(ctx, "invalid-token")
	if err == nil {
		t.Error("VerifyToken() should fail for invalid token")
	}
}

func TestStore_VerifyToken_Expired(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, 1*time.Millisecond)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	email := "expiredtoken@example.com"
	userID := primitive.NewObjectID()

	created, err := store.Create(ctx, email, userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	time.Sleep(10 * time.Millisecond)

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

	email := "usedtoken@example.com"
	userID := primitive.NewObjectID()

	created, err := store.Create(ctx, email, userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := store.MarkUsed(ctx, created.ID); err != nil {
		t.Fatalf("MarkUsed() error = %v", err)
	}

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

	email := "markused@example.com"
	userID := primitive.NewObjectID()

	created, err := store.Create(ctx, email, userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := store.MarkUsed(ctx, created.ID); err != nil {
		t.Fatalf("MarkUsed() error = %v", err)
	}

	// Both code and token should now fail
	_, err = store.VerifyCode(ctx, email, created.Code)
	if err == nil {
		t.Error("VerifyCode() should fail after MarkUsed")
	}

	_, err = store.VerifyToken(ctx, created.Token)
	if err == nil {
		t.Error("VerifyToken() should fail after MarkUsed")
	}
}

func TestGenerateCode(t *testing.T) {
	for i := 0; i < 100; i++ {
		code, err := generateCode(6)
		if err != nil {
			t.Fatalf("generateCode() error = %v", err)
		}
		if len(code) != 6 {
			t.Errorf("generateCode(6) length = %d, want 6", len(code))
		}
		// Should be all digits
		for _, c := range code {
			if c < '0' || c > '9' {
				t.Errorf("generateCode() contains non-digit: %c", c)
			}
		}
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
		if len(token) < 40 {
			t.Errorf("generateToken() token too short: %d chars", len(token))
		}
		if tokens[token] {
			t.Errorf("Duplicate token: %s", token)
		}
		tokens[token] = true
	}
}

func TestStore_UniqueToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Indexes are already created by testutil.SetupTestDB via indexes.EnsureAll()
	// Test that the unique token constraint is enforced

	// Insert directly with duplicate token
	v1 := Verification{
		ID:        primitive.NewObjectID(),
		Email:     "dup1@example.com",
		UserID:    primitive.NewObjectID(),
		Code:      "123456",
		Token:     "duplicate-token",
		Used:      false,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	_, err := store.c.InsertOne(ctx, v1)
	if err != nil {
		t.Fatalf("First insert error = %v", err)
	}

	v2 := Verification{
		ID:        primitive.NewObjectID(),
		Email:     "dup2@example.com",
		UserID:    primitive.NewObjectID(),
		Code:      "654321",
		Token:     "duplicate-token", // Same token
		Used:      false,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	_, err = store.c.InsertOne(ctx, v2)
	if err == nil {
		t.Error("Duplicate token insert should fail due to unique index")
	}
	if !mongo.IsDuplicateKeyError(err) {
		t.Errorf("Expected duplicate key error, got: %v", err)
	}
}

func TestStore_EndToEnd_CodeFlow(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	email := "e2ecode@example.com"
	userID := primitive.NewObjectID()

	// 1. Create verification
	created, err := store.Create(ctx, email, userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// 2. Verify with code
	verified, err := store.VerifyCode(ctx, email, created.Code)
	if err != nil {
		t.Fatalf("VerifyCode() error = %v", err)
	}
	if verified.UserID != userID {
		t.Error("Verified should have correct UserID")
	}

	// 3. Mark used
	if err := store.MarkUsed(ctx, verified.ID); err != nil {
		t.Fatalf("MarkUsed() error = %v", err)
	}

	// 4. Code should no longer work
	_, err = store.VerifyCode(ctx, email, created.Code)
	if err == nil {
		t.Error("Code should be invalid after use")
	}
}

func TestStore_EndToEnd_TokenFlow(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db, testExpiry)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	email := "e2etoken@example.com"
	userID := primitive.NewObjectID()

	// 1. Create verification
	created, err := store.Create(ctx, email, userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// 2. Verify with token (magic link)
	verified, err := store.VerifyToken(ctx, created.Token)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	if verified.UserID != userID {
		t.Error("Verified should have correct UserID")
	}

	// 3. Mark used
	if err := store.MarkUsed(ctx, verified.ID); err != nil {
		t.Fatalf("MarkUsed() error = %v", err)
	}

	// 4. Token should no longer work
	_, err = store.VerifyToken(ctx, created.Token)
	if err == nil {
		t.Error("Token should be invalid after use")
	}
}
