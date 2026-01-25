// internal/app/features/activity/export.go
package activity

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/waffle/pantry/templates"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// ServeExport renders the export UI with filters and aggregated stats.
func (h *Handler) ServeExport(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Parse date range from query params (default to last 30 days)
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)

	if s := r.URL.Query().Get("start"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			startDate = t
		}
	}
	if e := r.URL.Query().Get("end"); e != "" {
		if t, err := time.Parse("2006-01-02", e); err == nil {
			endDate = t.Add(24*time.Hour - time.Second) // end of day
		}
	}

	// Compute aggregated stats
	stats := h.computeAggregateStats(ctx, startDate, endDate)

	totalDurationMins := int(stats.TotalDurationSecs / 60)

	data := exportData{
		BaseVM:    viewdata.NewBaseVM(r, h.DB, "Data Export", "/activity"),
		StartDate: startDate.Format("2006-01-02"),
		EndDate:   endDate.Format("2006-01-02"),

		TotalSessions:    stats.TotalSessions,
		TotalUsers:       stats.TotalUsers,
		TotalDurationStr: formatMinutes(totalDurationMins),
		AvgSessionMins:   safeDiv(int(stats.TotalDurationSecs/60), stats.TotalSessions),
		PeakHour:         findPeakHour(stats.SessionsByHour),
		MostActiveDay:    findMostActiveDay(stats.SessionsByDay),
	}

	templates.Render(w, r, "activity_export", data)
}

// ServeSessionsCSV exports sessions as CSV.
func (h *Handler) ServeSessionsCSV(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	startDate, endDate := parseDateRange(r)

	rows, err := h.fetchSessionExportRows(ctx, startDate, endDate)
	if err != nil {
		h.ErrLog.Log(r, "fetch sessions for export failed", err)
		http.Error(w, "A database error occurred", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("sessions_%s_%s.csv", startDate.Format("20060102"), endDate.Format("20060102"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, url.PathEscape(filename)))

	// UTF-8 BOM for Excel
	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		h.Log.Error("CSV write failed (BOM)", zap.Error(err))
		return
	}

	cw := csv.NewWriter(w)
	cw.UseCRLF = true
	defer cw.Flush()

	// Header
	if err := cw.Write([]string{"user_id", "user_name", "email", "role", "login_at", "logout_at", "end_reason", "duration_secs", "ip"}); err != nil {
		h.Log.Error("CSV write failed (header)", zap.Error(err))
		return
	}

	// Rows
	for _, row := range rows {
		if err := cw.Write([]string{
			row.UserID,
			sanitizeCSVField(row.UserName),
			row.Email,
			row.Role,
			row.LoginAt.Format(time.RFC3339),
			row.LogoutAt,
			row.EndReason,
			fmt.Sprintf("%d", row.DurationSecs),
			row.IP,
		}); err != nil {
			h.Log.Error("CSV write failed (row)", zap.Error(err))
			return
		}
	}

	h.Log.Info("sessions CSV exported", zap.Int("rows", len(rows)))
}

// ServeSessionsJSON exports sessions as JSON.
func (h *Handler) ServeSessionsJSON(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	startDate, endDate := parseDateRange(r)

	rows, err := h.fetchSessionExportRows(ctx, startDate, endDate)
	if err != nil {
		h.ErrLog.Log(r, "fetch sessions for export failed", err)
		http.Error(w, "A database error occurred", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("sessions_%s_%s.json", startDate.Format("20060102"), endDate.Format("20060102"))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, url.PathEscape(filename)))

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rows); err != nil {
		h.Log.Error("JSON encode failed", zap.Error(err))
	}

	h.Log.Info("sessions JSON exported", zap.Int("rows", len(rows)))
}

