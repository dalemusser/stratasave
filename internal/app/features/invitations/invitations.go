// internal/app/features/invitations/invitations.go
package invitations

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"net/http"
	"net/mail"
	"strings"
	"time"

	errorsfeature "github.com/dalemusser/stratasave/internal/app/features/errors"
	"github.com/dalemusser/stratasave/internal/app/store/invitation"
	"github.com/dalemusser/stratasave/internal/app/store/sessions"
	settingsstore "github.com/dalemusser/stratasave/internal/app/store/settings"
	userstore "github.com/dalemusser/stratasave/internal/app/store/users"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/app/system/auditlog"
	"github.com/dalemusser/stratasave/internal/app/system/mailer"
	"github.com/dalemusser/stratasave/internal/app/system/network"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/stratasave/internal/domain/models"
	"github.com/dalemusser/waffle/pantry/templates"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Handler provides invitation handlers.
type Handler struct {
	invitationStore *invitation.Store
	userStore       *userstore.Store
	settingsStore   *settingsstore.Store
	sessionMgr      *auth.SessionManager
	sessionsStore   *sessions.Store
	errLog          *errorsfeature.ErrorLogger
	mailer          *mailer.Mailer
	auditLogger     *auditlog.Logger
	baseURL         string
	logger          *zap.Logger
}

// NewHandler creates a new invitations Handler.
func NewHandler(
	db *mongo.Database,
	sessionMgr *auth.SessionManager,
	sessionsStore *sessions.Store,
	errLog *errorsfeature.ErrorLogger,
	m *mailer.Mailer,
	auditLogger *auditlog.Logger,
	baseURL string,
	inviteExpiry time.Duration,
	logger *zap.Logger,
) *Handler {
	if inviteExpiry == 0 {
		inviteExpiry = 7 * 24 * time.Hour // 7 days default
	}

	return &Handler{
		invitationStore: invitation.New(db, inviteExpiry),
		userStore:       userstore.New(db),
		settingsStore:   settingsstore.New(db),
		sessionMgr:      sessionMgr,
		sessionsStore:   sessionsStore,
		errLog:          errLog,
		mailer:          m,
		auditLogger:     auditLogger,
		baseURL:         baseURL,
		logger:          logger,
	}
}

// invitationRow represents an invitation in the list.
type invitationRow struct {
	ID        string
	Email     string
	Role      string
	ExpiresAt time.Time
	Expired   bool
}

// ListVM is the view model for the invitations list.
type ListVM struct {
	viewdata.BaseVM
	Invitations []invitationRow
	Success     string
	Error       string
}

// AdminRoutes returns a chi.Router with admin invitation routes mounted.
func AdminRoutes(h *Handler, sessionMgr *auth.SessionManager) http.Handler {
	r := chi.NewRouter()
	r.Use(sessionMgr.RequireRole("admin"))

	r.Get("/", h.list)
	r.Get("/new", h.showNew)
	r.Post("/new", h.create)
	r.Get("/{id}/manage_modal", h.manageModal)
	r.Post("/{id}/revoke", h.revoke)
	r.Post("/{id}/resend", h.resend)

	return r
}

// AcceptRoutes returns a chi.Router with public invitation acceptance routes.
func AcceptRoutes(h *Handler) http.Handler {
	r := chi.NewRouter()

	r.Get("/", h.showAccept)
	r.Post("/", h.handleAccept)

	return r
}

// list displays pending invitations.
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	invitations, err := h.invitationStore.ListPending(r.Context())
	if err != nil {
		h.errLog.Log(r, "failed to list invitations", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	rows := make([]invitationRow, 0, len(invitations))
	now := time.Now()
	for _, inv := range invitations {
		rows = append(rows, invitationRow{
			ID:        inv.ID.Hex(),
			Email:     inv.Email,
			Role:      inv.Role,
			ExpiresAt: inv.ExpiresAt,
			Expired:   inv.ExpiresAt.Before(now),
		})
	}

	vm := ListVM{
		BaseVM:      viewdata.New(r),
		Invitations: rows,
	}
	vm.Title = "Invitations"

	if r.URL.Query().Get("success") == "1" {
		vm.Success = "Invitation sent successfully"
	}
	if r.URL.Query().Get("revoked") == "1" {
		vm.Success = "Invitation revoked"
	}
	if r.URL.Query().Get("resent") == "1" {
		vm.Success = "Invitation resent"
	}
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		vm.Error = errMsg
	}

	templates.Render(w, r, "invitations/list", vm)
}

// NewVM is the view model for creating a new invitation.
type NewVM struct {
	viewdata.BaseVM
	Email          string
	Role           string
	AvailableRoles []string
	Error          string
}

