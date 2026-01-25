// internal/app/features/dashboard/dashboard.go
package dashboard

import (
	"net/http"

	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/waffle/pantry/templates"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Handler provides dashboard handlers.
type Handler struct {
	db     *mongo.Database
	logger *zap.Logger
}

// NewHandler creates a new dashboard Handler.
func NewHandler(db *mongo.Database, logger *zap.Logger) *Handler {
	return &Handler{
		db:     db,
		logger: logger,
	}
}

// DashboardVM is the view model for the dashboard.
type DashboardVM struct {
	viewdata.BaseVM
}

// Routes returns a chi.Router with dashboard routes mounted.
func Routes(h *Handler, sessionMgr *auth.SessionManager) http.Handler {
	r := chi.NewRouter()
	r.Use(sessionMgr.RequireAuth)
	r.Get("/", h.showDashboard)
	return r
}

// showDashboard displays the role-based dashboard.
func (h *Handler) showDashboard(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := auth.CurrentUser(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	vm := DashboardVM{
		BaseVM: viewdata.New(r),
	}
	vm.Title = "Dashboard"

	// Render role-specific dashboard
	switch sessionUser.Role {
	case "admin":
		templates.Render(w, r, "dashboard/admin", vm)
	default:
		templates.Render(w, r, "dashboard/default", vm)
	}
}