// ServeEventsCSV exports activity events as CSV.
func (h *Handler) ServeEventsCSV(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	startDate, endDate := parseDateRange(r)

	rows, err := h.fetchEventExportRows(ctx, startDate, endDate)
	if err != nil {
		h.ErrLog.Log(r, "fetch events for export failed", err)
		http.Error(w, "A database error occurred", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("activity_events_%s_%s.csv", startDate.Format("20060102"), endDate.Format("20060102"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, url.PathEscape(filename)))

	// UTF-8 BOM for Excel
	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		h.Log.Error("CSV write failed (BOM)", zap.Error(err))
		return
	}

	cw := csv.NewWriter(w)
	cw.UseCRLF = true
	defer cw.Flush()

	// Header
	if err := cw.Write([]string{"user_id", "user_name", "session_id", "timestamp", "event_type", "page_path", "details"}); err != nil {
		h.Log.Error("CSV write failed (header)", zap.Error(err))
		return
	}

	// Rows
	for _, row := range rows {
		detailsJSON := ""
		if len(row.Details) > 0 {
			if b, err := json.Marshal(row.Details); err == nil {
				detailsJSON = string(b)
			}
		}
		if err := cw.Write([]string{
			row.UserID,
			sanitizeCSVField(row.UserName),
			row.SessionID,
			row.Timestamp.Format(time.RFC3339),
			row.EventType,
			row.PagePath,
			detailsJSON,
		}); err != nil {
			h.Log.Error("CSV write failed (row)", zap.Error(err))
			return
		}
	}

	h.Log.Info("events CSV exported", zap.Int("rows", len(rows)))
}

// ServeEventsJSON exports activity events as JSON.
func (h *Handler) ServeEventsJSON(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	startDate, endDate := parseDateRange(r)

	rows, err := h.fetchEventExportRows(ctx, startDate, endDate)
	if err != nil {
		h.ErrLog.Log(r, "fetch events for export failed", err)
		http.Error(w, "A database error occurred", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("activity_events_%s_%s.json", startDate.Format("20060102"), endDate.Format("20060102"))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, url.PathEscape(filename)))

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rows); err != nil {
		h.Log.Error("JSON encode failed", zap.Error(err))
	}

	h.Log.Info("events JSON exported", zap.Int("rows", len(rows)))
}

// parseDateRange extracts start and end dates from query params.
func parseDateRange(r *http.Request) (time.Time, time.Time) {
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)

	if s := r.URL.Query().Get("start"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			startDate = t
		}
	}
	if e := r.URL.Query().Get("end"); e != "" {
		if t, err := time.Parse("2006-01-02", e); err == nil {
			endDate = t.Add(24*time.Hour - time.Second)
		}
	}

	return startDate, endDate
}

