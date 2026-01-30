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

	// Player list
	Players      []PlayerRowVM
	PlayerSearch string
	SelectedUser string

	// Pagination for players
	PlayerTotal      int64
	PlayerPage       int
	PlayerHasPrev    bool
	PlayerHasNext    bool
	PlayerRangeStart int
	PlayerRangeEnd   int
	PlayerPrevPage   int
	PlayerNextPage   int

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

// PlayerRowVM represents a row in the players table.
type PlayerRowVM struct {
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

// PlayersPartialVM is the view model for the players table HTMX partial.
type PlayersPartialVM struct {
	SelectedGame     string
	SelectedUser     string
	PlayerSearch     string
	Players          []PlayerRowVM
	PlayerTotal      int64
	PlayerPage       int
	PlayerHasPrev    bool
	PlayerHasNext    bool
	PlayerRangeStart int
	PlayerRangeEnd   int
	PlayerPrevPage   int
	PlayerNextPage   int
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
