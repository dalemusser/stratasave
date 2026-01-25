// internal/app/system/auditlog/logger.go
package auditlog

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"context"
	"net/http"
	"strconv"

	"github.com/dalemusser/stratasave/internal/app/store/audit"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// Config holds audit logging configuration.
type Config struct {
	// Auth controls logging for authentication events (login, logout, password, verification).
	// Values: "all" (MongoDB + zap), "db" (MongoDB only), "log" (zap only), "off" (disabled)
	Auth string
	// Admin controls logging for admin action events (user CRUD, settings changes).
	// Values: "all" (MongoDB + zap), "db" (MongoDB only), "log" (zap only), "off" (disabled)
	Admin string
}

// Logger provides convenience methods for logging audit events.
// It logs to both MongoDB (via audit.Store) and structured logs (via zap).
type Logger struct {
	store  *audit.Store
	zapLog *zap.Logger
	config Config
}

// New creates a new audit Logger.
func New(store *audit.Store, zapLog *zap.Logger, config Config) *Logger {
	return &Logger{
		store:  store,
		zapLog: zapLog,
		config: config,
	}
}

// getClientIP extracts the client IP from the request.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for reverse proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// logToZap logs the event to zap with consistent structure.
func (l *Logger) logToZap(event audit.Event) {
	fields := []zap.Field{
		zap.Bool("audit", true),
		zap.String("category", event.Category),
		zap.String("event_type", event.EventType),
		zap.Bool("success", event.Success),
		zap.String("ip", event.IP),
	}

	if event.UserID != nil {
		fields = append(fields, zap.String("user_id", event.UserID.Hex()))
	}
	if event.ActorID != nil {
		fields = append(fields, zap.String("actor_id", event.ActorID.Hex()))
	}
	if event.FailureReason != "" {
		fields = append(fields, zap.String("failure_reason", event.FailureReason))
	}
	for k, v := range event.Details {
		fields = append(fields, zap.String("detail_"+k, v))
	}

	if event.Success {
		l.zapLog.Info("audit event", fields...)
	} else {
		l.zapLog.Warn("audit event", fields...)
	}
}

// Log records an audit event based on configuration.
// If the logger is nil, this is a no-op (allows tests to use nil audit logger).
// Logging destination is controlled by config: "all", "db", "log", or "off".
func (l *Logger) Log(ctx context.Context, event audit.Event) {
	if l == nil {
		return
	}

	// Determine which config setting applies based on event category
	var setting string
	switch event.Category {
	case audit.CategoryAuth:
		setting = l.config.Auth
	case audit.CategoryAdmin:
		setting = l.config.Admin
	default:
		setting = "all" // Default to logging everything for unknown categories
	}

	// Check if logging is disabled for this category
	if setting == "off" {
		return
	}

	// Log to zap if configured
	if setting == "all" || setting == "log" {
		l.logToZap(event)
	}

	// Log to MongoDB if configured
	if setting == "all" || setting == "db" {
		if err := l.store.Log(ctx, event); err != nil {
			l.zapLog.Error("failed to store audit event",
				zap.Error(err),
				zap.String("event_type", event.EventType),
			)
		}
	}
}

// --- Authentication Events ---

// LoginSuccess logs a successful login.
func (l *Logger) LoginSuccess(ctx context.Context, r *http.Request, userID primitive.ObjectID, authMethod, loginID string) {
	l.Log(ctx, audit.Event{
		Category:  audit.CategoryAuth,
		EventType: audit.EventLoginSuccess,
		UserID:    &userID,
		IP:        getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   true,
		Details: map[string]string{
			"auth_method": authMethod,
			"login_id":    loginID,
		},
	})
}

// LoginFailedUserNotFound logs a failed login due to user not found.
func (l *Logger) LoginFailedUserNotFound(ctx context.Context, r *http.Request, attemptedLoginID string) {
	l.Log(ctx, audit.Event{
		Category:      audit.CategoryAuth,
		EventType:     audit.EventLoginFailedUserNotFound,
		IP:            getClientIP(r),
		UserAgent:     r.UserAgent(),
		Success:       false,
		FailureReason: "user not found",
		Details: map[string]string{
			"attempted_login_id": attemptedLoginID,
		},
	})
}

