// internal/app/features/login/login.go
package login

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"fmt"
	"net/http"
	"time"

	errorsfeature "github.com/dalemusser/stratasave/internal/app/features/errors"
	"github.com/dalemusser/stratasave/internal/app/store/activity"
	"github.com/dalemusser/stratasave/internal/app/store/emailverify"
	"github.com/dalemusser/stratasave/internal/app/store/passwordreset"
	"github.com/dalemusser/stratasave/internal/app/store/ratelimit"
	"github.com/dalemusser/stratasave/internal/app/store/sessions"
	userstore "github.com/dalemusser/stratasave/internal/app/store/users"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/app/system/auditlog"
	"github.com/dalemusser/stratasave/internal/app/system/authutil"
	"github.com/dalemusser/stratasave/internal/app/system/mailer"
	"github.com/dalemusser/stratasave/internal/app/system/network"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/waffle/pantry/query"
	"github.com/dalemusser/waffle/pantry/templates"
	"github.com/dalemusser/waffle/pantry/urlutil"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Handler provides login handlers.
type Handler struct {
	userStore          *userstore.Store
	emailVerifyStore   *emailverify.Store
	passwordResetStore *passwordreset.Store
	sessionsStore      *sessions.Store
	activityStore      *activity.Store
	rateLimitStore     *ratelimit.Store // nil if rate limiting disabled
	sessionMgr         *auth.SessionManager
	errLog             *errorsfeature.ErrorLogger
	mailer             *mailer.Mailer
	auditLogger        *auditlog.Logger
	baseURL            string
	emailVerifyExpiry  time.Duration
	googleEnabled      bool
	trustLoginEnabled  bool // Only enable in dev mode for security
	logger             *zap.Logger
}

// NewHandler creates a new login Handler.
// Set trustLoginEnabled to true only in development mode.
// rateLimitStore can be nil to disable rate limiting.
func NewHandler(
	db *mongo.Database,
	sessionMgr *auth.SessionManager,
	errLog *errorsfeature.ErrorLogger,
	m *mailer.Mailer,
	auditLogger *auditlog.Logger,
	sessionsStore *sessions.Store,
	activityStore *activity.Store,
	rateLimitStore *ratelimit.Store,
	baseURL string,
	emailVerifyExpiry time.Duration,
	googleEnabled bool,
	trustLoginEnabled bool,
	logger *zap.Logger,
) *Handler {
	// Use same expiry for password reset as email verification (default 10 minutes)
	passwordResetExpiry := emailVerifyExpiry
	if passwordResetExpiry == 0 {
		passwordResetExpiry = 10 * time.Minute
	}

	return &Handler{
		userStore:          userstore.New(db),
		emailVerifyStore:   emailverify.New(db, emailVerifyExpiry),
		passwordResetStore: passwordreset.New(db, passwordResetExpiry),
		sessionsStore:      sessionsStore,
		activityStore:      activityStore,
		rateLimitStore:     rateLimitStore,
		sessionMgr:         sessionMgr,
		errLog:             errLog,
		mailer:             m,
		auditLogger:        auditLogger,
		baseURL:            baseURL,
		emailVerifyExpiry:  emailVerifyExpiry,
		googleEnabled:      googleEnabled,
		trustLoginEnabled:  trustLoginEnabled,
		logger:             logger,
	}
}

// LoginVM is the view model for the login page.
type LoginVM struct {
	viewdata.BaseVM
	GoogleEnabled bool
	Error         string
	LoginID       string
	ReturnURL     string
}

