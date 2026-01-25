// internal/app/features/authgoogle/authgoogle.go
package authgoogle

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	errorsfeature "github.com/dalemusser/stratasave/internal/app/features/errors"
	"github.com/dalemusser/stratasave/internal/app/store/oauthstate"
	"github.com/dalemusser/stratasave/internal/app/store/sessions"
	userstore "github.com/dalemusser/stratasave/internal/app/store/users"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/app/system/auditlog"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Handler provides Google OAuth handlers.
type Handler struct {
	userStore       *userstore.Store
	sessionMgr      *auth.SessionManager
	errLog          *errorsfeature.ErrorLogger
	auditLogger     *auditlog.Logger
	sessionsStore   *sessions.Store
	oauthStateStore *oauthstate.Store
	oauthConfig     *oauth2.Config
	logger          *zap.Logger
}

// NewHandler creates a new Google OAuth Handler.
func NewHandler(
	db *mongo.Database,
	sessionMgr *auth.SessionManager,
	errLog *errorsfeature.ErrorLogger,
	auditLogger *auditlog.Logger,
	sessionsStore *sessions.Store,
	oauthStateStore *oauthstate.Store,
	clientID string,
	clientSecret string,
	baseURL string,
	logger *zap.Logger,
) *Handler {
	return &Handler{
		userStore:       userstore.New(db),
		sessionMgr:      sessionMgr,
		errLog:          errLog,
		auditLogger:     auditLogger,
		sessionsStore:   sessionsStore,
		oauthStateStore: oauthStateStore,
		oauthConfig: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  baseURL + "/auth/google/callback",
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
		logger: logger,
	}
}

// Routes returns a chi.Router with Google OAuth routes mounted.
func Routes(h *Handler) http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.startAuth)
	r.Get("/callback", h.handleCallback)
	return r
}

// startAuth initiates the Google OAuth flow.
func (h *Handler) startAuth(w http.ResponseWriter, r *http.Request) {
	// Generate state token
	state, err := generateState()
	if err != nil {
		h.errLog.Log(r, "failed to generate state", err)
		http.Redirect(w, r, "/login?error=oauth_error", http.StatusSeeOther)
		return
	}

	// Store state in database
	if err := h.oauthStateStore.Create(r.Context(), state); err != nil {
		h.errLog.Log(r, "failed to store state", err)
		http.Redirect(w, r, "/login?error=oauth_error", http.StatusSeeOther)
		return
	}

	// Redirect to Google
	url := h.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// handleCallback processes the Google OAuth callback.
func (h *Handler) handleCallback(w http.ResponseWriter, r *http.Request) {
	// Verify state
	state := r.URL.Query().Get("state")
	if !h.oauthStateStore.Verify(r.Context(), state) {
		h.logger.Warn("invalid oauth state")
		http.Redirect(w, r, "/login?error=invalid_state", http.StatusSeeOther)
		return
	}

	// Check for error from Google
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		h.logger.Warn("oauth error from google", zap.String("error", errMsg))
		http.Redirect(w, r, "/login?error="+errMsg, http.StatusSeeOther)
		return
	}

	// Exchange code for token
	code := r.URL.Query().Get("code")
	token, err := h.oauthConfig.Exchange(r.Context(), code)
	if err != nil {
		h.errLog.Log(r, "failed to exchange code", err)
		http.Redirect(w, r, "/login?error=token_exchange_failed", http.StatusSeeOther)
		return
	}

	// Get user info from Google
	userInfo, err := h.getUserInfo(r.Context(), token)
	if err != nil {
		h.errLog.Log(r, "failed to get user info", err)
		http.Redirect(w, r, "/login?error=userinfo_failed", http.StatusSeeOther)
		return
	}

	// Find or create user
	user, err := h.userStore.GetByEmail(r.Context(), userInfo.Email)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// User doesn't exist - redirect to login with error
			// (Google auth requires existing user for security)
			h.auditLogger.LoginFailedUserNotFound(r.Context(), r, userInfo.Email)
			http.Redirect(w, r, "/login?error=user_not_found", http.StatusSeeOther)
			return
		}
		h.errLog.Log(r, "failed to get user by email", err)
		http.Redirect(w, r, "/login?error=database_error", http.StatusSeeOther)
		return
	}

	// Check if user is active
	if user.Status != "active" {
		h.auditLogger.LogAuthEvent(r, &user.ID, "login_failed_user_disabled", false, "user disabled")
		http.Redirect(w, r, "/login?error=account_disabled", http.StatusSeeOther)
		return
	}

	// Create session
	if err := h.createTrackedSession(w, r, user.ID, user.Role); err != nil {
		h.errLog.Log(r, "failed to create session", err)
		http.Redirect(w, r, "/login?error=session_error", http.StatusSeeOther)
		return
	}

	h.auditLogger.LogAuthEvent(r, &user.ID, "login_success", true, "")

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// GoogleUserInfo represents user info from Google.
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// getUserInfo fetches user info from Google.
func (h *Handler) getUserInfo(ctx context.Context, token *oauth2.Token) (*GoogleUserInfo, error) {
	client := h.oauthConfig.Client(ctx, token)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var userInfo GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

// generateState generates a random state token.
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
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
		IPAddress:    getClientIP(r),
		UserAgent:    r.UserAgent(),
		LoginAt:      now,
		LastActivity: now,
		ExpiresAt:    now.Add(24 * 30 * time.Hour), // 30 days
	}

	// Best effort - don't fail login if tracking fails
	if err := h.sessionsStore.Create(r.Context(), session); err != nil {
		h.logger.Warn("failed to track session", zap.Error(err))
	}

	return nil
}

// getClientIP extracts the client IP from the request.
func getClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}
