// internal/app/features/auditlog/auditlog.go
package auditlog

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	errorsfeature "github.com/dalemusser/stratasave/internal/app/features/errors"
	"github.com/dalemusser/stratasave/internal/app/store/audit"
	userstore "github.com/dalemusser/stratasave/internal/app/store/users"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/app/system/timezones"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/waffle/pantry/templates"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

const pageSize = 50

// Handler provides audit log handlers.
type Handler struct {
	auditStore *audit.Store
	userStore  *userstore.Store
	errLog     *errorsfeature.ErrorLogger
	logger     *zap.Logger
}

// NewHandler creates a new audit log Handler.
func NewHandler(
	db *mongo.Database,
	errLog *errorsfeature.ErrorLogger,
	logger *zap.Logger,
) *Handler {
	return &Handler{
		auditStore: audit.New(db),
		userStore:  userstore.New(db),
		errLog:     errLog,
		logger:     logger,
	}
}

// listItem represents a single audit event row for display.
type listItem struct {
	ID        string
	Timestamp time.Time
	Category  string
	EventType string
	ActorName string // Resolved from ActorID
	IP        string
	Success   bool
	Details   map[string]string
}

// listData is the view model for the audit log list page.
type listData struct {
	viewdata.BaseVM

	Items []listItem

	// Filters
	Category  string
	EventType string
	StartDate string
	EndDate   string
	Timezone  string

	// Filter options
	Categories []categoryOption
	EventTypes []string

	// Timezone selector
	TimezoneGroups []timezones.ZoneGroup

	// Pagination
	Page       int
	TotalPages int
	Total      int64
	Shown      int
	RangeStart int
	RangeEnd   int
	HasPrev    bool
	HasNext    bool
	PrevPage   int
	NextPage   int
}

// categoryOption represents a category for the filter dropdown.
type categoryOption struct {
	Value string
	Label string
}

// allCategories returns the available categories for filtering.
func allCategories() []categoryOption {
	return []categoryOption{
		{Value: audit.CategoryAuth, Label: "Authentication"},
		{Value: audit.CategoryAdmin, Label: "Administration"},
	}
}

// eventTypesForCategory returns the event types for a given category.
// If category is empty, returns all event types.
func eventTypesForCategory(category string) []string {
	authEvents := []string{
		audit.EventLoginSuccess,
		audit.EventLoginFailedUserNotFound,
		audit.EventLoginFailedWrongPassword,
		audit.EventLoginFailedUserDisabled,
		audit.EventLogout,
		audit.EventPasswordChanged,
		audit.EventVerificationCodeSent,
		audit.EventVerificationCodeResent,
		audit.EventVerificationCodeFailed,
		audit.EventMagicLinkUsed,
	}

	adminEvents := []string{
		audit.EventUserCreated,
		audit.EventUserUpdated,
		audit.EventUserDisabled,
		audit.EventUserEnabled,
		audit.EventUserDeleted,
		audit.EventSettingsUpdated,
		audit.EventPageUpdated,
	}

	switch category {
	case audit.CategoryAuth:
		return authEvents
	case audit.CategoryAdmin:
		return adminEvents
	case "":
		// Return all event types when no category selected
		all := make([]string, 0, len(authEvents)+len(adminEvents))
		all = append(all, authEvents...)
		all = append(all, adminEvents...)
		return all
	default:
		return nil
	}
}

// Routes returns a chi.Router with audit log routes mounted.
func Routes(h *Handler, sessionMgr *auth.SessionManager) http.Handler {
	r := chi.NewRouter()
	r.Use(sessionMgr.RequireRole("admin"))

	r.Get("/", h.list)

	return r
}

