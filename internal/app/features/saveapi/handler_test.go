package saveapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dalemusser/stratasave/internal/testutil"
	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"
)

func TestHandler_SaveHandler(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()
	h := NewHandler(db, logger, "all")

	t.Run("successful save", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id": "player123",
			"game":    "testgame",
			"save_data": map[string]interface{}{
				"level":     5,
				"score":     1000,
				"inventory": []string{"sword", "shield"},
			},
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/save", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.SaveHandler(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("SaveHandler() status = %d, want %d", rec.Code, http.StatusCreated)
		}

		var resp PlayerState
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.UserID != "player123" {
			t.Errorf("response user_id = %q, want %q", resp.UserID, "player123")
		}
		if resp.Game != "testgame" {
			t.Errorf("response game = %q, want %q", resp.Game, "testgame")
		}
		if resp.ID.IsZero() {
			t.Error("response id should not be empty")
		}
		if resp.Timestamp.IsZero() {
			t.Error("response timestamp should not be empty")
		}
		if resp.SaveData == nil {
			t.Error("response save_data should not be nil")
		}
	})

	t.Run("missing user_id", func(t *testing.T) {
		body := map[string]interface{}{
			"game":      "testgame",
			"save_data": map[string]interface{}{"level": 1},
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/save", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.SaveHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("SaveHandler() status = %d, want %d", rec.Code, http.StatusBadRequest)
		}

		var resp map[string]string
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp["error"] != "Missing required fields" {
			t.Errorf("error message = %q, want %q", resp["error"], "Missing required fields")
		}
	})

	t.Run("missing game", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id":   "player123",
			"save_data": map[string]interface{}{"level": 1},
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/save", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.SaveHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("SaveHandler() status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing save_data", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id": "player123",
			"game":    "testgame",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/save", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.SaveHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("SaveHandler() status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/save", bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.SaveHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("SaveHandler() status = %d, want %d", rec.Code, http.StatusBadRequest)
		}

		var resp map[string]string
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp["error"] != "Invalid JSON payload" {
			t.Errorf("error message = %q, want %q", resp["error"], "Invalid JSON payload")
		}
	})
}