// Routes returns a chi.Router with login routes mounted.
func Routes(h *Handler) http.Handler {
	r := chi.NewRouter()

	r.Get("/", h.showLogin)
	r.Post("/", h.handleLogin)

	// Trust auth - only enable in development mode for security
	// In production, these routes should not be accessible
	if h.trustLoginEnabled {
		r.Get("/trust", h.showTrustLogin)
		r.Post("/trust", h.handleTrustLogin)
	}

	// Password auth
	r.Get("/password", h.showPasswordLogin)
	r.Post("/password", h.handlePasswordLogin)

	// Password reset
	r.Get("/forgot-password", h.showForgotPassword)
	r.Post("/forgot-password", h.handleForgotPassword)
	r.Get("/reset-password", h.showResetPassword)
	r.Post("/reset-password", h.handleResetPassword)

	// Email verification auth
	r.Get("/email", h.showEmailLogin)
	r.Post("/email", h.handleEmailLogin)
	r.Get("/email/verify", h.showEmailVerify)
	r.Post("/email/verify", h.handleEmailVerify)
	r.Get("/email/magic", h.handleMagicLink)

	return r
}

// showLogin displays the login page with login_id field.
func (h *Handler) showLogin(w http.ResponseWriter, r *http.Request) {
	vm := LoginVM{
		BaseVM:        viewdata.New(r),
		GoogleEnabled: h.googleEnabled,
		ReturnURL:     query.Get(r, "return"),
		Error:         r.URL.Query().Get("error"),
	}
	vm.Title = "Login"

	templates.Render(w, r, "login/index", vm)
}

