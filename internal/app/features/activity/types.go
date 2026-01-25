// internal/app/features/activity/types.go
package activity

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"time"

	"github.com/dalemusser/stratasave/internal/app/system/timezones"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
)

// Status represents a user's online status.
type Status string

const (
	StatusOnline  Status = "online"  // Heartbeat within 2 minutes
	StatusIdle    Status = "idle"    // Heartbeat 2-10 minutes ago
	StatusOffline Status = "offline" // No active session or heartbeat > 10 minutes
)

// OnlineThreshold is the duration within which a user is considered "online".
const OnlineThreshold = 2 * time.Minute

// IdleThreshold is the duration after which a user is considered "idle" (but not offline).
const IdleThreshold = 10 * time.Minute

// userRow represents a user in the activity dashboard.
type userRow struct {
	ID              string
	Name            string
	LoginID         string
	Email           string
	Role            string
	Status          Status
	StatusLabel     string
	CurrentActivity string
	TimeTodayMins   int    // For sorting
	TimeTodayStr    string // Pre-formatted "Xh Ym" or "X min"
	LastActiveAt    *time.Time
}

// dashboardData is the view model for the real-time dashboard.
type dashboardData struct {
	viewdata.BaseVM

	// Filter state
	StatusFilter string // "all", "online", "idle", "offline"
	SearchQuery  string

	// Sorting
	SortBy  string // "name", "time"
	SortDir string // "asc", "desc"

	// Pagination
	Page       int
	Total      int
	RangeStart int
	RangeEnd   int
	HasPrev    bool
	HasNext    bool
	PrevPage   int
	NextPage   int

	// Summary stats (before filtering)
	TotalUsers   int
	OnlineCount  int
	IdleCount    int
	OfflineCount int

	// User rows (paginated)
	Users []userRow
}

// summaryRow represents a user in the weekly summary view.
type summaryRow struct {
	ID           string
	Name         string
	LoginID      string
	Email        string
	Role         string
	SessionCount int
	TotalTimeStr string // Pre-formatted "Xh Ym" or "X min"
	OutsideHours int    // Sessions at unusual times
}

// summaryData is the view model for the weekly summary view.
type summaryData struct {
	viewdata.BaseVM

	WeekStart  string
	WeekEnd    string
	WeekParam  string // For navigation links (2006-01-02 format)
	PrevWeek   string
	NextWeek   string
	IsThisWeek bool

	// User rows
	Users []summaryRow
}

// activityEvent represents an event in the user detail timeline.
type activityEvent struct {
	Time        time.Time
	TimeLabel   string // Formatted time (fallback)
	TimeISO     string // ISO 8601 format for client-side formatting
	EventType   string
	Description string
}

// sessionBlock represents a session in the user detail view.
type sessionBlock struct {
	Date          string
	LoginTime     string // Formatted time (fallback)
	LoginTimeISO  string // ISO 8601 format for client-side formatting
	LogoutTime    string // Formatted time (fallback)
	LogoutTimeISO string // ISO 8601 format for client-side formatting (empty if active)
	Duration      string
	EndReason     string
	Events        []activityEvent
}

// userDetailData is the view model for the user detail view.
type userDetailData struct {
	viewdata.BaseVM

	// User info
	UserID   string
	UserName string
	LoginID  string
	Email    string
	UserRole string

	// Timezone selector
	Timezone       string              // Selected timezone ID (e.g., "America/Denver")
	TimezoneGroups []timezones.ZoneGroup // Grouped timezone options for dropdown

	// Stats
	TotalSessions  int
	TotalTimeStr   string // Pre-formatted "Xh Ym" or "X min"
	AvgSessionMins int
	PageViews      int

	// Session history (most recent first)
	Sessions []sessionBlock
}

// exportData is the view model for the export page.
type exportData struct {
	viewdata.BaseVM

	// Filter state
	StartDate string
	EndDate   string

	// Aggregated stats (for the summary section)
	TotalSessions    int
	TotalUsers       int
	TotalDurationStr string // Pre-formatted "Xh Ym" or "X min"
	AvgSessionMins   int
	PeakHour         string
	MostActiveDay    string
}

// sessionExportRow represents a session for CSV/JSON export.
type sessionExportRow struct {
	UserID       string    `json:"user_id"`
	UserName     string    `json:"user_name"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	LoginAt      time.Time `json:"login_at"`
	LogoutAt     string    `json:"logout_at"` // string to handle nil
	EndReason    string    `json:"end_reason"`
	DurationSecs int64     `json:"duration_secs"`
	IP           string    `json:"ip"`
}

// eventExportRow represents an activity event for CSV/JSON export.
type eventExportRow struct {
	UserID    string                 `json:"user_id"`
	UserName  string                 `json:"user_name"`
	SessionID string                 `json:"session_id"`
	Timestamp time.Time              `json:"timestamp"`
	EventType string                 `json:"event_type"`
	PagePath  string                 `json:"page_path,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// aggregateStats holds computed statistics for a date range.
type aggregateStats struct {
	TotalSessions     int
	TotalUsers        int
	TotalDurationSecs int64
	SessionsByHour    map[int]int    // hour -> count
	SessionsByDay     map[string]int // weekday -> count
}
