// internal/app/bootstrap/routes.go
package bootstrap

import (
	"context"
	"net/http"
	"time"

	activityfeature "github.com/dalemusser/stratasave/internal/app/features/activity"
	announcementsfeature "github.com/dalemusser/stratasave/internal/app/features/announcements"
	apikeysfeature "github.com/dalemusser/stratasave/internal/app/features/apikeys"
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

	r := chi.NewRouter()

	// Request timeout middleware: prevents requests from hanging indefinitely.
	// Requests exceeding 30 seconds will be cancelled and return a 503 Service Unavailable.
	r.Use(chimw.Timeout(30 * time.Second))

	// CORS middleware: must be early in the chain to handle preflight requests.
	// Only active when enable_cors=true in config.
	r.Use(middleware.CORSFromConfig(coreCfg))

	// Security headers middleware: adds X-Frame-Options, X-Content-Type-Options, etc.
	// Enabled by default with secure values. Configure via enable_security_headers and related options.
	r.Use(middleware.SecurityHeadersFromConfig(coreCfg))

	// Global auth middleware: loads SessionUser into context if logged in.
	// This makes the current user available to all handlers via auth.CurrentUser(r).
	r.Use(sessionMgr.LoadSessionUser)

	// CSRF protection middleware: protects POST/PUT/DELETE requests from cross-site request forgery.
	// The CSRF token must be included in forms as a hidden field or in the X-CSRF-Token header.
	csrfOpts := []csrf.Option{
		csrf.Secure(secure),
		csrf.Path("/"),
		csrf.CookieName("csrf_token"),
		csrf.FieldName("csrf_token"),
		csrf.SameSite(csrf.SameSiteLaxMode),
		csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Warn("CSRF validation failed",
				zap.String("path", r.URL.Path),
				zap.String("method", r.Method),
				zap.String("reason", csrf.FailureReason(r).Error()),
			)
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusForbidden)
				return
			}
			http.Error(w, "CSRF token invalid or missing", http.StatusForbidden)
		})),
	}
	// In dev mode, trust localhost origins for CSRF validation.
	// gorilla/csrf validates the Origin header's Host against TrustedOrigins (not the full URL).
	trustedOrigins := []string{
		"localhost:8080",
		"localhost:3000",
		"127.0.0.1:8080",
		"127.0.0.1:3000",
	}
	if !secure {
		csrfOpts = append(csrfOpts, csrf.TrustedOrigins(trustedOrigins))
	}
	// Set CSRF cookie domain to match session domain when configured.
	// This ensures CSRF cookie is shared across subdomains (if any).
	if appCfg.SessionDomain != "" {
		csrfOpts = append(csrfOpts, csrf.Domain(appCfg.SessionDomain))
	}
	csrfMiddleware := csrf.Protect([]byte(appCfg.CSRFKey), csrfOpts...)
	r.Use(csrfMiddleware)

	// Health check endpoints for load balancers and orchestrators
	// Provides:
	//   /health      - full health check with service status
	//   /health/ready, /health/live - sub-routes
	//   /ready, /readyz - Kubernetes readiness probes (root level)
	//   /livez       - Kubernetes liveness probe (root level)
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
		googleEnabled,
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

	// 404 catch-all for unmatched routes
	r.NotFound(errorsHandler.NotFound)

	return r, nil
}