// handleLogin looks up the user by login_id and redirects to the appropriate auth method.
func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	loginID := r.FormValue("login_id")
	returnURL := r.FormValue("return")

	if loginID == "" {
		vm := LoginVM{
			BaseVM:        viewdata.New(r),
			GoogleEnabled: h.googleEnabled,
			Error:         "Please enter your Login ID",
			ReturnURL:     returnURL,
		}
		vm.Title = "Login"
		templates.Render(w, r, "login/index", vm)
		return
	}

	// Look up user by login_id
	user, err := h.userStore.GetByLoginID(r.Context(), loginID)
	if err != nil {
		// User not found - show error
		h.auditLogger.LoginFailedUserNotFound(r.Context(), r, loginID)
		vm := LoginVM{
			BaseVM:        viewdata.New(r),
			GoogleEnabled: h.googleEnabled,
			Error:         "User not found",
			LoginID:       loginID,
			ReturnURL:     returnURL,
		}
		vm.Title = "Login"
		templates.Render(w, r, "login/index", vm)
		return
	}

	if user.Status != "active" {
		h.auditLogger.LogAuthEvent(r, &user.ID, "login_failed_user_disabled", false, "user disabled")
		vm := LoginVM{
			BaseVM:        viewdata.New(r),
			GoogleEnabled: h.googleEnabled,
			Error:         "Account is disabled",
			LoginID:       loginID,
			ReturnURL:     returnURL,
		}
		vm.Title = "Login"
		templates.Render(w, r, "login/index", vm)
		return
	}

	// Redirect based on user's auth method
	returnParam := ""
	if returnURL != "" {
		returnParam = "?return=" + returnURL
	}

	switch user.AuthMethod {
	case "trust":
		// Trust auth - log in immediately
		if err := h.createTrackedSession(w, r, user.ID, user.Role); err != nil {
			h.errLog.Log(r, "failed to create session", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		h.auditLogger.LogAuthEvent(r, &user.ID, "login_success", true, "")
		http.Redirect(w, r, urlutil.SafeReturn(returnURL, "", "/dashboard"), http.StatusSeeOther)
	case "password":
		http.Redirect(w, r, "/login/password?login_id="+loginID+returnParam, http.StatusSeeOther)
	case "email":
		http.Redirect(w, r, "/login/email?login_id="+loginID+returnParam, http.StatusSeeOther)
	case "google":
		http.Redirect(w, r, "/auth/google"+returnParam, http.StatusSeeOther)
	default:
		// Default to password if auth_method is not set
		http.Redirect(w, r, "/login/password?login_id="+loginID+returnParam, http.StatusSeeOther)
	}
}

// TrustLoginVM is the view model for trust login.
type TrustLoginVM struct {
	viewdata.BaseVM
	Error   string
	LoginID string
}

// showTrustLogin displays the trust login form.
func (h *Handler) showTrustLogin(w http.ResponseWriter, r *http.Request) {
	vm := TrustLoginVM{
		BaseVM: viewdata.New(r),
	}
	vm.Title = "Trust Login"

	templates.Render(w, r, "login/trust", vm)
}

// handleTrustLogin processes trust login (development only).
func (h *Handler) handleTrustLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	loginID := r.FormValue("login_id")

	user, err := h.userStore.GetByLoginID(r.Context(), loginID)
	if err != nil {
		h.auditLogger.LoginFailedUserNotFound(r.Context(), r, loginID)

		vm := TrustLoginVM{
			BaseVM:  viewdata.New(r),
			Error:   "User not found",
			LoginID: loginID,
		}
		templates.Render(w, r, "login/trust", vm)
		return
	}

	if user.Status != "active" {
		h.auditLogger.LogAuthEvent(r, &user.ID, "login_failed_user_disabled", false, "user disabled")

		vm := TrustLoginVM{
			BaseVM:  viewdata.New(r),
			Error:   "Account is disabled",
			LoginID: loginID,
		}
		templates.Render(w, r, "login/trust", vm)
		return
	}

	// Create session
	if err := h.createTrackedSession(w, r, user.ID, user.Role); err != nil {
		h.errLog.Log(r, "failed to create session", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.auditLogger.LogAuthEvent(r, &user.ID, "login_success", true, "")

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// PasswordLoginVM is the view model for password login.
type PasswordLoginVM struct {
	viewdata.BaseVM
	Error     string
	LoginID   string
	ReturnURL string
}

// showPasswordLogin displays the password login form.
func (h *Handler) showPasswordLogin(w http.ResponseWriter, r *http.Request) {
	vm := PasswordLoginVM{
		BaseVM:    viewdata.New(r),
		LoginID:   r.URL.Query().Get("login_id"),
		ReturnURL: query.Get(r, "return"),
	}
	vm.Title = "Enter Password"

	templates.Render(w, r, "login/password", vm)
}

// handlePasswordLogin processes password login.
func (h *Handler) handlePasswordLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	loginID := r.FormValue("login_id")
	password := r.FormValue("password")
	returnURL := r.FormValue("return")

	// Check rate limit before processing
	if h.rateLimitStore != nil {
		allowed, _, lockedUntil := h.rateLimitStore.CheckAllowed(r.Context(), loginID)
		if !allowed {
			h.auditLogger.LogAuthEvent(r, nil, "login_rate_limited", false, "rate limit exceeded for "+loginID)

			errorMsg := "Too many failed login attempts. Please try again later."
			if lockedUntil != nil {
				remaining := time.Until(*lockedUntil)
				if remaining > time.Minute {
					errorMsg = fmt.Sprintf("Too many failed login attempts. Please try again in %d minute(s).", int(remaining.Minutes())+1)
				} else {
					errorMsg = fmt.Sprintf("Too many failed login attempts. Please try again in %d second(s).", int(remaining.Seconds())+1)
				}
			}

			vm := PasswordLoginVM{
				BaseVM:    viewdata.New(r),
				Error:     errorMsg,
				LoginID:   loginID,
				ReturnURL: returnURL,
			}
			templates.Render(w, r, "login/password", vm)
			return
		}
	}

	user, err := h.userStore.GetByLoginID(r.Context(), loginID)
	if err != nil {
		// Record failure for rate limiting (even though user doesn't exist)
		if h.rateLimitStore != nil {
			h.rateLimitStore.RecordFailure(r.Context(), loginID)
		}
		h.auditLogger.LoginFailedUserNotFound(r.Context(), r, loginID)

		vm := PasswordLoginVM{
			BaseVM:  viewdata.New(r),
			Error:   "Invalid credentials",
			LoginID: loginID,
		}
		templates.Render(w, r, "login/password", vm)
		return
	}

	if user.Status != "active" {
		// Record failure for rate limiting
		if h.rateLimitStore != nil {
			h.rateLimitStore.RecordFailure(r.Context(), loginID)
		}
		h.auditLogger.LogAuthEvent(r, &user.ID, "login_failed_user_disabled", false, "user disabled")

		vm := PasswordLoginVM{
			BaseVM:  viewdata.New(r),
			Error:   "Account is disabled",
			LoginID: loginID,
		}
		templates.Render(w, r, "login/password", vm)
		return
	}

	if user.PasswordHash == nil || !authutil.CheckPassword(password, *user.PasswordHash) {
		// Record failure for rate limiting
		if h.rateLimitStore != nil {
			lockedOut, lockedUntil := h.rateLimitStore.RecordFailure(r.Context(), loginID)
			if lockedOut {
				h.auditLogger.LogAuthEvent(r, &user.ID, "login_locked_out", false, "too many failed attempts")
				errorMsg := "Too many failed login attempts. Please try again later."
				if lockedUntil != nil {
					remaining := time.Until(*lockedUntil)
					if remaining > time.Minute {
						errorMsg = fmt.Sprintf("Too many failed login attempts. Please try again in %d minute(s).", int(remaining.Minutes())+1)
					} else {
						errorMsg = fmt.Sprintf("Too many failed login attempts. Please try again in %d second(s).", int(remaining.Seconds())+1)
					}
				}
				vm := PasswordLoginVM{
					BaseVM:    viewdata.New(r),
					Error:     errorMsg,
					LoginID:   loginID,
					ReturnURL: returnURL,
				}
				templates.Render(w, r, "login/password", vm)
				return
			}
		}
		h.auditLogger.LogAuthEvent(r, &user.ID, "login_failed_wrong_password", false, "wrong password")

		vm := PasswordLoginVM{
			BaseVM:  viewdata.New(r),
			Error:   "Invalid credentials",
			LoginID: loginID,
		}
		templates.Render(w, r, "login/password", vm)
		return
	}

	// Clear rate limit on successful login
	if h.rateLimitStore != nil {
		h.rateLimitStore.ClearOnSuccess(r.Context(), loginID)
	}

	// Create session
	if err := h.createTrackedSession(w, r, user.ID, user.Role); err != nil {
		h.errLog.Log(r, "failed to create session", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.auditLogger.LogAuthEvent(r, &user.ID, "login_success", true, "")

	// Check if password change is required
	if user.PasswordTemp != nil && *user.PasswordTemp {
		http.Redirect(w, r, "/profile/change-password?required=1", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, urlutil.SafeReturn(returnURL, "", "/dashboard"), http.StatusSeeOther)
}

// EmailLoginVM is the view model for email login.
type EmailLoginVM struct {
	viewdata.BaseVM
	Error string
	Email string
}

// showEmailLogin displays the email login form.
func (h *Handler) showEmailLogin(w http.ResponseWriter, r *http.Request) {
	vm := EmailLoginVM{
		BaseVM: viewdata.New(r),
	}
	vm.Title = "Email Login"

	templates.Render(w, r, "login/email", vm)
}

// handleEmailLogin sends a verification code to the email.
func (h *Handler) handleEmailLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")

	user, err := h.userStore.GetByEmail(r.Context(), email)
	if err != nil {
		// Don't reveal if email exists
		h.auditLogger.LogAuthEvent(r, nil, "verification_code_sent", true, "")
		http.Redirect(w, r, "/login/email/verify?email="+email, http.StatusSeeOther)
		return
	}

	if user.Status != "active" {
		h.auditLogger.LogAuthEvent(r, &user.ID, "login_failed_user_disabled", false, "user disabled")
		http.Redirect(w, r, "/login/email/verify?email="+email, http.StatusSeeOther)
		return
	}

	// Generate and send verification code
	verification, err := h.emailVerifyStore.Create(r.Context(), email, user.ID)
	if err != nil {
		h.errLog.Log(r, "failed to create verification", err)
		http.Redirect(w, r, "/login/email/verify?email="+email, http.StatusSeeOther)
		return
	}

	// Send email with code
	if h.mailer != nil {
		magicURL := h.baseURL + "/login/email/magic?token=" + verification.Token
		textBody, htmlBody := mailer.LoginCodeEmail(mailer.LoginCodeEmailData{
			AppName:  h.mailer.FromName(),
			Code:     verification.Code,
			MagicURL: magicURL,
		})
		err = h.mailer.Send(mailer.Email{
			To:       email,
			Subject:  "Your Login Code",
			TextBody: textBody,
			HTMLBody: htmlBody,
		})
		if err != nil {
			h.errLog.Log(r, "failed to send verification email", err)
		}
	}

	h.auditLogger.LogAuthEvent(r, &user.ID, "verification_code_sent", true, "")

	http.Redirect(w, r, "/login/email/verify?email="+email, http.StatusSeeOther)
}

// EmailVerifyVM is the view model for email verification.
type EmailVerifyVM struct {
	viewdata.BaseVM
	Error string
	Email string
}

// showEmailVerify displays the email verification form.
func (h *Handler) showEmailVerify(w http.ResponseWriter, r *http.Request) {
	vm := EmailVerifyVM{
		BaseVM: viewdata.New(r),
		Email:  r.URL.Query().Get("email"),
	}
	vm.Title = "Verify Email"

	templates.Render(w, r, "login/email_verify", vm)
}

// handleEmailVerify verifies the code and logs in.
func (h *Handler) handleEmailVerify(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	code := r.FormValue("code")

	verification, err := h.emailVerifyStore.VerifyCode(r.Context(), email, code)
	if err != nil {
		h.auditLogger.LogAuthEvent(r, nil, "verification_code_failed", false, "invalid code")

		vm := EmailVerifyVM{
			BaseVM: viewdata.New(r),
			Error:  "Invalid or expired code",
			Email:  email,
		}
		templates.Render(w, r, "login/email_verify", vm)
		return
	}

	user, err := h.userStore.GetByID(r.Context(), verification.UserID)
	if err != nil || user.Status != "active" {
		vm := EmailVerifyVM{
			BaseVM: viewdata.New(r),
			Error:  "Account not found or disabled",
			Email:  email,
		}
		templates.Render(w, r, "login/email_verify", vm)
		return
	}

	// Mark verification as used
	h.emailVerifyStore.MarkUsed(r.Context(), verification.ID)

	// Create session
	if err := h.createTrackedSession(w, r, user.ID, user.Role); err != nil {
		h.errLog.Log(r, "failed to create session", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.auditLogger.LogAuthEvent(r, &user.ID, "login_success", true, "")

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// handleMagicLink handles magic link login.
func (h *Handler) handleMagicLink(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")

	verification, err := h.emailVerifyStore.VerifyToken(r.Context(), token)
	if err != nil {
		h.auditLogger.LogAuthEvent(r, nil, "magic_link_failed", false, "invalid token")
		http.Redirect(w, r, "/login?error=invalid_token", http.StatusSeeOther)
		return
	}

	user, err := h.userStore.GetByID(r.Context(), verification.UserID)
	if err != nil || user.Status != "active" {
		http.Redirect(w, r, "/login?error=account_disabled", http.StatusSeeOther)
		return
	}

	// Mark verification as used
	h.emailVerifyStore.MarkUsed(r.Context(), verification.ID)

	// Create session
	if err := h.createTrackedSession(w, r, user.ID, user.Role); err != nil {
		h.errLog.Log(r, "failed to create session", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.auditLogger.LogAuthEvent(r, &user.ID, "magic_link_used", true, "")

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// ForgotPasswordVM is the view model for forgot password.
type ForgotPasswordVM struct {
	viewdata.BaseVM
	Error   string
	Success string
	LoginID string
}

// showForgotPassword displays the forgot password form.
func (h *Handler) showForgotPassword(w http.ResponseWriter, r *http.Request) {
	vm := ForgotPasswordVM{
		BaseVM: viewdata.New(r),
	}
	vm.Title = "Forgot Password"

	templates.Render(w, r, "login/forgot_password", vm)
}

// handleForgotPassword sends a password reset email.
func (h *Handler) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	loginID := r.FormValue("login_id")

	// Success message shown when we send a reset link
	successVM := ForgotPasswordVM{
		BaseVM:  viewdata.New(r),
		Success: "If your account has an email address on file, you will receive a password reset link.",
	}
	successVM.Title = "Forgot Password"

	if loginID == "" {
		vm := ForgotPasswordVM{
			BaseVM: viewdata.New(r),
			Error:  "Please enter your Login ID",
		}
		vm.Title = "Forgot Password"
		templates.Render(w, r, "login/forgot_password", vm)
		return
	}

	// Look up user by login_id
	user, err := h.userStore.GetByLoginID(r.Context(), loginID)
	if err != nil {
		// User not found - still show success to avoid enumeration
		h.auditLogger.LogAuthEvent(r, nil, "password_reset_requested", true, "user not found")
		templates.Render(w, r, "login/forgot_password", successVM)
		return
	}

	if user.Status != "active" {
		// Disabled user - still show success
		h.auditLogger.LogAuthEvent(r, &user.ID, "password_reset_requested", false, "user disabled")
		templates.Render(w, r, "login/forgot_password", successVM)
		return
	}

	// Only allow password reset for password auth users
	if user.AuthMethod != "password" && user.AuthMethod != "" {
		h.auditLogger.LogAuthEvent(r, &user.ID, "password_reset_requested", false, "not password auth")
		templates.Render(w, r, "login/forgot_password", successVM)
		return
	}

	// Check if user has an email address
	if user.Email == nil || *user.Email == "" {
		h.auditLogger.LogAuthEvent(r, &user.ID, "password_reset_requested", false, "no email address")
		vm := ForgotPasswordVM{
			BaseVM:  viewdata.New(r),
			LoginID: loginID,
			Error:   "Your account does not have an email address on file. Please contact an administrator to reset your password.",
		}
		vm.Title = "Forgot Password"
		templates.Render(w, r, "login/forgot_password", vm)
		return
	}

	// Create password reset token
	reset, err := h.passwordResetStore.Create(r.Context(), user.ID, *user.Email)
	if err != nil {
		h.errLog.Log(r, "failed to create password reset", err)
		templates.Render(w, r, "login/forgot_password", successVM)
		return
	}

	// Send email with reset link
	if h.mailer != nil {
		resetURL := h.baseURL + "/login/reset-password?token=" + reset.Token
		expiryMin := int(h.emailVerifyExpiry.Minutes())
		if expiryMin < 1 {
			expiryMin = 10 // default
		}
		textBody, htmlBody := mailer.PasswordResetEmail(mailer.PasswordResetEmailData{
			AppName:   h.mailer.FromName(),
			ResetURL:  resetURL,
			ExpiryMin: expiryMin,
		})
		err = h.mailer.Send(mailer.Email{
			To:       *user.Email,
			Subject:  "Password Reset Request",
			TextBody: textBody,
			HTMLBody: htmlBody,
		})
		if err != nil {
			h.errLog.Log(r, "failed to send password reset email", err)
		}
	}

	h.auditLogger.LogAuthEvent(r, &user.ID, "password_reset_requested", true, "")

	templates.Render(w, r, "login/forgot_password", successVM)
}

// ResetPasswordVM is the view model for reset password.
type ResetPasswordVM struct {
	viewdata.BaseVM
	Error   string
	Success string
	Token   string
}

// showResetPassword displays the reset password form.
func (h *Handler) showResetPassword(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")

	// Verify token is valid before showing form
	_, err := h.passwordResetStore.VerifyToken(r.Context(), token)
	if err != nil {
		vm := ResetPasswordVM{
			BaseVM: viewdata.New(r),
			Error:  "Invalid or expired reset link. Please request a new one.",
		}
		vm.Title = "Reset Password"
		templates.Render(w, r, "login/reset_password", vm)
		return
	}

	vm := ResetPasswordVM{
		BaseVM: viewdata.New(r),
		Token:  token,
	}
	vm.Title = "Reset Password"

	templates.Render(w, r, "login/reset_password", vm)
}

// handleResetPassword processes the password reset.
func (h *Handler) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.errLog.Log(r, "failed to parse form", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	// Verify token
	reset, err := h.passwordResetStore.VerifyToken(r.Context(), token)
	if err != nil {
		h.auditLogger.LogAuthEvent(r, nil, "password_reset_failed", false, "invalid token")
		vm := ResetPasswordVM{
			BaseVM: viewdata.New(r),
			Error:  "Invalid or expired reset link. Please request a new one.",
		}
		vm.Title = "Reset Password"
		templates.Render(w, r, "login/reset_password", vm)
		return
	}

	// Validate passwords
	if password == "" {
		vm := ResetPasswordVM{
			BaseVM: viewdata.New(r),
			Token:  token,
			Error:  "Password is required",
		}
		vm.Title = "Reset Password"
		templates.Render(w, r, "login/reset_password", vm)
		return
	}

	if len(password) < 8 {
		vm := ResetPasswordVM{
			BaseVM: viewdata.New(r),
			Token:  token,
			Error:  "Password must be at least 8 characters",
		}
		vm.Title = "Reset Password"
		templates.Render(w, r, "login/reset_password", vm)
		return
	}

	if password != confirmPassword {
		vm := ResetPasswordVM{
			BaseVM: viewdata.New(r),
			Token:  token,
			Error:  "Passwords do not match",
		}
		vm.Title = "Reset Password"
		templates.Render(w, r, "login/reset_password", vm)
		return
	}

	// Hash new password
	hash, err := authutil.HashPassword(password)
	if err != nil {
		h.errLog.Log(r, "failed to hash password", err)
		vm := ResetPasswordVM{
			BaseVM: viewdata.New(r),
			Token:  token,
			Error:  "Failed to reset password. Please try again.",
		}
		vm.Title = "Reset Password"
		templates.Render(w, r, "login/reset_password", vm)
		return
	}

	// Update user password
	if err := h.userStore.UpdatePassword(r.Context(), reset.UserID, hash); err != nil {
		h.errLog.Log(r, "failed to update password", err)
		vm := ResetPasswordVM{
			BaseVM: viewdata.New(r),
			Token:  token,
			Error:  "Failed to reset password. Please try again.",
		}
		vm.Title = "Reset Password"
		templates.Render(w, r, "login/reset_password", vm)
		return
	}

	// Mark reset token as used
	h.passwordResetStore.MarkUsed(r.Context(), reset.ID)

	h.auditLogger.LogAuthEvent(r, &reset.UserID, "password_reset_completed", true, "")

	// Send password changed confirmation email
	if h.mailer != nil {
		loginURL := h.baseURL + "/login"
		textBody, htmlBody := mailer.PasswordChangedEmail(mailer.PasswordChangedEmailData{
			AppName:  h.mailer.FromName(),
			LoginURL: loginURL,
		})
		err = h.mailer.Send(mailer.Email{
			To:       reset.Email,
			Subject:  "Your Password Has Been Changed",
			TextBody: textBody,
			HTMLBody: htmlBody,
		})
		if err != nil {
			h.errLog.Log(r, "failed to send password changed confirmation email", err)
		}
	}

	// Show success and redirect to login
	vm := ResetPasswordVM{
		BaseVM:  viewdata.New(r),
		Success: "Your password has been reset. You can now log in with your new password.",
	}
	vm.Title = "Reset Password"
	templates.Render(w, r, "login/reset_password", vm)
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
	// Note: Login time is captured in the session record (login_at), so we don't need
	// a separate activity event for login - it would be redundant.
	if err := h.sessionsStore.Create(r.Context(), session); err != nil {
		h.logger.Warn("failed to track session", zap.Error(err))
	}

	return nil
}
