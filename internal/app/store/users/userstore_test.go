package userstore

import (
	"testing"

	"github.com/dalemusser/stratasave/internal/domain/models"
	"github.com/dalemusser/stratasave/internal/testutil"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

func TestStore_Create(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	loginID := "test@example.com"
	user := models.User{
		FullName:   "Test User",
		LoginID:    &loginID,
		AuthMethod: "password",
		Role:       "admin",
	}

	created, err := store.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify ID was assigned
	if created.ID.IsZero() {
		t.Error("Create() did not assign ID")
	}

	// Verify timestamps were set
	if created.CreatedAt.IsZero() {
		t.Error("Create() did not set CreatedAt")
	}
	if created.UpdatedAt.IsZero() {
		t.Error("Create() did not set UpdatedAt")
	}

	// Verify status defaulted to active
	if created.Status != "active" {
		t.Errorf("Create() Status = %q, want %q", created.Status, "active")
	}

	// Verify normalization
	if created.FullNameCI == "" {
		t.Error("Create() did not set FullNameCI")
	}
	if created.LoginIDCI == nil || *created.LoginIDCI == "" {
		t.Error("Create() did not set LoginIDCI")
	}
}

func TestStore_Create_InvalidRole(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	loginID := "test@example.com"
	user := models.User{
		FullName:   "Test User",
		LoginID:    &loginID,
		AuthMethod: "password",
		Role:       "invalid_role",
	}

	_, err := store.Create(ctx, user)
	if err == nil {
		t.Error("Create() with invalid role should return error")
	}
}

func TestStore_Create_DuplicateLoginID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	loginID := "duplicate@example.com"
	user1 := models.User{
		FullName:   "User One",
		LoginID:    &loginID,
		AuthMethod: "password",
		Role:       "admin",
	}

	_, err := store.Create(ctx, user1)
	if err != nil {
		t.Fatalf("Create() first user error = %v", err)
	}

	// Try to create second user with same login ID
	user2 := models.User{
		FullName:   "User Two",
		LoginID:    &loginID,
		AuthMethod: "password",
		Role:       "admin",
	}

	_, err = store.Create(ctx, user2)
	if err != ErrDuplicateLoginID {
		t.Errorf("Create() duplicate error = %v, want %v", err, ErrDuplicateLoginID)
	}
}

func TestStore_GetByID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create a user first
	loginID := "getbyid@example.com"
	user := models.User{
		FullName:   "Get By ID User",
		LoginID:    &loginID,
		AuthMethod: "password",
		Role:       "admin",
	}

	created, err := store.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Get by ID
	found, err := store.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("GetByID() ID = %v, want %v", found.ID, created.ID)
	}
	if found.FullName != created.FullName {
		t.Errorf("GetByID() FullName = %q, want %q", found.FullName, created.FullName)
	}
}

func TestStore_GetByID_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Try to get non-existent user
	nonExistentID := primitive.NewObjectID()
	_, err := store.GetByID(ctx, nonExistentID)
	if err != mongo.ErrNoDocuments {
		t.Errorf("GetByID() error = %v, want %v", err, mongo.ErrNoDocuments)
	}
}

func TestStore_GetByLoginID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create a user
	loginID := "getbylogin@example.com"
	user := models.User{
		FullName:   "Get By LoginID User",
		LoginID:    &loginID,
		AuthMethod: "password",
		Role:       "admin",
	}

	created, err := store.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Get by login ID (exact case)
	found, err := store.GetByLoginID(ctx, loginID)
	if err != nil {
		t.Fatalf("GetByLoginID() error = %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("GetByLoginID() ID = %v, want %v", found.ID, created.ID)
	}

	// Get by login ID (different case - should still work due to folding)
	found2, err := store.GetByLoginID(ctx, "GETBYLOGIN@EXAMPLE.COM")
	if err != nil {
		t.Fatalf("GetByLoginID() case-insensitive error = %v", err)
	}

	if found2.ID != created.ID {
		t.Errorf("GetByLoginID() case-insensitive ID = %v, want %v", found2.ID, created.ID)
	}
}