func TestHandler_LoadHandler(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()
	h := NewHandler(db, logger, "all")

	// First, create some test saves
	setupTestSaves := func() {
		coll := db.Collection(CollectionName)
		ctx, cancel := testutil.TestContext()
		defer cancel()

		saves := []interface{}{
			bson.M{
				"user_id":   "player123",
				"game":      "testgame",
				"timestamp": "2026-01-24T10:00:00Z",
				"save_data": bson.M{"level": 1, "score": 100},
			},
			bson.M{
				"user_id":   "player123",
				"game":      "testgame",
				"timestamp": "2026-01-24T11:00:00Z",
				"save_data": bson.M{"level": 2, "score": 200},
			},
			bson.M{
				"user_id":   "player123",
				"game":      "testgame",
				"timestamp": "2026-01-24T12:00:00Z",
				"save_data": bson.M{"level": 3, "score": 300},
			},
			bson.M{
				"user_id":   "otherplayer",
				"game":      "testgame",
				"timestamp": "2026-01-24T10:00:00Z",
				"save_data": bson.M{"level": 5, "score": 500},
			},
		}
		coll.InsertMany(ctx, saves)
	}

	t.Run("load single save (default limit)", func(t *testing.T) {
		setupTestSaves()

		body := map[string]interface{}{
			"user_id": "player123",
			"game":    "testgame",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/load", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.LoadHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("LoadHandler() status = %d, want %d", rec.Code, http.StatusOK)
		}

		var resp []PlayerState
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(resp) != 1 {
			t.Errorf("response length = %d, want 1", len(resp))
		}
	})

	t.Run("load with limit", func(t *testing.T) {
		setupTestSaves()

		body := map[string]interface{}{
			"user_id": "player123",
			"game":    "testgame",
			"limit":   3,
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/load", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.LoadHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("LoadHandler() status = %d, want %d", rec.Code, http.StatusOK)
		}

		var resp []PlayerState
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(resp) != 3 {
			t.Errorf("response length = %d, want 3", len(resp))
		}

		// Verify they're the correct user's saves
		for _, save := range resp {
			if save.UserID != "player123" {
				t.Errorf("save user_id = %q, want %q", save.UserID, "player123")
			}
		}
	})

	t.Run("load no results", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id": "nonexistent",
			"game":    "testgame",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/load", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.LoadHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("LoadHandler() status = %d, want %d", rec.Code, http.StatusOK)
		}

		var resp []PlayerState
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Should return empty array, not null
		if resp == nil {
			t.Error("response should be empty array, not nil")
		}
		if len(resp) != 0 {
			t.Errorf("response length = %d, want 0", len(resp))
		}
	})

	t.Run("missing user_id", func(t *testing.T) {
		body := map[string]interface{}{
			"game": "testgame",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/load", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.LoadHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("LoadHandler() status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing game", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id": "player123",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/load", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.LoadHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("LoadHandler() status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/load", bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.LoadHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("LoadHandler() status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}

func TestHandler_SaveAndLoad_Integration(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()
	h := NewHandler(db, logger, "all")

	// Save some data
	saveBody := map[string]interface{}{
		"user_id": "integrationtest",
		"game":    "integrationgame",
		"save_data": map[string]interface{}{
			"checkpoint": "boss_room",
			"health":     100,
			"items":      []string{"key", "potion"},
		},
	}
	saveBytes, _ := json.Marshal(saveBody)

	saveReq := httptest.NewRequest(http.MethodPost, "/save", bytes.NewReader(saveBytes))
	saveReq.Header.Set("Content-Type", "application/json")
	saveRec := httptest.NewRecorder()

	h.SaveHandler(saveRec, saveReq)

	if saveRec.Code != http.StatusCreated {
		t.Fatalf("SaveHandler() status = %d, want %d", saveRec.Code, http.StatusCreated)
	}

	var savedState PlayerState
	json.NewDecoder(saveRec.Body).Decode(&savedState)

	// Load the data back
	loadBody := map[string]interface{}{
		"user_id": "integrationtest",
		"game":    "integrationgame",
	}
	loadBytes, _ := json.Marshal(loadBody)

	loadReq := httptest.NewRequest(http.MethodPost, "/load", bytes.NewReader(loadBytes))
	loadReq.Header.Set("Content-Type", "application/json")
	loadRec := httptest.NewRecorder()

	h.LoadHandler(loadRec, loadReq)

	if loadRec.Code != http.StatusOK {
		t.Fatalf("LoadHandler() status = %d, want %d", loadRec.Code, http.StatusOK)
	}

	var loadedStates []PlayerState
	json.NewDecoder(loadRec.Body).Decode(&loadedStates)

	if len(loadedStates) != 1 {
		t.Fatalf("expected 1 save, got %d", len(loadedStates))
	}

	loaded := loadedStates[0]
	if loaded.ID != savedState.ID {
		t.Errorf("loaded ID = %s, want %s", loaded.ID.Hex(), savedState.ID.Hex())
	}
	if loaded.UserID != "integrationtest" {
		t.Errorf("loaded user_id = %q, want %q", loaded.UserID, "integrationtest")
	}
	if loaded.Game != "integrationgame" {
		t.Errorf("loaded game = %q, want %q", loaded.Game, "integrationgame")
	}

	// Verify save_data contents
	saveData := loaded.SaveData
	if saveData["checkpoint"] != "boss_room" {
		t.Errorf("save_data checkpoint = %v, want %q", saveData["checkpoint"], "boss_room")
	}
}

func TestRoutes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()
	h := NewHandler(db, logger, "all")

	router := Routes(h, nil, "test-api-key", logger)
	if router == nil {
		t.Fatal("Routes() returned nil")
	}

	t.Run("save without auth returns 401", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id":   "player123",
			"game":      "testgame",
			"save_data": map[string]interface{}{"level": 1},
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/save", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("unauthenticated request status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("save with valid auth succeeds", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id":   "player123",
			"game":      "testgame",
			"save_data": map[string]interface{}{"level": 1},
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/save", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-api-key")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("authenticated request status = %d, want %d", rec.Code, http.StatusCreated)
		}
	})

	t.Run("load without auth returns 401", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id": "player123",
			"game":    "testgame",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/load", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("unauthenticated request status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("load with valid auth succeeds", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id": "player123",
			"game":    "testgame",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/load", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-api-key")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("authenticated request status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("wrong api key returns 401", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id":   "player123",
			"game":      "testgame",
			"save_data": map[string]interface{}{"level": 1},
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/save", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer wrong-key")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("wrong key request status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})
}

func TestHandler_WriteJSONError(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	writeJSONError(rec, req, "test error message", http.StatusBadRequest)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("writeJSONError() status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["error"] != "test error message" {
		t.Errorf("error message = %q, want %q", resp["error"], "test error message")
	}
}

func TestParseMaxSaves(t *testing.T) {
	tests := []struct {
		name   string
		config string
		want   int
	}{
		{"empty string means all", "", -1},
		{"all means no limit", "all", -1},
		{"ALL is case insensitive", "ALL", -1},
		{"All is case insensitive", "All", -1},
		{"valid number 5", "5", 5},
		{"valid number 1", "1", 1},
		{"valid number 100", "100", 100},
		{"zero defaults to all", "0", -1},
		{"negative defaults to all", "-1", -1},
		{"invalid string defaults to all", "invalid", -1},
		{"float defaults to all", "5.5", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMaxSaves(tt.config)
			if got != tt.want {
				t.Errorf("parseMaxSaves(%q) = %d, want %d", tt.config, got, tt.want)
			}
		})
	}
}

