package files

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dalemusser/stratasave/internal/app/store/file"
	"github.com/dalemusser/stratasave/internal/app/store/folder"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/testutil"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

func TestHandler_NewHandler(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	h := NewHandler(db, nil, nil, nil, logger)

	if h == nil {
		t.Fatal("NewHandler() returned nil")
	}
	if h.folderStore == nil {
		t.Error("folderStore should not be nil")
	}
	if h.fileStore == nil {
		t.Error("fileStore should not be nil")
	}
}

func TestFolder_Create(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	folderStore := folder.New(db)
	userID := primitive.NewObjectID()

	// Create a folder at root
	input := folder.CreateInput{
		Name:        "Test Folder",
		ParentID:    nil,
		Description: "A test folder",
		CreatedByID: userID,
	}

	created, err := folderStore.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if created.ID.IsZero() {
		t.Error("created folder should have non-zero ID")
	}
	if created.Name != "Test Folder" {
		t.Errorf("Name = %q, want %q", created.Name, "Test Folder")
	}
	if created.Description != "A test folder" {
		t.Errorf("Description = %q, want %q", created.Description, "A test folder")
	}
	if created.ParentID != nil {
		t.Error("ParentID should be nil for root folder")
	}
}

func TestFolder_CreateNested(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	folderStore := folder.New(db)
	userID := primitive.NewObjectID()

	// Create parent folder
	parent, err := folderStore.Create(ctx, folder.CreateInput{
		Name:        "Parent",
		CreatedByID: userID,
	})
	if err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	// Create child folder
	child, err := folderStore.Create(ctx, folder.CreateInput{
		Name:        "Child",
		ParentID:    &parent.ID,
		CreatedByID: userID,
	})
	if err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	if child.ParentID == nil {
		t.Fatal("child ParentID should not be nil")
	}
	if *child.ParentID != parent.ID {
		t.Errorf("ParentID = %v, want %v", *child.ParentID, parent.ID)
	}
}

func TestFolder_GetPath(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	folderStore := folder.New(db)
	userID := primitive.NewObjectID()

	// Create folder hierarchy: Root -> Level1 -> Level2
	root, _ := folderStore.Create(ctx, folder.CreateInput{Name: "Root", CreatedByID: userID})
	level1, _ := folderStore.Create(ctx, folder.CreateInput{Name: "Level1", ParentID: &root.ID, CreatedByID: userID})
	level2, _ := folderStore.Create(ctx, folder.CreateInput{Name: "Level2", ParentID: &level1.ID, CreatedByID: userID})

	// Get path to level2
	path, err := folderStore.GetPath(ctx, level2.ID)
	if err != nil {
		t.Fatalf("GetPath() error = %v", err)
	}

	if len(path) != 3 {
		t.Fatalf("path length = %d, want 3", len(path))
	}
	if path[0].Name != "Root" {
		t.Errorf("path[0].Name = %q, want %q", path[0].Name, "Root")
	}
	if path[1].Name != "Level1" {
		t.Errorf("path[1].Name = %q, want %q", path[1].Name, "Level1")
	}
	if path[2].Name != "Level2" {
		t.Errorf("path[2].Name = %q, want %q", path[2].Name, "Level2")
	}
}

func TestFolder_NameExists(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	folderStore := folder.New(db)
	userID := primitive.NewObjectID()

	// Create a folder
	existing, _ := folderStore.Create(ctx, folder.CreateInput{
		Name:        "Existing",
		CreatedByID: userID,
	})

	// Check for duplicate name
	exists, err := folderStore.NameExistsInParent(ctx, "Existing", nil, nil)
	if err != nil {
		t.Fatalf("NameExistsInParent() error = %v", err)
	}
	if !exists {
		t.Error("should detect existing folder name")
	}

	// Check excluding self
	exists, err = folderStore.NameExistsInParent(ctx, "Existing", nil, &existing.ID)
	if err != nil {
		t.Fatalf("NameExistsInParent() error = %v", err)
	}
	if exists {
		t.Error("should not detect self as duplicate")
	}

	// Check non-existent name
	exists, err = folderStore.NameExistsInParent(ctx, "NonExistent", nil, nil)
	if err != nil {
		t.Fatalf("NameExistsInParent() error = %v", err)
	}
	if exists {
		t.Error("should not detect non-existent name")
	}
}

