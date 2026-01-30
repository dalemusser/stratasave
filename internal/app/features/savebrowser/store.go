// Package savebrowser provides a web UI for browsing and managing game saves.
package savebrowser

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// CollectionName is the MongoDB collection for player game states.
const CollectionName = "player_states"

// PlayerState represents a saved game state in the database.
// This matches the saveapi format for consistency.
type PlayerState struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    string             `bson:"user_id"       json:"user_id"`
	Game      string             `bson:"game"          json:"game"`
	Timestamp time.Time          `bson:"timestamp"     json:"timestamp"`
	SaveData  bson.M             `bson:"save_data"     json:"save_data"`
}

// Store provides database operations for the save browser.
type Store struct {
	db     *mongo.Database
	logger *zap.Logger
}

// NewStore creates a new save browser store.
func NewStore(db *mongo.Database, logger *zap.Logger) *Store {
	return &Store{
		db:     db,
		logger: logger,
	}
}

// ListGames returns all distinct game names from the player_states collection.
func (s *Store) ListGames(ctx context.Context) ([]string, error) {
	coll := s.db.Collection(CollectionName)

	// Get distinct game values
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

// ListUsers returns distinct user_ids for a game, with optional search prefix.
func (s *Store) ListUsers(ctx context.Context, game, search string, limit int) ([]string, bool, error) {
	coll := s.db.Collection(CollectionName)

	// Build aggregation pipeline
	pipeline := mongo.Pipeline{
		// Filter by game
		bson.D{{Key: "$match", Value: bson.M{"game": game}}},
	}

	// Optional search filter
	if search != "" {
		pipeline = append(pipeline, bson.D{
			{Key: "$match", Value: bson.M{"user_id": bson.M{"$regex": search, "$options": "i"}}},
		})
	}

	// Group by user_id to get distinct values
	pipeline = append(pipeline,
		bson.D{{Key: "$group", Value: bson.M{"_id": "$user_id"}}},
		bson.D{{Key: "$sort", Value: bson.M{"_id": 1}}},
		bson.D{{Key: "$limit", Value: limit + 1}}, // +1 to detect if there are more
	)

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, false, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		ID string `bson:"_id"`
	}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, false, err
	}

	users := make([]string, 0, len(results))
	for _, r := range results {
		users = append(users, r.ID)
	}

	// Check if there are more results
	hasMore := len(users) > limit
	if hasMore {
		users = users[:limit]
	}

	return users, hasMore, nil
}

// ListSaves returns saves for a user/game with keyset pagination.
// Returns saves, hasPrev, hasNext, and any error.
func (s *Store) ListSaves(ctx context.Context, game, userID string, limit int, afterID, beforeID string) ([]PlayerState, bool, bool, error) {
	coll := s.db.Collection(CollectionName)

	filter := bson.M{"user_id": userID, "game": game}
	opts := options.Find().SetLimit(int64(limit + 1))

	// Handle keyset pagination
	if afterID != "" {
		oid, err := primitive.ObjectIDFromHex(afterID)
		if err == nil {
			filter["_id"] = bson.M{"$lt": oid}
		}
		opts.SetSort(bson.D{{Key: "_id", Value: -1}})
	} else if beforeID != "" {
		oid, err := primitive.ObjectIDFromHex(beforeID)
		if err == nil {
			filter["_id"] = bson.M{"$gt": oid}
		}
		opts.SetSort(bson.D{{Key: "_id", Value: 1}})
	} else {
		// Default: newest first
		opts.SetSort(bson.D{{Key: "_id", Value: -1}})
	}

	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, false, false, err
	}
	defer cursor.Close(ctx)

	var saves []PlayerState
	if err := cursor.All(ctx, &saves); err != nil {
		return nil, false, false, err
	}

	// If we were paginating backwards, reverse the results
	if beforeID != "" {
		for i, j := 0, len(saves)-1; i < j; i, j = i+1, j-1 {
			saves[i], saves[j] = saves[j], saves[i]
		}
	}

	// Determine hasNext
	hasNext := len(saves) > limit
	if hasNext {
		saves = saves[:limit]
	}

	// Determine hasPrev by checking if there are older records
	hasPrev := false
	if len(saves) > 0 && (afterID != "" || beforeID != "") {
		hasPrev = afterID != "" // If we used afterID, there are previous records
	}
	// If we used beforeID and got results, check if there are more before
	if beforeID != "" && len(saves) > 0 {
		hasNext = true // We came from the "next" direction, so there's definitely more
		// Check if there's anything before our first result
		checkFilter := bson.M{
			"user_id": userID,
			"game":    game,
			"_id":     bson.M{"$gt": saves[len(saves)-1].ID},
		}
		count, _ := coll.CountDocuments(ctx, checkFilter, options.Count().SetLimit(1))
		hasPrev = count > 0
	}

	return saves, hasPrev, hasNext, nil
}

