// Package settingsapi provides the settings save/load API endpoints for player settings persistence.
//
// Endpoints:
//   - POST /settings/save - Save player settings (protected with API key)
//   - POST /settings/load - Load player settings (protected with API key)
//
// All player settings are stored in the player_settings collection.
// Unlike game state, settings are one-per-user-per-game (upsert behavior).
package settingsapi

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/dalemusser/stratasave/internal/app/system/ledger"
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

// Handler handles settings save/load API requests.
type Handler struct {
	db           *mongo.Database
	logger       *zap.Logger
	indexEnsured sync.Once // Ensure index is created once
}

// NewHandler creates a new settingsapi handler.
func NewHandler(db *mongo.Database, logger *zap.Logger) *Handler {
	return &Handler{
		db:     db,
		logger: logger,
	}
}

// SaveHandler handles POST /settings/save requests.
// It saves player settings to the player_settings collection.
// Uses upsert - one settings document per user per game.
//
// Request body:
//
//	{
//	    "user_id": "player123",
//	    "game": "mygame",
//	    "settings_data": { "audio": 0.8, "graphics": "high", ... }
//	}
//
// Response (200 OK):
//
//	{
//	    "id": "...",
//	    "user_id": "player123",
//	    "game": "mygame",
//	    "timestamp": "2026-01-26T...",
//	    "settings_data": { ... }
//	}
func (h *Handler) SaveHandler(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID       string `json:"user_id"`
		Game         string `json:"game"`
		SettingsData bson.M `json:"settings_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSONError(w, r, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	if in.UserID == "" || in.Game == "" || in.SettingsData == nil {
		writeJSONError(w, r, "Missing required fields", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	coll := h.db.Collection(CollectionName)

	// Upsert: update existing or insert new
	filter := bson.M{"user_id": in.UserID, "game": in.Game}
	update := bson.M{
		"$set": bson.M{
			"settings_data": in.SettingsData,
			"timestamp":     now,
		},
		"$setOnInsert": bson.M{
			"user_id": in.UserID,
			"game":    in.Game,
		},
	}
	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)

	var settings PlayerSettings
	err := coll.FindOneAndUpdate(r.Context(), filter, update, opts).Decode(&settings)
	if err != nil {
		h.logger.Error("failed to save player settings",
			zap.String("game", in.Game),
			zap.String("user_id", in.UserID),
			zap.Error(err),
		)
		writeJSONError(w, r, "Failed to save settings: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.logger.Debug("player settings saved",
		zap.String("game", in.Game),
		zap.String("user_id", in.UserID),
		zap.String("id", settings.ID.Hex()),
	)

	// Ensure index exists (once per handler lifetime)
	h.indexEnsured.Do(func() {
		if err := h.ensureIndex(r.Context()); err != nil {
			h.logger.Warn("failed to ensure player_settings index", zap.Error(err))
		}
	})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(settings); err != nil {
		h.logger.Error("failed to encode settings response", zap.Error(err))
	}
}

// LoadHandler handles POST /settings/load requests.
// It loads player settings from the player_settings collection.
//
// Request body:
//
//	{
//	    "user_id": "player123",
//	    "game": "mygame"
//	}
//
// Response (200 OK): The settings object, or null if not found
//
//	{
//	    "id": "...",
//	    "user_id": "player123",
//	    "game": "mygame",
//	    "timestamp": "2026-01-26T...",
//	    "settings_data": { ... }
//	}
func (h *Handler) LoadHandler(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID string `json:"user_id"`
		Game   string `json:"game"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSONError(w, r, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	if in.UserID == "" || in.Game == "" {
		writeJSONError(w, r, "Missing required fields", http.StatusBadRequest)
		return
	}

	coll := h.db.Collection(CollectionName)
	filter := bson.M{"user_id": in.UserID, "game": in.Game}

	var settings PlayerSettings
	err := coll.FindOne(r.Context(), filter).Decode(&settings)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// No settings found - return null
			h.logger.Debug("no settings found for user",
				zap.String("game", in.Game),
				zap.String("user_id", in.UserID),
			)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("null"))
			return
		}
		h.logger.Error("failed to load player settings",
			zap.String("game", in.Game),
			zap.String("user_id", in.UserID),
			zap.Error(err),
		)
		writeJSONError(w, r, "Failed to load settings: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.logger.Debug("player settings loaded",
		zap.String("game", in.Game),
		zap.String("user_id", in.UserID),
	)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(settings); err != nil {
		h.logger.Error("failed to encode settings response", zap.Error(err))
	}
}

// ensureIndex creates the unique index for efficient settings lookup.
// This is called once per handler lifetime on first save.
func (h *Handler) ensureIndex(ctx context.Context) error {
	coll := h.db.Collection(CollectionName)
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "game", Value: 1},
			{Key: "user_id", Value: 1},
		},
		Options: options.Index().
			SetName("idx_game_user").
			SetUnique(true),
	}
	_, err := coll.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		return err
	}
	h.logger.Debug("ensured player_settings index",
		zap.String("collection", CollectionName),
		zap.String("index", "idx_game_user"),
	)
	return nil
}

// writeJSONError writes a JSON error response and logs the error to the ledger.
func writeJSONError(w http.ResponseWriter, r *http.Request, msg string, code int) {
	// Set error message in ledger context for debugging
	ledger.SetErrorMessage(r.Context(), msg)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