// LoginFailedWrongPassword logs a failed login due to wrong password.
func (l *Logger) LoginFailedWrongPassword(ctx context.Context, r *http.Request, userID primitive.ObjectID, loginID string) {
	l.Log(ctx, audit.Event{
		Category:      audit.CategoryAuth,
		EventType:     audit.EventLoginFailedWrongPassword,
		UserID:        &userID,
		IP:            getClientIP(r),
		UserAgent:     r.UserAgent(),
		Success:       false,
		FailureReason: "wrong password",
		Details: map[string]string{
			"login_id": loginID,
		},
	})
}

// LoginFailedUserDisabled logs a failed login due to disabled account.
func (l *Logger) LoginFailedUserDisabled(ctx context.Context, r *http.Request, userID primitive.ObjectID, loginID string) {
	l.Log(ctx, audit.Event{
		Category:      audit.CategoryAuth,
		EventType:     audit.EventLoginFailedUserDisabled,
		UserID:        &userID,
		IP:            getClientIP(r),
		UserAgent:     r.UserAgent(),
		Success:       false,
		FailureReason: "user disabled",
		Details: map[string]string{
			"login_id": loginID,
		},
	})
}

// Logout logs a user logout.
// Accepts string IDs from SessionUser and converts them to ObjectIDs.
func (l *Logger) Logout(ctx context.Context, r *http.Request, userIDStr string) {
	var userID *primitive.ObjectID

	if oid, err := primitive.ObjectIDFromHex(userIDStr); err == nil {
		userID = &oid
	}

	l.Log(ctx, audit.Event{
		Category:  audit.CategoryAuth,
		EventType: audit.EventLogout,
		UserID:    userID,
		IP:        getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   true,
	})
}

// PasswordChanged logs a password change.
func (l *Logger) PasswordChanged(ctx context.Context, r *http.Request, userID primitive.ObjectID, wasTemporary bool) {
	l.Log(ctx, audit.Event{
		Category:  audit.CategoryAuth,
		EventType: audit.EventPasswordChanged,
		UserID:    &userID,
		IP:        getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   true,
		Details: map[string]string{
			"was_temporary": boolToString(wasTemporary),
		},
	})
}

// VerificationCodeSent logs when a verification code is sent.
func (l *Logger) VerificationCodeSent(ctx context.Context, r *http.Request, userID primitive.ObjectID, email string) {
	l.Log(ctx, audit.Event{
		Category:  audit.CategoryAuth,
		EventType: audit.EventVerificationCodeSent,
		UserID:    &userID,
		IP:        getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   true,
		Details: map[string]string{
			"email": email,
		},
	})
}

// VerificationCodeResent logs when a verification code is resent.
func (l *Logger) VerificationCodeResent(ctx context.Context, r *http.Request, userID primitive.ObjectID, email string, resendCount int) {
	l.Log(ctx, audit.Event{
		Category:  audit.CategoryAuth,
		EventType: audit.EventVerificationCodeResent,
		UserID:    &userID,
		IP:        getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   true,
		Details: map[string]string{
			"email":        email,
			"resend_count": intToString(resendCount),
		},
	})
}

// VerificationCodeFailed logs a failed verification code attempt.
func (l *Logger) VerificationCodeFailed(ctx context.Context, r *http.Request, userID primitive.ObjectID, reason string) {
	l.Log(ctx, audit.Event{
		Category:      audit.CategoryAuth,
		EventType:     audit.EventVerificationCodeFailed,
		UserID:        &userID,
		IP:            getClientIP(r),
		UserAgent:     r.UserAgent(),
		Success:       false,
		FailureReason: reason,
	})
}

// MagicLinkUsed logs when a magic link is used for login.
func (l *Logger) MagicLinkUsed(ctx context.Context, r *http.Request, userID primitive.ObjectID, email string) {
	l.Log(ctx, audit.Event{
		Category:  audit.CategoryAuth,
		EventType: audit.EventMagicLinkUsed,
		UserID:    &userID,
		IP:        getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   true,
		Details: map[string]string{
			"email": email,
		},
	})
}

// --- Admin Events ---

// UserCreated logs when an admin creates a user.
func (l *Logger) UserCreated(ctx context.Context, r *http.Request, actorID, targetUserID primitive.ObjectID, actorRole, role, authMethod string) {
	l.Log(ctx, audit.Event{
		Category:  audit.CategoryAdmin,
		EventType: audit.EventUserCreated,
		UserID:    &targetUserID,
		ActorID:   &actorID,
		IP:        getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   true,
		Details: map[string]string{
			"actor_role":  actorRole,
			"role":        role,
			"auth_method": authMethod,
		},
	})
}

