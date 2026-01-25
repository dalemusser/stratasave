package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dalemusser/stratasave/internal/testutil"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

func TestHandler_Check(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	h := NewHandler(db.Client(), logger)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.Check(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Check() status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("response status = %q, want %q", resp.Status, "ok")
	}
	if resp.Services["mongodb"] != "ok" {
		t.Errorf("mongodb status = %q, want %q", resp.Services["mongodb"], "ok")
	}
}

func TestHandler_Ready(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	h := NewHandler(db.Client(), logger)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	h.Ready(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Ready() status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if body != `{"status":"ready"}` {
		t.Errorf("Ready() body = %q, want %q", body, `{"status":"ready"}`)
	}
}

func TestHandler_Live(t *testing.T) {
	logger := zap.NewNop()

	// Live doesn't need DB - just check the handler works
	h := NewHandler(nil, logger)

	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rec := httptest.NewRecorder()

	h.Live(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Live() status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if body != `{"status":"alive"}` {
		t.Errorf("Live() body = %q, want %q", body, `{"status":"alive"}`)
	}
}

func TestRoutes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	h := NewHandler(db.Client(), logger)
	router := Routes(h)

	if router == nil {
		t.Fatal("Routes() returned nil")
	}
}

func TestMountRootEndpoints(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	h := NewHandler(db.Client(), logger)
	r := chi.NewRouter()
	MountRootEndpoints(r, h)

	// Test /ready
	t.Run("/ready", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("/ready status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	// Test /readyz (alias)
	t.Run("/readyz", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("/readyz status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	// Test /livez
	t.Run("/livez", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/livez", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("/livez status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}

func TestResponse_JSON(t *testing.T) {
	resp := Response{
		Status: "ok",
		Services: map[string]string{
			"mongodb": "ok",
			"cache":   "degraded",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var decoded Response
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if decoded.Status != "ok" {
		t.Errorf("decoded status = %q, want %q", decoded.Status, "ok")
	}
	if decoded.Services["mongodb"] != "ok" {
		t.Errorf("mongodb status = %q, want %q", decoded.Services["mongodb"], "ok")
	}
	if decoded.Services["cache"] != "degraded" {
		t.Errorf("cache status = %q, want %q", decoded.Services["cache"], "degraded")
	}
}
