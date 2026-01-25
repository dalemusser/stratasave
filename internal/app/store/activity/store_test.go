package activity

import (
	"testing"
	"time"

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

	event := Event{
		UserID:    primitive.NewObjectID(),
		SessionID: primitive.NewObjectID(),
		EventType: EventPageView,
		PagePath:  "/dashboard",
	}

	err := store.Create(ctx, event)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestStore_Create_AutoGeneratesIDAndTimestamp(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	sessionID := primitive.NewObjectID()

	event := Event{
		UserID:    userID,
		SessionID: sessionID,
		EventType: EventPageView,
		PagePath:  "/test",
	}

	err := store.Create(ctx, event)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Retrieve and verify
	events, _ := store.GetBySession(ctx, sessionID)
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	created := events[0]
	if created.ID.IsZero() {
		t.Error("ID should be auto-generated")
	}
	if created.Timestamp.IsZero() {
		t.Error("Timestamp should be auto-generated")
	}
}

func TestStore_RecordPageView(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	sessionID := primitive.NewObjectID()
	pagePath := "/profile"

	err := store.RecordPageView(ctx, userID, sessionID, pagePath)
	if err != nil {
		t.Fatalf("RecordPageView() error = %v", err)
	}

	// Verify
	events, _ := store.GetBySession(ctx, sessionID)
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.UserID != userID {
		t.Errorf("UserID = %v, want %v", event.UserID, userID)
	}
	if event.SessionID != sessionID {
		t.Errorf("SessionID = %v, want %v", event.SessionID, sessionID)
	}
	if event.EventType != EventPageView {
		t.Errorf("EventType = %v, want %v", event.EventType, EventPageView)
	}
	if event.PagePath != pagePath {
		t.Errorf("PagePath = %v, want %v", event.PagePath, pagePath)
	}
}

func TestStore_GetBySession(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	sessionID := primitive.NewObjectID()
	otherSessionID := primitive.NewObjectID()

	// Create events for our session
	store.RecordPageView(ctx, userID, sessionID, "/page1")
	store.RecordPageView(ctx, userID, sessionID, "/page2")
	store.RecordPageView(ctx, userID, sessionID, "/page3")

	// Create event for other session
	store.RecordPageView(ctx, userID, otherSessionID, "/other")

	events, err := store.GetBySession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetBySession() error = %v", err)
	}
	if len(events) != 3 {
		t.Errorf("GetBySession() count = %d, want 3", len(events))
	}

	// Should be sorted by timestamp ascending
	for i := 1; i < len(events); i++ {
		if events[i].Timestamp.Before(events[i-1].Timestamp) {
			t.Error("Events should be sorted by timestamp ascending")
		}
	}
}

func TestStore_GetByUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	otherUserID := primitive.NewObjectID()
	sessionID := primitive.NewObjectID()

	// Create events for our user
	for i := 0; i < 5; i++ {
		store.RecordPageView(ctx, userID, sessionID, "/page"+string(rune('0'+i)))
	}

	// Create event for other user
	store.RecordPageView(ctx, otherUserID, sessionID, "/other")

	// Get with limit
	events, err := store.GetByUser(ctx, userID, 3)
	if err != nil {
		t.Fatalf("GetByUser() error = %v", err)
	}
	if len(events) != 3 {
		t.Errorf("GetByUser(limit=3) count = %d, want 3", len(events))
	}

	// Should be sorted by timestamp descending (most recent first)
	for i := 1; i < len(events); i++ {
		if events[i].Timestamp.After(events[i-1].Timestamp) {
			t.Error("Events should be sorted by timestamp descending")
		}
	}
}

func TestStore_GetByUserInTimeRange(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	sessionID := primitive.NewObjectID()

	// Create events at different times
	now := time.Now().UTC()
	past := now.Add(-2 * time.Hour)
	recent := now.Add(-30 * time.Minute)
	future := now.Add(1 * time.Hour)

	// Insert events with specific timestamps
	store.Create(ctx, Event{
		UserID:    userID,
		SessionID: sessionID,
		EventType: EventPageView,
		PagePath:  "/past",
		Timestamp: past,
	})
	store.Create(ctx, Event{
		UserID:    userID,
		SessionID: sessionID,
		EventType: EventPageView,
		PagePath:  "/recent",
		Timestamp: recent,
	})
	store.Create(ctx, Event{
		UserID:    userID,
		SessionID: sessionID,
		EventType: EventPageView,
		PagePath:  "/now",
		Timestamp: now,
	})

	// Query for last hour only
	events, err := store.GetByUserInTimeRange(ctx, userID, now.Add(-1*time.Hour), future)
	if err != nil {
		t.Fatalf("GetByUserInTimeRange() error = %v", err)
	}
	if len(events) != 2 { // recent and now
		t.Errorf("GetByUserInTimeRange() count = %d, want 2", len(events))
	}

	// All events
	events, _ = store.GetByUserInTimeRange(ctx, userID, past.Add(-time.Minute), future)
	if len(events) != 3 {
		t.Errorf("GetByUserInTimeRange(all) count = %d, want 3", len(events))
	}
}

func TestStore_CountByUserInTimeRange(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	sessionID := primitive.NewObjectID()

	now := time.Now().UTC()
	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)

	// Create page view events
	for i := 0; i < 5; i++ {
		store.RecordPageView(ctx, userID, sessionID, "/page"+string(rune('0'+i)))
	}

	count, err := store.CountByUserInTimeRange(ctx, userID, EventPageView, start, end)
	if err != nil {
		t.Fatalf("CountByUserInTimeRange() error = %v", err)
	}
	if count != 5 {
		t.Errorf("CountByUserInTimeRange() = %d, want 5", count)
	}

	// Count for different event type (should be 0)
	count, _ = store.CountByUserInTimeRange(ctx, userID, "other_event", start, end)
	if count != 0 {
		t.Errorf("CountByUserInTimeRange(other_event) = %d, want 0", count)
	}
}

func TestStore_GetBySession_Empty(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	events, err := store.GetBySession(ctx, primitive.NewObjectID())
	if err != nil {
		t.Fatalf("GetBySession() error = %v", err)
	}
	// Note: MongoDB cursor.All returns nil for empty results
	if len(events) != 0 {
		t.Errorf("GetBySession() for nonexistent session should return empty, got %d", len(events))
	}
}

func TestEventConstants(t *testing.T) {
	if EventPageView != "page_view" {
		t.Errorf("EventPageView = %q, want 'page_view'", EventPageView)
	}
}
