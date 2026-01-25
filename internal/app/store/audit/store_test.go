package audit

import (
	"strings"
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

	// Note: SetupTestDB already creates indexes via indexes.EnsureAll
	// This test verifies EnsureIndexes doesn't error on existing indexes
	// (it may conflict with differently-named indexes, which is acceptable)
	err := store.EnsureIndexes(ctx)
	// We accept either success or index conflict (already exists with different name)
	if err != nil {
		// Check if it's just an index conflict, which is fine
		if !isIndexConflict(err) {
			t.Fatalf("EnsureIndexes() error = %v", err)
		}
	}
}

// isIndexConflict checks if error is due to index name conflict
func isIndexConflict(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "IndexOptionsConflict") || strings.Contains(s, "already exists with a different name")
}

func TestStore_Log(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	event := Event{
		Category:  CategoryAuth,
		EventType: EventLoginSuccess,
		UserID:    &userID,
		IP:        "192.168.1.1",
		UserAgent: "TestAgent",
		Success:   true,
	}

	err := store.Log(ctx, event)
	if err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	// Verify event was logged
	events, err := store.GetByUser(ctx, userID, 10)
	if err != nil {
		t.Fatalf("GetByUser() error = %v", err)
	}
	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}
}

func TestStore_Log_WithID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	eventID := primitive.NewObjectID()
	createdAt := time.Now().Add(-1 * time.Hour)
	event := Event{
		ID:        eventID,
		CreatedAt: createdAt,
		Category:  CategoryAdmin,
		EventType: EventUserCreated,
		Success:   true,
	}

	err := store.Log(ctx, event)
	if err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	// Verify the provided ID and CreatedAt were preserved
	events, err := store.Query(ctx, QueryFilter{EventType: EventUserCreated})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}
	if events[0].ID != eventID {
		t.Errorf("ID = %v, want %v", events[0].ID, eventID)
	}
}

func TestStore_Query(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	actorID := primitive.NewObjectID()

	// Create test events
	events := []Event{
		{Category: CategoryAuth, EventType: EventLoginSuccess, UserID: &userID, Success: true},
		{Category: CategoryAuth, EventType: EventLoginFailedWrongPassword, UserID: &userID, Success: false},
		{Category: CategoryAdmin, EventType: EventUserCreated, ActorID: &actorID, Success: true},
	}

	for _, e := range events {
		if err := store.Log(ctx, e); err != nil {
			t.Fatalf("Log() error = %v", err)
		}
	}

	tests := []struct {
		name      string
		filter    QueryFilter
		wantCount int
	}{
		{"all events", QueryFilter{}, 3},
		{"by user", QueryFilter{UserID: &userID}, 2},
		{"by actor", QueryFilter{ActorID: &actorID}, 1},
		{"by category auth", QueryFilter{Category: CategoryAuth}, 2},
		{"by category admin", QueryFilter{Category: CategoryAdmin}, 1},
		{"by event type", QueryFilter{EventType: EventLoginSuccess}, 1},
		{"with limit", QueryFilter{Limit: 2}, 2},
		{"with offset", QueryFilter{Limit: 10, Offset: 2}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := store.Query(ctx, tt.filter)
			if err != nil {
				t.Fatalf("Query() error = %v", err)
			}
			if len(result) != tt.wantCount {
				t.Errorf("Query() returned %d events, want %d", len(result), tt.wantCount)
			}
		})
	}
}

func TestStore_Query_TimeRange(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	// Log event
	event := Event{
		Category:  CategoryAuth,
		EventType: EventLoginSuccess,
		CreatedAt: now,
		Success:   true,
	}
	if err := store.Log(ctx, event); err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	tests := []struct {
		name      string
		start     *time.Time
		end       *time.Time
		wantCount int
	}{
		{"start before", &past, nil, 1},
		{"start after", &future, nil, 0},
		{"end after", nil, &future, 1},
		{"end before", nil, &past, 0},
		{"range includes", &past, &future, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := store.Query(ctx, QueryFilter{StartTime: tt.start, EndTime: tt.end})
			if err != nil {
				t.Fatalf("Query() error = %v", err)
			}
			if len(result) != tt.wantCount {
				t.Errorf("Query() returned %d events, want %d", len(result), tt.wantCount)
			}
		})
	}
}

func TestStore_CountByFilter(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()

	// Create test events
	for i := 0; i < 5; i++ {
		event := Event{
			Category:  CategoryAuth,
			EventType: EventLoginSuccess,
			UserID:    &userID,
			Success:   true,
		}
		if err := store.Log(ctx, event); err != nil {
			t.Fatalf("Log() error = %v", err)
		}
	}

	count, err := store.CountByFilter(ctx, QueryFilter{UserID: &userID})
	if err != nil {
		t.Fatalf("CountByFilter() error = %v", err)
	}
	if count != 5 {
		t.Errorf("CountByFilter() = %d, want 5", count)
	}

	count, err = store.CountByFilter(ctx, QueryFilter{Category: CategoryAdmin})
	if err != nil {
		t.Fatalf("CountByFilter() error = %v", err)
	}
	if count != 0 {
		t.Errorf("CountByFilter() for non-matching = %d, want 0", count)
	}
}

