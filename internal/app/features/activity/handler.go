// internal/app/features/activity/handler.go
package activity

import (
	uierrors "github.com/dalemusser/stratasave/internal/app/features/errors"
	"github.com/dalemusser/stratasave/internal/app/store/activity"
	"github.com/dalemusser/stratasave/internal/app/store/sessions"
	userstore "github.com/dalemusser/stratasave/internal/app/store/users"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Handler owns the activity dashboard handlers.
type Handler struct {
	DB         *mongo.Database
	Sessions   *sessions.Store
	Activity   *activity.Store
	Users      *userstore.Store
	SessionMgr *auth.SessionManager
	Log        *zap.Logger
	ErrLog     *uierrors.ErrorLogger
}

// NewHandler creates a new activity Handler.
func NewHandler(db *mongo.Database, sessStore *sessions.Store, activityStore *activity.Store, userStore *userstore.Store, sessionMgr *auth.SessionManager, errLog *uierrors.ErrorLogger, logger *zap.Logger) *Handler {
	return &Handler{
		DB:         db,
		Sessions:   sessStore,
		Activity:   activityStore,
		Users:      userStore,
		SessionMgr: sessionMgr,
		ErrLog:     errLog,
		Log:        logger,
	}
}
