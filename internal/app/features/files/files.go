// Package files provides the Files feature with nested folder support.
package files

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	errorsfeature "github.com/dalemusser/stratasave/internal/app/features/errors"
	"github.com/dalemusser/stratasave/internal/app/store/file"
	"github.com/dalemusser/stratasave/internal/app/store/folder"
	"github.com/dalemusser/stratasave/internal/app/system/auditlog"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/waffle/pantry/storage"
	"github.com/dalemusser/waffle/pantry/templates"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/csrf"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

const maxUploadSize = 32 << 20 // 32MB

// Handler provides file management handlers.
type Handler struct {
	folderStore *folder.Store
	fileStore   *file.Store
	fileStorage storage.Store
	errLog      *errorsfeature.ErrorLogger
	auditLogger *auditlog.Logger
	logger      *zap.Logger
}

// NewHandler creates a new files Handler.
func NewHandler(
	db *mongo.Database,
	fileStorage storage.Store,
	errLog *errorsfeature.ErrorLogger,
	auditLogger *auditlog.Logger,
	logger *zap.Logger,
) *Handler {
	return &Handler{
		folderStore: folder.New(db),
		fileStore:   file.New(db),
		fileStorage: fileStorage,
		errLog:      errLog,
		auditLogger: auditLogger,
		logger:      logger,
	}
}

// Routes returns a chi.Router with file routes mounted.
func Routes(h *Handler, sessionMgr *auth.SessionManager) http.Handler {
	r := chi.NewRouter()
	r.Use(sessionMgr.RequireAuth) // All routes require authentication

	// Browse routes (all authenticated users)
	r.Get("/", h.browse)
	r.Get("/folder/{id}", h.browse)
	r.Get("/folder/{id}/info_modal", h.folderInfoModal)
	r.Get("/file/{id}/info_modal", h.fileInfoModal)
	r.Get("/file/{id}/view", h.view)
	r.Get("/file/{id}/download", h.download)

	// Admin-only routes
	r.Group(func(r chi.Router) {
		r.Use(sessionMgr.RequireRole("admin"))

		// Folder management
		r.Get("/folder/new", h.showNewFolder)
		r.Post("/folder/new", h.createFolder)
		r.Get("/folder/{id}/edit", h.showEditFolder)
		r.Post("/folder/{id}", h.updateFolder)
		r.Get("/folder/{id}/manage_modal", h.folderManageModal)
		r.Post("/folder/{id}/delete", h.deleteFolder)

		// File management
		r.Get("/file/upload", h.showUpload)
		r.Post("/file/upload", h.upload)
		r.Get("/file/{id}/edit", h.showEditFile)
		r.Post("/file/{id}", h.updateFile)
		r.Get("/file/{id}/manage_modal", h.fileManageModal)
		r.Post("/file/{id}/delete", h.deleteFile)
	})

	return r
}

// BreadcrumbItem represents an item in the breadcrumb trail.
type BreadcrumbItem struct {
	ID   string
	Name string
	URL  string
}

// FolderRow represents a folder in the browse view.
type FolderRow struct {
	ID          string
	Name        string
	Description string
	ItemCount   int64
	CreatedAt   string
	UpdatedAt   string
}

// FileRow represents a file in the browse view.
type FileRow struct {
	ID          string
	Name        string
	Description string
	Size        string
	SizeBytes   int64
	ContentType string
	TypeIcon    string
	IsViewable  bool
	CreatedAt   string
	UpdatedAt   string
}

// BrowseVM is the view model for the browse page.
type BrowseVM struct {
	viewdata.BaseVM
	CurrentFolder   *FolderRow
	CurrentFolderID string
	ParentURL       string // URL to go up one level (empty if at root)
	Breadcrumbs     []BreadcrumbItem
	Folders         []FolderRow
	Files           []FileRow
	IsAdmin         bool
	SortBy          string
	SortOrder       string
	TypeFilter      string
	SearchQuery     string
	TotalFolders    int
	TotalFiles      int
	Success         string
	Error           string
}