// UserUpdated logs when an admin updates a user.
func (l *Logger) UserUpdated(ctx context.Context, r *http.Request, actorID, targetUserID primitive.ObjectID, actorRole, fieldsChanged string) {
	l.Log(ctx, audit.Event{
		Category:  audit.CategoryAdmin,
		EventType: audit.EventUserUpdated,
		UserID:    &targetUserID,
		ActorID:   &actorID,
		IP:        getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   true,
		Details: map[string]string{
			"actor_role":     actorRole,
			"fields_changed": fieldsChanged,
		},
	})
}

// UserDisabled logs when an admin disables a user account.
func (l *Logger) UserDisabled(ctx context.Context, r *http.Request, actorID, targetUserID primitive.ObjectID, actorRole string) {
	l.Log(ctx, audit.Event{
		Category:  audit.CategoryAdmin,
		EventType: audit.EventUserDisabled,
		UserID:    &targetUserID,
		ActorID:   &actorID,
		IP:        getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   true,
		Details: map[string]string{
			"actor_role": actorRole,
		},
	})
}

// UserEnabled logs when an admin enables a user account.
func (l *Logger) UserEnabled(ctx context.Context, r *http.Request, actorID, targetUserID primitive.ObjectID, actorRole string) {
	l.Log(ctx, audit.Event{
		Category:  audit.CategoryAdmin,
		EventType: audit.EventUserEnabled,
		UserID:    &targetUserID,
		ActorID:   &actorID,
		IP:        getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   true,
		Details: map[string]string{
			"actor_role": actorRole,
		},
	})
}

// UserDeleted logs when an admin deletes a user.
func (l *Logger) UserDeleted(ctx context.Context, r *http.Request, actorID, targetUserID primitive.ObjectID, actorRole, role string) {
	l.Log(ctx, audit.Event{
		Category:  audit.CategoryAdmin,
		EventType: audit.EventUserDeleted,
		UserID:    &targetUserID,
		ActorID:   &actorID,
		IP:        getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   true,
		Details: map[string]string{
			"actor_role": actorRole,
			"role":       role,
		},
	})
}

// SettingsUpdated logs when admin updates site settings.
func (l *Logger) SettingsUpdated(ctx context.Context, r *http.Request, actorID primitive.ObjectID, actorRole, fieldsChanged string) {
	l.Log(ctx, audit.Event{
		Category:  audit.CategoryAdmin,
		EventType: audit.EventSettingsUpdated,
		ActorID:   &actorID,
		IP:        getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   true,
		Details: map[string]string{
			"actor_role":     actorRole,
			"fields_changed": fieldsChanged,
		},
	})
}

// PageUpdated logs when admin updates a page.
func (l *Logger) PageUpdated(ctx context.Context, r *http.Request, actorID primitive.ObjectID, actorRole, pageSlug string) {
	l.Log(ctx, audit.Event{
		Category:  audit.CategoryAdmin,
		EventType: audit.EventPageUpdated,
		ActorID:   &actorID,
		IP:        getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   true,
		Details: map[string]string{
			"actor_role": actorRole,
			"page_slug":  pageSlug,
		},
	})
}

// --- Helper functions ---

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func intToString(i int) string {
	return strconv.Itoa(i)
}

// LogAuthEvent is a convenience method for logging auth events with flexible parameters.
// Used by features that need a simpler interface.
func (l *Logger) LogAuthEvent(r *http.Request, userID *primitive.ObjectID, eventType string, success bool, failureReason string) {
	l.Log(r.Context(), audit.Event{
		Category:      audit.CategoryAuth,
		EventType:     eventType,
		UserID:        userID,
		IP:            getClientIP(r),
		UserAgent:     r.UserAgent(),
		Success:       success,
		FailureReason: failureReason,
	})
}

// LogAdminEvent is a convenience method for logging admin events with flexible parameters.
// Used by features that need a simpler interface.
func (l *Logger) LogAdminEvent(r *http.Request, actorID, targetUserID *primitive.ObjectID, eventType string, details map[string]string) {
	l.Log(r.Context(), audit.Event{
		Category:  audit.CategoryAdmin,
		EventType: eventType,
		UserID:    targetUserID,
		ActorID:   actorID,
		IP:        getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   true,
		Details:   details,
	})
}
