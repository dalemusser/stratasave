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
			"user_id": "111111111111111111111111",
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

		if resp.UserID != "111111111111111111111111" {
			t.Errorf("response user_id = %q, want %q", resp.UserID, "111111111111111111111111")
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
			"user_id":       "222222222222222222222222",
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
			"user_id":       "222222222222222222222222",
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
			"user_id": "111111111111111111111111",
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
			"user_id":       "333333333333333333333333",
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
			"user_id": "333333333333333333333333",
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

		if resp.UserID != "333333333333333333333333" {
			t.Errorf("user_id = %q, want %q", resp.UserID, "333333333333333333333333")
		}
		if resp.SettingsData["volume"] != 0.7 {
			t.Errorf("volume = %v, want 0.7", resp.SettingsData["volume"])
		}
	})

	t.Run("load non-existent returns null", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id": "999999999999999999999999",
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

func TestHandler_DeleteHandler(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()
	h := NewHandler(db, logger)

	save := func(userID, game string, data map[string]interface{}) {
		body := map[string]interface{}{"user_id": userID, "game": game, "settings_data": data}
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/settings/save", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		h.SaveHandler(httptest.NewRecorder(), req)
	}

	doDelete := func(body map[string]interface{}) *httptest.ResponseRecorder {
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/settings/delete", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.DeleteHandler(rec, req)
		return rec
	}

	t.Run("deletes settings and isolates others", func(t *testing.T) {
		save("111111111111111111111111", "delgame", map[string]interface{}{"volume": 0.5})
		save("111111111111111111111111", "othergame", map[string]interface{}{"volume": 0.9})

		rec := doDelete(map[string]interface{}{
			"user_id": "111111111111111111111111",
			"game":    "delgame",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("DeleteHandler() status = %d, want %d. Body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
		var resp struct {
			Deleted int64 `json:"deleted"`
		}
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp.Deleted != 1 {
			t.Errorf("deleted = %d, want 1", resp.Deleted)
		}

		// Deleted game returns null on load.
		loadBody, _ := json.Marshal(map[string]interface{}{"user_id": "111111111111111111111111", "game": "delgame"})
		loadReq := httptest.NewRequest(http.MethodPost, "/settings/load", bytes.NewReader(loadBody))
		loadReq.Header.Set("Content-Type", "application/json")
		loadRec := httptest.NewRecorder()
		h.LoadHandler(loadRec, loadReq)
		if loadRec.Body.String() != "null" {
			t.Errorf("expected deleted settings to load as null, got %q", loadRec.Body.String())
		}

		// Other game is untouched.
		loadBody2, _ := json.Marshal(map[string]interface{}{"user_id": "111111111111111111111111", "game": "othergame"})
		loadReq2 := httptest.NewRequest(http.MethodPost, "/settings/load", bytes.NewReader(loadBody2))
		loadReq2.Header.Set("Content-Type", "application/json")
		loadRec2 := httptest.NewRecorder()
		h.LoadHandler(loadRec2, loadReq2)
		if loadRec2.Body.String() == "null" {
			t.Error("other game settings should not have been deleted")
		}
	})

	t.Run("delete with no matching data returns 200 deleted 0", func(t *testing.T) {
		rec := doDelete(map[string]interface{}{
			"user_id": "999999999999999999999999",
			"game":    "nope",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		var resp struct {
			Deleted int64 `json:"deleted"`
		}
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp.Deleted != 0 {
			t.Errorf("deleted = %d, want 0", resp.Deleted)
		}
	})

	t.Run("missing user_id", func(t *testing.T) {
		rec := doDelete(map[string]interface{}{"game": "testgame"})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing game", func(t *testing.T) {
		rec := doDelete(map[string]interface{}{"user_id": "111111111111111111111111"})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid user_id form", func(t *testing.T) {
		rec := doDelete(map[string]interface{}{"user_id": "not-a-hex", "game": "testgame"})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/settings/delete", bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.DeleteHandler(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}

func TestRoutes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()
	h := NewHandler(db, logger)

	router := Routes(h, nil, []string{"test-api-key"}, []string{"test-admin-key"}, logger)
	if router == nil {
		t.Fatal("Routes() returned nil")
	}

	t.Run("save without auth returns 401", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id":       "111111111111111111111111",
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
			"user_id":       "111111111111111111111111",
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
			"user_id": "111111111111111111111111",
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
			"user_id": "111111111111111111111111",
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

	t.Run("delete without auth returns 401", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id": "111111111111111111111111",
			"game":    "testgame",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/delete", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("unauthenticated request status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("delete with valid auth succeeds", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id": "111111111111111111111111",
			"game":    "testgame",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/delete", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-admin-key")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("authenticated request status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}

// Delete takes its own key: the key game clients hold must not be able to
// remove a student's settings (see saveapi.Routes).
func TestRoutesDeleteKey(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()
	h := NewHandler(db, logger)
	router := Routes(h, nil, []string{"client-key", "client-key-2"}, []string{"admin-key"}, logger)

	post := func(path, key string, body map[string]interface{}) int {
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}
	save := map[string]interface{}{
		"user_id":       "111111111111111111111111",
		"game":          "testgame",
		"settings_data": map[string]interface{}{"volume": 0.5},
	}
	who := map[string]interface{}{"user_id": "111111111111111111111111", "game": "testgame"}

	if got := post("/save", "client-key-2", save); got < 200 || got > 299 {
		t.Fatalf("save with the second client key: status = %d, want 2xx", got)
	}
	if got := post("/save", "admin-key", save); got != http.StatusUnauthorized {
		t.Errorf("save with the admin key: status = %d, want %d", got, http.StatusUnauthorized)
	}
	if got := post("/delete", "client-key", who); got != http.StatusUnauthorized {
		t.Errorf("delete with a client key: status = %d, want %d", got, http.StatusUnauthorized)
	}
	if got := post("/delete", "", who); got != http.StatusUnauthorized {
		t.Errorf("delete without a key: status = %d, want %d", got, http.StatusUnauthorized)
	}
	if got := post("/load", "client-key", who); got != http.StatusOK {
		t.Errorf("load after the refused deletes: status = %d, want %d (the settings are still there)", got, http.StatusOK)
	}
	if got := post("/delete", "admin-key", who); got != http.StatusOK {
		t.Errorf("delete with the admin key: status = %d, want %d", got, http.StatusOK)
	}
}