// browse displays the contents of a folder (or root).
func (h *Handler) browse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, _ := auth.CurrentUser(r)

	// Parse folder ID from URL (nil = root)
	var folderID *primitive.ObjectID
	idStr := chi.URLParam(r, "id")
	if idStr != "" {
		id, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		folderID = &id
	}

	// Get current folder details if not at root
	var currentFolder *FolderRow
	var currentFolderID string
	if folderID != nil {
		f, err := h.folderStore.GetByID(ctx, *folderID)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		currentFolderID = f.ID.Hex()
		currentFolder = &FolderRow{
			ID:          f.ID.Hex(),
			Name:        f.Name,
			Description: f.Description,
			UpdatedAt:   f.UpdatedAt.Format("Jan 2, 2006"),
		}
	}

	// Build breadcrumbs
	breadcrumbs := []BreadcrumbItem{{Name: "Library", URL: "/library"}}
	if folderID != nil {
		path, err := h.folderStore.GetPath(ctx, *folderID)
		if err == nil {
			for _, f := range path {
				breadcrumbs = append(breadcrumbs, BreadcrumbItem{
					ID:   f.ID.Hex(),
					Name: f.Name,
					URL:  "/library/folder/" + f.ID.Hex(),
				})
			}
		}
	}

	// Determine parent URL for "up" navigation (second-to-last breadcrumb)
	var parentURL string
	if len(breadcrumbs) > 1 {
		parentURL = breadcrumbs[len(breadcrumbs)-2].URL
	}

	// Parse sort options
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "name"
	}
	sortOrder := 1
	if r.URL.Query().Get("order") == "desc" {
		sortOrder = -1
	}

	// Get subfolders
	folderOpts := folder.ListOptions{SortBy: sortBy, SortOrder: sortOrder}
	folders, err := h.folderStore.ListByParent(ctx, folderID, folderOpts)
	if err != nil {
		h.errLog.Log(r, "failed to list folders", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Build folder rows with item counts
	folderRows := make([]FolderRow, 0, len(folders))
	for _, f := range folders {
		// Count items in folder (subfolders + files)
		subfolderCount, _ := h.folderStore.CountByParent(ctx, &f.ID)
		fileCount, _ := h.fileStore.CountByFolderID(ctx, f.ID)
		itemCount := subfolderCount + fileCount

		folderRows = append(folderRows, FolderRow{
			ID:          f.ID.Hex(),
			Name:        f.Name,
			Description: f.Description,
			ItemCount:   itemCount,
			UpdatedAt:   f.UpdatedAt.Format("Jan 2, 2006"),
		})
	}

	// Get files
	typeFilter := r.URL.Query().Get("type")
	searchQuery := r.URL.Query().Get("q")
	fileOpts := file.ListOptions{
		SortBy:      sortBy,
		SortOrder:   sortOrder,
		ContentType: typeFilter,
		Search:      searchQuery,
	}
	files, err := h.fileStore.ListByFolder(ctx, folderID, fileOpts)
	if err != nil {
		h.errLog.Log(r, "failed to list files", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Build file rows
	fileRows := make([]FileRow, 0, len(files))
	for _, f := range files {
		fileRows = append(fileRows, FileRow{
			ID:          f.ID.Hex(),
			Name:        f.Name,
			Description: f.Description,
			Size:        FormatFileSize(f.Size),
			SizeBytes:   f.Size,
			ContentType: f.ContentType,
			TypeIcon:    FileTypeIcon(f.ContentType),
			IsViewable:  IsViewable(f.ContentType),
			UpdatedAt:   f.UpdatedAt.Format("Jan 2, 2006"),
		})
	}

	// Determine sort order string for UI
	sortOrderStr := "asc"
	if sortOrder == -1 {
		sortOrderStr = "desc"
	}

	vm := BrowseVM{
		BaseVM:          viewdata.New(r),
		CurrentFolder:   currentFolder,
		CurrentFolderID: currentFolderID,
		ParentURL:       parentURL,
		Breadcrumbs:     breadcrumbs,
		Folders:         folderRows,
		Files:           fileRows,
		IsAdmin:         actor.Role == "admin",
		SortBy:          sortBy,
		SortOrder:       sortOrderStr,
		TypeFilter:      typeFilter,
		SearchQuery:     searchQuery,
		TotalFolders:    len(folderRows),
		TotalFiles:      len(fileRows),
	}
	vm.Title = "Library"
	vm.BackURL = "/dashboard"

	// Handle flash messages
	switch r.URL.Query().Get("success") {
	case "folder_created":
		vm.Success = "Folder created successfully"
	case "folder_updated":
		vm.Success = "Folder updated successfully"
	case "folder_deleted":
		vm.Success = "Folder deleted successfully"
	case "uploaded":
		vm.Success = "File uploaded successfully"
	case "file_updated":
		vm.Success = "File updated successfully"
	case "file_deleted":
		vm.Success = "File deleted successfully"
	}

	switch r.URL.Query().Get("error") {
	case "delete_failed":
		vm.Error = "Failed to delete item"
	}

	templates.Render(w, r, "files/browse", vm)
}

// FolderFormVM is the view model for folder new/edit forms.
type FolderFormVM struct {
	viewdata.BaseVM
	ID          string
	Name        string
	Description string
	ParentID    string
	ParentName  string
	Error       string
}

// showNewFolder displays the new folder form.
func (h *Handler) showNewFolder(w http.ResponseWriter, r *http.Request) {
	parentID := r.URL.Query().Get("parent")
	var parentName string

	if parentID != "" {
		id, err := primitive.ObjectIDFromHex(parentID)
		if err == nil {
			parent, err := h.folderStore.GetByID(r.Context(), id)
			if err == nil {
				parentName = parent.Name
			}
		}
	}

	backURL := "/library"
	if parentID != "" {
		backURL = "/library/folder/" + parentID
	}

	vm := FolderFormVM{
		BaseVM:     viewdata.New(r),
		ParentID:   parentID,
		ParentName: parentName,
	}
	vm.Title = "New Folder"
	vm.BackURL = backURL

	templates.Render(w, r, "files/folder_new", vm)
}

// createFolder creates a new folder.
func (h *Handler) createFolder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, _ := auth.CurrentUser(r)

	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	parentIDStr := r.FormValue("parent_id")

	var parentID *primitive.ObjectID
	if parentIDStr != "" {
		id, err := primitive.ObjectIDFromHex(parentIDStr)
		if err == nil {
			parentID = &id
		}
	}

	// Validate name
	if name == "" {
		vm := FolderFormVM{
			BaseVM:      viewdata.New(r),
			Name:        name,
			Description: description,
			ParentID:    parentIDStr,
			Error:       "Folder name is required",
		}
		vm.Title = "New Folder"
		vm.BackURL = "/library"
		templates.Render(w, r, "files/folder_new", vm)
		return
	}

	// Check for duplicate name
	exists, err := h.folderStore.NameExistsInParent(ctx, name, parentID, nil)
	if err != nil {
		h.errLog.Log(r, "failed to check folder name", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if exists {
		vm := FolderFormVM{
			BaseVM:      viewdata.New(r),
			Name:        name,
			Description: description,
			ParentID:    parentIDStr,
			Error:       "A folder with this name already exists",
		}
		vm.Title = "New Folder"
		vm.BackURL = "/library"
		templates.Render(w, r, "files/folder_new", vm)
		return
	}

	// Create folder
	input := folder.CreateInput{
		Name:        name,
		ParentID:    parentID,
		Description: description,
		CreatedByID: actor.UserID(),
	}
	created, err := h.folderStore.Create(ctx, input)
	if err != nil {
		h.errLog.Log(r, "failed to create folder", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Audit log
	actorID := actor.UserID()
	h.auditLogger.LogAdminEvent(r, &actorID, &created.ID, "folder_created", nil)

	// Redirect to parent folder
	redirectURL := "/library?success=folder_created"
	if parentID != nil {
		redirectURL = "/library/folder/" + parentID.Hex() + "?success=folder_created"
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// showEditFolder displays the edit folder form.
func (h *Handler) showEditFolder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	f, err := h.folderStore.GetByID(r.Context(), objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	backURL := "/library"
	if f.ParentID != nil {
		backURL = "/library/folder/" + f.ParentID.Hex()
	}

	vm := FolderFormVM{
		BaseVM:      viewdata.New(r),
		ID:          id,
		Name:        f.Name,
		Description: f.Description,
	}
	vm.Title = "Edit Folder"
	vm.BackURL = backURL

	templates.Render(w, r, "files/folder_edit", vm)
}

// updateFolder updates a folder.
func (h *Handler) updateFolder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, _ := auth.CurrentUser(r)

	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	f, err := h.folderStore.GetByID(ctx, objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))

	// Validate name
	if name == "" {
		vm := FolderFormVM{
			BaseVM:      viewdata.New(r),
			ID:          id,
			Name:        name,
			Description: description,
			Error:       "Folder name is required",
		}
		vm.Title = "Edit Folder"
		vm.BackURL = "/library"
		templates.Render(w, r, "files/folder_edit", vm)
		return
	}

	// Check for duplicate name (excluding self)
	exists, err := h.folderStore.NameExistsInParent(ctx, name, f.ParentID, &objID)
	if err != nil {
		h.errLog.Log(r, "failed to check folder name", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if exists {
		vm := FolderFormVM{
			BaseVM:      viewdata.New(r),
			ID:          id,
			Name:        name,
			Description: description,
			Error:       "A folder with this name already exists",
		}
		vm.Title = "Edit Folder"
		vm.BackURL = "/library"
		templates.Render(w, r, "files/folder_edit", vm)
		return
	}

	// Update folder
	input := folder.UpdateInput{
		Name:        &name,
		Description: &description,
	}
	if err := h.folderStore.Update(ctx, objID, input); err != nil {
		h.errLog.Log(r, "failed to update folder", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Audit log
	actorID := actor.UserID()
	h.auditLogger.LogAdminEvent(r, &actorID, &objID, "folder_updated", nil)

	// Redirect to parent folder
	redirectURL := "/library?success=folder_updated"
	if f.ParentID != nil {
		redirectURL = "/library/folder/" + f.ParentID.Hex() + "?success=folder_updated"
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// FolderManageModalVM is the view model for the folder manage modal.
type FolderManageModalVM struct {
	ID          string
	Name        string
	Description string
	ItemCount   int64
	BackURL     string
	CSRFToken   string
}

// folderManageModal displays the manage modal for a folder.
func (h *Handler) folderManageModal(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	f, err := h.folderStore.GetByID(ctx, objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Count items in folder
	subfolderCount, _ := h.folderStore.CountByParent(ctx, &objID)
	fileCount, _ := h.fileStore.CountByFolderID(ctx, objID)
	itemCount := subfolderCount + fileCount

	backURL := r.URL.Query().Get("return")
	if backURL == "" {
		backURL = "/library"
	}

	vm := FolderManageModalVM{
		ID:          id,
		Name:        f.Name,
		Description: f.Description,
		ItemCount:   itemCount,
		BackURL:     backURL,
		CSRFToken:   csrf.Token(r),
	}

	templates.RenderSnippet(w, "files/folder_manage_modal", vm)
}

// FolderInfoModalVM is the view model for the folder info modal.
type FolderInfoModalVM struct {
	ID          string
	Name        string
	Description string
	ItemCount   int64
	CreatedAt   string
	UpdatedAt   string
}

// folderInfoModal displays the info modal for a folder.
func (h *Handler) folderInfoModal(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	f, err := h.folderStore.GetByID(ctx, objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Count items in folder
	subfolderCount, _ := h.folderStore.CountByParent(ctx, &objID)
	fileCount, _ := h.fileStore.CountByFolderID(ctx, objID)
	itemCount := subfolderCount + fileCount

	vm := FolderInfoModalVM{
		ID:          id,
		Name:        f.Name,
		Description: f.Description,
		ItemCount:   itemCount,
		CreatedAt:   f.CreatedAt.Format("Jan 2, 2006 3:04 PM"),
		UpdatedAt:   f.UpdatedAt.Format("Jan 2, 2006 3:04 PM"),
	}

	templates.RenderSnippet(w, "files/folder_info_modal", vm)
}

// deleteFolderContents recursively deletes all files and subfolders within a folder.
func (h *Handler) deleteFolderContents(ctx context.Context, folderID primitive.ObjectID) error {
	// Get and delete all files in this folder
	files, err := h.fileStore.ListByFolder(ctx, &folderID, file.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing files: %w", err)
	}
	for _, f := range files {
		// Delete from storage
		if err := h.fileStorage.Delete(ctx, f.StoragePath); err != nil {
			h.logger.Warn("failed to delete file from storage",
				zap.String("path", f.StoragePath),
				zap.Error(err))
		}
		// Delete from database
		if err := h.fileStore.Delete(ctx, f.ID); err != nil {
			return fmt.Errorf("deleting file %s: %w", f.ID.Hex(), err)
		}
	}

	// Get and recursively delete all subfolders
	subfolders, err := h.folderStore.ListByParent(ctx, &folderID, folder.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing subfolders: %w", err)
	}
	for _, sf := range subfolders {
		if err := h.deleteFolderContents(ctx, sf.ID); err != nil {
			return err
		}
		if err := h.folderStore.Delete(ctx, sf.ID); err != nil {
			return fmt.Errorf("deleting subfolder %s: %w", sf.ID.Hex(), err)
		}
	}

	return nil
}

// deleteFolder deletes a folder.
func (h *Handler) deleteFolder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, _ := auth.CurrentUser(r)

	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	f, err := h.folderStore.GetByID(ctx, objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Delete all contents recursively (files and subfolders)
	if err := h.deleteFolderContents(ctx, objID); err != nil {
		h.errLog.Log(r, "failed to delete folder contents", err)
		redirectURL := "/library?error=delete_failed"
		if f.ParentID != nil {
			redirectURL = "/library/folder/" + f.ParentID.Hex() + "?error=delete_failed"
		}
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	// Delete the folder itself
	if err := h.folderStore.Delete(ctx, objID); err != nil {
		h.errLog.Log(r, "failed to delete folder", err)
		redirectURL := "/library?error=delete_failed"
		if f.ParentID != nil {
			redirectURL = "/library/folder/" + f.ParentID.Hex() + "?error=delete_failed"
		}
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	// Audit log
	actorID := actor.UserID()
	h.auditLogger.LogAdminEvent(r, &actorID, &objID, "folder_deleted", nil)

	// Redirect to parent folder
	redirectURL := "/library?success=folder_deleted"
	if f.ParentID != nil {
		redirectURL = "/library/folder/" + f.ParentID.Hex() + "?success=folder_deleted"
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// FileUploadVM is the view model for the file upload form.
type FileUploadVM struct {
	viewdata.BaseVM
	FolderID   string
	FolderName string
	Error      string
	MaxSize    string
}

// showUpload displays the file upload form.
func (h *Handler) showUpload(w http.ResponseWriter, r *http.Request) {
	folderID := r.URL.Query().Get("folder")
	var folderName string

	backURL := "/library"
	if folderID != "" {
		id, err := primitive.ObjectIDFromHex(folderID)
		if err == nil {
			f, err := h.folderStore.GetByID(r.Context(), id)
			if err == nil {
				folderName = f.Name
				backURL = "/library/folder/" + folderID
			}
		}
	}

	vm := FileUploadVM{
		BaseVM:     viewdata.New(r),
		FolderID:   folderID,
		FolderName: folderName,
		MaxSize:    "32 MB",
	}
	vm.Title = "Upload File"
	vm.BackURL = backURL

	templates.Render(w, r, "files/file_upload", vm)
}

// upload handles file upload.
func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, _ := auth.CurrentUser(r)

	// Parse multipart form
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		h.errLog.Log(r, "failed to parse multipart form", err)
		vm := FileUploadVM{
			BaseVM:  viewdata.New(r),
			Error:   "File too large (max 32MB)",
			MaxSize: "32 MB",
		}
		vm.Title = "Upload File"
		vm.BackURL = "/library"
		templates.Render(w, r, "files/file_upload", vm)
		return
	}

	// Get folder ID
	folderIDStr := r.FormValue("folder_id")
	var folderID *primitive.ObjectID
	if folderIDStr != "" {
		id, err := primitive.ObjectIDFromHex(folderIDStr)
		if err == nil {
			folderID = &id
		}
	}

	// Get uploaded file
	uploadedFile, header, err := r.FormFile("file")
	if err != nil {
		vm := FileUploadVM{
			BaseVM:     viewdata.New(r),
			FolderID:   folderIDStr,
			Error:      "Please select a file to upload",
			MaxSize:    "32 MB",
		}
		vm.Title = "Upload File"
		vm.BackURL = "/library"
		templates.Render(w, r, "files/file_upload", vm)
		return
	}
	defer uploadedFile.Close()

	description := strings.TrimSpace(r.FormValue("description"))

	// Generate storage path: files/YYYY/MM/uuid-filename
	now := time.Now().UTC()
	ext := filepath.Ext(header.Filename)
	uniqueName := fmt.Sprintf("%s%s", uuid.New().String()[:8], ext)
	storagePath := fmt.Sprintf("files/%04d/%02d/%s", now.Year(), int(now.Month()), uniqueName)

	// Get content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Upload to storage
	opts := &storage.PutOptions{
		ContentType: contentType,
	}
	if err := h.fileStorage.Put(ctx, storagePath, uploadedFile, opts); err != nil {
		h.errLog.Log(r, "failed to upload file", err)
		vm := FileUploadVM{
			BaseVM:     viewdata.New(r),
			FolderID:   folderIDStr,
			Error:      "Failed to upload file",
			MaxSize:    "32 MB",
		}
		vm.Title = "Upload File"
		vm.BackURL = "/library"
		templates.Render(w, r, "files/file_upload", vm)
		return
	}

	// Create database record
	input := file.CreateInput{
		FolderID:    folderID,
		Name:        header.Filename,
		StoragePath: storagePath,
		Size:        header.Size,
		ContentType: contentType,
		Description: description,
		CreatedByID: actor.UserID(),
	}

	createdFile, err := h.fileStore.Create(ctx, input)
	if err != nil {
		// Clean up uploaded file on DB error
		_ = h.fileStorage.Delete(ctx, storagePath)
		h.errLog.Log(r, "failed to create file record", err)
		vm := FileUploadVM{
			BaseVM:     viewdata.New(r),
			FolderID:   folderIDStr,
			Error:      "Failed to save file record",
			MaxSize:    "32 MB",
		}
		vm.Title = "Upload File"
		vm.BackURL = "/library"
		templates.Render(w, r, "files/file_upload", vm)
		return
	}

	// Audit log
	actorID := actor.UserID()
	h.auditLogger.LogAdminEvent(r, &actorID, &createdFile.ID, "file_uploaded", nil)

	// Redirect back to folder
	redirectURL := "/library?success=uploaded"
	if folderID != nil {
		redirectURL = "/library/folder/" + folderID.Hex() + "?success=uploaded"
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// FileFormVM is the view model for file edit form.
type FileFormVM struct {
	viewdata.BaseVM
	ID          string
	Name        string
	Description string
	Size        string
	ContentType string
	Error       string
}

// showEditFile displays the edit file form.
func (h *Handler) showEditFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	f, err := h.fileStore.GetByID(r.Context(), objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	backURL := "/library"
	if f.FolderID != nil {
		backURL = "/library/folder/" + f.FolderID.Hex()
	}

	vm := FileFormVM{
		BaseVM:      viewdata.New(r),
		ID:          id,
		Name:        f.Name,
		Description: f.Description,
		Size:        FormatFileSize(f.Size),
		ContentType: f.ContentType,
	}
	vm.Title = "Edit File"
	vm.BackURL = backURL

	templates.Render(w, r, "files/file_edit", vm)
}

// updateFile updates a file.
func (h *Handler) updateFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, _ := auth.CurrentUser(r)

	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	f, err := h.fileStore.GetByID(ctx, objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))

	// Validate name
	if name == "" {
		vm := FileFormVM{
			BaseVM:      viewdata.New(r),
			ID:          id,
			Name:        name,
			Description: description,
			Size:        FormatFileSize(f.Size),
			ContentType: f.ContentType,
			Error:       "File name is required",
		}
		vm.Title = "Edit File"
		vm.BackURL = "/library"
		templates.Render(w, r, "files/file_edit", vm)
		return
	}

	// Check for duplicate name (excluding self)
	exists, err := h.fileStore.NameExistsInFolder(ctx, name, f.FolderID, &objID)
	if err != nil {
		h.errLog.Log(r, "failed to check file name", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if exists {
		vm := FileFormVM{
			BaseVM:      viewdata.New(r),
			ID:          id,
			Name:        name,
			Description: description,
			Size:        FormatFileSize(f.Size),
			ContentType: f.ContentType,
			Error:       "A file with this name already exists",
		}
		vm.Title = "Edit File"
		vm.BackURL = "/library"
		templates.Render(w, r, "files/file_edit", vm)
		return
	}

	// Update file
	input := file.UpdateInput{
		Name:        &name,
		Description: &description,
	}
	if err := h.fileStore.Update(ctx, objID, input); err != nil {
		h.errLog.Log(r, "failed to update file", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Audit log
	actorID := actor.UserID()
	h.auditLogger.LogAdminEvent(r, &actorID, &objID, "file_updated", nil)

	// Redirect to folder
	redirectURL := "/library?success=file_updated"
	if f.FolderID != nil {
		redirectURL = "/library/folder/" + f.FolderID.Hex() + "?success=file_updated"
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// FileManageModalVM is the view model for the file manage modal.
type FileManageModalVM struct {
	ID          string
	Name        string
	Description string
	Size        string
	ContentType string
	TypeIcon    string
	IsViewable  bool
	BackURL     string
	CSRFToken   string
}

// fileManageModal displays the manage modal for a file.
func (h *Handler) fileManageModal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	f, err := h.fileStore.GetByID(r.Context(), objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	backURL := r.URL.Query().Get("return")
	if backURL == "" {
		backURL = "/library"
	}

	vm := FileManageModalVM{
		ID:          id,
		Name:        f.Name,
		Description: f.Description,
		Size:        FormatFileSize(f.Size),
		ContentType: f.ContentType,
		TypeIcon:    FileTypeIcon(f.ContentType),
		IsViewable:  IsViewable(f.ContentType),
		BackURL:     backURL,
		CSRFToken:   csrf.Token(r),
	}

	templates.RenderSnippet(w, "files/file_manage_modal", vm)
}

// FileInfoModalVM is the view model for the file info modal.
type FileInfoModalVM struct {
	ID          string
	Name        string
	Description string
	Size        string
	ContentType string
	TypeIcon    string
	IsViewable  bool
	CreatedAt   string
	UpdatedAt   string
}

// fileInfoModal displays the info modal for a file.
func (h *Handler) fileInfoModal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	f, err := h.fileStore.GetByID(r.Context(), objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	vm := FileInfoModalVM{
		ID:          id,
		Name:        f.Name,
		Description: f.Description,
		Size:        FormatFileSize(f.Size),
		ContentType: f.ContentType,
		TypeIcon:    FileTypeIcon(f.ContentType),
		IsViewable:  IsViewable(f.ContentType),
		CreatedAt:   f.CreatedAt.Format("Jan 2, 2006 3:04 PM"),
		UpdatedAt:   f.UpdatedAt.Format("Jan 2, 2006 3:04 PM"),
	}

	templates.RenderSnippet(w, "files/file_info_modal", vm)
}

// deleteFile deletes a file.
func (h *Handler) deleteFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, _ := auth.CurrentUser(r)

	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	f, err := h.fileStore.GetByID(ctx, objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Delete from storage
	if err := h.fileStorage.Delete(ctx, f.StoragePath); err != nil {
		h.logger.Warn("failed to delete file from storage",
			zap.String("path", f.StoragePath),
			zap.Error(err))
		// Continue with DB deletion anyway
	}

	// Delete from database
	if err := h.fileStore.Delete(ctx, objID); err != nil {
		h.errLog.Log(r, "failed to delete file", err)
		redirectURL := "/library?error=delete_failed"
		if f.FolderID != nil {
			redirectURL = "/library/folder/" + f.FolderID.Hex() + "?error=delete_failed"
		}
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	// Audit log
	actorID := actor.UserID()
	h.auditLogger.LogAdminEvent(r, &actorID, &objID, "file_deleted", nil)

	// Redirect to folder
	redirectURL := "/library?success=file_deleted"
	if f.FolderID != nil {
		redirectURL = "/library/folder/" + f.FolderID.Hex() + "?success=file_deleted"
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// view handles inline file viewing (opens in browser).
func (h *Handler) view(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	f, err := h.fileStore.GetByID(ctx, objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Try to get the file content and serve it
	reader, err := h.fileStorage.Get(ctx, f.StoragePath)
	if err != nil {
		h.errLog.Log(r, "failed to get file from storage", err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer reader.Close()

	// Set headers for inline viewing
	w.Header().Set("Content-Type", f.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", f.Name))

	// Stream the file
	if _, err := io.Copy(w, reader); err != nil {
		h.logger.Warn("failed to stream file",
			zap.String("path", f.StoragePath),
			zap.Error(err))
	}
}

// download handles file download.
func (h *Handler) download(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	f, err := h.fileStore.GetByID(ctx, objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Try to get the file content and serve it
	reader, err := h.fileStorage.Get(ctx, f.StoragePath)
	if err != nil {
		h.errLog.Log(r, "failed to get file from storage", err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer reader.Close()

	// Set headers
	w.Header().Set("Content-Type", f.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", f.Name))

	// Stream the file
	if _, err := io.Copy(w, reader); err != nil {
		h.logger.Warn("failed to stream file",
			zap.String("path", f.StoragePath),
			zap.Error(err))
	}
}
