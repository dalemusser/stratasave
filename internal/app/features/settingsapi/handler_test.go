package settingsapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dalemusser/stratasave/internal/testutil"
	"go.uber.org/zap"
)

func TestHandler_SaveHandler(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()
	h := NewHandler(db, logger)

	t.Run("successful save", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id": "player123",
			"game":    "testgame",
			"settings_data": map[string]interface{}{
				"audio":    0.8,
				"graphics": "high",
			},
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/settings/save", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.SaveHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("SaveHandler() status = %d, want %d. Body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var resp PlayerSettings
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
		if resp.SettingsData == nil {
			t.Error("response settings_data should not be nil")
		}
	})

	t.Run("upsert updates existing", func(t *testing.T) {
		// First save
		body1 := map[string]interface{}{
			"user_id":       "upsert_user",
			"game":          "upsert_game",
			"settings_data": map[string]interface{}{"audio": 0.5},
		}
		bodyBytes1, _ := json.Marshal(body1)
		req1 := httptest.NewRequest(http.MethodPost, "/settings/save", bytes.NewReader(bodyBytes1))
		req1.Header.Set("Content-Type", "application/json")
		rec1 := httptest.NewRecorder()
		h.SaveHandler(rec1, req1)

		var resp1 PlayerSettings
		json.NewDecoder(rec1.Body).Decode(&resp1)
		firstID := resp1.ID

		// Second save (should update, not create new)
		body2 := map[string]interface{}{
			"user_id":       "upsert_user",
			"game":          "upsert_game",
			"settings_data": map[string]interface{}{"audio": 0.9},
		}
		bodyBytes2, _ := json.Marshal(body2)
		req2 := httptest.NewRequest(http.MethodPost, "/settings/save", bytes.NewReader(bodyBytes2))
		req2.Header.Set("Content-Type", "application/json")
		rec2 := httptest.NewRecorder()
		h.SaveHandler(rec2, req2)

		var resp2 PlayerSettings
		json.NewDecoder(rec2.Body).Decode(&resp2)

		// Should have same ID (upsert, not new document)
		if resp2.ID != firstID {
			t.Errorf("upsert created new document: ID %s != %s", resp2.ID.Hex(), firstID.Hex())
		}
		// Should have updated value
		if resp2.SettingsData["audio"] != 0.9 {
			t.Errorf("settings not updated: audio = %v, want 0.9", resp2.SettingsData["audio"])
		}
	})

	t.Run("missing user_id", func(t *testing.T) {
		body := map[string]interface{}{
			"game":          "testgame",
			"settings_data": map[string]interface{}{"audio": 0.5},
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/settings/save", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.SaveHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("SaveHandler() status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing settings_data", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id": "player123",
			"game":    "testgame",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/settings/save", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.SaveHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("SaveHandler() status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/settings/save", bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.SaveHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("SaveHandler() status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}

func TestHandler_LoadHandler(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()
	h := NewHandler(db, logger)

	t.Run("load existing settings", func(t *testing.T) {
		// First save some settings
		saveBody := map[string]interface{}{
			"user_id":       "load_user",
			"game":          "load_game",
			"settings_data": map[string]interface{}{"volume": 0.7},
		}
		saveBytes, _ := json.Marshal(saveBody)
		saveReq := httptest.NewRequest(http.MethodPost, "/settings/save", bytes.NewReader(saveBytes))
		saveReq.Header.Set("Content-Type", "application/json")
		saveRec := httptest.NewRecorder()
		h.SaveHandler(saveRec, saveReq)

		// Now load them
		loadBody := map[string]interface{}{
			"user_id": "load_user",
			"game":    "load_game",
		}
		loadBytes, _ := json.Marshal(loadBody)
		loadReq := httptest.NewRequest(http.MethodPost, "/settings/load", bytes.NewReader(loadBytes))
		loadReq.Header.Set("Content-Type", "application/json")
		loadRec := httptest.NewRecorder()

		h.LoadHandler(loadRec, loadReq)

		if loadRec.Code != http.StatusOK {
			t.Errorf("LoadHandler() status = %d, want %d", loadRec.Code, http.StatusOK)
		}

		var resp PlayerSettings
		if err := json.NewDecoder(loadRec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.UserID != "load_user" {
			t.Errorf("user_id = %q, want %q", resp.UserID, "load_user")
		}
		if resp.SettingsData["volume"] != 0.7 {
			t.Errorf("volume = %v, want 0.7", resp.SettingsData["volume"])
		}
	})

	t.Run("load non-existent returns null", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id": "nonexistent_user",
			"game":    "nonexistent_game",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/settings/load", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.LoadHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("LoadHandler() status = %d, want %d", rec.Code, http.StatusOK)
		}

		respBody := rec.Body.String()
		if respBody != "null" {
			t.Errorf("response body = %q, want %q", respBody, "null")
		}
	})

	t.Run("missing user_id", func(t *testing.T) {
		body := map[string]interface{}{
			"game": "testgame",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/settings/load", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.LoadHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("LoadHandler() status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/settings/load", bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.LoadHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("LoadHandler() status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}

func TestRoutes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()
	h := NewHandler(db, logger)

	router := Routes(h, nil, "test-api-key", logger)
	if router == nil {
		t.Fatal("Routes() returned nil")
	}

	t.Run("save without auth returns 401", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id":       "player123",
			"game":          "testgame",
			"settings_data": map[string]interface{}{"volume": 0.5},
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
			"user_id":       "player123",
			"game":          "testgame",
			"settings_data": map[string]interface{}{"volume": 0.5},
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/save", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-api-key")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("authenticated request status = %d, want %d", rec.Code, http.StatusOK)
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
}
