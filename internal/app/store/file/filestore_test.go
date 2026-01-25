package file

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
		Name:        "test.txt",
		StoragePath: "files/2024/01/abc123.txt",
		Size:        1024,
		ContentType: "text/plain",
		Description: "Test file",
		CreatedByID: primitive.NewObjectID(),
	}

	file, err := store.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if file.ID.IsZero() {
		t.Error("ID should not be zero")
	}
	if file.Name != input.Name {
		t.Errorf("Name = %v, want %v", file.Name, input.Name)
	}
	if file.StoragePath != input.StoragePath {
		t.Errorf("StoragePath = %v, want %v", file.StoragePath, input.StoragePath)
	}
	if file.Size != input.Size {
		t.Errorf("Size = %v, want %v", file.Size, input.Size)
	}
	if file.ContentType != input.ContentType {
		t.Errorf("ContentType = %v, want %v", file.ContentType, input.ContentType)
	}
	if file.FolderID != nil {
		t.Error("FolderID should be nil for root file")
	}
}

func TestStore_Create_InFolder(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	folderID := primitive.NewObjectID()
	input := CreateInput{
		FolderID:    &folderID,
		Name:        "nested.txt",
		StoragePath: "files/2024/01/nested123.txt",
		Size:        512,
		ContentType: "text/plain",
		CreatedByID: primitive.NewObjectID(),
	}

	file, err := store.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if file.FolderID == nil || *file.FolderID != folderID {
		t.Errorf("FolderID = %v, want %v", file.FolderID, folderID)
	}
}

func TestStore_GetByID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	input := CreateInput{
		Name:        "getbyid.txt",
		StoragePath: "files/2024/01/getbyid.txt",
		Size:        100,
		ContentType: "text/plain",
		CreatedByID: primitive.NewObjectID(),
	}

	created, _ := store.Create(ctx, input)

	// Valid ID
	file, err := store.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if file.ID != created.ID {
		t.Errorf("ID = %v, want %v", file.ID, created.ID)
	}
	if file.Name != input.Name {
		t.Errorf("Name = %v, want %v", file.Name, input.Name)
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

	input := CreateInput{
		Name:        "original.txt",
		StoragePath: "files/2024/01/original.txt",
		Size:        100,
		ContentType: "text/plain",
		Description: "Original description",
		CreatedByID: primitive.NewObjectID(),
	}

	created, _ := store.Create(ctx, input)

	// Update name and description
	newName := "updated.txt"
	newDesc := "Updated description"
	updateInput := UpdateInput{
		Name:        &newName,
		Description: &newDesc,
	}

	if err := store.Update(ctx, created.ID, updateInput); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify update
	file, _ := store.GetByID(ctx, created.ID)
	if file.Name != newName {
		t.Errorf("Name = %v, want %v", file.Name, newName)
	}
	if file.Description != newDesc {
		t.Errorf("Description = %v, want %v", file.Description, newDesc)
	}
}

func TestStore_Delete(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	input := CreateInput{
		Name:        "todelete.txt",
		StoragePath: "files/2024/01/todelete.txt",
		Size:        100,
		ContentType: "text/plain",
		CreatedByID: primitive.NewObjectID(),
	}

	created, _ := store.Create(ctx, input)

	if err := store.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deleted
	_, err := store.GetByID(ctx, created.ID)
	if err != mongo.ErrNoDocuments {
		t.Errorf("GetByID() after delete error = %v, want %v", err, mongo.ErrNoDocuments)
	}
}

func TestStore_ListByFolder(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	creatorID := primitive.NewObjectID()

	// Create root-level files
	for i := 0; i < 3; i++ {
		store.Create(ctx, CreateInput{
			Name:        "root" + string(rune('a'+i)) + ".txt",
			StoragePath: "files/2024/01/root" + string(rune('a'+i)) + ".txt",
			Size:        100,
			ContentType: "text/plain",
			CreatedByID: creatorID,
		})
	}

	// Create files in a folder
	folderID := primitive.NewObjectID()
	for i := 0; i < 2; i++ {
		store.Create(ctx, CreateInput{
			FolderID:    &folderID,
			Name:        "nested" + string(rune('a'+i)) + ".txt",
			StoragePath: "files/2024/01/nested" + string(rune('a'+i)) + ".txt",
			Size:        200,
			ContentType: "text/plain",
			CreatedByID: creatorID,
		})
	}

	// List root files
	rootFiles, err := store.ListByFolder(ctx, nil, ListOptions{})
	if err != nil {
		t.Fatalf("ListByFolder(nil) error = %v", err)
	}
	if len(rootFiles) != 3 {
		t.Errorf("ListByFolder(nil) count = %d, want 3", len(rootFiles))
	}

	// List folder files
	folderFiles, err := store.ListByFolder(ctx, &folderID, ListOptions{})
	if err != nil {
		t.Fatalf("ListByFolder(folderID) error = %v", err)
	}
	if len(folderFiles) != 2 {
		t.Errorf("ListByFolder(folderID) count = %d, want 2", len(folderFiles))
	}
}

