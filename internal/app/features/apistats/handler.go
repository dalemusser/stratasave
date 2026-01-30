package apistats

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	errorsfeature "github.com/dalemusser/stratasave/internal/app/features/errors"
	apistatsstore "github.com/dalemusser/stratasave/internal/app/store/apistats"
	apistatsystem "github.com/dalemusser/stratasave/internal/app/system/apistats"
	"github.com/dalemusser/stratasave/internal/app/system/auth"
	"github.com/dalemusser/stratasave/internal/app/system/timeouts"
	"github.com/dalemusser/stratasave/internal/app/system/timezones"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/waffle/pantry/templates"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Handler handles API stats HTTP requests.
type Handler struct {
	db       *mongo.Database
	store    *apistatsstore.Store
	recorder *apistatsystem.Recorder
	errLog   *errorsfeature.ErrorLogger
	logger   *zap.Logger
}

// NewHandler creates a new API stats handler.
func NewHandler(db *mongo.Database, store *apistatsstore.Store, recorder *apistatsystem.Recorder, errLog *errorsfeature.ErrorLogger, logger *zap.Logger) *Handler {
	return &Handler{
		db:       db,
		store:    store,
		recorder: recorder,
		errLog:   errLog,
		logger:   logger,
	}
}

// ServeList renders the main API stats page.
func (h *Handler) ServeList(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	// Parse query params
	timeRange := r.URL.Query().Get("range")
	if timeRange == "" {
		timeRange = "24h"
	}

	bucketFilter := r.URL.Query().Get("bucket")

	// API filter: "state", "settings", or "" (all)
	apiFilter := r.URL.Query().Get("api")
	if apiFilter != "" && apiFilter != "state" && apiFilter != "settings" {
		apiFilter = "" // Invalid filter, reset to all
	}

	// Calculate time range
	endTime := time.Now().UTC()
	var startTime time.Time
	switch timeRange {
	case "1h":
		startTime = endTime.Add(-1 * time.Hour)
	case "6h":
		startTime = endTime.Add(-6 * time.Hour)
	case "24h":
		startTime = endTime.Add(-24 * time.Hour)
	case "7d":
		startTime = endTime.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = endTime.Add(-30 * 24 * time.Hour)
	default:
		startTime = endTime.Add(-24 * time.Hour)
		timeRange = "24h"
	}

	// Get current bucket duration
	currentBucket := h.recorder.GetBucketDuration().String()

	// Get distinct bucket durations in the data
	dataResolutions, _ := h.store.GetDistinctDurations(ctx)

	// Get summaries for all stat types
	summaries, err := h.store.GetSummary(ctx, startTime, endTime)
	if err != nil {
		h.logger.Warn("failed to get API stats summary", zap.Error(err))
	}

	// Convert to view models and filter based on apiFilter
	var summaryVMs []SummaryVM
	for _, s := range summaries {
		// Filter based on API type
		isStateType := s.StatType == apistatsstore.StatTypeSaveState || s.StatType == apistatsstore.StatTypeLoadState
		isSettingsType := s.StatType == apistatsstore.StatTypeSaveSettings || s.StatType == apistatsstore.StatTypeLoadSettings

		if apiFilter == "state" && !isStateType {
			continue
		}
		if apiFilter == "settings" && !isSettingsType {
			continue
		}

		vm := SummaryVM{
			StatType:      string(s.StatType),
			Label:         StatTypeLabel(s.StatType),
			TotalRequests: s.TotalRequests,
			TotalErrors:   s.TotalErrors,
			ErrorRate:     float64(s.TotalErrors) / float64(s.TotalRequests) * 100,
			AvgMs:         s.AvgMs,
			MinMs:         s.MinMs,
			MaxMs:         s.MaxMs,
		}
		if s.TotalRequests == 0 {
			vm.ErrorRate = 0
		}
		summaryVMs = append(summaryVMs, vm)
	}

	// Get time series data for each stat type (only for relevant APIs based on filter)
	var stateSaveData, stateLoadData, settingsSaveData, settingsLoadData []DataPointVM

	if apiFilter == "" || apiFilter == "state" {
		stateSaveData = h.getTimeSeriesData(ctx, apistatsstore.StatTypeSaveState, startTime, endTime, bucketFilter)
		stateLoadData = h.getTimeSeriesData(ctx, apistatsstore.StatTypeLoadState, startTime, endTime, bucketFilter)
	}
	if apiFilter == "" || apiFilter == "settings" {
		settingsSaveData = h.getTimeSeriesData(ctx, apistatsstore.StatTypeSaveSettings, startTime, endTime, bucketFilter)
		settingsLoadData = h.getTimeSeriesData(ctx, apistatsstore.StatTypeLoadSettings, startTime, endTime, bucketFilter)
	}

	// Build available buckets list
	availableBuckets := []string{}
	for _, opt := range AvailableBucketOptions() {
		availableBuckets = append(availableBuckets, opt.Value)
	}

	// Check if user is admin
	isAdmin := false
	if user, ok := auth.CurrentUser(r); ok {
		isAdmin = user.Role == "admin"
	}

	// Load timezone groups
	tzGroups, _ := timezones.Groups()

	data := ListVM{
		BaseVM:           viewdata.NewBaseVM(r, h.db, "API Statistics", "/dashboard"),
		TimezoneGroups:   tzGroups,
		CurrentBucket:    currentBucket,
		AvailableBuckets: availableBuckets,
		StartTime:        startTime,
		EndTime:          endTime,
		TimeRange:        timeRange,
		APIFilter:        apiFilter,
		Summaries:        summaryVMs,
		StateSaveData:    stateSaveData,
		StateLoadData:    stateLoadData,
		SettingsSaveData: settingsSaveData,
		SettingsLoadData: settingsLoadData,
		DataResolutions:  dataResolutions,
		IsAdmin:          isAdmin,
	}

	templates.Render(w, r, "apistats/list", data)
}

