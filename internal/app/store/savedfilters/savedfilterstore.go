// internal/app/store/savedfilters/savedfilterstore.go
package savedfilterstore

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SavedFilter represents a user's saved filter configuration.
type SavedFilter struct {
	ID        primitive.ObjectID `bson:"_id"`
	UserID    primitive.ObjectID `bson:"user_id"`
	Feature   string             `bson:"feature"`    // "ledger", "jobs", app-specific
	Name      string             `bson:"name"`       // "Last 24h errors"
	Filters   map[string]string  `bson:"filters"`    // Query params
	IsDefault bool               `bson:"is_default"` // Auto-apply on page load
	CreatedAt time.Time          `bson:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"`
}

var (
	// ErrNotFound is returned when a saved filter is not found.
	ErrNotFound = errors.New("saved filter not found")
	// ErrDuplicateName is returned when a filter with the same name exists.
	ErrDuplicateName = errors.New("a filter with this name already exists")
	// ErrNotOwner is returned when a user tries to modify a filter they don't own.
	ErrNotOwner = errors.New("you do not own this filter")
)

// Store provides saved filter persistence.
type Store struct {
	c *mongo.Collection
}

// New creates a new saved filter store.
func New(db *mongo.Database) *Store {
	return &Store{c: db.Collection("saved_filters")}
}

// CreateInput holds the fields for creating a new saved filter.
type CreateInput struct {
	UserID    primitive.ObjectID
	Feature   string
	Name      string
	Filters   map[string]string
	IsDefault bool
}

// Create creates a new saved filter.
func (s *Store) Create(ctx context.Context, input CreateInput) (SavedFilter, error) {
	now := time.Now()

	// If this is being set as default, clear any existing default for this user/feature
	if input.IsDefault {
		_, err := s.c.UpdateMany(ctx, bson.M{
			"user_id": input.UserID,
			"feature": input.Feature,
		}, bson.M{
			"$set": bson.M{
				"is_default": false,
				"updated_at": now,
			},
		})
		if err != nil {
			return SavedFilter{}, err
		}
	}

	filter := SavedFilter{
		ID:        primitive.NewObjectID(),
		UserID:    input.UserID,
		Feature:   input.Feature,
		Name:      input.Name,
		Filters:   input.Filters,
		IsDefault: input.IsDefault,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if _, err := s.c.InsertOne(ctx, filter); err != nil {
		if isDuplicateKeyError(err) {
			return SavedFilter{}, ErrDuplicateName
		}
		return SavedFilter{}, err
	}

	return filter, nil
}

// isDuplicateKeyError checks if the error is a duplicate key error.
func isDuplicateKeyError(err error) bool {
	var we mongo.WriteException
	if errors.As(err, &we) {
		for _, e := range we.WriteErrors {
			if e.Code == 11000 {
				return true
			}
		}
	}
	return false
}

// GetByID retrieves a saved filter by ID.
func (s *Store) GetByID(ctx context.Context, id primitive.ObjectID) (*SavedFilter, error) {
	var filter SavedFilter
	if err := s.c.FindOne(ctx, bson.M{"_id": id}).Decode(&filter); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &filter, nil
}

// ListForUser returns all saved filters for a user and feature.
func (s *Store) ListForUser(ctx context.Context, userID primitive.ObjectID, feature string) ([]SavedFilter, error) {
	query := bson.M{"user_id": userID}
	if feature != "" {
		query["feature"] = feature
	}

	opts := options.Find().SetSort(bson.D{
		{Key: "is_default", Value: -1}, // Default first
		{Key: "name", Value: 1},
	})
	cur, err := s.c.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var filters []SavedFilter
	if err := cur.All(ctx, &filters); err != nil {
		return nil, err
	}
	return filters, nil
}

// GetDefault returns the default filter for a user and feature.
func (s *Store) GetDefault(ctx context.Context, userID primitive.ObjectID, feature string) (*SavedFilter, error) {
	var filter SavedFilter
	err := s.c.FindOne(ctx, bson.M{
		"user_id":    userID,
		"feature":    feature,
		"is_default": true,
	}).Decode(&filter)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil // No default is not an error
		}
		return nil, err
	}
	return &filter, nil
}

// UpdateInput holds fields that can be updated for a saved filter.
type UpdateInput struct {
	Name      *string
	Filters   map[string]string
	IsDefault *bool
}

// Update updates a saved filter.
func (s *Store) Update(ctx context.Context, id, userID primitive.ObjectID, input UpdateInput) error {
	// Get the filter to check ownership and feature
	filter, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if filter.UserID != userID {
		return ErrNotOwner
	}

	now := time.Now()
	set := bson.M{
		"updated_at": now,
	}

	if input.Name != nil {
		set["name"] = *input.Name
	}
	if input.Filters != nil {
		set["filters"] = input.Filters
	}
	if input.IsDefault != nil {
		set["is_default"] = *input.IsDefault

		// If setting as default, clear other defaults for this user/feature
		if *input.IsDefault {
			_, err := s.c.UpdateMany(ctx, bson.M{
				"user_id": userID,
				"feature": filter.Feature,
				"_id":     bson.M{"$ne": id},
			}, bson.M{
				"$set": bson.M{
					"is_default": false,
					"updated_at": now,
				},
			})
			if err != nil {
				return err
			}
		}
	}

	result, err := s.c.UpdateOne(ctx, bson.M{"_id": id, "user_id": userID}, bson.M{"$set": set})
	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrDuplicateName
		}
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete deletes a saved filter.
func (s *Store) Delete(ctx context.Context, id, userID primitive.ObjectID) error {
	result, err := s.c.DeleteOne(ctx, bson.M{
		"_id":     id,
		"user_id": userID, // Only owner can delete
	})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// SetDefault sets a filter as the default (and clears other defaults).
func (s *Store) SetDefault(ctx context.Context, id, userID primitive.ObjectID) error {
	// Get the filter to check ownership and feature
	filter, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if filter.UserID != userID {
		return ErrNotOwner
	}

	now := time.Now()

	// Clear other defaults for this user/feature
	_, err = s.c.UpdateMany(ctx, bson.M{
		"user_id": userID,
		"feature": filter.Feature,
	}, bson.M{
		"$set": bson.M{
			"is_default": false,
			"updated_at": now,
		},
	})
	if err != nil {
		return err
	}

	// Set this one as default
	_, err = s.c.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{
			"is_default": true,
			"updated_at": now,
		},
	})
	return err
}

// ClearDefault clears the default filter for a user and feature.
func (s *Store) ClearDefault(ctx context.Context, userID primitive.ObjectID, feature string) error {
	now := time.Now()
	_, err := s.c.UpdateMany(ctx, bson.M{
		"user_id": userID,
		"feature": feature,
	}, bson.M{
		"$set": bson.M{
			"is_default": false,
			"updated_at": now,
		},
	})
	return err
}

// DeleteAllForUser deletes all saved filters for a user.
func (s *Store) DeleteAllForUser(ctx context.Context, userID primitive.ObjectID) (int64, error) {
	result, err := s.c.DeleteMany(ctx, bson.M{"user_id": userID})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

// CountForUser returns the number of saved filters for a user.
func (s *Store) CountForUser(ctx context.Context, userID primitive.ObjectID) (int64, error) {
	return s.c.CountDocuments(ctx, bson.M{"user_id": userID})
}
