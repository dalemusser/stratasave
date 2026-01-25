package loginstore

import (
	"net/http"
	"testing"
	"time"

	"github.com/dalemusser/stratasave/internal/domain/models"
	"github.com/dalemusser/stratasave/internal/testutil"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestNew(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	if store == nil {
		t.Fatal("New() returned nil")
	}
}

func TestStore_EnsureIndexes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	if err := store.EnsureIndexes(ctx); err != nil {
		t.Fatalf("EnsureIndexes() error = %v", err)
	}

	// Should be idempotent
	if err := store.EnsureIndexes(ctx); err != nil {
		t.Fatalf("EnsureIndexes() second call error = %v", err)
	}
}

func TestStore_Create(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	rec := models.LoginRecord{
		UserID:   userID.Hex(),
		IP:       "192.168.1.1",
		Provider: "password",
	}

	err := store.Create(ctx, rec)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify
	records, _ := store.GetByUser(ctx, userID, 10)
	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	created := records[0]
	if created.UserID != userID.Hex() {
		t.Errorf("UserID = %v, want %v", created.UserID, userID.Hex())
	}
	if created.IP != rec.IP {
		t.Errorf("IP = %v, want %v", created.IP, rec.IP)
	}
	if created.Provider != rec.Provider {
		t.Errorf("Provider = %v, want %v", created.Provider, rec.Provider)
	}
	if created.CreatedAt.IsZero() {
		t.Error("CreatedAt should be auto-set when zero")
	}
}

func TestStore_Create_WithTimestamp(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	specificTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	rec := models.LoginRecord{
		UserID:    userID.Hex(),
		CreatedAt: specificTime,
		IP:        "10.0.0.1",
		Provider:  "google",
	}

	err := store.Create(ctx, rec)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify the specific timestamp was preserved
	records, _ := store.GetByUser(ctx, userID, 10)
	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	if !records[0].CreatedAt.Equal(specificTime) {
		t.Errorf("CreatedAt = %v, want %v", records[0].CreatedAt, specificTime)
	}
}

func TestStore_CreateFrom(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()

	// Create a mock request
	req, _ := http.NewRequest("POST", "/login", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	err := store.CreateFrom(ctx, req, userID, "password")
	if err != nil {
		t.Fatalf("CreateFrom() error = %v", err)
	}

	// Verify
	records, _ := store.GetByUser(ctx, userID, 10)
	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.UserID != userID.Hex() {
		t.Errorf("UserID = %v, want %v", rec.UserID, userID.Hex())
	}
	if rec.IP != "192.168.1.100" {
		t.Errorf("IP = %v, want '192.168.1.100'", rec.IP)
	}
	if rec.Provider != "password" {
		t.Errorf("Provider = %v, want 'password'", rec.Provider)
	}
}

func TestStore_CreateFrom_XForwardedFor(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()

	req, _ := http.NewRequest("POST", "/login", nil)
	req.Header.Set("X-Forwarded-For", "10.20.30.40, 192.168.1.1")
	req.RemoteAddr = "127.0.0.1:8080"

	err := store.CreateFrom(ctx, req, userID, "google")
	if err != nil {
		t.Fatalf("CreateFrom() error = %v", err)
	}

	records, _ := store.GetByUser(ctx, userID, 10)
	if records[0].IP != "10.20.30.40" {
		t.Errorf("IP = %v, want '10.20.30.40' from X-Forwarded-For", records[0].IP)
	}
}

func TestStore_CreateFrom_XRealIP(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()

	req, _ := http.NewRequest("POST", "/login", nil)
	req.Header.Set("X-Real-IP", "172.16.0.1")
	req.RemoteAddr = "127.0.0.1:8080"

	err := store.CreateFrom(ctx, req, userID, "password")
	if err != nil {
		t.Fatalf("CreateFrom() error = %v", err)
	}

	records, _ := store.GetByUser(ctx, userID, 10)
	if records[0].IP != "172.16.0.1" {
		t.Errorf("IP = %v, want '172.16.0.1' from X-Real-IP", records[0].IP)
	}
}

func TestStore_GetByUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	otherUserID := primitive.NewObjectID()

	// Create records for our user
	for i := 0; i < 5; i++ {
		store.Create(ctx, models.LoginRecord{
			UserID:   userID.Hex(),
			IP:       "192.168.1." + string(rune('0'+i)),
			Provider: "password",
		})
	}

	// Create record for other user
	store.Create(ctx, models.LoginRecord{
		UserID:   otherUserID.Hex(),
		IP:       "10.0.0.1",
		Provider: "password",
	})

	// Get all for user
	records, err := store.GetByUser(ctx, userID, 10)
	if err != nil {
		t.Fatalf("GetByUser() error = %v", err)
	}
	if len(records) != 5 {
		t.Errorf("GetByUser() count = %d, want 5", len(records))
	}

	// Should be sorted by created_at descending (most recent first)
	for i := 1; i < len(records); i++ {
		if records[i].CreatedAt.After(records[i-1].CreatedAt) {
			t.Error("Records should be sorted by created_at descending")
		}
	}

	// Test limit
	records, _ = store.GetByUser(ctx, userID, 3)
	if len(records) != 3 {
		t.Errorf("GetByUser(limit=3) count = %d, want 3", len(records))
	}
}

