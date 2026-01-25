// internal/app/features/stats/types.go
package statsfeature

import (
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
)

// StatCardVM represents a single statistic card.
type StatCardVM struct {
	Label       string
	Value       string
	Delta       string // e.g., "+12%" or "-5%"
	DeltaType   string // "positive", "negative", "neutral"
	Description string
}

// TimeSeriesPointVM represents a single point in a time series.
type TimeSeriesPointVM struct {
	Date  string
	Value float64
}

// DailyStatsVM represents stats for a single day.
type DailyStatsVM struct {
	Date     string
	StatType string
	Counters map[string]int64
	Gauges   map[string]float64
}

// StatsDashboardVM is the view model for the stats dashboard.
type StatsDashboardVM struct {
	viewdata.BaseVM
	Period        string // "day", "week", "month"
	StartDate     string
	EndDate       string
	StatTypes     []string
	SelectedType  string
	Cards         []StatCardVM
	DailyStats    []DailyStatsVM
	CounterSeries map[string][]TimeSeriesPointVM // counter name -> time series
	GaugeSeries   map[string][]TimeSeriesPointVM // gauge name -> time series
}

// StatsDetailVM is the view model for detailed stats view.
type StatsDetailVM struct {
	viewdata.BaseVM
	StatType      string
	StartDate     string
	EndDate       string
	TotalCounters map[string]int64
	AvgGauges     map[string]float64
	DailyStats    []DailyStatsVM
	CounterSeries map[string][]TimeSeriesPointVM
	GaugeSeries   map[string][]TimeSeriesPointVM
}
