// Package apistats provides a web UI for viewing API request statistics.
package apistats

import (
	"time"

	"github.com/dalemusser/stratasave/internal/app/store/apistats"
	"github.com/dalemusser/stratasave/internal/app/system/timezones"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
)

// ListVM is the view model for the main API stats page.
type ListVM struct {
	viewdata.BaseVM

	// Timezone selection
	TimezoneGroups []timezones.ZoneGroup

	// Current recording settings
	CurrentBucket    string   // Current bucket duration (e.g., "1h")
	AvailableBuckets []string // Available bucket durations for selection

	// Time range
	StartTime time.Time
	EndTime   time.Time
	TimeRange string // "1h", "24h", "7d", "30d"

	// API filter: "", "state", or "settings"
	APIFilter string

	// Summary statistics
	Summaries []SummaryVM

	// Time series data for charts
	StateSaveData    []DataPointVM
	StateLoadData    []DataPointVM
	SettingsSaveData []DataPointVM
	SettingsLoadData []DataPointVM

	// Data resolutions present in the range
	DataResolutions []string

	// IsAdmin indicates if the current user can change settings and manage data
	IsAdmin bool
}

// SummaryVM represents a summary of stats for a stat type.
type SummaryVM struct {
	StatType      string
	Label         string // Human-readable label
	TotalRequests int64
	TotalErrors   int64
	ErrorRate     float64
	AvgMs         float64
	MinMs         int64
	MaxMs         int64
}

// DataPointVM represents a single data point for charting.
type DataPointVM struct {
	Timestamp time.Time
	Requests  int64
	Errors    int64
	AvgMs     float64
	MinMs     int64
	MaxMs     int64
}

// BucketVM represents a bucket option for the UI.
type BucketVM struct {
	Value    string // Duration string (e.g., "1h")
	Label    string // Human-readable label (e.g., "1 hour")
	Selected bool
}

// AvailableBucketOptions returns the available bucket duration options.
func AvailableBucketOptions() []BucketVM {
	return []BucketVM{
		{Value: "1m", Label: "1 minute"},
		{Value: "5m", Label: "5 minutes"},
		{Value: "15m", Label: "15 minutes"},
		{Value: "30m", Label: "30 minutes"},
		{Value: "1h", Label: "1 hour"},
		{Value: "2h", Label: "2 hours"},
		{Value: "6h", Label: "6 hours"},
		{Value: "12h", Label: "12 hours"},
		{Value: "24h", Label: "24 hours"},
	}
}

// TimeRangeOptions returns the available time range options.
func TimeRangeOptions() []struct {
	Value string
	Label string
} {
	return []struct {
		Value string
		Label string
	}{
		{Value: "1h", Label: "Last hour"},
		{Value: "6h", Label: "Last 6 hours"},
		{Value: "24h", Label: "Last 24 hours"},
		{Value: "7d", Label: "Last 7 days"},
		{Value: "30d", Label: "Last 30 days"},
	}
}

// StatTypeLabel returns a human-readable label for a stat type.
func StatTypeLabel(st apistats.StatType) string {
	switch st {
	case apistats.StatTypeSaveState:
		return "Save State"
	case apistats.StatTypeLoadState:
		return "Load State"
	case apistats.StatTypeSaveSettings:
		return "Save Settings"
	case apistats.StatTypeLoadSettings:
		return "Load Settings"
	default:
		return string(st)
	}
}