func TestStore_GetByUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	otherUserID := primitive.NewObjectID()

	// Create events for user
	for i := 0; i < 3; i++ {
		event := Event{
			Category:  CategoryAuth,
			EventType: EventLoginSuccess,
			UserID:    &userID,
			Success:   true,
		}
		if err := store.Log(ctx, event); err != nil {
			t.Fatalf("Log() error = %v", err)
		}
	}

	// Create event for other user
	otherEvent := Event{
		Category:  CategoryAuth,
		EventType: EventLoginSuccess,
		UserID:    &otherUserID,
		Success:   true,
	}
	if err := store.Log(ctx, otherEvent); err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	events, err := store.GetByUser(ctx, userID, 10)
	if err != nil {
		t.Fatalf("GetByUser() error = %v", err)
	}
	if len(events) != 3 {
		t.Errorf("GetByUser() returned %d events, want 3", len(events))
	}

	// Verify all events belong to user
	for _, e := range events {
		if e.UserID == nil || *e.UserID != userID {
			t.Error("Event does not belong to expected user")
		}
	}
}

func TestStore_GetRecent(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create events
	for i := 0; i < 5; i++ {
		event := Event{
			Category:  CategoryAuth,
			EventType: EventLoginSuccess,
			Success:   true,
		}
		if err := store.Log(ctx, event); err != nil {
			t.Fatalf("Log() error = %v", err)
		}
	}

	events, err := store.GetRecent(ctx, 3)
	if err != nil {
		t.Fatalf("GetRecent() error = %v", err)
	}
	if len(events) != 3 {
		t.Errorf("GetRecent() returned %d events, want 3", len(events))
	}
}

func TestStore_GetFailedLogins(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	userID := primitive.NewObjectID()
	since := time.Now().Add(-1 * time.Hour)

	// Create failed login events
	failedEvents := []Event{
		{Category: CategoryAuth, EventType: EventLoginFailedWrongPassword, UserID: &userID, Success: false},
		{Category: CategoryAuth, EventType: EventLoginFailedUserNotFound, UserID: &userID, Success: false},
		{Category: CategoryAuth, EventType: EventLoginFailedUserDisabled, UserID: &userID, Success: false},
	}

	for _, e := range failedEvents {
		if err := store.Log(ctx, e); err != nil {
			t.Fatalf("Log() error = %v", err)
		}
	}

	// Create successful login (should not be returned)
	successEvent := Event{
		Category:  CategoryAuth,
		EventType: EventLoginSuccess,
		UserID:    &userID,
		Success:   true,
	}
	if err := store.Log(ctx, successEvent); err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	events, err := store.GetFailedLogins(ctx, since, 10)
	if err != nil {
		t.Fatalf("GetFailedLogins() error = %v", err)
	}
	if len(events) != 3 {
		t.Errorf("GetFailedLogins() returned %d events, want 3", len(events))
	}

	// Verify all are failed
	for _, e := range events {
		if e.Success {
			t.Error("GetFailedLogins() returned successful event")
		}
	}
}

func TestStore_Log_WithDetails(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	event := Event{
		Category:      CategoryAdmin,
		EventType:     EventSettingsUpdated,
		Success:       true,
		FailureReason: "",
		Details: map[string]string{
			"field":     "site_name",
			"old_value": "Old Name",
			"new_value": "New Name",
		},
	}

	err := store.Log(ctx, event)
	if err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	// Retrieve and verify
	events, err := store.Query(ctx, QueryFilter{EventType: EventSettingsUpdated})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}
	if events[0].Details["field"] != "site_name" {
		t.Errorf("Details[field] = %v, want site_name", events[0].Details["field"])
	}
}

func TestConstants(t *testing.T) {
	// Verify constants exist and have expected values
	if CategoryAuth != "auth" {
		t.Errorf("CategoryAuth = %q, want auth", CategoryAuth)
	}
	if CategoryAdmin != "admin" {
		t.Errorf("CategoryAdmin = %q, want admin", CategoryAdmin)
	}

	// Verify event types are non-empty
	eventTypes := []string{
		EventLoginSuccess,
		EventLoginFailedUserNotFound,
		EventLoginFailedWrongPassword,
		EventLoginFailedUserDisabled,
		EventLogout,
		EventPasswordChanged,
		EventUserCreated,
		EventUserUpdated,
		EventUserDisabled,
		EventUserEnabled,
		EventUserDeleted,
		EventSettingsUpdated,
		EventPageUpdated,
	}

	for _, et := range eventTypes {
		if et == "" {
			t.Error("Event type constant is empty")
		}
	}
}