func TestFile_Create(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	fileStore := file.New(db)
	userID := primitive.NewObjectID()

	input := file.CreateInput{
		FolderID:    nil,
		Name:        "test.pdf",
		StoragePath: "files/2024/01/abc123.pdf",
		Size:        1024,
		ContentType: "application/pdf",
		Description: "Test PDF file",
		CreatedByID: userID,
	}

	created, err := fileStore.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if created.ID.IsZero() {
		t.Error("created file should have non-zero ID")
	}
	if created.Name != "test.pdf" {
		t.Errorf("Name = %q, want %q", created.Name, "test.pdf")
	}
	if created.Size != 1024 {
		t.Errorf("Size = %d, want %d", created.Size, 1024)
	}
	if created.ContentType != "application/pdf" {
		t.Errorf("ContentType = %q, want %q", created.ContentType, "application/pdf")
	}
}

func TestFile_ListByFolder(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	fileStore := file.New(db)
	folderStore := folder.New(db)
	userID := primitive.NewObjectID()

	// Create a folder
	f, _ := folderStore.Create(ctx, folder.CreateInput{Name: "Files Folder", CreatedByID: userID})

	// Create files in the folder
	for i := 0; i < 3; i++ {
		fileStore.Create(ctx, file.CreateInput{
			FolderID:    &f.ID,
			Name:        "file" + string(rune('1'+i)) + ".txt",
			StoragePath: "files/test" + string(rune('1'+i)) + ".txt",
			Size:        100,
			ContentType: "text/plain",
			CreatedByID: userID,
		})
	}

	// Create file at root (no folder)
	fileStore.Create(ctx, file.CreateInput{
		FolderID:    nil,
		Name:        "root.txt",
		StoragePath: "files/root.txt",
		Size:        50,
		ContentType: "text/plain",
		CreatedByID: userID,
	})

	// List files in folder
	files, err := fileStore.ListByFolder(ctx, &f.ID, file.ListOptions{})
	if err != nil {
		t.Fatalf("ListByFolder() error = %v", err)
	}
	if len(files) != 3 {
		t.Errorf("got %d files, want 3", len(files))
	}

	// List files at root
	rootFiles, err := fileStore.ListByFolder(ctx, nil, file.ListOptions{})
	if err != nil {
		t.Fatalf("ListByFolder(nil) error = %v", err)
	}
	if len(rootFiles) != 1 {
		t.Errorf("got %d root files, want 1", len(rootFiles))
	}
}

func TestFile_NameExistsInFolder(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx, cancel := testutil.TestContext()
	defer cancel()

	fileStore := file.New(db)
	userID := primitive.NewObjectID()

	// Create a file
	existing, _ := fileStore.Create(ctx, file.CreateInput{
		FolderID:    nil,
		Name:        "existing.txt",
		StoragePath: "files/existing.txt",
		Size:        100,
		ContentType: "text/plain",
		CreatedByID: userID,
	})

	// Check for duplicate
	exists, err := fileStore.NameExistsInFolder(ctx, "existing.txt", nil, nil)
	if err != nil {
		t.Fatalf("NameExistsInFolder() error = %v", err)
	}
	if !exists {
		t.Error("should detect existing file name")
	}

	// Check excluding self
	exists, err = fileStore.NameExistsInFolder(ctx, "existing.txt", nil, &existing.ID)
	if err != nil {
		t.Fatalf("NameExistsInFolder() error = %v", err)
	}
	if exists {
		t.Error("should not detect self as duplicate")
	}
}