func TestHandler_CleanupOldStates(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	// Create handler with limit of 3 saves
	h := NewHandler(db, logger, "3")

	game := "cleanup_test_game"
	userID := "cleanup_user"
	coll := db.Collection(CollectionName)

	// Insert 5 saves with distinct timestamps
	ctx, cancel := testutil.TestContext()
	defer cancel()

	baseTime := time.Now().UTC()
	for i := 0; i < 5; i++ {
		save := bson.M{
			"user_id":   userID,
			"game":      game,
			"timestamp": baseTime.Add(time.Duration(i) * time.Second),
			"save_data": bson.M{"index": i},
		}
		_, err := coll.InsertOne(ctx, save)
		if err != nil {
			t.Fatalf("failed to insert test save: %v", err)
		}
	}

	// Verify we have 5 saves before cleanup
	count, _ := coll.CountDocuments(ctx, bson.M{"user_id": userID, "game": game})
	if count != 5 {
		t.Fatalf("expected 5 saves before cleanup, got %d", count)
	}

	// Run cleanup synchronously for testing
	h.cleanupOldStates(userID, game)

	// Verify only 3 saves remain
	count, _ = coll.CountDocuments(ctx, bson.M{"user_id": userID, "game": game})
	if count != 3 {
		t.Errorf("expected 3 saves after cleanup, got %d", count)
	}

	// Verify the 3 most recent saves are kept (indexes 2, 3, 4)
	cursor, _ := coll.Find(ctx, bson.M{"user_id": userID, "game": game})
	var saves []bson.M
	cursor.All(ctx, &saves)

	for _, save := range saves {
		idx := save["save_data"].(bson.M)["index"].(int32)
		if idx < 2 {
			t.Errorf("old save with index %d should have been deleted", idx)
		}
	}
}

