// internal/app/store/users/fetcher.go
package userstore

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"context"

	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/app/system/normalize"
	"github.com/dalemusser/stratasave/internal/app/system/timeouts"
	"github.com/dalemusser/stratasave/internal/domain/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// Fetcher implements auth.UserFetcher to load fresh user data on each request.
// It fetches user data from MongoDB.
type Fetcher struct {
	users  *mongo.Collection
	logger *zap.Logger
}

// NewFetcher creates a UserFetcher that queries the given database.
func NewFetcher(db *mongo.Database, logger *zap.Logger) *Fetcher {
	return &Fetcher{
		users:  db.Collection("users"),
		logger: logger,
	}
}

// FetchUser retrieves a user by ID and returns nil if the user is not found,
// disabled, or if any error occurs. This implements auth.UserFetcher.
func (f *Fetcher) FetchUser(ctx context.Context, userID string) *auth.SessionUser {
	// Parse the user ID
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil
	}

	// Use a short timeout for the DB query
	ctx, cancel := context.WithTimeout(ctx, timeouts.Short())
	defer cancel()

	// Fetch the user with projection for only needed fields
	var u models.User
	proj := options.FindOne().SetProjection(bson.M{
		"_id":              1,
		"full_name":        1,
		"login_id":         1,
		"login_id_ci":      1,
		"auth_method":      1,
		"role":             1,
		"status":           1,
		"theme_preference": 1,
	})

	if err := f.users.FindOne(ctx, bson.M{"_id": oid}, proj).Decode(&u); err != nil {
		// User not found or DB error
		return nil
	}

	// Check if user is disabled
	if normalize.Status(u.Status) == "disabled" {
		return nil
	}

	// Build the session user
	loginID := ""
	if u.LoginID != nil {
		loginID = *u.LoginID
	}
	su := &auth.SessionUser{
		ID:              u.ID.Hex(),
		Name:            u.FullName,
		LoginID:         loginID,
		Role:            normalize.Role(u.Role),
		ThemePreference: u.ThemePreference,
	}

	return su
}
