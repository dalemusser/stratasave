package sessions

import (
	"testing"
	"time"

	"github.com/dalemusser/stratasave/internal/testutil"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestNew(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	if store == nil {
		t.Fatal("New() returned nil")
	}
}

func TestStore_EnsureIndexes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	err := store.EnsureIndexes(ctx)
	if err != nil {
		t.Fatalf("EnsureIndexes() error = %v", err)
	}
}

func TestStore_Create(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	session := Session{
		Token:     "test-token-123",
		UserID:    primitive.NewObjectID(),
		IPAddress: "192.168.1.1",
		UserAgent: "Mozilla/5.0",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := store.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify session was created
	retrieved, err := store.GetByToken(ctx, "test-token-123")
	if err != nil {
		t.Fatalf("GetByToken() error = %v", err)
	}
	if retrieved.Token != session.Token {
		t.Errorf("Token = %v, want %v", retrieved.Token, session.Token)
	}
	if retrieved.IPAddress != session.IPAddress {
		t.Errorf("IPAddress = %v, want %v", retrieved.IPAddress, session.IPAddress)
	}
}

func TestStore_Create_WithID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	sessionID := primitive.NewObjectID()
	session := Session{
		ID:        sessionID,
		Token:     "test-token-with-id",
		UserID:    primitive.NewObjectID(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := store.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify the ID was preserved
	retrieved, err := store.GetByID(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if retrieved.ID != sessionID {
		t.Errorf("ID = %v, want %v", retrieved.ID, sessionID)
	}
}

func TestStore_GetByToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	session := Session{
		Token:     "get-by-token-test",
		UserID:    userID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := store.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{"valid token", "get-by-token-test", false},
		{"invalid token", "nonexistent-token", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.GetByToken(ctx, tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetByToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.Token != tt.token {
				t.Errorf("Token = %v, want %v", got.Token, tt.token)
			}
		})
	}
}

func TestStore_GetByToken_Expired(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create expired session
	session := Session{
		Token:     "expired-token",
		UserID:    primitive.NewObjectID(),
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired
	}

	err := store.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Should not return expired session
	_, err = store.GetByToken(ctx, "expired-token")
	if err != mongo.ErrNoDocuments {
		t.Errorf("GetByToken() for expired session error = %v, want %v", err, mongo.ErrNoDocuments)
	}
}

func TestStore_GetByID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	sessionID := primitive.NewObjectID()
	session := Session{
		ID:        sessionID,
		Token:     "get-by-id-test",
		UserID:    primitive.NewObjectID(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := store.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Valid ID
	got, err := store.GetByID(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.ID != sessionID {
		t.Errorf("ID = %v, want %v", got.ID, sessionID)
	}

	// Invalid ID
	_, err = store.GetByID(ctx, primitive.NewObjectID())
	if err != mongo.ErrNoDocuments {
		t.Errorf("GetByID() for nonexistent ID error = %v, want %v", err, mongo.ErrNoDocuments)
	}
}

func TestStore_Delete(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	session := Session{
		Token:     "delete-test-token",
		UserID:    primitive.NewObjectID(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := store.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify exists
	_, err = store.GetByToken(ctx, "delete-test-token")
	if err != nil {
		t.Fatalf("Session should exist before delete")
	}

	// Delete
	err = store.Delete(ctx, "delete-test-token")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deleted
	_, err = store.GetByToken(ctx, "delete-test-token")
	if err != mongo.ErrNoDocuments {
		t.Errorf("Session should be deleted, got error = %v", err)
	}
}

func TestStore_DeleteByID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	sessionID := primitive.NewObjectID()
	session := Session{
		ID:        sessionID,
		Token:     "delete-by-id-test",
		UserID:    primitive.NewObjectID(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := store.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	err = store.DeleteByID(ctx, sessionID)
	if err != nil {
		t.Fatalf("DeleteByID() error = %v", err)
	}

	// Verify deleted
	_, err = store.GetByID(ctx, sessionID)
	if err != mongo.ErrNoDocuments {
		t.Errorf("Session should be deleted")
	}
}

func TestStore_DeleteByUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()

	// Create multiple sessions for user
	for i := 0; i < 3; i++ {
		session := Session{
			Token:     "user-session-" + string(rune('a'+i)),
			UserID:    userID,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		if err := store.Create(ctx, session); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	// Delete all user sessions
	err := store.DeleteByUser(ctx, userID)
	if err != nil {
		t.Fatalf("DeleteByUser() error = %v", err)
	}

	// Verify all deleted
	sessions, err := store.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser() error = %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("Expected 0 sessions after DeleteByUser, got %d", len(sessions))
	}
}

func TestStore_DeleteByUserExcept(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	keepToken := "keep-this-token"

	// Create multiple sessions
	tokens := []string{keepToken, "delete-token-1", "delete-token-2"}
	for _, token := range tokens {
		session := Session{
			Token:     token,
			UserID:    userID,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		if err := store.Create(ctx, session); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	// Delete all except keepToken
	err := store.DeleteByUserExcept(ctx, userID, keepToken)
	if err != nil {
		t.Fatalf("DeleteByUserExcept() error = %v", err)
	}

	// Verify only keepToken remains
	sessions, err := store.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser() error = %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(sessions))
	}
	if len(sessions) > 0 && sessions[0].Token != keepToken {
		t.Errorf("Token = %v, want %v", sessions[0].Token, keepToken)
	}
}

func TestStore_ListByUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	otherUserID := primitive.NewObjectID()

	// Create sessions for user
	for i := 0; i < 3; i++ {
		session := Session{
			Token:     "list-user-session-" + string(rune('a'+i)),
			UserID:    userID,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		if err := store.Create(ctx, session); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	// Create session for other user
	otherSession := Session{
		Token:     "other-user-session",
		UserID:    otherUserID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := store.Create(ctx, otherSession); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Create expired session for user (should not be returned)
	expiredSession := Session{
		Token:     "expired-user-session",
		UserID:    userID,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	if err := store.Create(ctx, expiredSession); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// List sessions
	sessions, err := store.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser() error = %v", err)
	}
	if len(sessions) != 3 {
		t.Errorf("Expected 3 sessions, got %d", len(sessions))
	}

	// Verify all sessions belong to user
	for _, s := range sessions {
		if s.UserID != userID {
			t.Errorf("Session UserID = %v, want %v", s.UserID, userID)
		}
	}
}

func TestStore_UpdateActivity(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	session := Session{
		Token:        "activity-test-token",
		UserID:       primitive.NewObjectID(),
		IPAddress:    "192.168.1.1",
		UserAgent:    "Old User Agent",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		LastActivity: time.Now().Add(-1 * time.Hour),
	}

	err := store.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Get original activity time
	original, _ := store.GetByToken(ctx, "activity-test-token")
	originalActivity := original.LastActivity

	// Update activity
	time.Sleep(10 * time.Millisecond) // Ensure time difference
	err = store.UpdateActivity(ctx, "activity-test-token", "10.0.0.1", "New User Agent")
	if err != nil {
		t.Fatalf("UpdateActivity() error = %v", err)
	}

	// Verify update
	updated, err := store.GetByToken(ctx, "activity-test-token")
	if err != nil {
		t.Fatalf("GetByToken() error = %v", err)
	}

	if !updated.LastActivity.After(originalActivity) {
		t.Error("LastActivity should be updated")
	}
	if updated.IPAddress != "10.0.0.1" {
		t.Errorf("IPAddress = %v, want %v", updated.IPAddress, "10.0.0.1")
	}
	if updated.UserAgent != "New User Agent" {
		t.Errorf("UserAgent = %v, want %v", updated.UserAgent, "New User Agent")
	}
}

func TestStore_UpdateActivity_PartialUpdate(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	session := Session{
		Token:     "partial-update-token",
		UserID:    primitive.NewObjectID(),
		IPAddress: "original-ip",
		UserAgent: "Original Agent",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := store.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Update with empty IP (should not update IP)
	err = store.UpdateActivity(ctx, "partial-update-token", "", "New Agent Only")
	if err != nil {
		t.Fatalf("UpdateActivity() error = %v", err)
	}

	updated, _ := store.GetByToken(ctx, "partial-update-token")
	if updated.IPAddress != "original-ip" {
		t.Errorf("IPAddress should remain unchanged, got %v", updated.IPAddress)
	}
	if updated.UserAgent != "New Agent Only" {
		t.Errorf("UserAgent = %v, want %v", updated.UserAgent, "New Agent Only")
	}
}

func TestStore_Create_DuplicateToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Ensure indexes for unique constraint
	if err := store.EnsureIndexes(ctx); err != nil {
		t.Fatalf("EnsureIndexes() error = %v", err)
	}

	session1 := Session{
		Token:     "duplicate-token",
		UserID:    primitive.NewObjectID(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	session2 := Session{
		Token:     "duplicate-token",
		UserID:    primitive.NewObjectID(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := store.Create(ctx, session1)
	if err != nil {
		t.Fatalf("First Create() error = %v", err)
	}

	err = store.Create(ctx, session2)
	if err == nil {
		t.Error("Second Create() with duplicate token should fail")
	}
}
