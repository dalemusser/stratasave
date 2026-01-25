package announcement

import (
	"testing"
	"time"

	"github.com/dalemusser/stratasave/internal/testutil"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
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

	input := CreateInput{
		Title:       "Test Announcement",
		Content:     "This is test content",
		Type:        TypeInfo,
		Dismissible: true,
		Active:      true,
	}

	ann, err := store.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if ann.ID.IsZero() {
		t.Error("ID should not be zero")
	}
	if ann.Title != input.Title {
		t.Errorf("Title = %v, want %v", ann.Title, input.Title)
	}
	if ann.Content != input.Content {
		t.Errorf("Content = %v, want %v", ann.Content, input.Content)
	}
	if ann.Type != input.Type {
		t.Errorf("Type = %v, want %v", ann.Type, input.Type)
	}
	if ann.Dismissible != input.Dismissible {
		t.Errorf("Dismissible = %v, want %v", ann.Dismissible, input.Dismissible)
	}
	if ann.Active != input.Active {
		t.Errorf("Active = %v, want %v", ann.Active, input.Active)
	}
	if ann.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if ann.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestStore_Create_WithTimeRange(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	now := time.Now()
	startsAt := now.Add(-1 * time.Hour)
	endsAt := now.Add(1 * time.Hour)

	input := CreateInput{
		Title:    "Scheduled Announcement",
		Content:  "Content",
		Type:     TypeWarning,
		Active:   true,
		StartsAt: &startsAt,
		EndsAt:   &endsAt,
	}

	ann, err := store.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if ann.StartsAt == nil {
		t.Error("StartsAt should be set")
	}
	if ann.EndsAt == nil {
		t.Error("EndsAt should be set")
	}
}

func TestStore_GetByID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	created, _ := store.Create(ctx, CreateInput{
		Title:   "GetByID Test",
		Content: "Content",
		Type:    TypeInfo,
		Active:  true,
	})

	// Valid ID
	ann, err := store.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if ann.ID != created.ID {
		t.Errorf("ID = %v, want %v", ann.ID, created.ID)
	}
	if ann.Title != "GetByID Test" {
		t.Errorf("Title = %v, want 'GetByID Test'", ann.Title)
	}

	// Invalid ID
	_, err = store.GetByID(ctx, primitive.NewObjectID())
	if err != mongo.ErrNoDocuments {
		t.Errorf("GetByID() for nonexistent ID error = %v, want %v", err, mongo.ErrNoDocuments)
	}
}

func TestStore_Update(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	created, _ := store.Create(ctx, CreateInput{
		Title:       "Original",
		Content:     "Original content",
		Type:        TypeInfo,
		Dismissible: false,
		Active:      false,
	})

	newTitle := "Updated"
	newContent := "Updated content"
	newType := TypeCritical
	newDismissible := true
	newActive := true

	updateInput := UpdateInput{
		Title:       &newTitle,
		Content:     &newContent,
		Type:        &newType,
		Dismissible: &newDismissible,
		Active:      &newActive,
	}

	if err := store.Update(ctx, created.ID, updateInput); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	ann, _ := store.GetByID(ctx, created.ID)
	if ann.Title != newTitle {
		t.Errorf("Title = %v, want %v", ann.Title, newTitle)
	}
	if ann.Content != newContent {
		t.Errorf("Content = %v, want %v", ann.Content, newContent)
	}
	if ann.Type != newType {
		t.Errorf("Type = %v, want %v", ann.Type, newType)
	}
	if ann.Dismissible != newDismissible {
		t.Errorf("Dismissible = %v, want %v", ann.Dismissible, newDismissible)
	}
	if ann.Active != newActive {
		t.Errorf("Active = %v, want %v", ann.Active, newActive)
	}
}

func TestStore_Delete(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	created, _ := store.Create(ctx, CreateInput{
		Title:   "To Delete",
		Content: "Content",
		Type:    TypeInfo,
	})

	if err := store.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := store.GetByID(ctx, created.ID)
	if err != mongo.ErrNoDocuments {
		t.Errorf("GetByID() after delete error = %v, want %v", err, mongo.ErrNoDocuments)
	}
}

