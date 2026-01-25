package folder

import (
	"testing"

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

func TestStore_Create(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	input := CreateInput{
		Name:        "Test Folder",
		Description: "Test description",
		CreatedByID: primitive.NewObjectID(),
	}

	folder, err := store.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if folder.ID.IsZero() {
		t.Error("ID should not be zero")
	}
	if folder.Name != input.Name {
		t.Errorf("Name = %v, want %v", folder.Name, input.Name)
	}
	if folder.Description != input.Description {
		t.Errorf("Description = %v, want %v", folder.Description, input.Description)
	}
	if folder.ParentID != nil {
		t.Error("ParentID should be nil for root folder")
	}
}

func TestStore_Create_Nested(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	creatorID := primitive.NewObjectID()

	// Create parent
	parent, _ := store.Create(ctx, CreateInput{
		Name:        "Parent",
		CreatedByID: creatorID,
	})

	// Create child
	child, err := store.Create(ctx, CreateInput{
		Name:        "Child",
		ParentID:    &parent.ID,
		CreatedByID: creatorID,
	})
	if err != nil {
		t.Fatalf("Create() child error = %v", err)
	}

	if child.ParentID == nil || *child.ParentID != parent.ID {
		t.Errorf("ParentID = %v, want %v", child.ParentID, parent.ID)
	}
}

func TestStore_GetByID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	created, _ := store.Create(ctx, CreateInput{
		Name:        "GetByID Test",
		CreatedByID: primitive.NewObjectID(),
	})

	// Valid ID
	folder, err := store.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if folder.ID != created.ID {
		t.Errorf("ID = %v, want %v", folder.ID, created.ID)
	}
	if folder.Name != "GetByID Test" {
		t.Errorf("Name = %v, want 'GetByID Test'", folder.Name)
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
		Name:        "Original",
		Description: "Original description",
		CreatedByID: primitive.NewObjectID(),
	})

	newName := "Updated"
	newDesc := "Updated description"
	updateInput := UpdateInput{
		Name:        &newName,
		Description: &newDesc,
	}

	if err := store.Update(ctx, created.ID, updateInput); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	folder, _ := store.GetByID(ctx, created.ID)
	if folder.Name != newName {
		t.Errorf("Name = %v, want %v", folder.Name, newName)
	}
	if folder.Description != newDesc {
		t.Errorf("Description = %v, want %v", folder.Description, newDesc)
	}
}

func TestStore_Delete(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	created, _ := store.Create(ctx, CreateInput{
		Name:        "To Delete",
		CreatedByID: primitive.NewObjectID(),
	})

	if err := store.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := store.GetByID(ctx, created.ID)
	if err != mongo.ErrNoDocuments {
		t.Errorf("GetByID() after delete error = %v, want %v", err, mongo.ErrNoDocuments)
	}
}

func TestStore_ListByParent(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	creatorID := primitive.NewObjectID()

	// Create root folders
	for i := 0; i < 3; i++ {
		store.Create(ctx, CreateInput{
			Name:        "Root " + string(rune('A'+i)),
			CreatedByID: creatorID,
		})
	}

	// Create nested folders
	parent, _ := store.Create(ctx, CreateInput{
		Name:        "Parent",
		CreatedByID: creatorID,
	})
	for i := 0; i < 2; i++ {
		store.Create(ctx, CreateInput{
			Name:        "Child " + string(rune('A'+i)),
			ParentID:    &parent.ID,
			CreatedByID: creatorID,
		})
	}

	// List root folders (not including Parent folder which was created after)
	rootFolders, err := store.ListByParent(ctx, nil, ListOptions{})
	if err != nil {
		t.Fatalf("ListByParent(nil) error = %v", err)
	}
	if len(rootFolders) != 4 { // 3 root + 1 parent
		t.Errorf("ListByParent(nil) count = %d, want 4", len(rootFolders))
	}

	// List child folders
	childFolders, err := store.ListByParent(ctx, &parent.ID, ListOptions{})
	if err != nil {
		t.Fatalf("ListByParent(parent.ID) error = %v", err)
	}
	if len(childFolders) != 2 {
		t.Errorf("ListByParent(parent.ID) count = %d, want 2", len(childFolders))
	}
}

func TestStore_ListByParent_Sorted(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	creatorID := primitive.NewObjectID()
	names := []string{"Charlie", "Alpha", "Bravo"}

	for _, name := range names {
		store.Create(ctx, CreateInput{
			Name:        name,
			CreatedByID: creatorID,
		})
	}

	// Sort ascending
	folders, _ := store.ListByParent(ctx, nil, ListOptions{SortBy: "name", SortOrder: 1})
	if folders[0].Name != "Alpha" || folders[1].Name != "Bravo" || folders[2].Name != "Charlie" {
		t.Error("Folders not sorted ascending by name")
	}

	// Sort descending
	folders, _ = store.ListByParent(ctx, nil, ListOptions{SortBy: "name", SortOrder: -1})
	if folders[0].Name != "Charlie" || folders[1].Name != "Bravo" || folders[2].Name != "Alpha" {
		t.Error("Folders not sorted descending by name")
	}
}

