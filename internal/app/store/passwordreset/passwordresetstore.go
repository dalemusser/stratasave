// internal/app/store/passwordreset/passwordresetstore.go
package passwordreset

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Reset represents a password reset request.
type Reset struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    primitive.ObjectID `bson:"user_id"`
	Email     string             `bson:"email"`
	Token     string             `bson:"token"`
	Used      bool               `bson:"used"`
	ExpiresAt time.Time          `bson:"expires_at"`
	CreatedAt time.Time          `bson:"created_at"`
}

// Store provides access to the password_resets collection.
type Store struct {
	c      *mongo.Collection
	expiry time.Duration
}

// New creates a new password reset store.
func New(db *mongo.Database, expiry time.Duration) *Store {
	return &Store{
		c:      db.Collection("password_resets"),
		expiry: expiry,
	}
}

// EnsureIndexes creates necessary indexes for the collection.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "token", Value: 1}},
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

// Create creates a new password reset record and returns it.
// Any existing unused reset tokens for this user are invalidated.
func (s *Store) Create(ctx context.Context, userID primitive.ObjectID, email string) (*Reset, error) {
	// Invalidate any existing unused tokens for this user
	_, _ = s.c.UpdateMany(
		ctx,
		bson.M{"user_id": userID, "used": false},
		bson.M{"$set": bson.M{"used": true}},
	)

	token, err := generateToken()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	r := Reset{
		ID:        primitive.NewObjectID(),
		UserID:    userID,
		Email:     email,
		Token:     token,
		Used:      false,
		ExpiresAt: now.Add(s.expiry),
		CreatedAt: now,
	}

	if _, err := s.c.InsertOne(ctx, r); err != nil {
		return nil, err
	}

	return &r, nil
}

// VerifyToken verifies a reset token and returns the reset record if valid.
func (s *Store) VerifyToken(ctx context.Context, token string) (*Reset, error) {
	var r Reset
	filter := bson.M{
		"token":      token,
		"used":       false,
		"expires_at": bson.M{"$gt": time.Now()},
	}

	if err := s.c.FindOne(ctx, filter).Decode(&r); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("invalid or expired token")
		}
		return nil, err
	}

	return &r, nil
}

// MarkUsed marks a reset token as used.
func (s *Store) MarkUsed(ctx context.Context, id primitive.ObjectID) error {
	_, err := s.c.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"used": true}},
	)
	return err
}

// generateToken generates a random URL-safe token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
