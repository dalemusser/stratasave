// internal/app/store/emailverify/emailverifystore.go
package emailverify

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

// Verification represents an email verification record.
type Verification struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Email     string             `bson:"email"`
	UserID    primitive.ObjectID `bson:"user_id"`
	Code      string             `bson:"code"`
	Token     string             `bson:"token"`
	Used      bool               `bson:"used"`
	ExpiresAt time.Time          `bson:"expires_at"`
	CreatedAt time.Time          `bson:"created_at"`
}

// Store provides access to the email_verifications collection.
type Store struct {
	c      *mongo.Collection
	expiry time.Duration
}

// New creates a new email verification store.
func New(db *mongo.Database, expiry time.Duration) *Store {
	return &Store{
		c:      db.Collection("email_verifications"),
		expiry: expiry,
	}
}

// EnsureIndexes creates necessary indexes for the collection.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "email", Value: 1}},
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

// Create creates a new verification record and returns it.
func (s *Store) Create(ctx context.Context, email string, userID primitive.ObjectID) (*Verification, error) {
	code, err := generateCode(6)
	if err != nil {
		return nil, err
	}

	token, err := generateToken()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	v := Verification{
		ID:        primitive.NewObjectID(),
		Email:     email,
		UserID:    userID,
		Code:      code,
		Token:     token,
		Used:      false,
		ExpiresAt: now.Add(s.expiry),
		CreatedAt: now,
	}

	if _, err := s.c.InsertOne(ctx, v); err != nil {
		return nil, err
	}

	return &v, nil
}

// VerifyCode verifies a code for an email and returns the verification if valid.
func (s *Store) VerifyCode(ctx context.Context, email, code string) (*Verification, error) {
	var v Verification
	filter := bson.M{
		"email":      email,
		"code":       code,
		"used":       false,
		"expires_at": bson.M{"$gt": time.Now()},
	}

	if err := s.c.FindOne(ctx, filter).Decode(&v); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("invalid or expired code")
		}
		return nil, err
	}

	return &v, nil
}

// VerifyToken verifies a magic link token and returns the verification if valid.
func (s *Store) VerifyToken(ctx context.Context, token string) (*Verification, error) {
	var v Verification
	filter := bson.M{
		"token":      token,
		"used":       false,
		"expires_at": bson.M{"$gt": time.Now()},
	}

	if err := s.c.FindOne(ctx, filter).Decode(&v); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("invalid or expired token")
		}
		return nil, err
	}

	return &v, nil
}

// MarkUsed marks a verification as used.
func (s *Store) MarkUsed(ctx context.Context, id primitive.ObjectID) error {
	_, err := s.c.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"used": true}},
	)
	return err
}

// generateCode generates a random numeric code of the specified length.
func generateCode(length int) (string, error) {
	const digits = "0123456789"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = digits[b[i]%10]
	}
	return string(b), nil
}

// generateToken generates a random URL-safe token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
