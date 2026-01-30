// Package settingsbrowser provides a web UI for browsing and managing player settings.
package settingsbrowser

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// CollectionName is the MongoDB collection for player settings.
const CollectionName = "player_settings"

// PlayerSettings represents a player's saved settings in the database.
type PlayerSettings struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"   json:"id"`
	UserID       string             `bson:"user_id"         json:"user_id"`
	Game         string             `bson:"game"            json:"game"`
	Timestamp    time.Time          `bson:"timestamp"       json:"timestamp"`
	SettingsData bson.M             `bson:"settings_data"   json:"settings_data"`
}

// Store provides database operations for the settings browser.
type Store struct {
	db     *mongo.Database
	logger *zap.Logger
}

// NewStore creates a new settings browser store.
func NewStore(db *mongo.Database, logger *zap.Logger) *Store {
	return &Store{
		db:     db,
		logger: logger,
	}
}

// ListGames returns all distinct game names from the player_settings collection.
func (s *Store) ListGames(ctx context.Context) ([]string, error) {
	coll := s.db.Collection(CollectionName)

	results, err := coll.Distinct(ctx, "game", bson.M{})
	if err != nil {
		return nil, err
	}

	games := make([]string, 0, len(results))
	for _, r := range results {
		if name, ok := r.(string); ok && name != "" {
			games = append(games, name)
		}
	}

	return games, nil
}

// ListUsers returns distinct user_ids for a game with pagination.
// Unlike state browser, no save count is needed since each user has exactly one setting.
func (s *Store) ListUsers(ctx context.Context, game, search string, page, limit int) ([]string, int64, error) {
	coll := s.db.Collection(CollectionName)

	// Build match filter
	matchFilter := bson.M{"game": game}
	if search != "" {
		matchFilter["user_id"] = bson.M{"$regex": search, "$options": "i"}
	}

	// Count total users first
	total, err := coll.CountDocuments(ctx, matchFilter)
	if err != nil {
		return nil, 0, err
	}

	// Find users with pagination
	opts := options.Find().
		SetProjection(bson.M{"user_id": 1}).
		SetSort(bson.M{"user_id": 1}).
		SetSkip(int64((page - 1) * limit)).
		SetLimit(int64(limit))

	cursor, err := coll.Find(ctx, matchFilter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		UserID string `bson:"user_id"`
	}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, 0, err
	}

	users := make([]string, len(results))
	for i, r := range results {
		users[i] = r.UserID
	}

	return users, total, nil
}

// GetSetting returns the setting for a user/game.
func (s *Store) GetSetting(ctx context.Context, game, userID string) (*PlayerSettings, error) {
	coll := s.db.Collection(CollectionName)
	var setting PlayerSettings
	err := coll.FindOne(ctx, bson.M{"game": game, "user_id": userID}).Decode(&setting)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

// DeleteSetting removes a setting for a user/game.
func (s *Store) DeleteSetting(ctx context.Context, game, userID string) error {
	coll := s.db.Collection(CollectionName)
	_, err := coll.DeleteOne(ctx, bson.M{"game": game, "user_id": userID})
	return err
}

// CreateSetting creates or updates a setting for a user/game (for dev tool).
func (s *Store) CreateSetting(ctx context.Context, game, userID string, data bson.M) error {
	coll := s.db.Collection(CollectionName)
	now := time.Now().UTC()

	filter := bson.M{"user_id": userID, "game": game}
	update := bson.M{
		"$set": bson.M{
			"settings_data": data,
			"timestamp":     now,
		},
		"$setOnInsert": bson.M{
			"user_id": userID,
			"game":    game,
		},
	}
	opts := options.Update().SetUpsert(true)
	_, err := coll.UpdateOne(ctx, filter, update, opts)
	return err
}