// ManageModalVM is the view model for the manage modal.
type ManageModalVM struct {
	ID        string
	Email     string
	Role      string
	Expired   bool
	BackURL   string
	CSRFToken string
}

// manageModal displays the manage modal for an invitation.
func (h *Handler) manageModal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inv, err := h.invitationStore.GetByID(r.Context(), objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	backURL := r.URL.Query().Get("return")
	if backURL == "" {
		backURL = "/invitations"
	}

	vm := ManageModalVM{
		ID:        id,
		Email:     inv.Email,
		Role:      inv.Role,
		Expired:   inv.ExpiresAt.Before(time.Now()),
		BackURL:   backURL,
		CSRFToken: csrf.Token(r),
	}

	templates.RenderSnippet(w, "invitations/manage_modal", vm)
}

// showNew displays the new invitation form.
func (h *Handler) showNew(w http.ResponseWriter, r *http.Request) {
	vm := NewVM{
		BaseVM:         viewdata.New(r),
		Role:           "admin", // Default role
		AvailableRoles: models.AllRoles(),
	}
	vm.Title = "Send Invitation"
	vm.BackURL = "/invitations"

	templates.Render(w, r, "invitations/new", vm)
}

// create sends a new invitation.
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.CurrentUser(r)

	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	role := r.FormValue("role")
	if role == "" || !models.IsValidRole(role) {
		role = "admin"
	}

	// Validate email
	if _, err := mail.ParseAddress(email); err != nil {
		vm := NewVM{
			BaseVM:         viewdata.New(r),
			Email:          email,
			Role:           role,
			AvailableRoles: models.AllRoles(),
			Error:          "Please enter a valid email address",
		}
		vm.BackURL = "/invitations"
		templates.Render(w, r, "invitations/new", vm)
		return
	}

	// Check if user already exists with this email or login_id
	existingUser, err := h.userStore.GetByEmail(r.Context(), email)
	if err != nil && err != mongo.ErrNoDocuments {
		h.errLog.Log(r, "failed to check existing email", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if existingUser == nil {
		existingUser, err = h.userStore.GetByLoginID(r.Context(), email)
		if err != nil && err != mongo.ErrNoDocuments {
			h.errLog.Log(r, "failed to check existing login_id", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
	if existingUser != nil {
		vm := NewVM{
			BaseVM:         viewdata.New(r),
			Email:          email,
			Role:           role,
			AvailableRoles: models.AllRoles(),
			Error:          "A user with this email already exists",
		}
		vm.BackURL = "/invitations"
		templates.Render(w, r, "invitations/new", vm)
		return
	}

	// Create invitation
	inv, err := h.invitationStore.Create(r.Context(), invitation.CreateInput{
		Email:     email,
		Role:      role,
		InvitedBy: actor.UserID(),
	})
	if err != nil {
		h.errLog.Log(r, "failed to create invitation", err)
		vm := NewVM{
			BaseVM:         viewdata.New(r),
			Email:          email,
			Role:           role,
			AvailableRoles: models.AllRoles(),
			Error:          "Failed to create invitation",
		}
		vm.BackURL = "/invitations"
		templates.Render(w, r, "invitations/new", vm)
		return
	}

	// Send invitation email
	if h.mailer != nil {
		inviteURL := h.baseURL + "/invite?token=" + inv.Token
		err = h.mailer.Send(mailer.Email{
			To:      email,
			Subject: "You're Invited!",
			TextBody: "You've been invited to join our platform.\n\n" +
				"Click the link below to set up your account:\n\n" +
				inviteURL + "\n\n" +
				"This invitation expires in 7 days.\n\n" +
				"If you did not expect this invitation, you can safely ignore this email.",
		})
		if err != nil {
			h.errLog.Log(r, "failed to send invitation email", err)
		}
	}

	actorID := actor.UserID()
	h.auditLogger.LogAdminEvent(r, &actorID, nil, "invitation_sent", map[string]string{
		"email": email,
		"role":  role,
	})

	http.Redirect(w, r, "/invitations?success=1", http.StatusSeeOther)
}

// revoke revokes an invitation.
func (h *Handler) revoke(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.CurrentUser(r)

	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inv, err := h.invitationStore.GetByID(r.Context(), objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := h.invitationStore.Revoke(r.Context(), objID); err != nil {
		h.errLog.Log(r, "failed to revoke invitation", err)
		http.Redirect(w, r, "/invitations?error=Failed+to+revoke+invitation", http.StatusSeeOther)
		return
	}

	actorID := actor.UserID()
	h.auditLogger.LogAdminEvent(r, &actorID, nil, "invitation_revoked", map[string]string{
		"email": inv.Email,
	})

	http.Redirect(w, r, "/invitations?revoked=1", http.StatusSeeOther)
}

// resend resends an invitation.
func (h *Handler) resend(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.CurrentUser(r)

	id := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inv, err := h.invitationStore.GetByID(r.Context(), objID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Check if already used or revoked
	if inv.UsedAt != nil || inv.Revoked {
		http.Redirect(w, r, "/invitations?error=Invitation+is+no+longer+valid", http.StatusSeeOther)
		return
	}

	// Revoke old invitation and create new one
	h.invitationStore.Revoke(r.Context(), objID)

	newInv, err := h.invitationStore.Create(r.Context(), invitation.CreateInput{
		Email:     inv.Email,
		Role:      inv.Role,
		InvitedBy: actor.UserID(),
	})
	if err != nil {
		h.errLog.Log(r, "failed to resend invitation", err)
		http.Redirect(w, r, "/invitations?error=Failed+to+resend+invitation", http.StatusSeeOther)
		return
	}

	// Send invitation email
	if h.mailer != nil {
		inviteURL := h.baseURL + "/invite?token=" + newInv.Token
		err = h.mailer.Send(mailer.Email{
			To:      inv.Email,
			Subject: "You're Invited!",
			TextBody: "You've been invited to join our platform.\n\n" +
				"Click the link below to set up your account:\n\n" +
				inviteURL + "\n\n" +
				"This invitation expires in 7 days.\n\n" +
				"If you did not expect this invitation, you can safely ignore this email.",
		})
		if err != nil {
			h.errLog.Log(r, "failed to send invitation email", err)
		}
	}

	actorID := actor.UserID()
	h.auditLogger.LogAdminEvent(r, &actorID, nil, "invitation_resent", map[string]string{
		"email": inv.Email,
	})

	http.Redirect(w, r, "/invitations?resent=1", http.StatusSeeOther)
}

// AcceptVM is the view model for accepting an invitation.
type AcceptVM struct {
	viewdata.BaseVM
	Token    string
	Email    string
	FullName string
	Error    string
}

// showAccept displays the accept invitation form.
func (h *Handler) showAccept(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")

	// Check if someone is already logged in
	currentUser, isLoggedIn := auth.CurrentUser(r)

	// Verify token is valid
	inv, err := h.invitationStore.VerifyToken(r.Context(), token)
	if err != nil {
		// Log invalid token attempt
		h.auditLogger.LogAuthEvent(r, nil, "invitation_invalid_token", false, "")

		vm := AcceptVM{
			BaseVM: viewdata.New(r),
			Error:  "This invitation link is invalid or has expired. Please contact an administrator for a new invitation.",
		}
		vm.Title = "Invalid Invitation"
		templates.Render(w, r, "invitations/accept", vm)
		return
	}

	// Check if user already exists with this email or login_id
	existingUser, err := h.userStore.GetByEmail(r.Context(), inv.Email)
	if err != nil && err != mongo.ErrNoDocuments {
		h.errLog.Log(r, "failed to check existing email", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if existingUser == nil {
		existingUser, err = h.userStore.GetByLoginID(r.Context(), inv.Email)
		if err != nil && err != mongo.ErrNoDocuments {
			h.errLog.Log(r, "failed to check existing login_id", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	// Handle case where someone is already logged in
	if isLoggedIn {
		// Check if logged in as the invited user
		if existingUser != nil && currentUser.ID == existingUser.ID.Hex() {
			// Already logged in as the right account - mark invitation used and go to dashboard
			h.invitationStore.MarkUsed(r.Context(), inv.ID)
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}

		// Logged in as a different user - log them out first
		// The person clicking this link has proven email access, so we trust them
		h.auditLogger.LogAuthEvent(r, nil, "session_cleared_for_invitation", true, inv.Email)
		h.sessionMgr.DestroySession(w, r)
	}

	if existingUser != nil {
		// Mark invitation as used since the account exists
		h.invitationStore.MarkUsed(r.Context(), inv.ID)

		// Log that invitation was blocked due to existing account
		h.auditLogger.LogAuthEvent(r, &existingUser.ID, "invitation_blocked_account_exists", true, inv.Email)

		vm := AcceptVM{
			BaseVM: viewdata.New(r),
			Error:  "An account with this email already exists. Please log in instead.",
		}
		vm.Title = "Account Already Exists"
		templates.Render(w, r, "invitations/accept", vm)
		return
	}

	vm := AcceptVM{
		BaseVM: viewdata.New(r),
		Token:  token,
		Email:  inv.Email,
	}
	vm.Title = "Complete Your Registration"

	templates.Render(w, r, "invitations/accept", vm)
}

// handleAccept processes the invitation acceptance.
// Creates the user account with email auth and logs them in immediately.
func (h *Handler) handleAccept(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	fullName := strings.TrimSpace(r.FormValue("full_name"))

	// Verify token
	inv, err := h.invitationStore.VerifyToken(r.Context(), token)
	if err != nil {
		h.auditLogger.LogAuthEvent(r, nil, "invitation_invalid_token", false, "")

		vm := AcceptVM{
			BaseVM: viewdata.New(r),
			Error:  "This invitation link is invalid or has expired. Please contact an administrator for a new invitation.",
		}
		vm.Title = "Invalid Invitation"
		templates.Render(w, r, "invitations/accept", vm)
		return
	}

	// Validate inputs
	if fullName == "" {
		vm := AcceptVM{
			BaseVM:   viewdata.New(r),
			Token:    token,
			Email:    inv.Email,
			FullName: fullName,
			Error:    "Full name is required",
		}
		vm.Title = "Complete Your Registration"
		templates.Render(w, r, "invitations/accept", vm)
		return
	}

	// Create user with email authentication
	// Using direct create instead of check-then-create to avoid race conditions.
	// MongoDB's unique index will prevent duplicates atomically.
	user, err := h.userStore.CreateFromInput(r.Context(), userstore.CreateInput{
		FullName:   fullName,
		Email:      inv.Email,
		AuthMethod: "email",
		Role:       inv.Role,
	})
	if err != nil {
		// Handle duplicate user (race-safe check)
		if err == userstore.ErrDuplicateLoginID {
			// Mark invitation as used since the account exists
			h.invitationStore.MarkUsed(r.Context(), inv.ID)

			h.auditLogger.LogAuthEvent(r, nil, "invitation_blocked_account_exists", true, inv.Email)

			vm := AcceptVM{
				BaseVM: viewdata.New(r),
				Error:  "An account with this email already exists. Please log in instead.",
			}
			vm.Title = "Account Already Exists"
			templates.Render(w, r, "invitations/accept", vm)
			return
		}

		h.errLog.Log(r, "failed to create user", err)
		vm := AcceptVM{
			BaseVM:   viewdata.New(r),
			Token:    token,
			Email:    inv.Email,
			FullName: fullName,
			Error:    "Failed to create account. Please try again.",
		}
		vm.Title = "Complete Your Registration"
		templates.Render(w, r, "invitations/accept", vm)
		return
	}

	// Mark invitation as used
	h.invitationStore.MarkUsed(r.Context(), inv.ID)

	h.auditLogger.LogAuthEvent(r, &user.ID, "user_registered_via_invitation", true, inv.Email)

	// Send welcome email if enabled
	if h.mailer != nil {
		settings, _ := h.settingsStore.Get(r.Context())
		if settings != nil && settings.NotifyUserOnWelcome {
			userEmail := inv.Email
			userName := fullName
			userRole := inv.Role
			siteName := settings.SiteName
			if siteName == "" {
				siteName = "Strata"
			}
			go func() {
				text, html := mailer.WelcomeEmail(mailer.WelcomeEmailData{
					AppName:  siteName,
					UserName: userName,
					LoginURL: h.baseURL + "/login",
					Role:     userRole,
				})
				_ = h.mailer.Send(mailer.Email{
					To:       userEmail,
					Subject:  "Welcome to " + siteName + "!",
					TextBody: text,
					HTMLBody: html,
				})
			}()
		}
	}

	// Log the user in immediately - they proved email ownership by clicking the invitation link
	if err := h.createTrackedSession(w, r, user.ID, user.Role); err != nil {
		h.errLog.Log(r, "failed to create session after registration", err)
		// Account was created, but session failed - redirect to login
		http.Redirect(w, r, "/login?registered=1", http.StatusSeeOther)
		return
	}

	h.auditLogger.LogAuthEvent(r, &user.ID, "login_success", true, "")

	// Redirect to dashboard
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// createTrackedSession creates a session in both the cookie and MongoDB for tracking.
func (h *Handler) createTrackedSession(w http.ResponseWriter, r *http.Request, userID primitive.ObjectID, role string) error {
	// Generate token first so we can use it for both cookie and MongoDB tracking
	token, err := auth.GenerateSessionToken()
	if err != nil {
		return err
	}

	// Create the cookie session with the generated token
	if err := h.sessionMgr.CreateSession(w, r, userID, role, token); err != nil {
		return err
	}

	// Store session in MongoDB for tracking
	now := time.Now()
	session := sessions.Session{
		Token:        token,
		UserID:       userID,
		IPAddress:    network.GetClientIP(r),
		UserAgent:    r.UserAgent(),
		LoginAt:      now,
		LastActivity: now,
		ExpiresAt:    now.Add(24 * 30 * time.Hour), // 30 days
	}

	// Best effort - don't fail login if tracking fails
	if err := h.sessionsStore.Create(r.Context(), session); err != nil {
		h.logger.Warn("failed to track session in MongoDB", zap.Error(err))
	}

	return nil
}
