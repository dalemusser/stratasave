// internal/app/store/users/userstore.go
package userstore

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"context"
	"errors"
	"time"

	"github.com/dalemusser/stratasave/internal/app/system/normalize"
	"github.com/dalemusser/stratasave/internal/app/system/status"
	"github.com/dalemusser/stratasave/internal/domain/models"
	wafflemongo "github.com/dalemusser/waffle/pantry/mongo"
	"github.com/dalemusser/waffle/pantry/text"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Store struct {
	c *mongo.Collection
}

func New(db *mongo.Database) *Store {
	return &Store{c: db.Collection("users")}
}

// GetByID loads a user by ObjectID.
func (s *Store) GetByID(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
	var u models.User
	if err := s.c.FindOne(ctx, bson.M{"_id": id}).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

// GetByIDs loads multiple users by their ObjectIDs.
func (s *Store) GetByIDs(ctx context.Context, ids []primitive.ObjectID) ([]models.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	cur, err := s.c.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var users []models.User
	if err := cur.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// GetByLoginID looks up a user by case/diacritic-insensitive login_id. Returns mongo.ErrNoDocuments if not found.
func (s *Store) GetByLoginID(ctx context.Context, loginID string) (*models.User, error) {
	var u models.User
	folded := text.Fold(loginID)
	if err := s.c.FindOne(ctx, bson.M{"login_id_ci": folded}).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

// GetByLoginIDAndAuthMethod looks up a user by login_id and auth_method.
// This is used for login to find the exact user account.
func (s *Store) GetByLoginIDAndAuthMethod(ctx context.Context, loginID, authMethod string) (*models.User, error) {
	var u models.User
	folded := text.Fold(loginID)
	if err := s.c.FindOne(ctx, bson.M{
		"login_id_ci": folded,
		"auth_method": authMethod,
	}).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

var (
	// ErrDuplicateLoginID is returned when attempting to create a user with a login_id that already exists.
	ErrDuplicateLoginID = errors.New("a user with this login ID already exists")
	errBadRole          = errors.New("invalid role")
	errBadStatus        = errors.New(`status must be "active"|"disabled"`)
)

// Create inserts a new user after normalizing & validating fields.
func (s *Store) Create(ctx context.Context, u models.User) (models.User, error) {
	// Normalize core fields
	u.ID = primitive.NewObjectID()
	u.FullName = normalize.Name(u.FullName)
	u.FullNameCI = text.Fold(u.FullName)

	// Normalize login_id fields
	if u.LoginID != nil && *u.LoginID != "" {
		loginID := normalize.Email(*u.LoginID) // lowercase
		loginIDCI := text.Fold(loginID)        // folded for case/diacritic-insensitive matching
		u.LoginID = &loginID
		u.LoginIDCI = &loginIDCI
	}

	// Normalize email if provided
	if u.Email != nil && *u.Email != "" {
		email := normalize.Email(*u.Email) // lowercase
		u.Email = &email
	}

	if u.Status == "" {
		u.Status = status.Active
	}

	// Validate role
	if !models.IsValidRole(u.Role) {
		return models.User{}, errBadRole
	}

	// Validate status
	if !status.IsValid(u.Status) {
		return models.User{}, errBadStatus
	}

	// Timestamps
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now

	// Insert
	if _, err := s.c.InsertOne(ctx, u); err != nil {
		if wafflemongo.IsDup(err) {
			return models.User{}, ErrDuplicateLoginID
		}
		return models.User{}, err
	}
	return u, nil
}

// UserUpdate holds the fields that can be updated for a user.
type UserUpdate struct {
	FullName     string
	LoginID      string
	Email        *string
	AuthMethod   string
	Role         string
	Status       string
	PasswordHash *string
	PasswordTemp *bool
}

// Update updates a user's fields.
// Returns ErrDuplicateLoginID if the login_id already exists for another user.
func (s *Store) Update(ctx context.Context, id primitive.ObjectID, upd UserUpdate) error {
	loginID := normalize.Email(upd.LoginID)
	loginIDCI := text.Fold(loginID)

	set := bson.M{
		"full_name":    upd.FullName,
		"full_name_ci": text.Fold(upd.FullName),
		"login_id":     loginID,
		"login_id_ci":  loginIDCI,
		"auth_method":  upd.AuthMethod,
		"role":         upd.Role,
		"status":       upd.Status,
		"updated_at":   time.Now(),
	}

	// Handle optional email
	if upd.Email != nil {
		set["email"] = *upd.Email
	}

	// Handle optional password reset
	if upd.PasswordHash != nil {
		set["password_hash"] = *upd.PasswordHash
		if upd.PasswordTemp != nil {
			set["password_temp"] = *upd.PasswordTemp
		}
	}

	_, err := s.c.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
	if err != nil {
		if wafflemongo.IsDup(err) {
			return ErrDuplicateLoginID
		}
		return err
	}
	return nil
}

// Delete deletes a user by ID.
// Returns the number of documents deleted (0 or 1).
func (s *Store) Delete(ctx context.Context, id primitive.ObjectID) (int64, error) {
	res, err := s.c.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

// LoginIDExistsForOther checks if a login_id already exists for a user other than the given ID.
func (s *Store) LoginIDExistsForOther(ctx context.Context, loginID string, excludeID primitive.ObjectID) (bool, error) {
	err := s.c.FindOne(ctx, bson.M{
		"login_id_ci": text.Fold(loginID),
		"_id":         bson.M{"$ne": excludeID},
	}).Err()
	if err == nil {
		return true, nil // found another user with this login_id
	}
	if err == mongo.ErrNoDocuments {
		return false, nil // no duplicate
	}
	return false, err // actual error
}

// CountActiveAdmins returns the number of users with role=admin and status=active.
func (s *Store) CountActiveAdmins(ctx context.Context) (int64, error) {
	return s.c.CountDocuments(ctx, bson.M{
		"role":   "admin",
		"status": "active",
	})
}

// Find returns users matching the given filter with optional find options.
// The caller is responsible for building the filter and options (pagination, sorting, projection).
func (s *Store) Find(ctx context.Context, filter bson.M, opts ...*options.FindOptions) ([]models.User, error) {
	cur, err := s.c.Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var users []models.User
	if err := cur.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// Count returns the number of users matching the given filter.
func (s *Store) Count(ctx context.Context, filter bson.M) (int64, error) {
	return s.c.CountDocuments(ctx, filter)
}

// UpdateThemePreference updates a user's theme preference.
// Valid values: "light", "dark", "system", or "" (empty = system default).
func (s *Store) UpdateThemePreference(ctx context.Context, id primitive.ObjectID, theme string) error {
	set := bson.M{
		"theme_preference": theme,
		"updated_at":       time.Now(),
	}
	_, err := s.c.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
	return err
}

// UpdatePassword updates a user's password hash and clears the temporary flag.
// This is used when a user changes their own password (not a temp password reset).
func (s *Store) UpdatePassword(ctx context.Context, id primitive.ObjectID, passwordHash string) error {
	set := bson.M{
		"password_hash": passwordHash,
		"password_temp": false,
		"updated_at":    time.Now(),
	}
	_, err := s.c.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
	return err
}

// ExistsByLoginID checks if a user with the given login_id exists.
func (s *Store) ExistsByLoginID(ctx context.Context, loginID string) (bool, error) {
	count, err := s.c.CountDocuments(ctx, bson.M{
		"login_id_ci": text.Fold(loginID),
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListAll returns all users sorted by full_name.
func (s *Store) ListAll(ctx context.Context) ([]models.User, error) {
	opts := options.Find().SetSort(bson.M{"full_name_ci": 1})
	cur, err := s.c.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var users []models.User
	if err := cur.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// GetByEmail looks up a user by email address (case-insensitive).
// Returns mongo.ErrNoDocuments if not found.
func (s *Store) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	normalizedEmail := normalize.Email(email)
	if err := s.c.FindOne(ctx, bson.M{"email": normalizedEmail}).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

// CreateInput holds the fields for creating a new user.
type CreateInput struct {
	FullName     string
	LoginID      string
	Email        string
	AuthMethod   string
	Role         string
	PasswordHash *string
	PasswordTemp *bool
}

// CreateFromInput creates a new user from CreateInput.
func (s *Store) CreateFromInput(ctx context.Context, input CreateInput) (models.User, error) {
	u := models.User{
		FullName:   input.FullName,
		AuthMethod: input.AuthMethod,
		Role:       input.Role,
	}

	if input.LoginID != "" {
		u.LoginID = &input.LoginID
	}
	if input.Email != "" {
		u.Email = &input.Email
	}
	if input.PasswordHash != nil {
		u.PasswordHash = input.PasswordHash
	}
	if input.PasswordTemp != nil {
		u.PasswordTemp = input.PasswordTemp
	}

	return s.Create(ctx, u)
}

// UpdateInput holds the optional fields for updating a user.
// All fields are pointers - nil means "don't update this field".
type UpdateInput struct {
	FullName        *string
	LoginID         *string
	Email           *string
	AuthMethod      *string
	Role            *string
	Status          *string
	PasswordHash    *string
	PasswordTemp    *bool
	ThemePreference *string
}

// UpdateFromInput updates a user using optional fields.
// Only non-nil fields in input are updated.
func (s *Store) UpdateFromInput(ctx context.Context, id primitive.ObjectID, input UpdateInput) error {
	set := bson.M{
		"updated_at": time.Now(),
	}

	if input.FullName != nil {
		set["full_name"] = *input.FullName
		set["full_name_ci"] = text.Fold(*input.FullName)
	}
	if input.LoginID != nil {
		loginID := normalize.Email(*input.LoginID)
		set["login_id"] = loginID
		set["login_id_ci"] = text.Fold(loginID)
	}
	if input.Email != nil {
		set["email"] = normalize.Email(*input.Email)
	}
	if input.AuthMethod != nil {
		set["auth_method"] = *input.AuthMethod
	}
	if input.Role != nil {
		set["role"] = *input.Role
	}
	if input.Status != nil {
		set["status"] = *input.Status
	}
	if input.PasswordHash != nil {
		set["password_hash"] = *input.PasswordHash
	}
	if input.PasswordTemp != nil {
		set["password_temp"] = *input.PasswordTemp
	}
	if input.ThemePreference != nil {
		set["theme_preference"] = *input.ThemePreference
	}

	_, err := s.c.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
	if err != nil {
		if wafflemongo.IsDup(err) {
			return ErrDuplicateLoginID
		}
		return err
	}
	return nil
}
