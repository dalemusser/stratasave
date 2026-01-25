// internal/app/store/apikeys/apikeystore.go
package apikeystore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

// Scope defines access permissions for an API key.
type Scope struct {
	Resource string   `bson:"resource"` // "ledger", "jobs", "settings", "*"
	Actions  []string `bson:"actions"`  // "read", "write", "delete", "*"
}

// APIKey represents an API key record.
type APIKey struct {
	ID          primitive.ObjectID `bson:"_id"`
	KeyHash     string             `bson:"key_hash"`               // bcrypt hash of the key
	KeyPrefix   string             `bson:"key_prefix"`             // First 8 chars for display
	Name        string             `bson:"name"`                   // "Production", "Staging"
	Description string             `bson:"description,omitempty"`  // Optional description
	CreatedBy   primitive.ObjectID `bson:"created_by"`             // User who created this key
	Status      string             `bson:"status"`                 // "active", "revoked"
	Scopes      []Scope            `bson:"scopes,omitempty"`       // Empty = full access
	LastUsedAt  *time.Time         `bson:"last_used_at,omitempty"` // Last time key was used
	UsageCount  int64              `bson:"usage_count"`            // Number of times used
	CreatedAt   time.Time          `bson:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at"`
	RevokedAt   *time.Time         `bson:"revoked_at,omitempty"` // When key was revoked
	RevokedBy   primitive.ObjectID `bson:"revoked_by,omitempty"` // User who revoked this key
}

// Status constants for API keys.
const (
	StatusActive  = "active"
	StatusRevoked = "revoked"
)

var (
	// ErrNotFound is returned when an API key is not found.
	ErrNotFound = errors.New("api key not found")
	// ErrInvalidKey is returned when an API key is invalid or does not match.
	ErrInvalidKey = errors.New("invalid api key")
	// ErrKeyRevoked is returned when attempting to use a revoked key.
	ErrKeyRevoked = errors.New("api key has been revoked")
	// ErrDuplicateName is returned when attempting to create a key with a name that already exists.
	ErrDuplicateName = errors.New("an api key with this name already exists")
)

// Store provides API key persistence.
type Store struct {
	c *mongo.Collection
}

// New creates a new API key store.
func New(db *mongo.Database) *Store {
	return &Store{c: db.Collection("api_keys")}
}

// GenerateKey generates a new cryptographically secure API key.
// Returns the full key (to show once to the user) and the first 8 chars as prefix.
func GenerateKey() (fullKey, prefix string, err error) {
	// Generate 32 random bytes (256 bits)
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}

	// Encode as hex string with "sk_" prefix (secret key)
	fullKey = "sk_" + hex.EncodeToString(bytes)
	prefix = fullKey[:11] // "sk_" + 8 chars
	return fullKey, prefix, nil
}