func TestStore_GetByTimeRange(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	now := time.Now().UTC()
	userID := primitive.NewObjectID()

	// Create records at different times
	times := []time.Time{
		now.Add(-3 * time.Hour),
		now.Add(-2 * time.Hour),
		now.Add(-1 * time.Hour),
		now,
	}

	for _, t := range times {
		store.Create(ctx, models.LoginRecord{
			UserID:    userID.Hex(),
			CreatedAt: t,
			IP:        "192.168.1.1",
			Provider:  "password",
		})
	}

	// Query for last 2 hours
	start := now.Add(-2*time.Hour - 30*time.Minute)
	end := now.Add(1 * time.Minute)

	records, err := store.GetByTimeRange(ctx, start, end)
	if err != nil {
		t.Fatalf("GetByTimeRange() error = %v", err)
	}
	if len(records) != 3 {
		t.Errorf("GetByTimeRange() count = %d, want 3", len(records))
	}

	// Should be sorted by created_at descending
	for i := 1; i < len(records); i++ {
		if records[i].CreatedAt.After(records[i-1].CreatedAt) {
			t.Error("Records should be sorted by created_at descending")
		}
	}
}

func TestStore_GetByUser_Empty(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	records, err := store.GetByUser(ctx, primitive.NewObjectID(), 10)
	if err != nil {
		t.Fatalf("GetByUser() error = %v", err)
	}
	// Note: MongoDB cursor.All returns nil for empty results
	if len(records) != 0 {
		t.Errorf("GetByUser() for nonexistent user should return empty, got %d", len(records))
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		xff        string
		xRealIP    string
		remoteAddr string
		want       string
	}{
		{
			name:       "X-Forwarded-For single",
			xff:        "192.168.1.1",
			remoteAddr: "127.0.0.1:8080",
			want:       "192.168.1.1",
		},
		{
			name:       "X-Forwarded-For multiple",
			xff:        "10.0.0.1, 192.168.1.1, 172.16.0.1",
			remoteAddr: "127.0.0.1:8080",
			want:       "10.0.0.1",
		},
		{
			name:       "X-Real-IP",
			xRealIP:    "10.20.30.40",
			remoteAddr: "127.0.0.1:8080",
			want:       "10.20.30.40",
		},
		{
			name:       "RemoteAddr with port",
			remoteAddr: "192.168.1.50:12345",
			want:       "192.168.1.50",
		},
		{
			name:       "RemoteAddr without port",
			remoteAddr: "192.168.1.50",
			want:       "192.168.1.50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/", nil)
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}
			req.RemoteAddr = tt.remoteAddr

			got := clientIP(req)
			if got != tt.want {
				t.Errorf("clientIP() = %v, want %v", got, tt.want)
			}
		})
	}
}