// fetchSessionExportRows fetches sessions for export.
func (h *Handler) fetchSessionExportRows(ctx context.Context, startDate, endDate time.Time) ([]sessionExportRow, error) {
	sessFilter := bson.M{
		"login_at": bson.M{"$gte": startDate, "$lte": endDate},
	}

	cur, err := h.DB.Collection("sessions").Find(ctx, sessFilter, options.Find().
		SetSort(bson.D{{Key: "login_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	type sessDoc struct {
		ID         primitive.ObjectID `bson:"_id"`
		UserID     primitive.ObjectID `bson:"user_id"`
		LoginAt    time.Time          `bson:"login_at"`
		LogoutAt   *time.Time         `bson:"logout_at"`
		EndReason  string             `bson:"end_reason"`
		Duration   int64              `bson:"duration_secs"`
		IP         string             `bson:"ip"`
	}

	var sessions []sessDoc
	userIDSet := make(map[primitive.ObjectID]struct{})

	for cur.Next(ctx) {
		var s sessDoc
		if err := cur.Decode(&s); err != nil {
			continue
		}
		sessions = append(sessions, s)
		userIDSet[s.UserID] = struct{}{}
	}

	// Batch fetch user info
	userInfo := h.fetchUserInfo(ctx, userIDSet)

	// Build export rows
	var rows []sessionExportRow
	for _, s := range sessions {
		ui := userInfo[s.UserID]
		logoutStr := ""
		if s.LogoutAt != nil {
			logoutStr = s.LogoutAt.Format(time.RFC3339)
		}
		rows = append(rows, sessionExportRow{
			UserID:       s.UserID.Hex(),
			UserName:     ui.FullName,
			Email:        ui.Email,
			Role:         ui.Role,
			LoginAt:      s.LoginAt,
			LogoutAt:     logoutStr,
			EndReason:    s.EndReason,
			DurationSecs: s.Duration,
			IP:           s.IP,
		})
	}

	return rows, nil
}

// fetchEventExportRows fetches activity events for export.
func (h *Handler) fetchEventExportRows(ctx context.Context, startDate, endDate time.Time) ([]eventExportRow, error) {
	eventFilter := bson.M{
		"timestamp": bson.M{"$gte": startDate, "$lte": endDate},
	}

	cur, err := h.DB.Collection("activity_events").Find(ctx, eventFilter, options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	type eventDoc struct {
		ID        primitive.ObjectID     `bson:"_id"`
		UserID    primitive.ObjectID     `bson:"user_id"`
		SessionID primitive.ObjectID     `bson:"session_id"`
		Timestamp time.Time              `bson:"timestamp"`
		EventType string                 `bson:"event_type"`
		PagePath  string                 `bson:"page_path"`
		Details   map[string]interface{} `bson:"details"`
	}

	var events []eventDoc
	userIDSet := make(map[primitive.ObjectID]struct{})

	for cur.Next(ctx) {
		var e eventDoc
		if err := cur.Decode(&e); err != nil {
			continue
		}
		events = append(events, e)
		userIDSet[e.UserID] = struct{}{}
	}

	// Batch fetch user info
	userInfo := h.fetchUserInfo(ctx, userIDSet)

	// Build export rows
	var rows []eventExportRow
	for _, e := range events {
		ui := userInfo[e.UserID]
		rows = append(rows, eventExportRow{
			UserID:    e.UserID.Hex(),
			UserName:  ui.FullName,
			SessionID: e.SessionID.Hex(),
			Timestamp: e.Timestamp,
			EventType: e.EventType,
			PagePath:  e.PagePath,
			Details:   e.Details,
		})
	}

	return rows, nil
}

// computeAggregateStats computes aggregate statistics for the date range.
func (h *Handler) computeAggregateStats(ctx context.Context, startDate, endDate time.Time) aggregateStats {
	stats := aggregateStats{
		SessionsByHour: make(map[int]int),
		SessionsByDay:  make(map[string]int),
	}

	sessFilter := bson.M{
		"login_at": bson.M{"$gte": startDate, "$lte": endDate},
	}

	cur, err := h.DB.Collection("sessions").Find(ctx, sessFilter)
	if err != nil {
		h.Log.Warn("fetch sessions for stats failed", zap.Error(err))
		return stats
	}
	defer cur.Close(ctx)

	userSet := make(map[primitive.ObjectID]struct{})
	for cur.Next(ctx) {
		var s struct {
			UserID       primitive.ObjectID `bson:"user_id"`
			LoginAt      time.Time          `bson:"login_at"`
			DurationSecs int64              `bson:"duration_secs"`
		}
		if err := cur.Decode(&s); err != nil {
			continue
		}

		stats.TotalSessions++
		stats.TotalDurationSecs += s.DurationSecs
		userSet[s.UserID] = struct{}{}

		// Track by hour and day
		hour := s.LoginAt.Hour()
		stats.SessionsByHour[hour]++

		day := s.LoginAt.Weekday().String()
		stats.SessionsByDay[day]++
	}

	stats.TotalUsers = len(userSet)

	return stats
}

type userInfoCache struct {
	FullName string
	Email    string
	Role     string
}

// fetchUserInfo batch fetches user names, emails, and roles.
func (h *Handler) fetchUserInfo(ctx context.Context, userIDs map[primitive.ObjectID]struct{}) map[primitive.ObjectID]userInfoCache {
	result := make(map[primitive.ObjectID]userInfoCache)
	if len(userIDs) == 0 {
		return result
	}

	ids := make([]primitive.ObjectID, 0, len(userIDs))
	for id := range userIDs {
		ids = append(ids, id)
	}

	cur, err := h.DB.Collection("users").Find(ctx, bson.M{"_id": bson.M{"$in": ids}}, options.Find().
		SetProjection(bson.M{"full_name": 1, "email": 1, "role": 1}))
	if err != nil {
		h.Log.Warn("fetch user info failed", zap.Error(err))
		return result
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		var u struct {
			ID       primitive.ObjectID `bson:"_id"`
			FullName string             `bson:"full_name"`
			Email    *string            `bson:"email"`
			Role     string             `bson:"role"`
		}
		if err := cur.Decode(&u); err != nil {
			continue
		}
		email := ""
		if u.Email != nil {
			email = *u.Email
		}
		result[u.ID] = userInfoCache{FullName: u.FullName, Email: email, Role: u.Role}
	}

	return result
}

// safeDiv performs integer division, returning 0 if divisor is 0.
func safeDiv(a, b int) int {
	if b == 0 {
		return 0
	}
	return a / b
}

// findPeakHour finds the hour with most sessions.
func findPeakHour(hourCounts map[int]int) string {
	if len(hourCounts) == 0 {
		return "N/A"
	}
	maxHour := 0
	maxCount := 0
	for hour, count := range hourCounts {
		if count > maxCount {
			maxCount = count
			maxHour = hour
		}
	}
	return fmt.Sprintf("%02d:00", maxHour)
}

// findMostActiveDay finds the weekday with most sessions.
func findMostActiveDay(dayCounts map[string]int) string {
	if len(dayCounts) == 0 {
		return "N/A"
	}
	maxDay := ""
	maxCount := 0
	for day, count := range dayCounts {
		if count > maxCount {
			maxCount = count
			maxDay = day
		}
	}
	return maxDay
}

// sanitizeCSVField prevents CSV formula injection.
func sanitizeCSVField(s string) string {
	if len(s) == 0 {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@':
		return "'" + s
	}
	return s
}

// formatMinutes formats a duration in minutes as "Xh Ym" or "X min".
func formatMinutes(mins int) string {
	if mins >= 60 {
		h := mins / 60
		m := mins % 60
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%d min", mins)
}