func TestStore_CountByParent(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	creatorID := primitive.NewObjectID()

	// Create root folders
	store.Create(ctx, CreateInput{Name: "Root A", CreatedByID: creatorID})
	store.Create(ctx, CreateInput{Name: "Root B", CreatedByID: creatorID})

	parent, _ := store.Create(ctx, CreateInput{Name: "Parent", CreatedByID: creatorID})
	store.Create(ctx, CreateInput{Name: "Child", ParentID: &parent.ID, CreatedByID: creatorID})

	rootCount, _ := store.CountByParent(ctx, nil)
	if rootCount != 3 { // Root A, Root B, Parent
		t.Errorf("CountByParent(nil) = %d, want 3", rootCount)
	}

	childCount, _ := store.CountByParent(ctx, &parent.ID)
	if childCount != 1 {
		t.Errorf("CountByParent(parent.ID) = %d, want 1", childCount)
	}
}

func TestStore_GetAncestors(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	creatorID := primitive.NewObjectID()

	// Create hierarchy: Root > Level1 > Level2 > Level3
	root, _ := store.Create(ctx, CreateInput{Name: "Root", CreatedByID: creatorID})
	level1, _ := store.Create(ctx, CreateInput{Name: "Level1", ParentID: &root.ID, CreatedByID: creatorID})
	level2, _ := store.Create(ctx, CreateInput{Name: "Level2", ParentID: &level1.ID, CreatedByID: creatorID})
	level3, _ := store.Create(ctx, CreateInput{Name: "Level3", ParentID: &level2.ID, CreatedByID: creatorID})

	// Get ancestors of level3 (should be Root, Level1, Level2)
	ancestors, err := store.GetAncestors(ctx, level3.ID)
	if err != nil {
		t.Fatalf("GetAncestors() error = %v", err)
	}
	if len(ancestors) != 3 {
		t.Fatalf("GetAncestors() count = %d, want 3", len(ancestors))
	}

	// Check order (root first)
	if ancestors[0].ID != root.ID {
		t.Error("First ancestor should be root")
	}
	if ancestors[1].ID != level1.ID {
		t.Error("Second ancestor should be level1")
	}
	if ancestors[2].ID != level2.ID {
		t.Error("Third ancestor should be level2")
	}
}

func TestStore_GetPath(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	creatorID := primitive.NewObjectID()

	// Create hierarchy
	root, _ := store.Create(ctx, CreateInput{Name: "Root", CreatedByID: creatorID})
	child, _ := store.Create(ctx, CreateInput{Name: "Child", ParentID: &root.ID, CreatedByID: creatorID})

	// Get path (should include ancestors + self)
	path, err := store.GetPath(ctx, child.ID)
	if err != nil {
		t.Fatalf("GetPath() error = %v", err)
	}
	if len(path) != 2 {
		t.Fatalf("GetPath() count = %d, want 2", len(path))
	}

	if path[0].ID != root.ID {
		t.Error("First in path should be root")
	}
	if path[1].ID != child.ID {
		t.Error("Second in path should be self")
	}
}

func TestStore_NameExistsInParent(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	creatorID := primitive.NewObjectID()

	created, _ := store.Create(ctx, CreateInput{
		Name:        "Existing",
		CreatedByID: creatorID,
	})

	// Same name exists
	exists, err := store.NameExistsInParent(ctx, "Existing", nil, nil)
	if err != nil {
		t.Fatalf("NameExistsInParent() error = %v", err)
	}
	if !exists {
		t.Error("NameExistsInParent() should return true for existing name")
	}

	// Case insensitive
	exists, _ = store.NameExistsInParent(ctx, "EXISTING", nil, nil)
	if !exists {
		t.Error("NameExistsInParent() should be case-insensitive")
	}

	// Different name
	exists, _ = store.NameExistsInParent(ctx, "Different", nil, nil)
	if exists {
		t.Error("NameExistsInParent() should return false for different name")
	}

	// Exclude self
	exists, _ = store.NameExistsInParent(ctx, "Existing", nil, &created.ID)
	if exists {
		t.Error("NameExistsInParent() should return false when excluding self")
	}
}

func TestStore_HasSubfolders(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	creatorID := primitive.NewObjectID()

	parent, _ := store.Create(ctx, CreateInput{Name: "Parent", CreatedByID: creatorID})
	empty, _ := store.Create(ctx, CreateInput{Name: "Empty", CreatedByID: creatorID})
	store.Create(ctx, CreateInput{Name: "Child", ParentID: &parent.ID, CreatedByID: creatorID})

	// Parent has subfolders
	has, err := store.HasSubfolders(ctx, parent.ID)
	if err != nil {
		t.Fatalf("HasSubfolders() error = %v", err)
	}
	if !has {
		t.Error("HasSubfolders() should return true for parent with children")
	}

	// Empty has no subfolders
	has, err = store.HasSubfolders(ctx, empty.ID)
	if err != nil {
		t.Fatalf("HasSubfolders() error = %v", err)
	}
	if has {
		t.Error("HasSubfolders() should return false for empty folder")
	}
}
