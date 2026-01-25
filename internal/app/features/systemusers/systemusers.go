// internal/app/features/systemusers/systemusers.go
package systemusers

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"

	errorsfeature "github.com/dalemusser/stratasave/internal/app/features/errors"
	settingsstore "github.com/dalemusser/stratasave/internal/app/store/settings"
	userstore "github.com/dalemusser/stratasave/internal/app/store/users"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/app/system/auditlog"
	"github.com/dalemusser/stratasave/internal/app/system/authutil"
	"github.com/dalemusser/stratasave/internal/app/system/mailer"
	"github.com/dalemusser/stratasave/internal/app/system/normalize"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/stratasave/internal/domain/models"
	"github.com/dalemusser/waffle/pantry/templates"
	"github.com/dalemusser/waffle/pantry/text"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

const pageSize = 20

// Handler provides system users management handlers.
type Handler struct {
	userStore     *userstore.Store
	settingsStore *settingsstore.Store
	mailer        *mailer.Mailer
	errLog        *errorsfeature.ErrorLogger
	auditLogger   *auditlog.Logger
	logger        *zap.Logger
}

// NewHandler creates a new system users Handler.
func NewHandler(
	db *mongo.Database,
	m *mailer.Mailer,
	errLog *errorsfeature.ErrorLogger,
	auditLogger *auditlog.Logger,
	logger *zap.Logger,
) *Handler {
	return &Handler{
		userStore:     userstore.New(db),
		settingsStore: settingsstore.New(db),
		mailer:        m,
		errLog:        errLog,
		auditLogger:   auditLogger,
		logger:        logger,
	}
}

// userRow represents a user in the list.
type userRow struct {
	ID       primitive.ObjectID
	FullName string
	LoginID  string
	Role     string
	Auth     string
	Status   string
}

// ListVM is the view model for the users list.
type ListVM struct {
	viewdata.BaseVM

	// Filter state
	SearchQuery    string
	Status         string   // "", active, disabled
	RoleFilter     string   // "", admin, developer (renamed to avoid shadowing BaseVM.Role)
	AvailableRoles []string // for dropdown

	// Pagination
	Page       int
	PrevPage   int
	NextPage   int
	Total      int64
	TotalPages int
	RangeStart int
	RangeEnd   int
	HasPrev    bool
	HasNext    bool

	// Data
	Rows []userRow

	Flash template.HTML
}

// Routes returns a chi.Router with system users routes mounted.
func Routes(h *Handler, sessionMgr *auth.SessionManager) http.Handler {
	r := chi.NewRouter()
	r.Use(sessionMgr.RequireRole("admin"))

	r.Get("/", h.list)
	r.Get("/new", h.showNew)
	r.Post("/new", h.create)
	r.Get("/{id}", h.show)
	r.Get("/{id}/edit", h.showEdit)
	r.Post("/{id}", h.update)
	r.Post("/{id}/disable", h.disable)
	r.Post("/{id}/enable", h.enable)
	r.Post("/{id}/reset-password", h.resetPassword)
	r.Post("/{id}/delete", h.delete)

	// Manage modal for HTMX
	r.Get("/{id}/manage_modal", h.manageModal)

	return r
}

