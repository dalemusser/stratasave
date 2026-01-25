// internal/app/features/apikeys/handler.go
package apikeysfeature

import (
	"context"
	"net/http"
	"strings"

	errorsfeature "github.com/dalemusser/stratasave/internal/app/features/errors"
	apikeystore "github.com/dalemusser/stratasave/internal/app/store/apikeys"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/app/system/timeouts"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/waffle/pantry/templates"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Handler handles API key management HTTP requests.
type Handler struct {
	DB     *mongo.Database
	ErrLog *errorsfeature.ErrorLogger
	Log    *zap.Logger
}

// NewHandler creates a new API keys handler.
func NewHandler(db *mongo.Database, errLog *errorsfeature.ErrorLogger, logger *zap.Logger) *Handler {
	return &Handler{
		DB:     db,
		ErrLog: errLog,
		Log:    logger,
	}
}

// ServeList handles GET /api-keys - list all API keys.
func (h *Handler) ServeList(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Short())
	defer cancel()

	store := apikeystore.New(h.DB)
	keys, err := store.List(ctx)
	if err != nil {
		h.ErrLog.Log(r, "failed to load API keys", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Convert to view models
	keyVMs := make([]APIKeyVM, len(keys))
	for i, k := range keys {
		keyVMs[i] = toAPIKeyVM(k)
	}

	base := viewdata.NewBaseVM(r, h.DB, "API Keys", "/dashboard")
	data := APIKeyListVM{
		BaseVM: base,
		Keys:   keyVMs,
	}

	templates.Render(w, r, "apikeys/list", data)
}

// ServeNew handles GET /api-keys/new - show create form.
func (h *Handler) ServeNew(w http.ResponseWriter, r *http.Request) {
	base := viewdata.NewBaseVM(r, h.DB, "Create API Key", "/api-keys")
	data := APIKeyFormVM{
		BaseVM: base,
	}
	templates.Render(w, r, "apikeys/new", data)
}

// HandleCreate handles POST /api-keys - create a new API key.
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Short())
	defer cancel()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))

	// Validate
	if name == "" {
		base := viewdata.NewBaseVM(r, h.DB, "Create API Key", "/api-keys")
		data := APIKeyFormVM{
			BaseVM:      base,
			Name:        name,
			Description: description,
			Error:       "Name is required",
		}
		templates.Render(w, r, "apikeys/new", data)
		return
	}

	// Get current user
	user, ok := auth.CurrentUser(r)
	if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Parse scopes if provided
	var scopes []apikeystore.Scope
	scopeResources := r.Form["scope_resource"]
	scopeActions := r.Form["scope_actions"]
	for i, resource := range scopeResources {
		if resource != "" {
			var actions []string
			if i < len(scopeActions) {
				actions = strings.Split(scopeActions[i], ",")
			}
			scopes = append(scopes, apikeystore.Scope{
				Resource: resource,
				Actions:  actions,
			})
		}
	}

	store := apikeystore.New(h.DB)
	result, err := store.Create(ctx, apikeystore.CreateInput{
		Name:        name,
		Description: description,
		CreatedBy:   user.UserID(),
		Scopes:      scopes,
	})
	if err != nil {
		if err == apikeystore.ErrDuplicateName {
			base := viewdata.NewBaseVM(r, h.DB, "Create API Key", "/api-keys")
			data := APIKeyFormVM{
				BaseVM:      base,
				Name:        name,
				Description: description,
				Error:       "An API key with this name already exists",
			}
			templates.Render(w, r, "apikeys/new", data)
			return
		}
		h.ErrLog.Log(r, "failed to create API key", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.Log.Info("API key created",
		zap.String("key_id", result.Key.ID.Hex()),
		zap.String("name", name),
		zap.String("created_by", user.ID))

	// Show the key once
	base := viewdata.NewBaseVM(r, h.DB, "API Key Created", "/api-keys")
	data := APIKeyCreatedVM{
		BaseVM:  base,
		Key:     toAPIKeyVM(result.Key),
		FullKey: result.FullKey,
	}
	templates.Render(w, r, "apikeys/created", data)
}

// ServeDetail handles GET /api-keys/{id} - view key details.
func (h *Handler) ServeDetail(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Short())
	defer cancel()

	idStr := chi.URLParam(r, "id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	store := apikeystore.New(h.DB)
	key, err := store.GetByID(ctx, id)
	if err != nil {
		if err == apikeystore.ErrNotFound {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		h.ErrLog.Log(r, "failed to load API key", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	base := viewdata.NewBaseVM(r, h.DB, "API Key Details", "/api-keys")
	data := APIKeyDetailVM{
		BaseVM: base,
		Key:    toAPIKeyVM(*key),
	}
	templates.Render(w, r, "apikeys/detail", data)
}

// ServeEdit handles GET /api-keys/{id}/edit - show edit form.
func (h *Handler) ServeEdit(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Short())
	defer cancel()

	idStr := chi.URLParam(r, "id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	store := apikeystore.New(h.DB)
	key, err := store.GetByID(ctx, id)
	if err != nil {
		if err == apikeystore.ErrNotFound {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		h.ErrLog.Log(r, "failed to load API key", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	base := viewdata.NewBaseVM(r, h.DB, "Edit API Key", "/api-keys/"+idStr)
	data := APIKeyFormVM{
		BaseVM:      base,
		ID:          key.ID.Hex(),
		Name:        key.Name,
		Description: key.Description,
		IsEdit:      true,
		IsActive:    key.Status == apikeystore.StatusActive,
	}
	templates.Render(w, r, "apikeys/edit", data)
}

// HandleUpdate handles POST /api-keys/{id}/edit - update key metadata.
func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Short())
	defer cancel()

	idStr := chi.URLParam(r, "id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))

	store := apikeystore.New(h.DB)

	// Fetch key to get status for form re-rendering
	key, err := store.GetByID(ctx, id)
	if err != nil {
		if err == apikeystore.ErrNotFound {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		h.ErrLog.Log(r, "failed to load API key", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	isActive := key.Status == apikeystore.StatusActive

	if name == "" {
		base := viewdata.NewBaseVM(r, h.DB, "Edit API Key", "/api-keys/"+idStr)
		data := APIKeyFormVM{
			BaseVM:      base,
			ID:          idStr,
			Name:        name,
			Description: description,
			IsEdit:      true,
			IsActive:    isActive,
			Error:       "Name is required",
		}
		templates.Render(w, r, "apikeys/edit", data)
		return
	}

	err = store.Update(ctx, id, apikeystore.UpdateInput{
		Name:        &name,
		Description: &description,
	})
	if err != nil {
		if err == apikeystore.ErrNotFound {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		if err == apikeystore.ErrDuplicateName {
			base := viewdata.NewBaseVM(r, h.DB, "Edit API Key", "/api-keys/"+idStr)
			data := APIKeyFormVM{
				BaseVM:      base,
				ID:          idStr,
				Name:        name,
				Description: description,
				IsEdit:      true,
				IsActive:    isActive,
				Error:       "An API key with this name already exists",
			}
			templates.Render(w, r, "apikeys/edit", data)
			return
		}
		h.ErrLog.Log(r, "failed to update API key", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.Log.Info("API key updated",
		zap.String("key_id", idStr),
		zap.String("name", name))

	http.Redirect(w, r, "/api-keys/"+idStr, http.StatusSeeOther)
}

// HandleRevoke handles POST /api-keys/{id}/revoke - revoke an API key.
func (h *Handler) HandleRevoke(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Short())
	defer cancel()

	idStr := chi.URLParam(r, "id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	user, ok := auth.CurrentUser(r)
	if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	store := apikeystore.New(h.DB)
	err = store.Revoke(ctx, id, user.UserID())
	if err != nil {
		if err == apikeystore.ErrNotFound {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		h.ErrLog.Log(r, "failed to revoke API key", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.Log.Info("API key revoked",
		zap.String("key_id", idStr),
		zap.String("revoked_by", user.ID))

	w.Header().Set("HX-Redirect", "/api-keys")
	w.WriteHeader(http.StatusOK)
}

// HandleDelete handles POST /api-keys/{id}/delete - permanently delete an API key.
func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Short())
	defer cancel()

	idStr := chi.URLParam(r, "id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	store := apikeystore.New(h.DB)
	err = store.Delete(ctx, id)
	if err != nil {
		if err == apikeystore.ErrNotFound {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		h.ErrLog.Log(r, "failed to delete API key", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.Log.Info("API key deleted", zap.String("key_id", idStr))

	w.Header().Set("HX-Redirect", "/api-keys")
	w.WriteHeader(http.StatusOK)
}

// ServeManageModal handles GET /api-keys/{id}/manage_modal - show manage modal.
func (h *Handler) ServeManageModal(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Short())
	defer cancel()

	idStr := chi.URLParam(r, "id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	store := apikeystore.New(h.DB)
	key, err := store.GetByID(ctx, id)
	if err != nil {
		if err == apikeystore.ErrNotFound {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		h.ErrLog.Log(r, "failed to load API key", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get return URL from query param
	backURL := r.URL.Query().Get("return")
	if backURL == "" {
		backURL = "/api-keys"
	}

	base := viewdata.NewBaseVM(r, h.DB, "Manage API Key", "/api-keys")
	data := APIKeyManageModalVM{
		BaseVM:  base,
		Key:     toAPIKeyVM(*key),
		BackURL: backURL,
	}
	templates.Render(w, r, "apikeys/manage_modal", data)
}

// toAPIKeyVM converts a store APIKey to a view model.
func toAPIKeyVM(k apikeystore.APIKey) APIKeyVM {
	vm := APIKeyVM{
		ID:          k.ID.Hex(),
		KeyPrefix:   k.KeyPrefix,
		Name:        k.Name,
		Description: k.Description,
		CreatedBy:   k.CreatedBy.Hex(),
		Status:      k.Status,
		UsageCount:  k.UsageCount,
		CreatedAt:   k.CreatedAt.Format("2006-01-02 15:04"),
		UpdatedAt:   k.UpdatedAt.Format("2006-01-02 15:04"),
		IsActive:    k.Status == apikeystore.StatusActive,
	}

	if k.LastUsedAt != nil {
		vm.LastUsedAt = k.LastUsedAt.Format("2006-01-02 15:04")
	}
	if k.RevokedAt != nil {
		vm.RevokedAt = k.RevokedAt.Format("2006-01-02 15:04")
	}

	// Convert scopes
	for _, s := range k.Scopes {
		vm.Scopes = append(vm.Scopes, ScopeVM{
			Resource: s.Resource,
			Actions:  s.Actions,
		})
	}

	return vm
}