func TestStore_Update(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create a user
	loginID := "update@example.com"
	user := models.User{
		FullName:   "Original Name",
		LoginID:    &loginID,
		AuthMethod: "password",
		Role:       "admin",
	}

	created, err := store.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Update the user
	newLoginID := "updated@example.com"
	err = store.Update(ctx, created.ID, UserUpdate{
		FullName:   "Updated Name",
		LoginID:    newLoginID,
		AuthMethod: "password",
		Role:       "admin",
		Status:     "active",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify update
	updated, err := store.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() after update error = %v", err)
	}

	if updated.FullName != "Updated Name" {
		t.Errorf("Update() FullName = %q, want %q", updated.FullName, "Updated Name")
	}
	if *updated.LoginID != newLoginID {
		t.Errorf("Update() LoginID = %q, want %q", *updated.LoginID, newLoginID)
	}
}

func TestStore_Delete(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create a user
	loginID := "delete@example.com"
	user := models.User{
		FullName:   "Delete User",
		LoginID:    &loginID,
		AuthMethod: "password",
		Role:       "admin",
	}

	created, err := store.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Delete the user
	count, err := store.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if count != 1 {
		t.Errorf("Delete() count = %d, want 1", count)
	}

	// Verify deletion
	_, err = store.GetByID(ctx, created.ID)
	if err != mongo.ErrNoDocuments {
		t.Errorf("GetByID() after delete error = %v, want %v", err, mongo.ErrNoDocuments)
	}
}

func TestStore_Delete_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Delete non-existent user
	count, err := store.Delete(ctx, primitive.NewObjectID())
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if count != 0 {
		t.Errorf("Delete() non-existent count = %d, want 0", count)
	}
}

