package savebrowser

import (
	"time"

	"github.com/dalemusser/stratasave/internal/app/system/timezones"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
)

// ListVM is the view model for the main save browser page.
type ListVM struct {
	viewdata.BaseVM

	// Timezone support
	TimezoneGroups []timezones.ZoneGroup

	// Game selection
	Games        []string
	SelectedGame string

	// User list
	Users        []UserRowVM
	UserSearch   string
	SelectedUser string

	// Pagination for users
	UserTotal        int64
	UserPage         int
	UserHasPrev      bool
	UserHasNext      bool
	UserRangeStart int
	UserRangeEnd   int
	UserPrevPage     int
	UserNextPage     int

	// Save results (when user selected)
	Saves      []SaveRowVM
	SaveTotal  int64
	SaveLimit  int
	HasPrev    bool
	HasNext    bool
	PrevCursor string // ID of first save (for "prev" pagination)
	NextCursor string // ID of last save (for "next" pagination)

	// Configuration
	DefaultLimit int
}

// UserRowVM represents a row in the users table.
type UserRowVM struct {
	UserID    string
	SaveCount int64
}

// SaveRowVM represents a single save in the list.
type SaveRowVM struct {
	ID        string
	UserID    string
	Game      string
	Timestamp time.Time
	SaveData  string // JSON string for display
}

// SavesPartialVM is the view model for the saves HTMX partial.
type SavesPartialVM struct {
	viewdata.BaseVM

	SelectedGame string
	SelectedUser string
	Saves        []SaveRowVM
	Total        int64
	Limit        int
	HasPrev      bool
	HasNext      bool
	PrevCursor   string
	NextCursor   string
}

// UsersPartialVM is the view model for the users table HTMX partial.
type UsersPartialVM struct {
	SelectedGame     string
	SelectedUser     string
	UserSearch       string
	Users            []UserRowVM
	UserTotal        int64
	UserPage         int
	UserHasPrev      bool
	UserHasNext      bool
	UserRangeStart int
	UserRangeEnd   int
	UserPrevPage     int
	UserNextPage     int
	Limit            int
}

// GamePickerVM is the view model for the game picker modal.
type GamePickerVM struct {
	Games      []GamePickerItem
	SelectedID string
	Query      string
}

// GamePickerItem represents a game in the picker list.
type GamePickerItem struct {
	Name     string
	Selected bool
}