func TestHandler_NoCleanupWhenAllConfigured(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	// Create handler with "all" (no limit)
	h := NewHandler(db, logger, "all")

	game := "no_cleanup_test_game"
	userID := "no_cleanup_user"
	coll := db.Collection(CollectionName)

	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Insert 10 saves
	baseTime := time.Now().UTC()
	for i := 0; i < 10; i++ {
		save := bson.M{
			"user_id":   userID,
			"game":      game,
			"timestamp": baseTime.Add(time.Duration(i) * time.Second),
			"save_data": bson.M{"index": i},
		}
		coll.InsertOne(ctx, save)
	}

	// Verify maxSavesPerUser is -1 (no limit)
	if h.maxSavesPerUser != -1 {
		t.Errorf("maxSavesPerUser = %d, want -1", h.maxSavesPerUser)
	}

	// Cleanup should be a no-op (never called since limit is -1)
	// But if called directly, it should do nothing
	h.cleanupOldStates(userID, game)

	// All 10 saves should still exist
	count, _ := coll.CountDocuments(ctx, bson.M{"user_id": userID, "game": game})
	if count != 10 {
		t.Errorf("expected 10 saves to remain when 'all' configured, got %d", count)
	}
}

func TestHandler_CleanupIsolatesUsers(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	// Create handler with limit of 2 saves
	h := NewHandler(db, logger, "2")

	game := "isolation_user_test"
	userA := "user_a"
	userB := "user_b"
	coll := db.Collection(CollectionName)

	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Insert 3 saves for each user
	baseTime := time.Now().UTC()
	for i := 0; i < 3; i++ {
		saveA := bson.M{
			"user_id":   userA,
			"game":      game,
			"timestamp": baseTime.Add(time.Duration(i) * time.Second),
			"save_data": bson.M{"user": "A", "index": i},
		}
		coll.InsertOne(ctx, saveA)

		saveB := bson.M{
			"user_id":   userB,
			"game":      game,
			"timestamp": baseTime.Add(time.Duration(i) * time.Second),
			"save_data": bson.M{"user": "B", "index": i},
		}
		coll.InsertOne(ctx, saveB)
	}

	// Cleanup only user A's saves
	h.cleanupOldStates(userA, game)

	// User A should have 2 saves
	countA, _ := coll.CountDocuments(ctx, bson.M{"user_id": userA, "game": game})
	if countA != 2 {
		t.Errorf("user A: expected 2 saves, got %d", countA)
	}

	// User B should still have 3 saves
	countB, _ := coll.CountDocuments(ctx, bson.M{"user_id": userB, "game": game})
	if countB != 3 {
		t.Errorf("user B: expected 3 saves (unchanged), got %d", countB)
	}
}

func TestHandler_CleanupIsolatesGames(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	// Create handler with limit of 2 saves
	h := NewHandler(db, logger, "2")

	gameA := "isolation_game_a"
	gameB := "isolation_game_b"
	userID := "game_isolation_user"
	coll := db.Collection(CollectionName)

	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Insert 3 saves for each game
	baseTime := time.Now().UTC()
	for i := 0; i < 3; i++ {
		saveA := bson.M{
			"user_id":   userID,
			"game":      gameA,
			"timestamp": baseTime.Add(time.Duration(i) * time.Second),
			"save_data": bson.M{"game": "A", "index": i},
		}
		coll.InsertOne(ctx, saveA)

		saveB := bson.M{
			"user_id":   userID,
			"game":      gameB,
			"timestamp": baseTime.Add(time.Duration(i) * time.Second),
			"save_data": bson.M{"game": "B", "index": i},
		}
		coll.InsertOne(ctx, saveB)
	}

	// Cleanup only game A's saves
	h.cleanupOldStates(userID, gameA)

	// Game A should have 2 saves
	countA, _ := coll.CountDocuments(ctx, bson.M{"user_id": userID, "game": gameA})
	if countA != 2 {
		t.Errorf("game A: expected 2 saves, got %d", countA)
	}

	// Game B should still have 3 saves
	countB, _ := coll.CountDocuments(ctx, bson.M{"user_id": userID, "game": gameB})
	if countB != 3 {
		t.Errorf("game B: expected 3 saves (unchanged), got %d", countB)
	}
}