func TestStore_CountActiveAdmins(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Initially should be 0
	count, err := store.CountActiveAdmins(ctx)
	if err != nil {
		t.Fatalf("CountActiveAdmins() error = %v", err)
	}
	if count != 0 {
		t.Errorf("CountActiveAdmins() initial = %d, want 0", count)
	}

	// Create an active admin
	loginID := "admin@example.com"
	_, err = store.Create(ctx, models.User{
		FullName:   "Active Admin",
		LoginID:    &loginID,
		AuthMethod: "password",
		Role:       "admin",
		Status:     "active",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Should be 1 now
	count, err = store.CountActiveAdmins(ctx)
	if err != nil {
		t.Fatalf("CountActiveAdmins() error = %v", err)
	}
	if count != 1 {
		t.Errorf("CountActiveAdmins() after create = %d, want 1", count)
	}
}

func TestStore_ExistsByLoginID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	loginID := "exists@example.com"

	// Should not exist initially
	exists, err := store.ExistsByLoginID(ctx, loginID)
	if err != nil {
		t.Fatalf("ExistsByLoginID() error = %v", err)
	}
	if exists {
		t.Error("ExistsByLoginID() should return false for non-existent user")
	}

	// Create user
	_, err = store.Create(ctx, models.User{
		FullName:   "Exists User",
		LoginID:    &loginID,
		AuthMethod: "password",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Should exist now
	exists, err = store.ExistsByLoginID(ctx, loginID)
	if err != nil {
		t.Fatalf("ExistsByLoginID() error = %v", err)
	}
	if !exists {
		t.Error("ExistsByLoginID() should return true for existing user")
	}
}

func TestStore_CreateFromInput(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	input := CreateInput{
		FullName:   "Input User",
		LoginID:    "input@example.com",
		Email:      "input@example.com",
		AuthMethod: "password",
		Role:       "admin",
	}

	created, err := store.CreateFromInput(ctx, input)
	if err != nil {
		t.Fatalf("CreateFromInput() error = %v", err)
	}

	if created.FullName != input.FullName {
		t.Errorf("CreateFromInput() FullName = %q, want %q", created.FullName, input.FullName)
	}
	if created.Email == nil || *created.Email != input.Email {
		t.Errorf("CreateFromInput() Email not set correctly")
	}
}

func TestStore_UpdateFromInput(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create a user
	loginID := "updateinput@example.com"
	created, err := store.Create(ctx, models.User{
		FullName:   "Original",
		LoginID:    &loginID,
		AuthMethod: "password",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Update with partial input
	newName := "Updated via Input"
	err = store.UpdateFromInput(ctx, created.ID, UpdateInput{
		FullName: &newName,
	})
	if err != nil {
		t.Fatalf("UpdateFromInput() error = %v", err)
	}

	// Verify only FullName changed
	updated, err := store.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if updated.FullName != newName {
		t.Errorf("UpdateFromInput() FullName = %q, want %q", updated.FullName, newName)
	}
	// LoginID should be unchanged
	if *updated.LoginID != loginID {
		t.Errorf("UpdateFromInput() changed LoginID unexpectedly")
	}
}

func TestStore_LoginIDExistsForOther(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	loginID := "checkother@example.com"

	// Create first user
	user1, err := store.Create(ctx, models.User{
		FullName:   "User One",
		LoginID:    &loginID,
		AuthMethod: "password",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Check if login ID exists for a different user (should return false - same user)
	exists, err := store.LoginIDExistsForOther(ctx, loginID, user1.ID)
	if err != nil {
		t.Fatalf("LoginIDExistsForOther() error = %v", err)
	}
	if exists {
		t.Error("LoginIDExistsForOther() should return false when checking same user")
	}

	// Create second user
	loginID2 := "another@example.com"
	user2, err := store.Create(ctx, models.User{
		FullName:   "User Two",
		LoginID:    &loginID2,
		AuthMethod: "password",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("Create() second user error = %v", err)
	}

	// Check if user1's login ID exists for user2 (should return true)
	exists, err = store.LoginIDExistsForOther(ctx, loginID, user2.ID)
	if err != nil {
		t.Fatalf("LoginIDExistsForOther() error = %v", err)
	}
	if !exists {
		t.Error("LoginIDExistsForOther() should return true when another user has the login ID")
	}
}

func TestStore_GetByEmail(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	email := "email@example.com"
	loginID := "emailuser@example.com"

	// Create user with email
	created, err := store.Create(ctx, models.User{
		FullName:   "Email User",
		LoginID:    &loginID,
		Email:      &email,
		AuthMethod: "password",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Get by email
	found, err := store.GetByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetByEmail() error = %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("GetByEmail() ID = %v, want %v", found.ID, created.ID)
	}

	// Get by email (different case)
	found2, err := store.GetByEmail(ctx, "EMAIL@EXAMPLE.COM")
	if err != nil {
		t.Fatalf("GetByEmail() case-insensitive error = %v", err)
	}

	if found2.ID != created.ID {
		t.Errorf("GetByEmail() case-insensitive ID = %v, want %v", found2.ID, created.ID)
	}

	// Non-existent email
	_, err = store.GetByEmail(ctx, "nonexistent@example.com")
	if err != mongo.ErrNoDocuments {
		t.Errorf("GetByEmail() non-existent error = %v, want %v", err, mongo.ErrNoDocuments)
	}
}

func TestStore_ListAll(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Initially empty
	users, err := store.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}
	if len(users) != 0 {
		t.Errorf("ListAll() initially = %d users, want 0", len(users))
	}

	// Create some users
	loginID1 := "zebra@example.com"
	loginID2 := "apple@example.com"

	_, err = store.Create(ctx, models.User{
		FullName:   "Zebra User",
		LoginID:    &loginID1,
		AuthMethod: "password",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("Create() first user error = %v", err)
	}

	_, err = store.Create(ctx, models.User{
		FullName:   "Apple User",
		LoginID:    &loginID2,
		AuthMethod: "password",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("Create() second user error = %v", err)
	}

	// List all - should be sorted by name
	users, err = store.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}
	if len(users) != 2 {
		t.Errorf("ListAll() = %d users, want 2", len(users))
	}

	// First should be Apple (sorted by name)
	if users[0].FullName != "Apple User" {
		t.Errorf("ListAll() first user = %q, want %q (sorted)", users[0].FullName, "Apple User")
	}
}

func TestStore_UpdateThemePreference(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create user
	loginID := "theme@example.com"
	created, err := store.Create(ctx, models.User{
		FullName:   "Theme User",
		LoginID:    &loginID,
		AuthMethod: "password",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Update theme
	err = store.UpdateThemePreference(ctx, created.ID, "dark")
	if err != nil {
		t.Fatalf("UpdateThemePreference() error = %v", err)
	}

	// Verify
	updated, err := store.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if updated.ThemePreference != "dark" {
		t.Errorf("UpdateThemePreference() = %q, want %q", updated.ThemePreference, "dark")
	}
}

func TestStore_UpdatePassword(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create user with temp password
	loginID := "password@example.com"
	passwordHash := "initial_hash"
	tempFlag := true
	created, err := store.Create(ctx, models.User{
		FullName:     "Password User",
		LoginID:      &loginID,
		AuthMethod:   "password",
		Role:         "admin",
		PasswordHash: &passwordHash,
		PasswordTemp: &tempFlag,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Update password
	newHash := "new_secure_hash"
	err = store.UpdatePassword(ctx, created.ID, newHash)
	if err != nil {
		t.Fatalf("UpdatePassword() error = %v", err)
	}

	// Verify
	updated, err := store.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if updated.PasswordHash == nil || *updated.PasswordHash != newHash {
		t.Error("UpdatePassword() did not set new hash")
	}
	if updated.PasswordTemp == nil || *updated.PasswordTemp != false {
		t.Error("UpdatePassword() should clear temp flag")
	}
}

func TestStore_Find(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create users
	loginID1 := "find1@example.com"
	loginID2 := "find2@example.com"

	_, err := store.Create(ctx, models.User{
		FullName:   "Find User Active",
		LoginID:    &loginID1,
		AuthMethod: "password",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("Create() first user error = %v", err)
	}

	created2, err := store.Create(ctx, models.User{
		FullName:   "Find User Disabled",
		LoginID:    &loginID2,
		AuthMethod: "password",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("Create() second user error = %v", err)
	}

	// Update second user to disabled
	store.Update(ctx, created2.ID, UserUpdate{
		FullName:   "Find User Disabled",
		LoginID:    loginID2,
		AuthMethod: "password",
		Role:       "admin",
		Status:     "disabled",
	})

	// Find all
	users, err := store.Find(ctx, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Find() all error = %v", err)
	}
	if len(users) != 2 {
		t.Errorf("Find() all = %d, want 2", len(users))
	}

	// Find active only
	activeUsers, err := store.Find(ctx, map[string]interface{}{"status": "active"})
	if err != nil {
		t.Fatalf("Find() active error = %v", err)
	}
	if len(activeUsers) != 1 {
		t.Errorf("Find() active = %d, want 1", len(activeUsers))
	}
}

func TestStore_Count(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Initially 0
	count, err := store.Count(ctx, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 0 {
		t.Errorf("Count() initial = %d, want 0", count)
	}

	// Create users
	loginID1 := "count1@example.com"
	loginID2 := "count2@example.com"

	_, err = store.Create(ctx, models.User{
		FullName:   "Count User 1",
		LoginID:    &loginID1,
		AuthMethod: "password",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("Create() first user error = %v", err)
	}

	_, err = store.Create(ctx, models.User{
		FullName:   "Count User 2",
		LoginID:    &loginID2,
		AuthMethod: "password",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("Create() second user error = %v", err)
	}

	// Count all
	count, err = store.Count(ctx, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 2 {
		t.Errorf("Count() = %d, want 2", count)
	}
}

func TestStore_GetByIDs(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Empty IDs should return nil
	users, err := store.GetByIDs(ctx, []primitive.ObjectID{})
	if err != nil {
		t.Fatalf("GetByIDs() empty error = %v", err)
	}
	if users != nil {
		t.Error("GetByIDs() empty should return nil")
	}

	// Create some users
	loginID1 := "getbyids1@example.com"
	loginID2 := "getbyids2@example.com"
	loginID3 := "getbyids3@example.com"

	user1, _ := store.Create(ctx, models.User{
		FullName:   "User One",
		LoginID:    &loginID1,
		AuthMethod: "password",
		Role:       "admin",
	})
	user2, _ := store.Create(ctx, models.User{
		FullName:   "User Two",
		LoginID:    &loginID2,
		AuthMethod: "password",
		Role:       "admin",
	})
	store.Create(ctx, models.User{
		FullName:   "User Three",
		LoginID:    &loginID3,
		AuthMethod: "password",
		Role:       "admin",
	})

	// Get by multiple IDs
	users, err = store.GetByIDs(ctx, []primitive.ObjectID{user1.ID, user2.ID})
	if err != nil {
		t.Fatalf("GetByIDs() error = %v", err)
	}
	if len(users) != 2 {
		t.Errorf("GetByIDs() = %d users, want 2", len(users))
	}

	// Get by single ID
	users, err = store.GetByIDs(ctx, []primitive.ObjectID{user1.ID})
	if err != nil {
		t.Fatalf("GetByIDs() single error = %v", err)
	}
	if len(users) != 1 {
		t.Errorf("GetByIDs() single = %d users, want 1", len(users))
	}

	// Get by non-existent ID
	users, err = store.GetByIDs(ctx, []primitive.ObjectID{primitive.NewObjectID()})
	if err != nil {
		t.Fatalf("GetByIDs() non-existent error = %v", err)
	}
	if len(users) != 0 {
		t.Errorf("GetByIDs() non-existent = %d users, want 0", len(users))
	}
}

func TestStore_GetByLoginIDAndAuthMethod(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	loginID := "authmethod@example.com"

	// Create user with password auth
	created, err := store.Create(ctx, models.User{
		FullName:   "Auth Method User",
		LoginID:    &loginID,
		AuthMethod: "password",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Find with correct auth method
	found, err := store.GetByLoginIDAndAuthMethod(ctx, loginID, "password")
	if err != nil {
		t.Fatalf("GetByLoginIDAndAuthMethod() error = %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("GetByLoginIDAndAuthMethod() ID = %v, want %v", found.ID, created.ID)
	}

	// Find with wrong auth method
	_, err = store.GetByLoginIDAndAuthMethod(ctx, loginID, "google")
	if err != mongo.ErrNoDocuments {
		t.Errorf("GetByLoginIDAndAuthMethod() wrong auth error = %v, want %v", err, mongo.ErrNoDocuments)
	}

	// Find with non-existent login ID
	_, err = store.GetByLoginIDAndAuthMethod(ctx, "nonexistent@example.com", "password")
	if err != mongo.ErrNoDocuments {
		t.Errorf("GetByLoginIDAndAuthMethod() non-existent error = %v, want %v", err, mongo.ErrNoDocuments)
	}

	// Case-insensitive lookup
	found2, err := store.GetByLoginIDAndAuthMethod(ctx, "AUTHMETHOD@EXAMPLE.COM", "password")
	if err != nil {
		t.Fatalf("GetByLoginIDAndAuthMethod() case-insensitive error = %v", err)
	}
	if found2.ID != created.ID {
		t.Errorf("GetByLoginIDAndAuthMethod() case-insensitive ID = %v, want %v", found2.ID, created.ID)
	}
}

func TestStore_Update_AllFields(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create user
	loginID := "updateall@example.com"
	created, err := store.Create(ctx, models.User{
		FullName:   "Original",
		LoginID:    &loginID,
		AuthMethod: "password",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Update all fields including password
	newLoginID := "updatedall@example.com"
	passwordHash := "new_hash_value"
	tempFlag := true
	err = store.Update(ctx, created.ID, UserUpdate{
		FullName:     "Updated All",
		LoginID:      newLoginID,
		AuthMethod:   "email",
		Role:         "admin",
		Status:       "active",
		PasswordHash: &passwordHash,
		PasswordTemp: &tempFlag,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify all fields updated
	updated, _ := store.GetByID(ctx, created.ID)
	if updated.FullName != "Updated All" {
		t.Errorf("Update() FullName = %q, want %q", updated.FullName, "Updated All")
	}
	if updated.AuthMethod != "email" {
		t.Errorf("Update() AuthMethod = %q, want %q", updated.AuthMethod, "email")
	}
	if updated.PasswordHash == nil || *updated.PasswordHash != passwordHash {
		t.Error("Update() did not set PasswordHash")
	}
	if updated.PasswordTemp == nil || *updated.PasswordTemp != true {
		t.Error("Update() did not set PasswordTemp")
	}
}

func TestStore_Update_DuplicateLoginID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create two users
	loginID1 := "dup1@example.com"
	loginID2 := "dup2@example.com"

	store.Create(ctx, models.User{
		FullName:   "User One",
		LoginID:    &loginID1,
		AuthMethod: "password",
		Role:       "admin",
	})

	user2, _ := store.Create(ctx, models.User{
		FullName:   "User Two",
		LoginID:    &loginID2,
		AuthMethod: "password",
		Role:       "admin",
	})

	// Try to update user2 with user1's login ID
	err := store.Update(ctx, user2.ID, UserUpdate{
		FullName:   "User Two",
		LoginID:    loginID1, // Duplicate!
		AuthMethod: "password",
		Role:       "admin",
		Status:     "active",
	})
	if err != ErrDuplicateLoginID {
		t.Errorf("Update() duplicate error = %v, want %v", err, ErrDuplicateLoginID)
	}
}

func TestStore_UpdateFromInput_AllFields(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create user
	loginID := "updateinputall@example.com"
	email := "original@example.com"
	created, _ := store.Create(ctx, models.User{
		FullName:   "Original",
		LoginID:    &loginID,
		Email:      &email,
		AuthMethod: "password",
		Role:       "admin",
	})

	// Update with all optional fields
	newName := "Updated Name"
	newLoginID := "newlogin@example.com"
	newEmail := "newemail@example.com"
	newAuthMethod := "email"
	newRole := "admin"
	newStatus := "active"
	newHash := "new_password_hash"
	newTemp := true
	newTheme := "dark"

	err := store.UpdateFromInput(ctx, created.ID, UpdateInput{
		FullName:        &newName,
		LoginID:         &newLoginID,
		Email:           &newEmail,
		AuthMethod:      &newAuthMethod,
		Role:            &newRole,
		Status:          &newStatus,
		PasswordHash:    &newHash,
		PasswordTemp:    &newTemp,
		ThemePreference: &newTheme,
	})
	if err != nil {
		t.Fatalf("UpdateFromInput() error = %v", err)
	}

	// Verify
	updated, _ := store.GetByID(ctx, created.ID)
	if updated.FullName != newName {
		t.Errorf("UpdateFromInput() FullName = %q, want %q", updated.FullName, newName)
	}
	if *updated.LoginID != newLoginID {
		t.Errorf("UpdateFromInput() LoginID = %q, want %q", *updated.LoginID, newLoginID)
	}
	if *updated.Email != newEmail {
		t.Errorf("UpdateFromInput() Email = %q, want %q", *updated.Email, newEmail)
	}
	if updated.ThemePreference != newTheme {
		t.Errorf("UpdateFromInput() ThemePreference = %q, want %q", updated.ThemePreference, newTheme)
	}
}

func TestStore_UpdateFromInput_DuplicateLoginID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create two users
	loginID1 := "inputdup1@example.com"
	loginID2 := "inputdup2@example.com"

	store.Create(ctx, models.User{
		FullName:   "User One",
		LoginID:    &loginID1,
		AuthMethod: "password",
		Role:       "admin",
	})

	user2, _ := store.Create(ctx, models.User{
		FullName:   "User Two",
		LoginID:    &loginID2,
		AuthMethod: "password",
		Role:       "admin",
	})

	// Try to update user2 with user1's login ID
	dupLoginID := loginID1
	err := store.UpdateFromInput(ctx, user2.ID, UpdateInput{
		LoginID: &dupLoginID,
	})
	if err != ErrDuplicateLoginID {
		t.Errorf("UpdateFromInput() duplicate error = %v, want %v", err, ErrDuplicateLoginID)
	}
}

func TestFetcher_NewFetcher(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	fetcher := NewFetcher(db, logger)
	if fetcher == nil {
		t.Error("NewFetcher() returned nil")
	}
}

func TestFetcher_FetchUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	logger := zap.NewNop()
	fetcher := NewFetcher(db, logger)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create an active user
	loginID := "fetchuser@example.com"
	created, err := store.Create(ctx, models.User{
		FullName:        "Fetch User",
		LoginID:         &loginID,
		AuthMethod:      "password",
		Role:            "admin",
		ThemePreference: "dark",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Fetch the user
	sessionUser := fetcher.FetchUser(ctx, created.ID.Hex())
	if sessionUser == nil {
		t.Fatal("FetchUser() returned nil for existing user")
	}

	if sessionUser.ID != created.ID.Hex() {
		t.Errorf("FetchUser() ID = %q, want %q", sessionUser.ID, created.ID.Hex())
	}
	if sessionUser.Name != "Fetch User" {
		t.Errorf("FetchUser() Name = %q, want %q", sessionUser.Name, "Fetch User")
	}
	if sessionUser.LoginID != loginID {
		t.Errorf("FetchUser() LoginID = %q, want %q", sessionUser.LoginID, loginID)
	}
	if sessionUser.Role != "admin" {
		t.Errorf("FetchUser() Role = %q, want %q", sessionUser.Role, "admin")
	}
	if sessionUser.ThemePreference != "dark" {
		t.Errorf("FetchUser() ThemePreference = %q, want %q", sessionUser.ThemePreference, "dark")
	}
}

func TestFetcher_FetchUser_InvalidID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()
	fetcher := NewFetcher(db, logger)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Invalid ObjectID format
	sessionUser := fetcher.FetchUser(ctx, "invalid-id")
	if sessionUser != nil {
		t.Error("FetchUser() invalid ID should return nil")
	}
}

func TestFetcher_FetchUser_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()
	fetcher := NewFetcher(db, logger)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Non-existent user
	sessionUser := fetcher.FetchUser(ctx, primitive.NewObjectID().Hex())
	if sessionUser != nil {
		t.Error("FetchUser() non-existent user should return nil")
	}
}

func TestFetcher_FetchUser_Disabled(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	logger := zap.NewNop()
	fetcher := NewFetcher(db, logger)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create user
	loginID := "disabled@example.com"
	created, _ := store.Create(ctx, models.User{
		FullName:   "Disabled User",
		LoginID:    &loginID,
		AuthMethod: "password",
		Role:       "admin",
	})

	// Disable the user directly in the database
	db.Collection("users").UpdateOne(ctx, bson.M{"_id": created.ID}, bson.M{
		"$set": bson.M{"status": "disabled"},
	})

	// Fetch should return nil for disabled user
	sessionUser := fetcher.FetchUser(ctx, created.ID.Hex())
	if sessionUser != nil {
		t.Error("FetchUser() disabled user should return nil")
	}
}

func TestFetcher_FetchUser_NoLoginID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	logger := zap.NewNop()
	fetcher := NewFetcher(db, logger)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create user without LoginID (e.g., OAuth user)
	email := "oauth@example.com"
	created, err := store.Create(ctx, models.User{
		FullName:   "OAuth User",
		Email:      &email,
		AuthMethod: "google",
		Role:       "admin",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Fetch should work and LoginID should be empty
	sessionUser := fetcher.FetchUser(ctx, created.ID.Hex())
	if sessionUser == nil {
		t.Fatal("FetchUser() returned nil for user without LoginID")
	}
	if sessionUser.LoginID != "" {
		t.Errorf("FetchUser() LoginID = %q, want empty", sessionUser.LoginID)
	}
}