func TestBrowseVM_Fields(t *testing.T) {
	vm := BrowseVM{
		CurrentFolderID: "abc123",
		ParentURL:       "/library",
		Breadcrumbs: []BreadcrumbItem{
			{ID: "", Name: "Library", URL: "/library"},
			{ID: "abc123", Name: "Docs", URL: "/library/folder/abc123"},
		},
		Folders: []FolderRow{
			{ID: "f1", Name: "Subfolder", ItemCount: 5},
		},
		Files: []FileRow{
			{ID: "file1", Name: "doc.pdf", Size: "1.5 MB", ContentType: "application/pdf"},
		},
		IsAdmin:      true,
		SortBy:       "name",
		SortOrder:    "asc",
		TypeFilter:   "application/pdf",
		SearchQuery:  "doc",
		TotalFolders: 1,
		TotalFiles:   1,
	}

	if vm.CurrentFolderID != "abc123" {
		t.Errorf("CurrentFolderID = %q, want %q", vm.CurrentFolderID, "abc123")
	}
	if len(vm.Breadcrumbs) != 2 {
		t.Errorf("Breadcrumbs length = %d, want 2", len(vm.Breadcrumbs))
	}
	if !vm.IsAdmin {
		t.Error("IsAdmin should be true")
	}
	if vm.TotalFolders != 1 {
		t.Errorf("TotalFolders = %d, want 1", vm.TotalFolders)
	}
}

func TestFolderFormVM_Fields(t *testing.T) {
	vm := FolderFormVM{
		ID:          "folder123",
		Name:        "Test Folder",
		Description: "A description",
		ParentID:    "parent456",
		ParentName:  "Parent Folder",
		Error:       "",
	}

	if vm.ID != "folder123" {
		t.Errorf("ID = %q, want %q", vm.ID, "folder123")
	}
	if vm.Name != "Test Folder" {
		t.Errorf("Name = %q, want %q", vm.Name, "Test Folder")
	}
	if vm.ParentID != "parent456" {
		t.Errorf("ParentID = %q, want %q", vm.ParentID, "parent456")
	}
}

func TestFileUploadVM_Fields(t *testing.T) {
	vm := FileUploadVM{
		FolderID:   "folder123",
		FolderName: "Documents",
		Error:      "",
		MaxSize:    "32 MB",
	}

	if vm.FolderID != "folder123" {
		t.Errorf("FolderID = %q, want %q", vm.FolderID, "folder123")
	}
	if vm.MaxSize != "32 MB" {
		t.Errorf("MaxSize = %q, want %q", vm.MaxSize, "32 MB")
	}
}

func TestFileRow_Fields(t *testing.T) {
	row := FileRow{
		ID:          "file123",
		Name:        "document.pdf",
		Description: "Important document",
		Size:        "2.5 MB",
		SizeBytes:   2621440,
		ContentType: "application/pdf",
		TypeIcon:    "file-pdf",
		IsViewable:  true,
		CreatedAt:   "Jan 1, 2024",
		UpdatedAt:   "Jan 2, 2024",
	}

	if row.ID != "file123" {
		t.Errorf("ID = %q, want %q", row.ID, "file123")
	}
	if row.SizeBytes != 2621440 {
		t.Errorf("SizeBytes = %d, want %d", row.SizeBytes, 2621440)
	}
	if !row.IsViewable {
		t.Error("IsViewable should be true for PDF")
	}
}

func TestFolderRow_Fields(t *testing.T) {
	row := FolderRow{
		ID:          "folder123",
		Name:        "Projects",
		Description: "Project files",
		ItemCount:   10,
		CreatedAt:   "Jan 1, 2024",
		UpdatedAt:   "Jan 2, 2024",
	}

	if row.ID != "folder123" {
		t.Errorf("ID = %q, want %q", row.ID, "folder123")
	}
	if row.ItemCount != 10 {
		t.Errorf("ItemCount = %d, want %d", row.ItemCount, 10)
	}
}

func TestBreadcrumbItem_Fields(t *testing.T) {
	item := BreadcrumbItem{
		ID:   "abc123",
		Name: "Documents",
		URL:  "/library/folder/abc123",
	}

	if item.ID != "abc123" {
		t.Errorf("ID = %q, want %q", item.ID, "abc123")
	}
	if item.Name != "Documents" {
		t.Errorf("Name = %q, want %q", item.Name, "Documents")
	}
	if item.URL != "/library/folder/abc123" {
		t.Errorf("URL = %q, want %q", item.URL, "/library/folder/abc123")
	}
}