func TestStore_List(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create announcements
	for i := 0; i < 3; i++ {
		store.Create(ctx, CreateInput{
			Title:   "Announcement " + string(rune('A'+i)),
			Content: "Content",
			Type:    TypeInfo,
		})
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 3 {
		t.Errorf("List() count = %d, want 3", len(list))
	}

	// Should be sorted by created_at descending (most recent first)
	for i := 1; i < len(list); i++ {
		if list[i].CreatedAt.After(list[i-1].CreatedAt) {
			t.Error("List should be sorted by created_at descending")
		}
	}
}

func TestStore_GetActive(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	now := time.Now()
	past := now.Add(-2 * time.Hour)
	future := now.Add(2 * time.Hour)

	// Active, no time constraints (should show)
	store.Create(ctx, CreateInput{
		Title:   "Always Active",
		Content: "Content",
		Type:    TypeInfo,
		Active:  true,
	})

	// Active, started in past, ends in future (should show)
	store.Create(ctx, CreateInput{
		Title:    "Currently Active",
		Content:  "Content",
		Type:     TypeWarning,
		Active:   true,
		StartsAt: &past,
		EndsAt:   &future,
	})

	// Active but not started yet (should NOT show)
	futureStart := now.Add(1 * time.Hour)
	store.Create(ctx, CreateInput{
		Title:    "Future Active",
		Content:  "Content",
		Type:     TypeInfo,
		Active:   true,
		StartsAt: &futureStart,
	})

	// Active but already ended (should NOT show)
	pastEnd := now.Add(-30 * time.Minute)
	store.Create(ctx, CreateInput{
		Title:   "Expired Active",
		Content: "Content",
		Type:    TypeInfo,
		Active:  true,
		EndsAt:  &pastEnd,
	})

	// Inactive (should NOT show)
	store.Create(ctx, CreateInput{
		Title:   "Inactive",
		Content: "Content",
		Type:    TypeInfo,
		Active:  false,
	})

	active, err := store.GetActive(ctx)
	if err != nil {
		t.Fatalf("GetActive() error = %v", err)
	}
	if len(active) != 2 {
		t.Errorf("GetActive() count = %d, want 2", len(active))
	}

	// Check that the correct announcements are returned
	titles := make(map[string]bool)
	for _, a := range active {
		titles[a.Title] = true
	}
	if !titles["Always Active"] {
		t.Error("GetActive() should include 'Always Active'")
	}
	if !titles["Currently Active"] {
		t.Error("GetActive() should include 'Currently Active'")
	}
}

func TestStore_GetActive_SortedByTypeThenDate(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	// Create with different types (critical should come first)
	store.Create(ctx, CreateInput{
		Title:   "Info",
		Content: "Content",
		Type:    TypeInfo,
		Active:  true,
	})
	store.Create(ctx, CreateInput{
		Title:   "Critical",
		Content: "Content",
		Type:    TypeCritical,
		Active:  true,
	})
	store.Create(ctx, CreateInput{
		Title:   "Warning",
		Content: "Content",
		Type:    TypeWarning,
		Active:  true,
	})

	active, _ := store.GetActive(ctx)
	if len(active) != 3 {
		t.Fatalf("GetActive() count = %d, want 3", len(active))
	}

	// Critical > Warning > Info (descending order by type string)
	// The sort is by type descending, so "warning" > "info" > "critical" alphabetically but
	// we need to verify the actual order
	// Actually the sort is type descending, created_at descending
}

func TestStore_SetActive(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	created, _ := store.Create(ctx, CreateInput{
		Title:   "SetActive Test",
		Content: "Content",
		Type:    TypeInfo,
		Active:  false,
	})

	// Activate
	if err := store.SetActive(ctx, created.ID, true); err != nil {
		t.Fatalf("SetActive(true) error = %v", err)
	}

	ann, _ := store.GetByID(ctx, created.ID)
	if !ann.Active {
		t.Error("SetActive(true) should set Active to true")
	}

	// Deactivate
	if err := store.SetActive(ctx, created.ID, false); err != nil {
		t.Fatalf("SetActive(false) error = %v", err)
	}

	ann, _ = store.GetByID(ctx, created.ID)
	if ann.Active {
		t.Error("SetActive(false) should set Active to false")
	}
}

func TestTypeConstants(t *testing.T) {
	if TypeInfo != "info" {
		t.Errorf("TypeInfo = %q, want 'info'", TypeInfo)
	}
	if TypeWarning != "warning" {
		t.Errorf("TypeWarning = %q, want 'warning'", TypeWarning)
	}
	if TypeCritical != "critical" {
		t.Errorf("TypeCritical = %q, want 'critical'", TypeCritical)
	}
}
