package saveapi

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// cleanupOldStates removes states exceeding the retention limit for a user/game.
// Runs asynchronously after each save.
func (h *Handler) cleanupOldStates(userID, game string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	coll := h.db.Collection(CollectionName)

	// Find the Nth state's _id (the cutoff point)
	filter := bson.M{"user_id": userID, "game": game}
	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetSkip(int64(h.maxSavesPerUser)).
		SetLimit(1).
		SetProjection(bson.M{"_id": 1})

	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		h.logger.Warn("cleanup: failed to find cutoff",
			zap.String("user_id", userID),
			zap.String("game", game),
			zap.Error(err),
		)
		return
	}
	defer cursor.Close(ctx)

	if !cursor.Next(ctx) {
		// User has <= maxSavesPerUser states, nothing to delete
		return
	}

	var cutoffDoc struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	if err := cursor.Decode(&cutoffDoc); err != nil {
		h.logger.Warn("cleanup: failed to decode cutoff document",
			zap.String("user_id", userID),
			zap.String("game", game),
			zap.Error(err),
		)
		return
	}

	// Delete all states older than or equal to the cutoff
	deleteFilter := bson.M{
		"user_id": userID,
		"game":    game,
		"_id":     bson.M{"$lte": cutoffDoc.ID},
	}
	result, err := coll.DeleteMany(ctx, deleteFilter)
	if err != nil {
		h.logger.Warn("cleanup: failed to delete old states",
			zap.String("user_id", userID),
			zap.String("game", game),
			zap.Error(err),
		)
		return
	}

	if result.DeletedCount > 0 {
		h.logger.Info("cleanup: removed old states",
			zap.String("user_id", userID),
			zap.String("game", game),
			zap.Int64("deleted", result.DeletedCount),
		)
	}
}

// ensureIndex creates the index for efficient state queries/cleanup.
// This is called once per handler lifetime on first save.
func (h *Handler) ensureIndex(ctx context.Context) error {
	coll := h.db.Collection(CollectionName)
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "game", Value: 1},
			{Key: "user_id", Value: 1},
			{Key: "timestamp", Value: -1},
		},
		Options: options.Index().SetName("idx_game_user_timestamp"),
	}
	_, err := coll.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		return err
	}
	h.logger.Debug("ensured player_states index",
		zap.String("collection", CollectionName),
		zap.String("index", "idx_game_user_timestamp"),
	)
	return nil
}
