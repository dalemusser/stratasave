// Package saveapi provides the save/load API endpoints for game state persistence.
//
// Endpoints:
//   - POST /save, POST /state/save - Save game state (protected with API key)
//   - POST /load, POST /state/load - Load game state (protected with API key)
//
// All game states are stored in the player_states collection.
package saveapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dalemusser/stratasave/internal/app/system/ledger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// CollectionName is the MongoDB collection for player game states.
const CollectionName = "player_states"

// PlayerState represents a saved game state in the database.
type PlayerState struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    string             `bson:"user_id"       json:"user_id"`
	Game      string             `bson:"game"          json:"game"`
	Timestamp time.Time          `bson:"timestamp"     json:"timestamp"`
	SaveData  bson.M             `bson:"save_data"     json:"save_data"`
}

// Handler handles save/load API requests.
type Handler struct {
	db              *mongo.Database
	logger          *zap.Logger
	maxSavesPerUser int       // -1 means "all" (no limit)
	indexEnsured    sync.Once // Ensure index is created once
}

// NewHandler creates a new saveapi handler.
func NewHandler(db *mongo.Database, logger *zap.Logger, maxSavesConfig string) *Handler {
	return &Handler{
		db:              db,
		logger:          logger,
		maxSavesPerUser: parseMaxSaves(maxSavesConfig),
	}
}

// parseMaxSaves parses the max_saves_per_user config value.
// Returns -1 for "all" (no limit), or the parsed number.
// Invalid values default to -1 (no limit) for safety.
func parseMaxSaves(config string) int {
	if config == "" || strings.EqualFold(config, "all") {
		return -1
	}
	n, err := strconv.Atoi(config)
	if err != nil || n <= 0 {
		return -1
	}
	return n
}

// SaveHandler handles POST /save and POST /state/save requests.
// It saves game state to the player_states collection.
//
// Request body:
//
//	{
//	    "user_id": "player123",
//	    "game": "mygame",
//	    "save_data": { ... any JSON ... }
//	}
//
// Response (201 Created):
//
//	{
//	    "id": "...",
//	    "user_id": "player123",
//	    "game": "mygame",
//	    "timestamp": "2026-01-24T...",
//	    "save_data": { ... }
//	}
func (h *Handler) SaveHandler(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID   string `json:"user_id"`
		Game     string `json:"game"`
		SaveData bson.M `json:"save_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSONError(w, r, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	if in.UserID == "" || in.Game == "" || in.SaveData == nil {
		writeJSONError(w, r, "Missing required fields", http.StatusBadRequest)
		return
	}

	state := PlayerState{
		UserID:    in.UserID,
		Game:      in.Game,
		Timestamp: time.Now().UTC(),
		SaveData:  in.SaveData,
	}

	coll := h.db.Collection(CollectionName)
	res, err := coll.InsertOne(r.Context(), state)
	if err != nil {
		h.logger.Error("failed to save game state",
			zap.String("game", in.Game),
			zap.String("user_id", in.UserID),
			zap.Error(err),
		)
		writeJSONError(w, r, "Failed to save data: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		state.ID = oid
	}

	h.logger.Debug("game state saved",
		zap.String("game", in.Game),
		zap.String("user_id", in.UserID),
		zap.String("id", state.ID.Hex()),
	)

	// Ensure index exists (once per handler lifetime)
	h.indexEnsured.Do(func() {
		if err := h.ensureIndex(r.Context()); err != nil {
			h.logger.Warn("failed to ensure player_states index", zap.Error(err))
		}
	})

	// Trigger async cleanup if retention limit is configured
	if h.maxSavesPerUser > 0 {
		go h.cleanupOldStates(in.UserID, in.Game)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(state); err != nil {
		h.logger.Error("failed to encode save response", zap.Error(err))
	}
}

// LoadHandler handles POST /load and POST /state/load requests.
// It loads game state from the player_states collection.
//
// Request body:
//
//	{
//	    "user_id": "player123",
//	    "game": "mygame",
//	    "limit": 3  // optional, defaults to 1
//	}
//
// Response (200 OK): Array of states, newest first
//
//	[
//	    {
//	        "id": "...",
//	        "user_id": "player123",
//	        "game": "mygame",
//	        "timestamp": "2026-01-24T...",
//	        "save_data": { ... }
//	    }
//	]
func (h *Handler) LoadHandler(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID string `json:"user_id"`
		Game   string `json:"game"`
		Limit  int64  `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSONError(w, r, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	if in.UserID == "" || in.Game == "" {
		writeJSONError(w, r, "Missing required fields", http.StatusBadRequest)
		return
	}
	if in.Limit <= 0 {
		in.Limit = 1
	}

	coll := h.db.Collection(CollectionName)
	filter := bson.M{"user_id": in.UserID, "game": in.Game}
	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetLimit(in.Limit)

	cur, err := coll.Find(r.Context(), filter, opts)
	if err != nil {
		h.logger.Error("failed to load game state",
			zap.String("game", in.Game),
			zap.String("user_id", in.UserID),
			zap.Error(err),
		)
		writeJSONError(w, r, "Failed to load saves: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer cur.Close(r.Context())

	var out []PlayerState
	if err := cur.All(r.Context(), &out); err != nil {
		h.logger.Error("failed to parse game state",
			zap.String("game", in.Game),
			zap.String("user_id", in.UserID),
			zap.Error(err),
		)
		writeJSONError(w, r, "Failed to parse saves: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return empty array instead of null if no states found
	if out == nil {
		out = []PlayerState{}
	}

	h.logger.Debug("game state loaded",
		zap.String("game", in.Game),
		zap.String("user_id", in.UserID),
		zap.Int("count", len(out)),
	)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		h.logger.Error("failed to encode load response", zap.Error(err))
	}
}

// writeJSONError writes a JSON error response and logs the error to the ledger.
func writeJSONError(w http.ResponseWriter, r *http.Request, msg string, code int) {
	// Set error message in ledger context for debugging
	ledger.SetErrorMessage(r.Context(), msg)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