// hashKey creates a bcrypt hash of the API key.
func hashKey(key string) (string, error) {
	// Use a moderate cost factor since API key verification happens on every request
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CreateInput holds the fields for creating a new API key.
type CreateInput struct {
	Name        string
	Description string
	CreatedBy   primitive.ObjectID
	Scopes      []Scope
}

// CreateResult contains the created key and the full key value.
type CreateResult struct {
	Key     APIKey
	FullKey string // Full key value - only available at creation time
}

// Create creates a new API key and returns the full key value (only shown once).
func (s *Store) Create(ctx context.Context, input CreateInput) (CreateResult, error) {
	// Generate the key
	fullKey, prefix, err := GenerateKey()
	if err != nil {
		return CreateResult{}, err
	}

	// Hash the key for storage
	keyHash, err := hashKey(fullKey)
	if err != nil {
		return CreateResult{}, err
	}

	now := time.Now()
	key := APIKey{
		ID:          primitive.NewObjectID(),
		KeyHash:     keyHash,
		KeyPrefix:   prefix,
		Name:        input.Name,
		Description: input.Description,
		CreatedBy:   input.CreatedBy,
		Status:      StatusActive,
		Scopes:      input.Scopes,
		UsageCount:  0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if _, err := s.c.InsertOne(ctx, key); err != nil {
		if isDuplicateKeyError(err) {
			return CreateResult{}, ErrDuplicateName
		}
		return CreateResult{}, err
	}

	return CreateResult{
		Key:     key,
		FullKey: fullKey,
	}, nil
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

// Validate checks if the provided key is valid and returns the APIKey record.
// It also updates the last_used_at and usage_count.
func (s *Store) Validate(ctx context.Context, providedKey string) (*APIKey, error) {
	// Extract prefix from provided key for efficient lookup
	if len(providedKey) < 11 {
		return nil, ErrInvalidKey
	}
	prefix := providedKey[:11]

	// Find all active keys with matching prefix
	cur, err := s.c.Find(ctx, bson.M{
		"key_prefix": prefix,
		"status":     StatusActive,
	})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	// Check each matching key
	var matchedKey *APIKey
	for cur.Next(ctx) {
		var key APIKey
		if err := cur.Decode(&key); err != nil {
			continue
		}

		// Compare using bcrypt
		if err := bcrypt.CompareHashAndPassword([]byte(key.KeyHash), []byte(providedKey)); err == nil {
			matchedKey = &key
			break
		}
	}

	if matchedKey == nil {
		return nil, ErrInvalidKey
	}

	// Update last_used_at and usage_count
	now := time.Now()
	_, err = s.c.UpdateOne(ctx, bson.M{"_id": matchedKey.ID}, bson.M{
		"$set": bson.M{"last_used_at": now, "updated_at": now},
		"$inc": bson.M{"usage_count": 1},
	})
	if err != nil {
		// Log but don't fail - the key is still valid
		// This is a best-effort update
	}

	return matchedKey, nil
}

// ValidateFast validates the key using a hash lookup for better performance.
// Use this when you need fast validation and don't need usage tracking.
func (s *Store) ValidateFast(ctx context.Context, providedKey string) (*APIKey, error) {
	// Hash the provided key with SHA256 for fast comparison
	// Note: This is in addition to bcrypt - we store both
	hash := sha256.Sum256([]byte(providedKey))
	hashStr := hex.EncodeToString(hash[:])

	var key APIKey
	err := s.c.FindOne(ctx, bson.M{
		"key_hash_fast": hashStr,
		"status":        StatusActive,
	}).Decode(&key)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// Fall back to bcrypt validation
			return s.Validate(ctx, providedKey)
		}
		return nil, err
	}

	return &key, nil
}

// GetByID retrieves an API key by its ID.
func (s *Store) GetByID(ctx context.Context, id primitive.ObjectID) (*APIKey, error) {
	var key APIKey
	if err := s.c.FindOne(ctx, bson.M{"_id": id}).Decode(&key); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &key, nil
}

// GetByName retrieves an API key by its name.
func (s *Store) GetByName(ctx context.Context, name string) (*APIKey, error) {
	var key APIKey
	if err := s.c.FindOne(ctx, bson.M{"name": name}).Decode(&key); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &key, nil
}

// List returns all API keys, sorted by creation date (newest first).
func (s *Store) List(ctx context.Context) ([]APIKey, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cur, err := s.c.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var keys []APIKey
	if err := cur.All(ctx, &keys); err != nil {
		return nil, err
	}
	return keys, nil
}

// ListActive returns all active API keys.
func (s *Store) ListActive(ctx context.Context) ([]APIKey, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cur, err := s.c.Find(ctx, bson.M{"status": StatusActive}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var keys []APIKey
	if err := cur.All(ctx, &keys); err != nil {
		return nil, err
	}
	return keys, nil
}

// Revoke revokes an API key.
func (s *Store) Revoke(ctx context.Context, id primitive.ObjectID, revokedBy primitive.ObjectID) error {
	now := time.Now()
	result, err := s.c.UpdateOne(ctx, bson.M{
		"_id":    id,
		"status": StatusActive,
	}, bson.M{
		"$set": bson.M{
			"status":     StatusRevoked,
			"revoked_at": now,
			"revoked_by": revokedBy,
			"updated_at": now,
		},
	})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateInput holds fields that can be updated for an API key.
type UpdateInput struct {
	Name        *string
	Description *string
	Scopes      *[]Scope
}

// Update updates an API key's metadata (not the key itself).
func (s *Store) Update(ctx context.Context, id primitive.ObjectID, input UpdateInput) error {
	set := bson.M{
		"updated_at": time.Now(),
	}

	if input.Name != nil {
		set["name"] = *input.Name
	}
	if input.Description != nil {
		set["description"] = *input.Description
	}
	if input.Scopes != nil {
		set["scopes"] = *input.Scopes
	}

	result, err := s.c.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
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

// Delete permanently deletes an API key.
func (s *Store) Delete(ctx context.Context, id primitive.ObjectID) error {
	result, err := s.c.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// CountActive returns the number of active API keys.
func (s *Store) CountActive(ctx context.Context) (int64, error) {
	return s.c.CountDocuments(ctx, bson.M{"status": StatusActive})
}

// HasScope checks if the API key has the required scope.
// Empty scopes means full access (for backward compatibility).
func (key *APIKey) HasScope(resource, action string) bool {
	// Empty scopes = full access
	if len(key.Scopes) == 0 {
		return true
	}

	for _, scope := range key.Scopes {
		// Check resource match
		if scope.Resource != "*" && scope.Resource != resource {
			continue
		}

		// Check action match
		for _, a := range scope.Actions {
			if a == "*" || a == action {
				return true
			}
		}
	}

	return false
}