func TestStore_ListByFolder_SortByName(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	creatorID := primitive.NewObjectID()
	names := []string{"charlie.txt", "alpha.txt", "bravo.txt"}

	for _, name := range names {
		store.Create(ctx, CreateInput{
			Name:        name,
			StoragePath: "files/2024/01/" + name,
			Size:        100,
			ContentType: "text/plain",
			CreatedByID: creatorID,
		})
	}

	// Sort ascending
	files, _ := store.ListByFolder(ctx, nil, ListOptions{SortBy: "name", SortOrder: 1})
	if files[0].Name != "alpha.txt" || files[1].Name != "bravo.txt" || files[2].Name != "charlie.txt" {
		t.Error("Files not sorted ascending by name")
	}

	// Sort descending
	files, _ = store.ListByFolder(ctx, nil, ListOptions{SortBy: "name", SortOrder: -1})
	if files[0].Name != "charlie.txt" || files[1].Name != "bravo.txt" || files[2].Name != "alpha.txt" {
		t.Error("Files not sorted descending by name")
	}
}

func TestStore_ListByFolder_FilterByContentType(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	creatorID := primitive.NewObjectID()

	// Create files with different types
	store.Create(ctx, CreateInput{
		Name:        "image.png",
		StoragePath: "files/2024/01/image.png",
		Size:        1000,
		ContentType: "image/png",
		CreatedByID: creatorID,
	})
	store.Create(ctx, CreateInput{
		Name:        "photo.jpg",
		StoragePath: "files/2024/01/photo.jpg",
		Size:        2000,
		ContentType: "image/jpeg",
		CreatedByID: creatorID,
	})
	store.Create(ctx, CreateInput{
		Name:        "doc.txt",
		StoragePath: "files/2024/01/doc.txt",
		Size:        100,
		ContentType: "text/plain",
		CreatedByID: creatorID,
	})

	// Filter by image prefix
	files, _ := store.ListByFolder(ctx, nil, ListOptions{ContentType: "image/"})
	if len(files) != 2 {
		t.Errorf("Filter by image/ count = %d, want 2", len(files))
	}

	// Filter by text prefix
	files, _ = store.ListByFolder(ctx, nil, ListOptions{ContentType: "text/"})
	if len(files) != 1 {
		t.Errorf("Filter by text/ count = %d, want 1", len(files))
	}
}

func TestStore_ListByFolder_ContainsFilter(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	creatorID := primitive.NewObjectID()

	store.Create(ctx, CreateInput{
		Name:        "doc.docx",
		StoragePath: "files/2024/01/doc.docx",
		Size:        5000,
		ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		CreatedByID: creatorID,
	})
	store.Create(ctx, CreateInput{
		Name:        "old.doc",
		StoragePath: "files/2024/01/old.doc",
		Size:        3000,
		ContentType: "application/msword",
		CreatedByID: creatorID,
	})
	store.Create(ctx, CreateInput{
		Name:        "other.txt",
		StoragePath: "files/2024/01/other.txt",
		Size:        100,
		ContentType: "text/plain",
		CreatedByID: creatorID,
	})

	// Use contains filter (prefix ~)
	files, _ := store.ListByFolder(ctx, nil, ListOptions{ContentType: "~word"})
	if len(files) != 2 {
		t.Errorf("Filter by ~word count = %d, want 2", len(files))
	}
}