func TestFormValidation_EmptyFolderName(t *testing.T) {
	// Test folder name validation
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty", "", true},
		{"whitespace only", "   ", true},
		{"valid name", "Documents", false},
		{"name with spaces", "My Documents", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trimmed := strings.TrimSpace(tt.value)
			hasErr := trimmed == ""
			if hasErr != tt.wantErr {
				t.Errorf("validation for %q: got err=%v, wantErr=%v", tt.value, hasErr, tt.wantErr)
			}
		})
	}
}

func TestFormValidation_EmptyFileName(t *testing.T) {
	// Test file name validation
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty", "", true},
		{"whitespace only", "   ", true},
		{"valid name", "document.pdf", false},
		{"name with spaces", "my document.pdf", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trimmed := strings.TrimSpace(tt.value)
			hasErr := trimmed == ""
			if hasErr != tt.wantErr {
				t.Errorf("validation for %q: got err=%v, wantErr=%v", tt.value, hasErr, tt.wantErr)
			}
		})
	}
}

func TestRoutes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zap.NewNop()

	h := NewHandler(db, nil, nil, nil, logger)

	// Create a mock session manager (we can't fully test auth without more setup)
	// Just verify Routes doesn't panic
	sessionMgr := &auth.SessionManager{}

	routes := Routes(h, sessionMgr)
	if routes == nil {
		t.Error("Routes() returned nil")
	}
}

func TestMaxUploadSize(t *testing.T) {
	// Verify the max upload size constant
	if maxUploadSize != 32<<20 {
		t.Errorf("maxUploadSize = %d, want %d (32MB)", maxUploadSize, 32<<20)
	}
}

func TestObjectIDParsing(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid ObjectID", primitive.NewObjectID().Hex(), false},
		{"invalid - too short", "abc", true},
		{"invalid - not hex", "gggggggggggggggggggggggg", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := primitive.ObjectIDFromHex(tt.id)
			gotErr := err != nil
			if gotErr != tt.wantErr {
				t.Errorf("ObjectIDFromHex(%q): got err=%v, wantErr=%v", tt.id, gotErr, tt.wantErr)
			}
		})
	}
}

func TestURLParams(t *testing.T) {
	// Test Chi URL parameter extraction
	r := chi.NewRouter()
	var capturedID string

	r.Get("/folder/{id}", func(w http.ResponseWriter, r *http.Request) {
		capturedID = chi.URLParam(r, "id")
	})

	req := httptest.NewRequest(http.MethodGet, "/folder/abc123def456789012345678", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if capturedID != "abc123def456789012345678" {
		t.Errorf("URLParam = %q, want %q", capturedID, "abc123def456789012345678")
	}
}

func TestQueryParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/library?sort=name&order=desc&type=pdf&q=search", nil)

	sortBy := req.URL.Query().Get("sort")
	order := req.URL.Query().Get("order")
	typeFilter := req.URL.Query().Get("type")
	query := req.URL.Query().Get("q")

	if sortBy != "name" {
		t.Errorf("sort = %q, want %q", sortBy, "name")
	}
	if order != "desc" {
		t.Errorf("order = %q, want %q", order, "desc")
	}
	if typeFilter != "pdf" {
		t.Errorf("type = %q, want %q", typeFilter, "pdf")
	}
	if query != "search" {
		t.Errorf("q = %q, want %q", query, "search")
	}
}

func TestFormParsing(t *testing.T) {
	form := url.Values{}
	form.Set("name", "Test Folder")
	form.Set("description", "A test folder")
	form.Set("parent_id", "abc123")

	req := httptest.NewRequest(http.MethodPost, "/folder/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm() error = %v", err)
	}

	if got := req.FormValue("name"); got != "Test Folder" {
		t.Errorf("name = %q, want %q", got, "Test Folder")
	}
	if got := req.FormValue("description"); got != "A test folder" {
		t.Errorf("description = %q, want %q", got, "A test folder")
	}
	if got := req.FormValue("parent_id"); got != "abc123" {
		t.Errorf("parent_id = %q, want %q", got, "abc123")
	}
}
