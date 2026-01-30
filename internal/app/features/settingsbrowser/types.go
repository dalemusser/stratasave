package settingsbrowser

import (
	"time"

	"github.com/dalemusser/stratasave/internal/app/system/timezones"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
)

// ListVM is the view model for the main settings browser page.
type ListVM struct {
	viewdata.BaseVM

	// Timezone support
	TimezoneGroups []timezones.ZoneGroup

	// Game selection
	Games        []string
	SelectedGame string

	// User list
	Users        []string
	UserSearch   string
	SelectedUser string

	// Pagination for users
	UserTotal      int64
	UserPage       int
	UserHasPrev    bool
	UserHasNext    bool
	UserRangeStart int
	UserRangeEnd   int
	UserPrevPage   int
	UserNextPage   int

	// Setting (when user selected)
	Setting *SettingVM
}

// SettingVM represents a single setting for display.
type SettingVM struct {
	ID           string
	UserID       string
	Game         string
	Timestamp    time.Time
	SettingsData string // JSON string for display
}

// UsersPartialVM is the view model for the users table HTMX partial.
type UsersPartialVM struct {
	SelectedGame   string
	SelectedUser   string
	UserSearch     string
	Users          []string
	UserTotal      int64
	UserPage       int
	UserHasPrev    bool
	UserHasNext    bool
	UserRangeStart int
	UserRangeEnd   int
	UserPrevPage   int
	UserNextPage   int
}

// SettingPartialVM is the view model for the setting HTMX partial.
type SettingPartialVM struct {
	viewdata.BaseVM

	SelectedGame string
	SelectedUser string
	Setting      *SettingVM
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
