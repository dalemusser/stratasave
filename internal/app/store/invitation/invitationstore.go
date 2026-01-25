// internal/app/store/invitation/invitationstore.go
package invitation

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

// Invitation represents a user invitation.
type Invitation struct {
	ID        primitive.ObjectID  `bson:"_id,omitempty"`
	Email     string              `bson:"email"`
	Token     string              `bson:"token"`
	Role      string              `bson:"role"`
	InvitedBy primitive.ObjectID  `bson:"invited_by"`
	ExpiresAt time.Time           `bson:"expires_at"`
	UsedAt    *time.Time          `bson:"used_at,omitempty"`
	Revoked   bool                `bson:"revoked"`
	CreatedAt time.Time           `bson:"created_at"`
}

// Store provides access to the invitations collection.
type Store struct {
	c      *mongo.Collection
	expiry time.Duration
}

// New creates a new invitation store.
func New(db *mongo.Database, expiry time.Duration) *Store {
	return &Store{
		c:      db.Collection("invitations"),
		expiry: expiry,
	}
}

// EnsureIndexes creates necessary indexes for the collection.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "token", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
	}

	_, err := s.c.Indexes().CreateMany(ctx, indexes)
	return err
}

// CreateInput contains the input for creating an invitation.
type CreateInput struct {
	Email     string
	Role      string
	InvitedBy primitive.ObjectID
}

// Create creates a new invitation and returns it.
func (s *Store) Create(ctx context.Context, input CreateInput) (*Invitation, error) {
	token, err := generateToken()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	inv := Invitation{
		ID:        primitive.NewObjectID(),
		Email:     input.Email,
		Token:     token,
		Role:      input.Role,
		InvitedBy: input.InvitedBy,
		ExpiresAt: now.Add(s.expiry),
		Revoked:   false,
		CreatedAt: now,
	}

	if _, err := s.c.InsertOne(ctx, inv); err != nil {
		return nil, err
	}

	return &inv, nil
}

// VerifyToken verifies an invitation token and returns the invitation if valid.
func (s *Store) VerifyToken(ctx context.Context, token string) (*Invitation, error) {
	var inv Invitation
	filter := bson.M{
		"token":      token,
		"used_at":    nil,
		"revoked":    false,
		"expires_at": bson.M{"$gt": time.Now()},
	}

	if err := s.c.FindOne(ctx, filter).Decode(&inv); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("invalid or expired invitation")
		}
		return nil, err
	}

	return &inv, nil
}

// MarkUsed marks an invitation as used.
func (s *Store) MarkUsed(ctx context.Context, id primitive.ObjectID) error {
	now := time.Now()
	_, err := s.c.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"used_at": now}},
	)
	return err
}

// Revoke revokes an invitation.
func (s *Store) Revoke(ctx context.Context, id primitive.ObjectID) error {
	_, err := s.c.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"revoked": true}},
	)
	return err
}

// ListPending returns all pending (unused, not expired, not revoked) invitations.
func (s *Store) ListPending(ctx context.Context) ([]Invitation, error) {
	filter := bson.M{
		"used_at":    nil,
		"revoked":    false,
		"expires_at": bson.M{"$gt": time.Now()},
	}

	cursor, err := s.c.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var invitations []Invitation
	if err := cursor.All(ctx, &invitations); err != nil {
		return nil, err
	}

	return invitations, nil
}

// GetByID returns an invitation by ID.
func (s *Store) GetByID(ctx context.Context, id primitive.ObjectID) (*Invitation, error) {
	var inv Invitation
	if err := s.c.FindOne(ctx, bson.M{"_id": id}).Decode(&inv); err != nil {
		return nil, err
	}
	return &inv, nil
}

// generateToken generates a random URL-safe token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