func TestStore_CountByFolder(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	creatorID := primitive.NewObjectID()
	folderID := primitive.NewObjectID()

	// Create root files
	store.Create(ctx, CreateInput{Name: "root.txt", StoragePath: "a", ContentType: "text/plain", CreatedByID: creatorID})
	store.Create(ctx, CreateInput{Name: "root2.txt", StoragePath: "b", ContentType: "text/plain", CreatedByID: creatorID})

	// Create folder file
	store.Create(ctx, CreateInput{FolderID: &folderID, Name: "nested.txt", StoragePath: "c", ContentType: "text/plain", CreatedByID: creatorID})

	rootCount, _ := store.CountByFolder(ctx, nil)
	if rootCount != 2 {
		t.Errorf("CountByFolder(nil) = %d, want 2", rootCount)
	}

	folderCount, _ := store.CountByFolderID(ctx, folderID)
	if folderCount != 1 {
		t.Errorf("CountByFolderID(folderID) = %d, want 1", folderCount)
	}
}

func TestStore_NameExistsInFolder(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	creatorID := primitive.NewObjectID()

	created, _ := store.Create(ctx, CreateInput{
		Name:        "existing.txt",
		StoragePath: "files/existing.txt",
		Size:        100,
		ContentType: "text/plain",
		CreatedByID: creatorID,
	})

	// Same name exists
	exists, err := store.NameExistsInFolder(ctx, "existing.txt", nil, nil)
	if err != nil {
		t.Fatalf("NameExistsInFolder() error = %v", err)
	}
	if !exists {
		t.Error("NameExistsInFolder() should return true for existing name")
	}

	// Case insensitive check
	exists, _ = store.NameExistsInFolder(ctx, "EXISTING.TXT", nil, nil)
	if !exists {
		t.Error("NameExistsInFolder() should be case-insensitive")
	}

	// Different name
	exists, _ = store.NameExistsInFolder(ctx, "different.txt", nil, nil)
	if exists {
		t.Error("NameExistsInFolder() should return false for different name")
	}

	// Exclude self
	exists, _ = store.NameExistsInFolder(ctx, "existing.txt", nil, &created.ID)
	if exists {
		t.Error("NameExistsInFolder() should return false when excluding self")
	}
}

func TestStore_DeleteByFolderID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	store := New(db)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	creatorID := primitive.NewObjectID()
	folderID := primitive.NewObjectID()

	// Create files in folder
	store.Create(ctx, CreateInput{FolderID: &folderID, Name: "a.txt", StoragePath: "a", ContentType: "text/plain", CreatedByID: creatorID})
	store.Create(ctx, CreateInput{FolderID: &folderID, Name: "b.txt", StoragePath: "b", ContentType: "text/plain", CreatedByID: creatorID})
	store.Create(ctx, CreateInput{FolderID: &folderID, Name: "c.txt", StoragePath: "c", ContentType: "text/plain", CreatedByID: creatorID})

	// Create root file
	store.Create(ctx, CreateInput{Name: "root.txt", StoragePath: "root", ContentType: "text/plain", CreatedByID: creatorID})

	deleted, err := store.DeleteByFolderID(ctx, folderID)
	if err != nil {
		t.Fatalf("DeleteByFolderID() error = %v", err)
	}
	if deleted != 3 {
		t.Errorf("DeleteByFolderID() deleted = %d, want 3", deleted)
	}

	// Verify folder is empty
	count, _ := store.CountByFolderID(ctx, folderID)
	if count != 0 {
		t.Errorf("Folder should be empty, has %d files", count)
	}

	// Root file should still exist
	rootCount, _ := store.CountByFolder(ctx, nil)
	if rootCount != 1 {
		t.Errorf("Root should have 1 file, has %d", rootCount)
	}
}

func TestFileTypeCategory(t *testing.T) {
	tests := []struct {
		contentType string
		want        string
	}{
		{"image/png", "image"},
		{"image/jpeg", "image"},
		{"video/mp4", "video"},
		{"audio/mpeg", "audio"},
		{"application/pdf", "pdf"},
		{"application/vnd.ms-excel", "spreadsheet"},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "spreadsheet"},
		{"application/msword", "document"},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", "document"},
		{"application/vnd.ms-powerpoint", "presentation"},
		{"application/zip", "archive"},
		{"application/x-compressed", "archive"},
		{"text/plain", "file"},
		{"unknown/type", "file"},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			got := FileTypeCategory(tt.contentType)
			if got != tt.want {
				t.Errorf("FileTypeCategory(%q) = %q, want %q", tt.contentType, got, tt.want)
			}
		})
	}
}
