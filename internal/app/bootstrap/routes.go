// internal/app/bootstrap/routes.go
package bootstrap

import (
	"context"
	"net/http"
	"strconv"
	"time"

	activityfeature "github.com/dalemusser/stratasave/internal/app/features/activity"
	apistatsfeature "github.com/dalemusser/stratasave/internal/app/features/apistats"
	announcementsfeature "github.com/dalemusser/stratasave/internal/app/features/announcements"
	apikeysfeature "github.com/dalemusser/stratasave/internal/app/features/apikeys"
	saveapifeature "github.com/dalemusser/stratasave/internal/app/features/saveapi"
	savebrowserfeature "github.com/dalemusser/stratasave/internal/app/features/savebrowser"
	settingsapifeature "github.com/dalemusser/stratasave/internal/app/features/settingsapi"
	settingsbrowserfeature "github.com/dalemusser/stratasave/internal/app/features/settingsbrowser"
	auditlogfeature "github.com/dalemusser/stratasave/internal/app/features/auditlog"
	authgooglefeature "github.com/dalemusser/stratasave/internal/app/features/authgoogle"
	dashboardfeature "github.com/dalemusser/stratasave/internal/app/features/dashboard"
	errorsfeature "github.com/dalemusser/stratasave/internal/app/features/errors"
	filesfeature "github.com/dalemusser/stratasave/internal/app/features/files"
	healthfeature "github.com/dalemusser/stratasave/internal/app/features/health"
	heartbeatfeature "github.com/dalemusser/stratasave/internal/app/features/heartbeat"
	homefeature "github.com/dalemusser/stratasave/internal/app/features/home"
	invitationsfeature "github.com/dalemusser/stratasave/internal/app/features/invitations"
	jobsfeature "github.com/dalemusser/stratasave/internal/app/features/jobs"
	ledgerfeature "github.com/dalemusser/stratasave/internal/app/features/ledger"
	loginfeature "github.com/dalemusser/stratasave/internal/app/features/login"
	logoutfeature "github.com/dalemusser/stratasave/internal/app/features/logout"
	pagesfeature "github.com/dalemusser/stratasave/internal/app/features/pages"
	profilefeature "github.com/dalemusser/stratasave/internal/app/features/profile"
	settingsfeature "github.com/dalemusser/stratasave/internal/app/features/settings"
	statsfeature "github.com/dalemusser/stratasave/internal/app/features/stats"
	statusfeature "github.com/dalemusser/stratasave/internal/app/features/status"
	systemusersfeature "github.com/dalemusser/stratasave/internal/app/features/systemusers"
	appresources "github.com/dalemusser/stratasave/internal/app/resources"
	"github.com/dalemusser/stratasave/internal/app/store/activity"
	apistatsstore "github.com/dalemusser/stratasave/internal/app/store/apistats"
	ledgerstore "github.com/dalemusser/stratasave/internal/app/store/ledger"
	"github.com/dalemusser/stratasave/internal/app/system/apistats"
	"github.com/dalemusser/stratasave/internal/app/system/ledger"
	announcementstore "github.com/dalemusser/stratasave/internal/app/store/announcement"
	"github.com/dalemusser/stratasave/internal/app/store/audit"
	"github.com/dalemusser/stratasave/internal/app/store/oauthstate"
	"github.com/dalemusser/stratasave/internal/app/store/ratelimit"
	"github.com/dalemusser/stratasave/internal/app/store/sessions"
	userstore "github.com/dalemusser/stratasave/internal/app/store/users"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/app/system/auditlog"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/waffle/config"
	"github.com/dalemusser/waffle/middleware"
	"github.com/dalemusser/waffle/pantry/fileserver"
	"github.com/dalemusser/waffle/pantry/templates"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/csrf"
	"go.uber.org/zap"
)