// CountSaves returns total saves for a user/game.
func (s *Store) CountSaves(ctx context.Context, game, userID string) (int64, error) {
	coll := s.db.Collection(CollectionName)
	return coll.CountDocuments(ctx, bson.M{"user_id": userID, "game": game})
}

// DeleteSave deletes a single save by ID.
func (s *Store) DeleteSave(ctx context.Context, game string, id primitive.ObjectID) error {
	coll := s.db.Collection(CollectionName)
	_, err := coll.DeleteOne(ctx, bson.M{"_id": id, "game": game})
	return err
}

// DeleteUserSaves deletes all saves for a user/game.
// Returns the number of deleted documents.
func (s *Store) DeleteUserSaves(ctx context.Context, game, userID string) (int64, error) {
	coll := s.db.Collection(CollectionName)
	result, err := coll.DeleteMany(ctx, bson.M{"user_id": userID, "game": game})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

// CreateState creates a new state for a user/game (for dev tool).
func (s *Store) CreateState(ctx context.Context, game, userID string, data bson.M) error {
	coll := s.db.Collection(CollectionName)
	now := time.Now().UTC()

	state := PlayerState{
		UserID:    userID,
		Game:      game,
		Timestamp: now,
		SaveData:  data,
	}

	_, err := coll.InsertOne(ctx, state)
	return err
}

// GetSave retrieves a single save by ID.
func (s *Store) GetSave(ctx context.Context, game string, id primitive.ObjectID) (*PlayerState, error) {
	coll := s.db.Collection(CollectionName)
	var save PlayerState
	err := coll.FindOne(ctx, bson.M{"_id": id, "game": game}).Decode(&save)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &save, nil
}

// UserWithCount represents a user with their save count.
type UserWithCount struct {
	UserID    string `bson:"_id"`
	SaveCount int64  `bson:"count"`
}

// ListUsersWithCounts returns distinct user_ids with their save counts for a game.
// Supports pagination and optional search filter.
func (s *Store) ListUsersWithCounts(ctx context.Context, game, search string, page, limit int) ([]UserWithCount, int64, error) {
	coll := s.db.Collection(CollectionName)

	// Build match filter
	matchFilter := bson.M{"game": game}
	if search != "" {
		matchFilter["user_id"] = bson.M{"$regex": search, "$options": "i"}
	}

	// Count total distinct users first
	countPipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: matchFilter}},
		bson.D{{Key: "$group", Value: bson.M{"_id": "$user_id"}}},
		bson.D{{Key: "$count", Value: "total"}},
	}

	countCursor, err := coll.Aggregate(ctx, countPipeline)
	if err != nil {
		return nil, 0, err
	}
	defer countCursor.Close(ctx)

	var countResult []struct {
		Total int64 `bson:"total"`
	}
	if err := countCursor.All(ctx, &countResult); err != nil {
		return nil, 0, err
	}

	var total int64
	if len(countResult) > 0 {
		total = countResult[0].Total
	}

	// Build aggregation pipeline for results
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: matchFilter}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id":   "$user_id",
			"count": bson.M{"$sum": 1},
		}}},
		bson.D{{Key: "$sort", Value: bson.M{"_id": 1}}},
		bson.D{{Key: "$skip", Value: int64((page - 1) * limit)}},
		bson.D{{Key: "$limit", Value: int64(limit)}},
	}

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var results []UserWithCount
	if err := cursor.All(ctx, &results); err != nil {
		return nil, 0, err
	}

	return results, total, nil
}