// list displays the audit log with filtering and pagination.
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	// Get filter parameters
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	eventType := strings.TrimSpace(r.URL.Query().Get("event_type"))
	startDate := strings.TrimSpace(r.URL.Query().Get("start_date"))
	endDate := strings.TrimSpace(r.URL.Query().Get("end_date"))
	tzParam := strings.TrimSpace(r.URL.Query().Get("tz"))
	pageStr := r.URL.Query().Get("page")

	page := 1
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}

	// Load timezone location for date parsing (fall back to Local if invalid)
	loc := time.Local
	if tzParam != "" {
		if parsedLoc, err := time.LoadLocation(tzParam); err == nil {
			loc = parsedLoc
		}
	}

	// Build query filter
	filter := audit.QueryFilter{
		Category:  category,
		EventType: eventType,
		Limit:     pageSize,
		Offset:    int64((page - 1) * pageSize),
	}

	// Parse dates in user's selected timezone
	if startDate != "" {
		if t, err := time.ParseInLocation("2006-01-02", startDate, loc); err == nil {
			filter.StartTime = &t
		}
	}
	if endDate != "" {
		if t, err := time.ParseInLocation("2006-01-02", endDate, loc); err == nil {
			// End of day
			endOfDay := t.Add(24*time.Hour - time.Second)
			filter.EndTime = &endOfDay
		}
	}

	// Query audit store
	events, err := h.auditStore.Query(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to query audit events", zap.Error(err))
		h.errLog.Log(r, "failed to query audit events", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	total, err := h.auditStore.CountByFilter(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to count audit events", zap.Error(err))
		total = 0
	}

	// Collect unique user IDs for name resolution
	userIDs := make(map[primitive.ObjectID]struct{})
	for _, e := range events {
		if e.ActorID != nil {
			userIDs[*e.ActorID] = struct{}{}
		}
		if e.UserID != nil {
			userIDs[*e.UserID] = struct{}{}
		}
	}

	// Batch fetch user names
	userNames := make(map[primitive.ObjectID]string)
	if len(userIDs) > 0 {
		ids := make([]primitive.ObjectID, 0, len(userIDs))
		for id := range userIDs {
			ids = append(ids, id)
		}
		users, err := h.userStore.GetByIDs(r.Context(), ids)
		if err != nil {
			h.logger.Warn("failed to fetch user names for audit log", zap.Error(err))
		} else {
			for _, u := range users {
				userNames[u.ID] = u.FullName
			}
		}
	}

	// Build list items
	items := make([]listItem, 0, len(events))
	for _, e := range events {
		item := listItem{
			ID:        e.ID.Hex(),
			Timestamp: e.CreatedAt,
			Category:  e.Category,
			EventType: e.EventType,
			IP:        e.IP,
			Success:   e.Success,
			Details:   e.Details,
		}
		// Resolve actor name
		if e.ActorID != nil {
			if name, ok := userNames[*e.ActorID]; ok {
				item.ActorName = name
			}
			// Don't show raw ObjectID for deleted users - leave blank
		} else if e.UserID != nil && e.Category == audit.CategoryAuth {
			// For auth events, the user is the actor (they're logging in/out themselves)
			if name, ok := userNames[*e.UserID]; ok {
				item.ActorName = name
			}
			// Don't show raw ObjectID for deleted users - leave blank
		}
		items = append(items, item)
	}

	// Calculate pagination
	totalPages := int((total + pageSize - 1) / pageSize)
	if totalPages < 1 {
		totalPages = 1
	}

	prevPage := page - 1
	if prevPage < 1 {
		prevPage = 1
	}
	nextPage := page + 1
	if nextPage > totalPages {
		nextPage = totalPages
	}

	// Get event types for selected category (or all if no category selected)
	eventTypes := eventTypesForCategory(category)

	// Get timezone groups for selector
	tzGroups, _ := timezones.Groups()

	// Calculate range for display
	rangeStart := (page-1)*pageSize + 1
	rangeEnd := rangeStart + len(items) - 1
	if len(items) == 0 {
		rangeStart = 0
		rangeEnd = 0
	}

	vm := listData{
		BaseVM:         viewdata.New(r),
		Items:          items,
		Category:       category,
		EventType:      eventType,
		StartDate:      startDate,
		EndDate:        endDate,
		Timezone:       tzParam,
		Categories:     allCategories(),
		EventTypes:     eventTypes,
		TimezoneGroups: tzGroups,
		Page:           page,
		TotalPages:     totalPages,
		Total:          total,
		Shown:          len(items),
		RangeStart:     rangeStart,
		RangeEnd:       rangeEnd,
		HasPrev:        page > 1,
		HasNext:        page < totalPages,
		PrevPage:       prevPage,
		NextPage:       nextPage,
	}
	vm.Title = "Audit Log"

	templates.RenderAutoMap(w, r, "auditlog/list", nil, vm)
}