// list displays all system users with search, filters, and pagination.
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Parse filters
	searchQ := strings.TrimSpace(q.Get("search"))
	status := normalize.Status(q.Get("status"))
	role := normalize.Role(q.Get("role"))

	// Parse page number
	page := 1
	if pageStr := q.Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	// Build filter - show all system users (admin and developer roles)
	filter := bson.M{"role": bson.M{"$in": models.AllRoles()}}

	// Apply role filter if specified
	if role != "" && models.IsValidRole(role) {
		filter["role"] = role
	}

	if status == "active" || status == "disabled" {
		filter["status"] = status
	}

	// Search by name
	if searchQ != "" {
		qFold := text.Fold(searchQ)
		hiFold := qFold + "\uffff"
		filter["full_name_ci"] = bson.M{"$gte": qFold, "$lt": hiFold}
	}

	// Count total
	total, err := h.userStore.Count(r.Context(), filter)
	if err != nil {
		h.errLog.Log(r, "failed to count users", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Calculate pagination
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	offset := (page - 1) * pageSize

	// Fetch users
	findOpts := options.Find().
		SetSort(bson.D{{Key: "full_name_ci", Value: 1}, {Key: "_id", Value: 1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(pageSize))

	users, err := h.userStore.Find(r.Context(), filter, findOpts)
	if err != nil {
		h.errLog.Log(r, "failed to list users", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Build rows
	rows := make([]userRow, 0, len(users))
	for _, u := range users {
		loginID := ""
		if u.LoginID != nil {
			loginID = *u.LoginID
		}
		rows = append(rows, userRow{
			ID:       u.ID,
			FullName: u.FullName,
			LoginID:  loginID,
			Role:     normalize.Role(u.Role),
			Auth:     formatAuthMethod(u.AuthMethod),
			Status:   normalize.Status(u.Status),
		})
	}

	// Calculate range
	rangeStart := offset + 1
	rangeEnd := offset + len(rows)
	if total == 0 {
		rangeStart = 0
		rangeEnd = 0
	}

	vm := ListVM{
		BaseVM:         viewdata.New(r),
		SearchQuery:    searchQ,
		Status:         status,
		RoleFilter:     role,
		AvailableRoles: models.AllRoles(),
		Page:           page,
		PrevPage:       page - 1,
		NextPage:       page + 1,
		Total:          total,
		TotalPages:     totalPages,
		RangeStart:     rangeStart,
		RangeEnd:       rangeEnd,
		HasPrev:        page > 1,
		HasNext:        page < totalPages,
		Rows:           rows,
	}
	vm.Title = "System Users"

	templates.RenderAutoMap(w, r, "systemusers/list", nil, vm)
}

// ManageModalVM is the view model for the manage modal.
type ManageModalVM struct {
	ID        string
	FullName  string
	LoginID   string
	Role      string
	Auth      string
	Status    string
	BackURL   string
	CSRFToken string
	IsSelf    bool
}

// manageModal renders the manage user modal.
func (h *Handler) manageModal(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.CurrentUser(r)

	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	user, err := h.userStore.GetByID(r.Context(), objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	loginID := ""
	if user.LoginID != nil {
		loginID = *user.LoginID
	}

	vm := ManageModalVM{
		ID:        id,
		FullName:  user.FullName,
		LoginID:   loginID,
		Role:      normalize.Role(user.Role),
		Auth:      formatAuthMethod(user.AuthMethod),
		Status:    normalize.Status(user.Status),
		BackURL:   r.URL.Query().Get("return"),
		CSRFToken: csrf.Token(r),
		IsSelf:    actor.UserID() == objID,
	}

	templates.RenderSnippet(w, "systemusers/manage_modal", vm)
}

// NewUserVM is the view model for creating a new user.
type NewUserVM struct {
	viewdata.BaseVM
	FullName       string
	LoginID        string
	Email          string
	AuthMethod     string
	SelectedRole   string
	AvailableRoles []string
	Error          string
}

// showNew displays the new user form.
func (h *Handler) showNew(w http.ResponseWriter, r *http.Request) {
	vm := NewUserVM{
		BaseVM:         viewdata.New(r),
		AuthMethod:     "trust",
		SelectedRole:   "admin",
		AvailableRoles: models.AllRoles(),
	}
	vm.Title = "New User"
	vm.BackURL = r.URL.Query().Get("return")
	if vm.BackURL == "" {
		vm.BackURL = "/system-users"
	}

	templates.Render(w, r, "systemusers/new", vm)
}

// create creates a new system user.
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.CurrentUser(r)

	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	returnURL := r.FormValue("return")
	if returnURL == "" {
		returnURL = "/system-users"
	}

	authMethod := r.FormValue("auth_method")
	loginID := r.FormValue("login_id")
	email := r.FormValue("email")
	role := r.FormValue("role")

	// Validate role
	if !models.IsValidRole(role) {
		role = "admin" // Default to admin if invalid
	}

	// For email/google auth, email is the login ID
	if authMethod == "email" || authMethod == "google" {
		loginID = email
	}

	input := userstore.CreateInput{
		FullName:   r.FormValue("full_name"),
		LoginID:    loginID,
		Email:      email,
		AuthMethod: authMethod,
		Role:       role,
	}

	// Handle password for password auth
	if input.AuthMethod == "password" {
		password := r.FormValue("temp_password")
		if password == "" {
			vm := NewUserVM{
				BaseVM:         viewdata.New(r),
				FullName:       input.FullName,
				LoginID:        r.FormValue("login_id"),
				Email:          email,
				AuthMethod:     input.AuthMethod,
				SelectedRole:   role,
				AvailableRoles: models.AllRoles(),
				Error:          "Password is required for password authentication",
			}
			vm.BackURL = returnURL
			templates.Render(w, r, "systemusers/new", vm)
			return
		}

		hash, err := authutil.HashPassword(password)
		if err != nil {
			h.errLog.Log(r, "failed to hash password", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		input.PasswordHash = &hash

		// Always mark password as temporary for new users
		temp := true
		input.PasswordTemp = &temp
	}

	user, err := h.userStore.CreateFromInput(r.Context(), input)
	if err != nil {
		h.errLog.Log(r, "failed to create user", err)

		vm := NewUserVM{
			BaseVM:         viewdata.New(r),
			FullName:       input.FullName,
			LoginID:        input.LoginID,
			Email:          input.Email,
			AuthMethod:     input.AuthMethod,
			SelectedRole:   role,
			AvailableRoles: models.AllRoles(),
			Error:          "Failed to create user. Login ID is already in use.",
		}
		vm.BackURL = returnURL
		templates.Render(w, r, "systemusers/new", vm)
		return
	}

	actorID := actor.UserID()
	h.auditLogger.LogAdminEvent(r, &actorID, &user.ID, "user_created", nil)

	// Send welcome email if enabled and user has email
	if h.mailer != nil && user.Email != nil && *user.Email != "" {
		settings, _ := h.settingsStore.Get(r.Context())
		if settings != nil && settings.NotifyUserOnCreate {
			userEmail := *user.Email
			userName := user.FullName
			siteName := settings.SiteName
			if siteName == "" {
				siteName = "Strata"
			}
			go func() {
				text, html := mailer.WelcomeEmail(mailer.WelcomeEmailData{
					AppName:  siteName,
					UserName: userName,
					LoginURL: "/login",
					Role:     user.Role,
				})
				_ = h.mailer.Send(mailer.Email{
					To:       userEmail,
					Subject:  "Welcome to " + siteName,
					TextBody: text,
					HTMLBody: html,
				})
			}()
		}
	}

	http.Redirect(w, r, returnURL, http.StatusSeeOther)
}

// ShowVM is the view model for viewing a user.
type ShowVM struct {
	viewdata.BaseVM
	ID       string
	FullName string
	LoginID  string
	Email    string
	UserRole string // renamed to avoid shadowing BaseVM.Role
	Auth     string
	Status   string
}

// show displays a single user.
func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	user, err := h.userStore.GetByID(r.Context(), objID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.NotFound(w, r)
			return
		}
		h.errLog.Log(r, "failed to get user", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	loginID := ""
	if user.LoginID != nil {
		loginID = *user.LoginID
	}
	email := ""
	if user.Email != nil {
		email = *user.Email
	}

	vm := ShowVM{
		BaseVM:   viewdata.New(r),
		ID:       id,
		FullName: user.FullName,
		LoginID:  loginID,
		Email:    email,
		UserRole: normalize.Role(user.Role),
		Auth:     formatAuthMethod(user.AuthMethod),
		Status:   normalize.Status(user.Status),
	}
	vm.Title = user.FullName
	vm.BackURL = r.URL.Query().Get("return")
	if vm.BackURL == "" {
		vm.BackURL = "/system-users"
	}

	templates.Render(w, r, "systemusers/show", vm)
}

// EditVM is the view model for editing a user.
type EditVM struct {
	viewdata.BaseVM
	ID             string
	FullName       string
	LoginID        string
	Email          string
	Auth           string // auth method
	SelectedRole   string
	AvailableRoles []string
	Status         string
	IsSelf         bool   // true if editing own account
	IsEdit         bool   // always true for edit (for template auth field logic)
	Success        string
	Error          string
}

// showEdit displays the edit user form.
func (h *Handler) showEdit(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.CurrentUser(r)

	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	user, err := h.userStore.GetByID(r.Context(), objID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.NotFound(w, r)
			return
		}
		h.errLog.Log(r, "failed to get user", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	loginID := ""
	if user.LoginID != nil {
		loginID = *user.LoginID
	}
	email := ""
	if user.Email != nil {
		email = *user.Email
	}

	vm := EditVM{
		BaseVM:         viewdata.New(r),
		ID:             id,
		FullName:       user.FullName,
		LoginID:        loginID,
		Email:          email,
		Auth:           user.AuthMethod,
		SelectedRole:   user.Role,
		AvailableRoles: models.AllRoles(),
		Status:         normalize.Status(user.Status),
		IsSelf:         actor.UserID() == objID,
		IsEdit:         true,
	}
	vm.Title = "Edit " + user.FullName
	vm.BackURL = r.URL.Query().Get("return")
	if vm.BackURL == "" {
		vm.BackURL = "/system-users"
	}

	if r.URL.Query().Get("success") == "1" {
		vm.Success = "User updated successfully"
	}
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		switch errMsg {
		case "cannot_disable_self":
			vm.Error = "You cannot disable your own account"
		case "cannot_delete_self":
			vm.Error = "You cannot delete your own account"
		case "password_required":
			vm.Error = "Password is required"
		default:
			vm.Error = "An error occurred"
		}
	}

	templates.Render(w, r, "systemusers/edit", vm)
}

// update updates a user.
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.CurrentUser(r)

	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	returnURL := r.FormValue("return")
	if returnURL == "" {
		returnURL = "/system-users/" + id + "/edit"
	}

	isSelf := actor.UserID() == objID

	fullName := r.FormValue("full_name")
	authMethod := r.FormValue("auth_method")
	loginID := r.FormValue("login_id")
	email := r.FormValue("email")
	role := r.FormValue("role")
	tempPassword := r.FormValue("temp_password")
	status := r.FormValue("status")

	// Validate role
	if !models.IsValidRole(role) {
		role = "admin" // Default to admin if invalid
	}

	// For email/google auth, email is the login ID
	if authMethod == "email" || authMethod == "google" {
		loginID = email
	}

	update := userstore.UpdateInput{
		FullName:   &fullName,
		AuthMethod: &authMethod,
		LoginID:    &loginID,
		Role:       &role,
	}
	if email != "" {
		update.Email = &email
	}

	// Handle temp password for password auth
	if authMethod == "password" && tempPassword != "" {
		hash, err := authutil.HashPassword(tempPassword)
		if err != nil {
			h.errLog.Log(r, "failed to hash password", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		update.PasswordHash = &hash
		tempTrue := true
		update.PasswordTemp = &tempTrue
	}

	// Only update status if not editing self
	if !isSelf && status != "" {
		update.Status = &status
	}

	if err := h.userStore.UpdateFromInput(r.Context(), objID, update); err != nil {
		h.errLog.Log(r, "failed to update user", err)

		vm := EditVM{
			BaseVM:         viewdata.New(r),
			ID:             id,
			FullName:       fullName,
			LoginID:        loginID,
			Email:          email,
			Auth:           authMethod,
			SelectedRole:   role,
			AvailableRoles: models.AllRoles(),
			Status:         status,
			IsSelf:         isSelf,
			IsEdit:         true,
			Error:          "Failed to update user. Login ID is already in use.",
		}
		vm.BackURL = returnURL
		templates.Render(w, r, "systemusers/edit", vm)
		return
	}

	actorID := actor.UserID()
	h.auditLogger.LogAdminEvent(r, &actorID, &objID, "user_updated", nil)

	http.Redirect(w, r, "/system-users/"+id+"/edit?success=1&return="+returnURL, http.StatusSeeOther)
}

// disable disables a user account.
func (h *Handler) disable(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.CurrentUser(r)

	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	returnURL := "/system-users"
	if ret := r.FormValue("return"); ret != "" {
		returnURL = ret
	}

	// Prevent disabling self
	if actor.UserID() == objID {
		http.Redirect(w, r, "/system-users/"+id+"/edit?error=cannot_disable_self", http.StatusSeeOther)
		return
	}

	// Get user before update to get their email and name
	user, err := h.userStore.GetByID(r.Context(), objID)
	if err != nil {
		h.errLog.Log(r, "failed to get user for disable", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	status := "disabled"
	if err := h.userStore.UpdateFromInput(r.Context(), objID, userstore.UpdateInput{
		Status: &status,
	}); err != nil {
		h.errLog.Log(r, "failed to disable user", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	actorID := actor.UserID()
	h.auditLogger.LogAdminEvent(r, &actorID, &objID, "user_disabled", nil)

	// Send disabled notification email if enabled
	if h.mailer != nil && user.Email != nil && *user.Email != "" {
		settings, _ := h.settingsStore.Get(r.Context())
		if settings != nil && settings.NotifyUserOnDisable {
			userEmail := *user.Email
			userName := user.FullName
			siteName := settings.SiteName
			if siteName == "" {
				siteName = "Strata"
			}
			go func() {
				text, html := mailer.AccountDisabledEmail(mailer.AccountDisabledEmailData{
					AppName:  siteName,
					UserName: userName,
				})
				_ = h.mailer.Send(mailer.Email{
					To:       userEmail,
					Subject:  "Your " + siteName + " account has been disabled",
					TextBody: text,
					HTMLBody: html,
				})
			}()
		}
	}

	http.Redirect(w, r, returnURL, http.StatusSeeOther)
}

// enable enables a user account.
func (h *Handler) enable(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.CurrentUser(r)

	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	returnURL := "/system-users"
	if ret := r.FormValue("return"); ret != "" {
		returnURL = ret
	}

	// Get user before update to get their email and name
	user, err := h.userStore.GetByID(r.Context(), objID)
	if err != nil {
		h.errLog.Log(r, "failed to get user for enable", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	status := "active"
	if err := h.userStore.UpdateFromInput(r.Context(), objID, userstore.UpdateInput{
		Status: &status,
	}); err != nil {
		h.errLog.Log(r, "failed to enable user", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	actorID := actor.UserID()
	h.auditLogger.LogAdminEvent(r, &actorID, &objID, "user_enabled", nil)

	// Send enabled notification email if enabled
	if h.mailer != nil && user.Email != nil && *user.Email != "" {
		settings, _ := h.settingsStore.Get(r.Context())
		if settings != nil && settings.NotifyUserOnEnable {
			userEmail := *user.Email
			userName := user.FullName
			siteName := settings.SiteName
			if siteName == "" {
				siteName = "Strata"
			}
			go func() {
				text, html := mailer.AccountEnabledEmail(mailer.AccountEnabledEmailData{
					AppName:  siteName,
					UserName: userName,
					LoginURL: "/login",
				})
				_ = h.mailer.Send(mailer.Email{
					To:       userEmail,
					Subject:  "Your " + siteName + " account has been enabled",
					TextBody: text,
					HTMLBody: html,
				})
			}()
		}
	}

	http.Redirect(w, r, returnURL, http.StatusSeeOther)
}

// resetPassword resets a user's password to a temporary one.
func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.CurrentUser(r)

	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	newPassword := r.FormValue("new_password")
	if newPassword == "" {
		http.Redirect(w, r, "/system-users/"+id+"/edit?error=password_required", http.StatusSeeOther)
		return
	}

	hash, err := authutil.HashPassword(newPassword)
	if err != nil {
		h.errLog.Log(r, "failed to hash password", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	tempTrue := true
	if err := h.userStore.UpdateFromInput(r.Context(), objID, userstore.UpdateInput{
		PasswordHash: &hash,
		PasswordTemp: &tempTrue,
	}); err != nil {
		h.errLog.Log(r, "failed to reset password", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	actorID := actor.UserID()
	h.auditLogger.LogAdminEvent(r, &actorID, &objID, "password_reset", nil)

	http.Redirect(w, r, "/system-users/"+id+"/edit?success=1", http.StatusSeeOther)
}

// delete deletes a user.
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.CurrentUser(r)

	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	returnURL := "/system-users"
	if ret := r.FormValue("return"); ret != "" {
		returnURL = ret
	}

	// Prevent deleting self
	if actor.UserID() == objID {
		http.Redirect(w, r, "/system-users/"+id+"/edit?error=cannot_delete_self", http.StatusSeeOther)
		return
	}

	if _, err := h.userStore.Delete(r.Context(), objID); err != nil {
		h.errLog.Log(r, "failed to delete user", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	actorID := actor.UserID()
	h.auditLogger.LogAdminEvent(r, &actorID, &objID, "user_deleted", nil)

	http.Redirect(w, r, returnURL, http.StatusSeeOther)
}

// formatAuthMethod returns a display string for auth method.
func formatAuthMethod(method string) string {
	switch method {
	case "password":
		return "password"
	case "email":
		return "email"
	case "google":
		return "google"
	case "trust":
		return "trust"
	default:
		return method
	}
}