// BuildHandler constructs the root HTTP handler (router) for this WAFFLE app.
//
// WAFFLE calls this after configuration, DB connections, schema setup, and
// any Startup hooks have completed. At this point you have access to:
//   - coreCfg: WAFFLE core configuration (ports, env, timeouts, etc.)
//   - appCfg: app-specific configuration defined in AppConfig
//   - deps: any DB or backend clients bundled in DBDeps
//   - logger: the fully configured zap.Logger for this app
//
// This function should:
//  1. Create a router (chi, standard mux, etc.)
//  2. Mount feature routers for different parts of your application
//  3. Add any additional middleware needed for specific routes
//  4. Return the configured router as an http.Handler
//
// # Mixed Authentication Routes
//
// For applications that need both session-based web UI and API key-based
// external API access, see docs/mixed_auth_routes.md for the recommended pattern.
//
// In summary:
//   - Web UI routes: session auth + CSRF + restrictive CORS
//   - API routes: API key auth + no CSRF + permissive CORS
//
// Strata provides helper packages for API routes:
//   - auth.APIKeyAuth: Bearer token authentication middleware
//   - apicors.Middleware: Permissive CORS for API endpoints
//   - jsonutil: JSON response helpers
func BuildHandler(coreCfg *config.CoreConfig, appCfg AppConfig, deps DBDeps, logger *zap.Logger) (http.Handler, error) {
	// Create the session manager using app config.
	// Secure cookies are enabled in production mode.
	secure := coreCfg.Env == "prod"
	sessionMgr, err := auth.NewSessionManager(appCfg.SessionKey, appCfg.SessionName, appCfg.SessionDomain, appCfg.SessionMaxAge, secure, logger)
	if err != nil {
		logger.Error("session manager init failed", zap.Error(err))
		return nil, err
	}

	// Set up the UserFetcher so LoadSessionUser fetches fresh user data on each request.
	// This ensures role changes, disabled accounts, and profile updates take effect immediately.
	sessionMgr.SetUserFetcher(userstore.NewFetcher(deps.MongoDatabase, logger))

	// Initialize and boot the template engine once at startup.
	// Dev mode enables template reloading for faster iteration.
	eng := templates.New(coreCfg.Env == "dev")
	if err := eng.Boot(logger); err != nil {
		logger.Error("template engine boot failed", zap.Error(err))
		return nil, err
	}
	templates.UseEngine(eng, logger)

	// Initialize viewdata with storage and database for settings loading.
	viewdata.Init(deps.FileStorage, deps.MongoDatabase)

	// Set up announcement loader for viewdata.
	// This allows BaseVM to include active announcements for banner display.
	annStore := announcementstore.New(deps.MongoDatabase)
	viewdata.SetAnnouncementLoader(func(ctx context.Context) []viewdata.AnnouncementVM {
		announcements, err := annStore.GetActive(ctx)
		if err != nil {
			logger.Warn("failed to load active announcements", zap.Error(err))
			return nil
		}
		result := make([]viewdata.AnnouncementVM, len(announcements))
		for i, ann := range announcements {
			result[i] = viewdata.AnnouncementVM{
				ID:          ann.ID.Hex(),
				Title:       ann.Title,
				Content:     ann.Content,
				Type:        string(ann.Type),
				Dismissible: ann.Dismissible,
			}
		}
		return result
	})

	// Create error logger for handlers.
	errLog := errorsfeature.NewErrorLogger(logger)

	// Create audit store and logger for security event tracking.
	auditStore := audit.New(deps.MongoDatabase)
	auditConfig := auditlog.Config{
		Auth:  appCfg.AuditLogAuth,
		Admin: appCfg.AuditLogAdmin,
	}
	auditLogger := auditlog.New(auditStore, logger, auditConfig)

	// Create sessions store for activity tracking.
	sessionsStore := sessions.New(deps.MongoDatabase)

	// Create activity store for logging user events.
	activityStore := activity.New(deps.MongoDatabase)

	// Create API stats store and recorder for tracking API request statistics.
	apiStatsStore := apistatsstore.New(deps.MongoDatabase)
	apiStatsRecorder := apistats.NewRecorder(apiStatsStore, logger, appCfg.APIStatsBucket)

	r := chi.NewRouter()

	// ─────────────────────────────────────────────────────────────────────────────
	// Global Middleware (applies to ALL routes)
	// ─────────────────────────────────────────────────────────────────────────────

	// Request timeout middleware: prevents requests from hanging indefinitely.
	r.Use(chimw.Timeout(30 * time.Second))

	// CORS middleware: must be early in the chain to handle preflight requests.
	r.Use(middleware.CORSFromConfig(coreCfg))

	// Security headers middleware: adds X-Frame-Options, X-Content-Type-Options, etc.
	r.Use(middleware.SecurityHeadersFromConfig(coreCfg))

	// Session middleware: loads SessionUser into context if logged in.
	// API routes will simply have no session, which is fine.
	r.Use(sessionMgr.LoadSessionUser)

	// CSRF protection middleware with path-based exemption for API routes.
	// Cookie name is "stratasave_csrf" to avoid collisions with other services
	// on the same domain (e.g., dev.adroit.games, log.adroit.games).
	csrfOpts := []csrf.Option{
		csrf.Secure(secure),
		csrf.Path("/"),
		csrf.CookieName("stratasave_csrf"),
		csrf.FieldName("csrf_token"),
		csrf.SameSite(csrf.SameSiteLaxMode),
		csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			logger.Warn("CSRF validation failed",
				zap.String("path", req.URL.Path),
				zap.String("method", req.Method),
				zap.String("reason", csrf.FailureReason(req).Error()),
			)
			if req.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusForbidden)
				return
			}
			http.Error(w, "CSRF token invalid or missing", http.StatusForbidden)
		})),
	}
	// In dev mode, trust localhost origins for CSRF validation.
	trustedOrigins := []string{
		"localhost:8080",
		"localhost:3000",
		"127.0.0.1:8080",
		"127.0.0.1:3000",
	}
	if !secure {
		csrfOpts = append(csrfOpts, csrf.TrustedOrigins(trustedOrigins))
	}
	if appCfg.SessionDomain != "" {
		csrfOpts = append(csrfOpts, csrf.Domain(appCfg.SessionDomain))
	}
	csrfProtect := csrf.Protect([]byte(appCfg.CSRFKey), csrfOpts...)

	// Wrap CSRF middleware to skip for API routes (they use API key auth or session auth with JS)
	csrfMiddleware := func(next http.Handler) http.Handler {
		csrfHandler := csrfProtect(next)
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			path := req.URL.Path
			// Skip CSRF for:
			// - Game API routes (use API key auth)
			// - Heartbeat API (internal JS calls with session auth)
			// - Invitation acceptance (the invitation token itself provides CSRF protection)
			switch path {
			case "/save", "/load", "/api/state/save", "/api/state/load", "/api/settings/save", "/api/settings/load", "/api/heartbeat", "/invite":
				next.ServeHTTP(w, req)
				return
			}
			csrfHandler.ServeHTTP(w, req)
		})
	}
	r.Use(csrfMiddleware)

	// ─────────────────────────────────────────────────────────────────────────────
	// Routes
	// ─────────────────────────────────────────────────────────────────────────────

	// ─────────────────────────────────────────────────────────────────────────────
	// API Error Ledger
	// Logs API errors (status >= 400) for debugging integration issues.
	// View errors at /ledger with filter for status >= 400.
	// ─────────────────────────────────────────────────────────────────────────────
	apiLedgerStore := ledgerstore.New(deps.MongoDatabase)
	apiLedgerConfig := ledger.Config{
		Store:          apiLedgerStore,
		Logger:         logger,
		MaxBodyPreview: 500,
		HeadersToCapture: []string{
			"Content-Type",
			"Accept",
			"User-Agent",
			"X-Request-ID",
		},
		CaptureErrors: true,
		OnlyErrors:    true, // Only log requests that result in errors (status >= 400)
	}

	// ─────────────────────────────────────────────────────────────────────────────
	// Game State API Routes
	// These routes use API key authentication. CSRF is handled above via path exemption.
	// API errors are logged to the ledger for debugging.
	// ─────────────────────────────────────────────────────────────────────────────
	saveapiHandler := saveapifeature.NewHandler(deps.MongoDatabase, logger, appCfg.MaxSavesPerUser)

	// New API endpoints: POST /api/state/save and POST /api/state/load
	r.Route("/api/state", func(r chi.Router) {
		r.Use(ledger.Middleware(apiLedgerConfig))
		r.Mount("/", saveapifeature.Routes(saveapiHandler, apiStatsRecorder, appCfg.APIKey, logger))
	})

	// Legacy endpoints for backward compatibility: POST /save and POST /load
	r.Route("/save", func(r chi.Router) {
		r.Use(ledger.Middleware(apiLedgerConfig))
		r.Mount("/", saveapifeature.LegacyRoutes(saveapiHandler, apiStatsRecorder, appCfg.APIKey, logger))
	})
	r.Route("/load", func(r chi.Router) {
		r.Use(ledger.Middleware(apiLedgerConfig))
		r.Mount("/", saveapifeature.LegacyLoadRoutes(saveapiHandler, apiStatsRecorder, appCfg.APIKey, logger))
	})

	// ─────────────────────────────────────────────────────────────────────────────
	// Player Settings API Routes
	// POST /api/settings/save and POST /api/settings/load
	// API errors are logged to the ledger for debugging.
	// ─────────────────────────────────────────────────────────────────────────────
	settingsapiHandler := settingsapifeature.NewHandler(deps.MongoDatabase, logger)
	r.Route("/api/settings", func(r chi.Router) {
		r.Use(ledger.Middleware(apiLedgerConfig))
		r.Mount("/", settingsapifeature.Routes(settingsapiHandler, apiStatsRecorder, appCfg.APIKey, logger))
	})

	// Health check endpoints for load balancers and orchestrators
	healthHandler := healthfeature.NewHandler(deps.MongoClient, logger)
	r.Mount("/health", healthfeature.Routes(healthHandler))
	healthfeature.MountRootEndpoints(r, healthHandler)

	// Static assets with pre-compressed file support (gzip/brotli)
	// /static/* serves files from disk (static directory)
	r.Handle("/static/*", fileserver.Handler("/static", "static"))

	// /assets/* serves embedded assets (bundled into the binary)
	r.Handle("/assets/*", appresources.AssetsHandler("/assets"))

	// Uploaded files (local storage only)
	// When using local storage, serve files from the configured path
	if appCfg.StorageType == "local" || appCfg.StorageType == "" {
		r.Handle(appCfg.StorageLocalURL+"/*", fileserver.Handler(appCfg.StorageLocalURL, appCfg.StorageLocalPath))
	}

	// Public pages
	homeHandler := homefeature.NewHandler(deps.MongoDatabase, logger)
	r.Mount("/", homefeature.Routes(homeHandler))

	// Dynamic content pages (about, contact, terms, privacy)
	pagesHandler := pagesfeature.NewHandler(deps.MongoDatabase, errLog, logger)
	r.Mount("/about", pagesHandler.AboutRouter())
	r.Mount("/contact", pagesHandler.ContactRouter())
	r.Mount("/terms", pagesHandler.TermsRouter())
	r.Mount("/privacy", pagesHandler.PrivacyRouter())
	r.Mount("/pages", pagesfeature.EditRoutes(pagesHandler, sessionMgr))

	// User Invitations (public accept route)
	invitationsHandler := invitationsfeature.NewHandler(
		deps.MongoDatabase,
		sessionMgr,
		sessionsStore,
		errLog,
		deps.Mailer,
		auditLogger,
		appCfg.BaseURL,
		7*24*time.Hour, // 7 days expiry
		logger,
	)
	r.Mount("/invite", invitationsfeature.AcceptRoutes(invitationsHandler))

	// Authentication
	googleEnabled := appCfg.GoogleClientID != "" && appCfg.GoogleClientSecret != ""
	// Trust login is only enabled in dev mode for security - it allows passwordless login
	trustLoginEnabled := coreCfg.Env == "dev"

	// Rate limiting for login attempts (nil if disabled)
	var rateLimitStore *ratelimit.Store
	if appCfg.RateLimitEnabled {
		rateLimitStore = ratelimit.New(
			deps.MongoDatabase,
			appCfg.RateLimitLoginAttempts,
			appCfg.RateLimitLoginWindow,
			appCfg.RateLimitLoginLockout,
		)
	}

	loginHandler := loginfeature.NewHandler(
		deps.MongoDatabase,
		sessionMgr,
		errLog,
		deps.Mailer,
		auditLogger,
		sessionsStore,
		activityStore,
		rateLimitStore,
		appCfg.BaseURL,
		appCfg.EmailVerifyExpiry,
		trustLoginEnabled,
		logger,
	)
	r.Mount("/login", loginfeature.Routes(loginHandler))

	logoutHandler := logoutfeature.NewHandler(sessionMgr, auditLogger, sessionsStore, logger)
	r.Mount("/logout", logoutfeature.Routes(logoutHandler, sessionMgr))

	// Heartbeat API for activity tracking
	heartbeatHandler := heartbeatfeature.NewHandler(sessionsStore, activityStore, sessionMgr, logger)
	heartbeatHandler.SetIdleLogoutConfig(appCfg.IdleLogoutEnabled, appCfg.IdleLogoutTimeout, appCfg.IdleLogoutWarning)
	r.Mount("/api/heartbeat", heartbeatfeature.Routes(heartbeatHandler, sessionMgr))

	// Google OAuth (only mount if configured)
	if googleEnabled {
		oauthStateStore := oauthstate.New(deps.MongoDatabase)
		googleHandler := authgooglefeature.NewHandler(
			deps.MongoDatabase,
			sessionMgr,
			errLog,
			auditLogger,
			sessionsStore,
			oauthStateStore,
			appCfg.GoogleClientID,
			appCfg.GoogleClientSecret,
			appCfg.BaseURL,
			logger,
		)
		r.Mount("/auth/google", authgooglefeature.Routes(googleHandler))
		logger.Info("Google OAuth enabled", zap.String("redirect_url", appCfg.BaseURL+"/auth/google/callback"))
	}

	// User profile (admin and developer users)
	profileHandler := profilefeature.NewHandler(deps.MongoDatabase, sessionsStore, errLog, logger)
	r.Route("/profile", func(sr chi.Router) {
		sr.Use(sessionMgr.RequireRole("admin", "developer"))
		sr.Mount("/", profilefeature.Routes(profileHandler, sessionMgr))
	})

	// Error pages
	errorsHandler := errorsfeature.NewHandler()
	r.Get("/forbidden", errorsHandler.Forbidden)
	r.Get("/unauthorized", errorsHandler.Unauthorized)

	// Role-based dashboards
	dashboardHandler := dashboardfeature.NewHandler(deps.MongoDatabase, logger)
	r.Mount("/dashboard", dashboardfeature.Routes(dashboardHandler, sessionMgr))

	// Active sessions dashboard (admin only)
	sessionsHandler := dashboardfeature.NewSessionsHandler(deps.MongoDatabase, sessionsStore, logger)
	r.Mount("/dashboard/sessions", dashboardfeature.SessionsRoutes(sessionsHandler, sessionMgr))

	// System user management (admin only)
	sysUsersHandler := systemusersfeature.NewHandler(deps.MongoDatabase, deps.Mailer, errLog, auditLogger, logger)
	r.Mount("/system-users", systemusersfeature.Routes(sysUsersHandler, sessionMgr))

	// Audit log (admin only)
	auditLogHandler := auditlogfeature.NewHandler(deps.MongoDatabase, errLog, logger)
	r.Mount("/audit", auditlogfeature.Routes(auditLogHandler, sessionMgr))

	// User Invitations management (admin only)
	r.Mount("/invitations", invitationsfeature.AdminRoutes(invitationsHandler, sessionMgr))

	// Announcements management (admin only)
	announcementsHandler := announcementsfeature.NewHandler(deps.MongoDatabase, errLog, logger)
	r.Mount("/announcements", announcementsfeature.Routes(announcementsHandler, sessionMgr))

	// User-facing announcements view (authenticated users)
	r.Mount("/my-announcements", announcementsfeature.ViewRoutes(announcementsHandler, sessionMgr))

	// Files feature (all authenticated users can browse, admins can manage)
	filesHandler := filesfeature.NewHandler(deps.MongoDatabase, deps.FileStorage, errLog, auditLogger, logger)
	r.Mount("/library", filesfeature.Routes(filesHandler, sessionMgr))

	// Site Settings (admin only)
	settingsHandler := settingsfeature.NewHandler(deps.MongoDatabase, deps.FileStorage, errLog, logger)
	r.Route("/settings", func(sr chi.Router) {
		sr.Use(sessionMgr.RequireRole("admin"))
		settingsHandler.MountRoutes(sr)
	})

	// System status page (admin only)
	statusAppCfg := statusfeature.AppConfig{
		MongoURI:           appCfg.MongoURI,
		MongoDatabase:      appCfg.MongoDatabase,
		MongoMaxPoolSize:   appCfg.MongoMaxPoolSize,
		MongoMinPoolSize:   appCfg.MongoMinPoolSize,
		SessionKey:         appCfg.SessionKey,
		SessionName:        appCfg.SessionName,
		SessionDomain:      appCfg.SessionDomain,
		SessionMaxAge:      appCfg.SessionMaxAge,
		IdleLogoutEnabled:      appCfg.IdleLogoutEnabled,
		IdleLogoutTimeout:      appCfg.IdleLogoutTimeout,
		IdleLogoutWarning:      appCfg.IdleLogoutWarning,
		RateLimitEnabled:       appCfg.RateLimitEnabled,
		RateLimitLoginAttempts: appCfg.RateLimitLoginAttempts,
		RateLimitLoginWindow:   appCfg.RateLimitLoginWindow,
		RateLimitLoginLockout:  appCfg.RateLimitLoginLockout,
		CSRFKey:                appCfg.CSRFKey,
		APIKey:                 appCfg.APIKey,
		StorageType:        appCfg.StorageType,
		StorageLocalPath:   appCfg.StorageLocalPath,
		StorageLocalURL:    appCfg.StorageLocalURL,
		StorageS3Region:    appCfg.StorageS3Region,
		StorageS3Bucket:    appCfg.StorageS3Bucket,
		StorageS3Prefix:    appCfg.StorageS3Prefix,
		StorageCFURL:       appCfg.StorageCFURL,
		StorageCFKeyPairID: appCfg.StorageCFKeyPairID,
		StorageCFKeyPath:   appCfg.StorageCFKeyPath,
		MailSMTPHost:       appCfg.MailSMTPHost,
		MailSMTPPort:       appCfg.MailSMTPPort,
		MailSMTPUser:       appCfg.MailSMTPUser,
		MailSMTPPass:       appCfg.MailSMTPPass,
		MailFrom:           appCfg.MailFrom,
		MailFromName:       appCfg.MailFromName,
		BaseURL:            appCfg.BaseURL,
		EmailVerifyExpiry:  appCfg.EmailVerifyExpiry,
		AuditLogAuth:       appCfg.AuditLogAuth,
		AuditLogAdmin:      appCfg.AuditLogAdmin,
		GoogleClientID:     appCfg.GoogleClientID,
		GoogleClientSecret: appCfg.GoogleClientSecret,
		SeedAdminEmail:     appCfg.SeedAdminEmail,
		SeedAdminName:      appCfg.SeedAdminName,
	}
	statusHandler := statusfeature.NewHandler(deps.MongoClient, appCfg.BaseURL, coreCfg, statusAppCfg, logger)
	r.Mount("/admin/status", statusfeature.Routes(statusHandler, sessionMgr))

	// Activity dashboard (admin only)
	activityHandler := activityfeature.NewHandler(
		deps.MongoDatabase,
		sessionsStore,
		activityStore,
		userstore.New(deps.MongoDatabase),
		sessionMgr,
		errLog,
		logger,
	)
	r.Mount("/activity", activityfeature.Routes(activityHandler, sessionMgr))

	// Request Ledger (admin and developer)
	ledgerHandler := ledgerfeature.NewHandler(deps.MongoDatabase, errLog, logger)
	r.Mount("/ledger", ledgerfeature.Routes(ledgerHandler, sessionMgr))

	// API Keys management (admin only)
	apikeysHandler := apikeysfeature.NewHandler(deps.MongoDatabase, errLog, logger)
	r.Mount("/api-keys", apikeysfeature.Routes(apikeysHandler, sessionMgr))

	// Jobs monitoring (admin and developer)
	jobsHandler := jobsfeature.NewHandler(deps.MongoDatabase, errLog, logger)
	r.Mount("/jobs", jobsfeature.Routes(jobsHandler, sessionMgr))

	// Statistics (admin and developer)
	statsHandler := statsfeature.NewHandler(deps.MongoDatabase, errLog, logger)
	r.Mount("/stats", statsfeature.Routes(statsHandler, sessionMgr))

	// API Statistics (admin and developer)
	apistatsHandler := apistatsfeature.NewHandler(deps.MongoDatabase, apiStatsStore, apiStatsRecorder, errLog, logger)
	r.Mount("/console/api/stats", apistatsfeature.Routes(apistatsHandler, sessionMgr))

	// State API Console (admin and developer)
	// Parse max saves config (default to 10 for browser display)
	stateBrowserLimit := 10
	if appCfg.MaxSavesPerUser != "" && appCfg.MaxSavesPerUser != "all" {
		if n, err := strconv.Atoi(appCfg.MaxSavesPerUser); err == nil && n > 0 {
			stateBrowserLimit = n
		}
	}
	stateBrowserHandler := savebrowserfeature.NewHandler(
		deps.MongoDatabase,
		errLog,
		stateBrowserLimit,
		appCfg.APIKey,
		logger,
	)
	r.Mount("/console/api/state", savebrowserfeature.Routes(stateBrowserHandler, sessionMgr))

	// Settings API Console (admin and developer)
	settingsBrowserHandler := settingsbrowserfeature.NewHandler(
		deps.MongoDatabase,
		errLog,
		appCfg.APIKey,
		logger,
	)
	r.Mount("/console/api/settings", settingsbrowserfeature.Routes(settingsBrowserHandler, sessionMgr))

	// 404 catch-all for unmatched routes
	r.NotFound(errorsHandler.NotFound)

	return r, nil
}
