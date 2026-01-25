// internal/app/features/activity/summary.go
package activity

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/waffle/pantry/query"
	"github.com/dalemusser/waffle/pantry/templates"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ServeSummary renders the weekly summary view.
// GET /activity/summary
func (h *Handler) ServeSummary(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	weekParam := query.Get(r, "week") // Format: 2025-01-13

	// Calculate week range
	now := time.Now().UTC()
	weekStart := getWeekStart(now)
	if weekParam != "" {
		if parsed, err := time.Parse("2006-01-02", weekParam); err == nil {
			weekStart = getWeekStart(parsed)
		}
	}
	weekEnd := weekStart.AddDate(0, 0, 7)

	// Get summary data for all users
	users, err := h.fetchWeeklySummary(ctx, weekStart, weekEnd)
	if err != nil {
		h.ErrLog.Log(r, "failed to fetch summary", err)
		http.Error(w, "A database error occurred", http.StatusInternalServerError)
		return
	}

	// Calculate previous and next week dates for navigation
	prevWeek := weekStart.AddDate(0, 0, -7).Format("2006-01-02")
	nextWeek := weekStart.AddDate(0, 0, 7).Format("2006-01-02")
	currentWeek := getWeekStart(now)

	data := summaryData{
		BaseVM:      viewdata.NewBaseVM(r, h.DB, "Weekly Summary", "/activity"),
		WeekStart:   weekStart.Format("Jan 2"),
		WeekEnd:     weekEnd.AddDate(0, 0, -1).Format("Jan 2, 2006"),
		WeekParam:   weekStart.Format("2006-01-02"),
		PrevWeek:    prevWeek,
		NextWeek:    nextWeek,
		IsThisWeek:  weekStart.Equal(currentWeek),
		Users:       users,
	}

	templates.Render(w, r, "activity_summary", data)
}

// getWeekStart returns the Monday of the week containing the given time.
func getWeekStart(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday = 7
	}
	return t.AddDate(0, 0, -(weekday - 1))
}

// fetchWeeklySummary gets session and activity summaries for all users.
func (h *Handler) fetchWeeklySummary(ctx context.Context, weekStart, weekEnd time.Time) ([]summaryRow, error) {
	db := h.DB

	// Get all active users
	cur, err := db.Collection("users").Find(ctx, bson.M{"status": "active"})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	type userInfo struct {
		ID      primitive.ObjectID `bson:"_id"`
		Name    string             `bson:"full_name"`
		Email   string             `bson:"email"`
		LoginID string             `bson:"login_id"`
	}

	var users []userInfo
	for cur.Next(ctx) {
		var u userInfo
		if err := cur.Decode(&u); err != nil {
			continue
		}
		users = append(users, u)
	}

	if len(users) == 0 {
		return nil, nil
	}

	var userIDs []primitive.ObjectID
	for _, u := range users {
		userIDs = append(userIDs, u.ID)
	}

	// Get session stats for the week
	sessionStats, err := h.getWeeklySessionStats(ctx, userIDs, weekStart, weekEnd)
	if err != nil {
		return nil, err
	}

	// Build summary rows
	var summaryRows []summaryRow
	for _, u := range users {
		stats := sessionStats[u.ID]

		summaryRows = append(summaryRows, summaryRow{
			ID:           u.ID.Hex(),
			Name:         u.Name,
			LoginID:      u.LoginID,
			Email:        u.Email,
			SessionCount: stats.SessionCount,
			TotalTimeStr: formatMins(stats.TotalMins),
			OutsideHours: stats.OutsideHours,
		})
	}

	// Sort by name
	sort.Slice(summaryRows, func(i, j int) bool {
		return summaryRows[i].Name < summaryRows[j].Name
	})

	return summaryRows, nil
}

type weeklyStats struct {
	SessionCount int
	TotalMins    int
	OutsideHours int
}

// getWeeklySessionStats calculates session statistics for users in a week.
func (h *Handler) getWeeklySessionStats(ctx context.Context, userIDs []primitive.ObjectID, weekStart, weekEnd time.Time) (map[primitive.ObjectID]weeklyStats, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	// Define "business hours" (8 AM - 6 PM weekdays) for outside hours detection
	pipeline := []bson.M{
		{"$match": bson.M{
			"user_id": bson.M{"$in": userIDs},
			"login_at": bson.M{
				"$gte": weekStart,
				"$lt":  weekEnd,
			},
		}},
		{"$project": bson.M{
			"user_id": 1,
			"duration_mins": bson.M{
				"$cond": bson.M{
					"if": bson.M{"$ne": bson.A{"$duration_secs", nil}},
					"then": bson.M{"$divide": bson.A{"$duration_secs", 60}},
					"else": bson.M{"$divide": bson.A{
						bson.M{"$subtract": bson.A{time.Now().UTC(), "$login_at"}},
						60000,
					}},
				},
			},
			"hour": bson.M{"$hour": "$login_at"},
			"dow":  bson.M{"$dayOfWeek": "$login_at"}, // 1=Sun, 7=Sat
		}},
		{"$group": bson.M{
			"_id":           "$user_id",
			"session_count": bson.M{"$sum": 1},
			"total_mins":    bson.M{"$sum": "$duration_mins"},
			"outside_hours": bson.M{"$sum": bson.M{
				"$cond": bson.M{
					"if": bson.M{"$or": bson.A{
						bson.M{"$in": bson.A{"$dow", bson.A{1, 7}}}, // Weekend
						bson.M{"$lt": bson.A{"$hour", 8}},           // Before 8 AM
						bson.M{"$gte": bson.A{"$hour", 18}},         // After 6 PM
					}},
					"then": 1,
					"else": 0,
				},
			}},
		}},
	}

	cur, err := h.DB.Collection("sessions").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	result := make(map[primitive.ObjectID]weeklyStats)
	for cur.Next(ctx) {
		var doc struct {
			ID           primitive.ObjectID `bson:"_id"`
			SessionCount int                `bson:"session_count"`
			TotalMins    float64            `bson:"total_mins"`
			OutsideHours int                `bson:"outside_hours"`
		}
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		result[doc.ID] = weeklyStats{
			SessionCount: doc.SessionCount,
			TotalMins:    int(doc.TotalMins),
			OutsideHours: doc.OutsideHours,
		}
	}

	return result, nil
}

// formatMins formats minutes as "Xh Ym" or "X min".
func formatMins(mins int) string {
	if mins >= 60 {
		return fmt.Sprintf("%dh %dm", mins/60, mins%60)
	}
	return fmt.Sprintf("%d min", mins)
}
