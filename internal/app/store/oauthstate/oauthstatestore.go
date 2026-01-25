// internal/app/store/oauthstate/oauthstatestore.go
package oauthstate

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// State represents an OAuth state token record.
type State struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	State     string             `bson:"state"`
	ExpiresAt time.Time          `bson:"expires_at"`
	CreatedAt time.Time          `bson:"created_at"`
}

// Store provides access to the oauth_states collection.
type Store struct {
	c *mongo.Collection
}

// New creates a new OAuth state store.
func New(db *mongo.Database) *Store {
	return &Store{
		c: db.Collection("oauth_states"),
	}
}

// EnsureIndexes creates necessary indexes for the collection.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "state", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
	}

	_, err := s.c.Indexes().CreateMany(ctx, indexes)
	return err
}

// Create stores a new OAuth state token (expires in 10 minutes).
func (s *Store) Create(ctx context.Context, state string) error {
	now := time.Now()
	doc := State{
		ID:        primitive.NewObjectID(),
		State:     state,
		ExpiresAt: now.Add(10 * time.Minute),
		CreatedAt: now,
	}

	_, err := s.c.InsertOne(ctx, doc)
	return err
}

// Verify checks if a state token is valid and deletes it (single use).
// Returns true if the state was valid, false otherwise.
func (s *Store) Verify(ctx context.Context, state string) bool {
	filter := bson.M{
		"state":      state,
		"expires_at": bson.M{"$gt": time.Now()},
	}

	result := s.c.FindOneAndDelete(ctx, filter)
	return result.Err() == nil
}
