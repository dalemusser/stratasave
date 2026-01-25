// internal/app/features/stats/handler.go
package statsfeature

import (
	"context"
	"fmt"
	"net/http"
	"time"

	errorsfeature "github.com/dalemusser/stratasave/internal/app/features/errors"
	statsstore "github.com/dalemusser/stratasave/internal/app/store/stats"
	"github.com/dalemusser/stratasave/internal/app/system/timeouts"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/waffle/pantry/templates"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Handler handles statistics HTTP requests.
type Handler struct {
	DB     *mongo.Database
	ErrLog *errorsfeature.ErrorLogger
	Log    *zap.Logger
}

// NewHandler creates a new stats handler.
func NewHandler(db *mongo.Database, errLog *errorsfeature.ErrorLogger, logger *zap.Logger) *Handler {
	return &Handler{
		DB:     db,
		ErrLog: errLog,
		Log:    logger,
	}
}

// ServeDashboard handles GET /stats - main statistics dashboard.
func (h *Handler) ServeDashboard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	// Parse time period
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "week"
	}

	// Calculate date range based on period
	now := time.Now()
	var startDate, endDate time.Time
	switch period {
	case "day":
		startDate = now.AddDate(0, 0, -1)
		endDate = now
	case "week":
		startDate = now.AddDate(0, 0, -7)
		endDate = now
	case "month":
		startDate = now.AddDate(0, -1, 0)
		endDate = now
	default:
		startDate = now.AddDate(0, 0, -7)
		endDate = now
		period = "week"
	}

	// Allow custom date range
	if start := r.URL.Query().Get("start"); start != "" {
		if t, err := time.Parse("2006-01-02", start); err == nil {
			startDate = t
		}
	}
	if end := r.URL.Query().Get("end"); end != "" {
		if t, err := time.Parse("2006-01-02", end); err == nil {
			endDate = t
		}
	}

	selectedType := r.URL.Query().Get("type")

	store := statsstore.New(h.DB)

	// Get available stat types
	statTypes, err := store.GetStatTypes(ctx)
	if err != nil {
		h.ErrLog.Log(r, "failed to load stat types", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// If no type selected and we have types, use the first one
	if selectedType == "" && len(statTypes) > 0 {
		selectedType = statTypes[0]
	}

	// Build view model
	data := StatsDashboardVM{
		BaseVM:        viewdata.NewBaseVM(r, h.DB, "Statistics", "/dashboard"),
		Period:        period,
		StartDate:     startDate.Format("2006-01-02"),
		EndDate:       endDate.Format("2006-01-02"),
		StatTypes:     statTypes,
		SelectedType:  selectedType,
		CounterSeries: make(map[string][]TimeSeriesPointVM),
		GaugeSeries:   make(map[string][]TimeSeriesPointVM),
	}

	// If we have a selected type, load its stats
	if selectedType != "" {
		// Get daily stats
		dailyStats, err := store.GetRange(ctx, startDate, endDate, selectedType)
		if err != nil {
			h.ErrLog.Log(r, "failed to load daily stats", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Convert to view models
		for _, ds := range dailyStats {
			data.DailyStats = append(data.DailyStats, DailyStatsVM{
				Date:     ds.Date.Format("2006-01-02"),
				StatType: ds.StatType,
				Counters: ds.Counters,
				Gauges:   ds.Gauges,
			})
		}

		// Get counter totals
		counterTotals, err := store.SumCounters(ctx, startDate, endDate, selectedType)
		if err != nil {
			h.Log.Warn("failed to load counter totals", zap.Error(err))
		}

		// Get gauge averages
		gaugeAvgs, err := store.AvgGauges(ctx, startDate, endDate, selectedType)
		if err != nil {
			h.Log.Warn("failed to load gauge averages", zap.Error(err))
		}

		// Build stat cards
		for name, total := range counterTotals {
			data.Cards = append(data.Cards, StatCardVM{
				Label: name,
				Value: formatInt64(total),
			})
		}
		for name, avg := range gaugeAvgs {
			data.Cards = append(data.Cards, StatCardVM{
				Label: name,
				Value: fmt.Sprintf("%.2f", avg),
			})
		}

		// Build time series for counters
		if len(dailyStats) > 0 && dailyStats[0].Counters != nil {
			for counterName := range dailyStats[0].Counters {
				series, err := store.GetCounterTimeSeries(ctx, startDate, endDate, selectedType, counterName)
				if err == nil {
					points := make([]TimeSeriesPointVM, len(series))
					for i, s := range series {
						points[i] = TimeSeriesPointVM{
							Date:  s.Date.Format("Jan 2"),
							Value: float64(s.Value),
						}
					}
					data.CounterSeries[counterName] = points
				}
			}
		}

		// Build time series for gauges
		if len(dailyStats) > 0 && dailyStats[0].Gauges != nil {
			for gaugeName := range dailyStats[0].Gauges {
				series, err := store.GetGaugeTimeSeries(ctx, startDate, endDate, selectedType, gaugeName)
				if err == nil {
					points := make([]TimeSeriesPointVM, len(series))
					for i, s := range series {
						points[i] = TimeSeriesPointVM{
							Date:  s.Date.Format("Jan 2"),
							Value: s.Value,
						}
					}
					data.GaugeSeries[gaugeName] = points
				}
			}
		}
	}

	templates.Render(w, r, "stats/dashboard", data)
}

// ServeDetail handles GET /stats/{type} - detailed view for a stat type.
func (h *Handler) ServeDetail(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	statType := r.URL.Query().Get("type")
	if statType == "" {
		http.Redirect(w, r, "/stats", http.StatusSeeOther)
		return
	}

	// Parse date range
	now := time.Now()
	startDate := now.AddDate(0, 0, -30) // Default to 30 days
	endDate := now

	if start := r.URL.Query().Get("start"); start != "" {
		if t, err := time.Parse("2006-01-02", start); err == nil {
			startDate = t
		}
	}
	if end := r.URL.Query().Get("end"); end != "" {
		if t, err := time.Parse("2006-01-02", end); err == nil {
			endDate = t
		}
	}

	store := statsstore.New(h.DB)

	// Get daily stats
	dailyStats, err := store.GetRange(ctx, startDate, endDate, statType)
	if err != nil {
		h.ErrLog.Log(r, "failed to load daily stats", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get totals and averages
	counterTotals, _ := store.SumCounters(ctx, startDate, endDate, statType)
	gaugeAvgs, _ := store.AvgGauges(ctx, startDate, endDate, statType)

	// Build view model
	data := StatsDetailVM{
		BaseVM:        viewdata.NewBaseVM(r, h.DB, "Statistics: "+statType, "/stats"),
		StatType:      statType,
		StartDate:     startDate.Format("2006-01-02"),
		EndDate:       endDate.Format("2006-01-02"),
		TotalCounters: counterTotals,
		AvgGauges:     gaugeAvgs,
		CounterSeries: make(map[string][]TimeSeriesPointVM),
		GaugeSeries:   make(map[string][]TimeSeriesPointVM),
	}

	// Convert daily stats
	for _, ds := range dailyStats {
		data.DailyStats = append(data.DailyStats, DailyStatsVM{
			Date:     ds.Date.Format("2006-01-02"),
			StatType: ds.StatType,
			Counters: ds.Counters,
			Gauges:   ds.Gauges,
		})
	}

	// Build time series
	if len(dailyStats) > 0 {
		if dailyStats[0].Counters != nil {
			for counterName := range dailyStats[0].Counters {
				series, err := store.GetCounterTimeSeries(ctx, startDate, endDate, statType, counterName)
				if err == nil {
					points := make([]TimeSeriesPointVM, len(series))
					for i, s := range series {
						points[i] = TimeSeriesPointVM{
							Date:  s.Date.Format("Jan 2"),
							Value: float64(s.Value),
						}
					}
					data.CounterSeries[counterName] = points
				}
			}
		}
		if dailyStats[0].Gauges != nil {
			for gaugeName := range dailyStats[0].Gauges {
				series, err := store.GetGaugeTimeSeries(ctx, startDate, endDate, statType, gaugeName)
				if err == nil {
					points := make([]TimeSeriesPointVM, len(series))
					for i, s := range series {
						points[i] = TimeSeriesPointVM{
							Date:  s.Date.Format("Jan 2"),
							Value: s.Value,
						}
					}
					data.GaugeSeries[gaugeName] = points
				}
			}
		}
	}

	templates.Render(w, r, "stats/detail", data)
}

// formatInt64 formats a large integer with commas.
func formatInt64(n int64) string {
	if n < 0 {
		return "-" + formatInt64(-n)
	}
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return formatInt64(n/1000) + "," + fmt.Sprintf("%03d", n%1000)
}