// getTimeSeriesData retrieves time series data for a stat type.
func (h *Handler) getTimeSeriesData(ctx context.Context, statType apistatsstore.StatType, startTime, endTime time.Time, bucketFilter string) []DataPointVM {
	buckets, err := h.store.GetRange(ctx, statType, startTime, endTime, bucketFilter)
	if err != nil {
		h.logger.Warn("failed to get time series data",
			zap.String("stat_type", string(statType)),
			zap.Error(err),
		)
		return nil
	}

	data := make([]DataPointVM, len(buckets))
	for i, b := range buckets {
		data[i] = DataPointVM{
			Timestamp: b.Bucket,
			Requests:  b.Requests,
			Errors:    b.Errors,
			AvgMs:     b.AvgMs(),
			MinMs:     b.MinMs,
			MaxMs:     b.MaxMs,
		}
	}
	return data
}

// HandleSetBucket handles POST /api-stats/bucket - update the recording bucket duration.
func (h *Handler) HandleSetBucket(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	bucketStr := r.FormValue("bucket")
	if bucketStr == "" {
		http.Error(w, "Bucket duration required", http.StatusBadRequest)
		return
	}

	duration, err := time.ParseDuration(bucketStr)
	if err != nil {
		http.Error(w, "Invalid bucket duration", http.StatusBadRequest)
		return
	}

	// Update the recorder's bucket duration
	h.recorder.SetBucketDuration(duration)

	h.logger.Info("API stats bucket duration updated",
		zap.String("bucket", bucketStr),
	)

	// Return success with HX-Trigger to refresh the page
	w.Header().Set("HX-Trigger", "bucket-updated")
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// ServeChartData handles GET /api-stats/chart-data - returns JSON data for charts.
func (h *Handler) ServeChartData(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	// Parse query params
	timeRange := r.URL.Query().Get("range")
	if timeRange == "" {
		timeRange = "24h"
	}

	statType := r.URL.Query().Get("type")
	bucketFilter := r.URL.Query().Get("bucket")

	// Calculate time range
	endTime := time.Now().UTC()
	var startTime time.Time
	switch timeRange {
	case "1h":
		startTime = endTime.Add(-1 * time.Hour)
	case "6h":
		startTime = endTime.Add(-6 * time.Hour)
	case "24h":
		startTime = endTime.Add(-24 * time.Hour)
	case "7d":
		startTime = endTime.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = endTime.Add(-30 * 24 * time.Hour)
	default:
		startTime = endTime.Add(-24 * time.Hour)
	}

	var data []DataPointVM
	if statType != "" {
		data = h.getTimeSeriesData(ctx, apistatsstore.StatType(statType), startTime, endTime, bucketFilter)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode chart data", zap.Error(err))
	}
}

// HandleRollUp handles POST /api-stats/rollup - roll up fine-grained data to coarser buckets.
func (h *Handler) HandleRollUp(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Long())
	defer cancel()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	sourceDuration := r.FormValue("source")
	targetDuration := r.FormValue("target")
	daysBackStr := r.FormValue("days")

	if sourceDuration == "" || targetDuration == "" {
		http.Error(w, "Source and target durations required", http.StatusBadRequest)
		return
	}

	source, err := time.ParseDuration(sourceDuration)
	if err != nil {
		http.Error(w, "Invalid source duration", http.StatusBadRequest)
		return
	}

	target, err := time.ParseDuration(targetDuration)
	if err != nil {
		http.Error(w, "Invalid target duration", http.StatusBadRequest)
		return
	}

	daysBack := 7
	if daysBackStr != "" {
		if d, err := time.ParseDuration(daysBackStr); err == nil {
			daysBack = int(d.Hours() / 24)
		}
	}

	endTime := time.Now().UTC()
	startTime := endTime.Add(-time.Duration(daysBack) * 24 * time.Hour)

	// Roll up each stat type
	statTypes := []apistatsstore.StatType{
		apistatsstore.StatTypeSaveState,
		apistatsstore.StatTypeLoadState,
		apistatsstore.StatTypeSaveSettings,
		apistatsstore.StatTypeLoadSettings,
	}

	for _, st := range statTypes {
		if err := h.store.RollUp(ctx, st, startTime, endTime, source, target); err != nil {
			h.logger.Error("failed to roll up stats",
				zap.String("stat_type", string(st)),
				zap.Error(err),
			)
		}
	}

	h.logger.Info("API stats rolled up",
		zap.String("source", sourceDuration),
		zap.String("target", targetDuration),
		zap.Int("days_back", daysBack),
	)

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// HandleDelete handles POST /api-stats/delete - delete old stats data.
func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	bucketDuration := r.FormValue("bucket")
	daysBackStr := r.FormValue("days")

	daysBack := 30
	if daysBackStr != "" {
		if d, err := time.ParseDuration(daysBackStr); err == nil {
			daysBack = int(d.Hours() / 24)
		}
	}

	cutoff := time.Now().UTC().Add(-time.Duration(daysBack) * 24 * time.Hour)

	deleted, err := h.store.DeleteOlderThan(ctx, cutoff, bucketDuration)
	if err != nil {
		h.errLog.Log(r, "failed to delete old API stats", err)
		http.Error(w, "Failed to delete stats", http.StatusInternalServerError)
		return
	}

	h.logger.Info("API stats deleted",
		zap.String("bucket_duration", bucketDuration),
		zap.Int("days_back", daysBack),
		zap.Int64("deleted", deleted),
	)

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}
