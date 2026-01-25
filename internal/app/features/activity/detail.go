// internal/app/features/activity/detail.go
package activity

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"time"

	activitystore "github.com/dalemusser/stratasave/internal/app/store/activity"
	"github.com/dalemusser/stratasave/internal/app/system/timezones"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/waffle/pantry/templates"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ServeUserDetail renders the detailed activity view for a specific user.
// GET /activity/user/{userID}
func (h *Handler) ServeUserDetail(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	userIDStr := chi.URLParam(r, "userID")
	userID, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	db := h.DB

	// Get user details
	var user struct {
		ID      primitive.ObjectID `bson:"_id"`
		Name    string             `bson:"full_name"`
		LoginID string             `bson:"login_id"`
		Email   string             `bson:"email"`
		Role    string             `bson:"role"`
	}
	if err := db.Collection("users").FindOne(ctx, bson.M{"_id": userID}).Decode(&user); err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Get session history (last 30 days)
	thirtyDaysAgo := time.Now().UTC().AddDate(0, 0, -30)
	sessions, err := h.getUserSessions(ctx, userID, thirtyDaysAgo)
	if err != nil {
		h.ErrLog.Log(r, "failed to fetch sessions", err)
		http.Error(w, "A database error occurred", http.StatusInternalServerError)
		return
	}

	// Get activity events for these sessions
	var sessionIDs []primitive.ObjectID
	for _, s := range sessions {
		sessionIDs = append(sessionIDs, s.ID)
	}

	events, err := h.getEventsForSessions(ctx, sessionIDs, userID, thirtyDaysAgo)
	if err != nil {
		h.ErrLog.Log(r, "failed to fetch events", err)
		http.Error(w, "A database error occurred", http.StatusInternalServerError)
		return
	}

	// Calculate stats
	totalSessions := len(sessions)
	var totalMins, pageViews int
	for _, s := range sessions {
		if s.DurationSecs > 0 {
			totalMins += int(s.DurationSecs / 60)
		} else if s.LogoutAt == nil {
			// Active session - calculate duration from login to now
			totalMins += int(time.Since(s.LoginAt).Minutes())
		}
	}
	for _, e := range events {
		if e.EventType == activitystore.EventPageView {
			pageViews++
		}
	}

	avgSessionMins := 0
	if totalSessions > 0 {
		avgSessionMins = totalMins / totalSessions
	}

	// Build session blocks with events (timestamps will be formatted client-side)
	sessionBlocks := h.buildSessionBlocks(sessions, events)

	// Get timezone groups for selector
	tzGroups, _ := timezones.Groups()

	data := userDetailData{
		BaseVM:         viewdata.NewBaseVM(r, h.DB, "Activity History", "/activity"),
		UserID:         userIDStr,
		UserName:       user.Name,
		LoginID:        user.LoginID,
		Email:          user.Email,
		UserRole:       user.Role,
		TimezoneGroups: tzGroups,
		TotalSessions:  totalSessions,
		TotalTimeStr:   formatDuration(int64(totalMins) * 60),
		AvgSessionMins: avgSessionMins,
		PageViews:      pageViews,
		Sessions:       sessionBlocks,
	}

	templates.Render(w, r, "activity_user_detail", data)
}

// ServeUserDetailContent renders just the refreshable content portion (HTMX partial).
// GET /activity/user/{userID}/content
func (h *Handler) ServeUserDetailContent(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	userIDStr := chi.URLParam(r, "userID")
	userID, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	db := h.DB

	// Verify user exists
	count, err := db.Collection("users").CountDocuments(ctx, bson.M{"_id": userID})
	if err != nil || count == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Get session history (last 30 days)
	thirtyDaysAgo := time.Now().UTC().AddDate(0, 0, -30)
	sessions, err := h.getUserSessions(ctx, userID, thirtyDaysAgo)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Get activity events for these sessions
	var sessionIDs []primitive.ObjectID
	for _, s := range sessions {
		sessionIDs = append(sessionIDs, s.ID)
	}

	events, err := h.getEventsForSessions(ctx, sessionIDs, userID, thirtyDaysAgo)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Calculate stats
	totalSessions := len(sessions)
	var totalMins, pageViews int
	for _, s := range sessions {
		if s.DurationSecs > 0 {
			totalMins += int(s.DurationSecs / 60)
		} else if s.LogoutAt == nil {
			totalMins += int(time.Since(s.LoginAt).Minutes())
		}
	}
	for _, e := range events {
		if e.EventType == activitystore.EventPageView {
			pageViews++
		}
	}

	avgSessionMins := 0
	if totalSessions > 0 {
		avgSessionMins = totalMins / totalSessions
	}

	// Build session blocks (timestamps will be formatted client-side)
	sessionBlocks := h.buildSessionBlocks(sessions, events)

	data := userDetailData{
		UserID:         userIDStr,
		TotalSessions:  totalSessions,
		TotalTimeStr:   formatDuration(int64(totalMins) * 60),
		AvgSessionMins: avgSessionMins,
		PageViews:      pageViews,
		Sessions:       sessionBlocks,
	}

	templates.RenderSnippet(w, "activity_user_detail_content", data)
}

// sessionRecord is a minimal session for the detail view.
type sessionRecord struct {
	ID           primitive.ObjectID `bson:"_id"`
	LoginAt      time.Time          `bson:"login_at"`
	LogoutAt     *time.Time         `bson:"logout_at"`
	LastActiveAt time.Time          `bson:"last_activity"`
	EndReason    string             `bson:"end_reason"`
	DurationSecs int64              `bson:"duration_secs"`
}

// getUserSessions gets sessions for a user since the given time.
// It also closes any stale sessions (open but inactive for more than 10 minutes).
func (h *Handler) getUserSessions(ctx context.Context, userID primitive.ObjectID, since time.Time) ([]sessionRecord, error) {
	// First, close any stale sessions for this user
	staleThreshold := time.Now().UTC().Add(-10 * time.Minute)
	h.closeStaleSessionsForUser(ctx, userID, staleThreshold)

	opts := options.Find().
		SetSort(bson.D{{Key: "login_at", Value: -1}}).
		SetLimit(100)

	cur, err := h.DB.Collection("sessions").Find(ctx, bson.M{
		"user_id":  userID,
		"login_at": bson.M{"$gte": since},
	}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var sessions []sessionRecord
	for cur.Next(ctx) {
		var s sessionRecord
		if err := cur.Decode(&s); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}

	return sessions, nil
}

// closeStaleSessionsForUser closes sessions that are open but inactive.
func (h *Handler) closeStaleSessionsForUser(ctx context.Context, userID primitive.ObjectID, threshold time.Time) {
	cur, err := h.DB.Collection("sessions").Find(ctx, bson.M{
		"user_id":        userID,
		"logout_at":      nil,
		"last_active_at": bson.M{"$lt": threshold},
	})
	if err != nil {
		return
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		var s struct {
			ID           primitive.ObjectID `bson:"_id"`
			LoginAt      time.Time          `bson:"login_at"`
			LastActiveAt time.Time          `bson:"last_active_at"`
		}
		if err := cur.Decode(&s); err != nil {
			continue
		}

		// Close the session - use last_active_at as the logout time
		duration := int64(s.LastActiveAt.Sub(s.LoginAt).Seconds())
		_, _ = h.DB.Collection("sessions").UpdateOne(ctx,
			bson.M{"_id": s.ID},
			bson.M{"$set": bson.M{
				"logout_at":     s.LastActiveAt,
				"end_reason":    "inactive",
				"duration_secs": duration,
			}},
		)
	}
}

// getEventsForSessions gets activity events for the given sessions.
func (h *Handler) getEventsForSessions(ctx context.Context, sessionIDs []primitive.ObjectID, userID primitive.ObjectID, since time.Time) ([]activitystore.Event, error) {
	if h.Activity == nil {
		return nil, nil
	}

	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}})

	filter := bson.M{
		"user_id":   userID,
		"timestamp": bson.M{"$gte": since},
	}

	cur, err := h.DB.Collection("activity_events").Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var events []activitystore.Event
	if err := cur.All(ctx, &events); err != nil {
		return nil, err
	}

	return events, nil
}

// buildSessionBlocks organizes sessions and events into display blocks.
// Events are matched to sessions by timestamp range (login_at to logout_at) or by session_id.
// Timestamps are provided in ISO format for client-side timezone formatting.
func (h *Handler) buildSessionBlocks(sessions []sessionRecord, events []activitystore.Event) []sessionBlock {
	// First, try to group events by session_id (direct match)
	eventsBySession := make(map[primitive.ObjectID][]activitystore.Event)
	unmatchedEvents := make([]activitystore.Event, 0)

	// Create a set of valid session IDs for quick lookup
	sessionIDSet := make(map[primitive.ObjectID]bool)
	for _, s := range sessions {
		sessionIDSet[s.ID] = true
	}

	for _, e := range events {
		if sessionIDSet[e.SessionID] {
			// Direct session_id match
			eventsBySession[e.SessionID] = append(eventsBySession[e.SessionID], e)
		} else {
			// No direct match - try to match by timestamp
			unmatchedEvents = append(unmatchedEvents, e)
		}
	}

	// For unmatched events, try to associate them with sessions by timestamp range
	for _, e := range unmatchedEvents {
		matched := false
		for _, s := range sessions {
			// Check if event timestamp falls within session's time range
			sessionEnd := time.Now().UTC()
			if s.LogoutAt != nil {
				sessionEnd = *s.LogoutAt
			}
			if (e.Timestamp.Equal(s.LoginAt) || e.Timestamp.After(s.LoginAt)) &&
				(e.Timestamp.Equal(sessionEnd) || e.Timestamp.Before(sessionEnd)) {
				eventsBySession[s.ID] = append(eventsBySession[s.ID], e)
				matched = true
				break
			}
		}
		// If still no match and there's an active session, add to the first active session
		if !matched {
			for _, s := range sessions {
				if s.LogoutAt == nil { // Active session
					eventsBySession[s.ID] = append(eventsBySession[s.ID], e)
					break
				}
			}
		}
	}

	var blocks []sessionBlock
	for _, s := range sessions {
		// Format times in UTC as fallback (client-side JS will format in selected timezone)
		date := s.LoginAt.UTC().Format("Jan 2, 2006")
		loginTime := s.LoginAt.UTC().Format("3:04 PM")
		loginTimeISO := s.LoginAt.UTC().Format(time.RFC3339)

		logoutTime := ""
		logoutTimeISO := ""
		if s.LogoutAt != nil {
			logoutTime = s.LogoutAt.UTC().Format("3:04 PM")
			logoutTimeISO = s.LogoutAt.UTC().Format(time.RFC3339)
		}

		duration := formatDuration(s.DurationSecs)
		if s.DurationSecs == 0 && s.LogoutAt == nil {
			// Active session
			duration = formatDuration(int64(time.Since(s.LoginAt).Seconds()))
			logoutTime = "(active)"
		}

		endReason := s.EndReason
		if endReason == "" && s.LogoutAt == nil {
			endReason = "active"
		}

		// Build events for this session
		var activityEvents []activityEvent

		// Add login event at the start
		loginDesc := "Logged in"
		loginEventType := "login"
		activityEvents = append(activityEvents, activityEvent{
			Time:        s.LoginAt,
			TimeLabel:   s.LoginAt.UTC().Format("3:04 PM"),
			TimeISO:     s.LoginAt.UTC().Format(time.RFC3339),
			EventType:   loginEventType,
			Description: loginDesc,
		})

		for _, e := range eventsBySession[s.ID] {
			// Skip events that occurred before this session started (orphan events from other sessions)
			if e.Timestamp.Before(s.LoginAt) {
				continue
			}

			ae := activityEvent{
				Time:      e.Timestamp,
				TimeLabel: e.Timestamp.UTC().Format("3:04 PM"),
				TimeISO:   e.Timestamp.UTC().Format(time.RFC3339),
				EventType: e.EventType,
			}

			switch e.EventType {
			case activitystore.EventPageView:
				ae.Description = fmt.Sprintf("Viewed %s", e.PagePath)
			default:
				ae.Description = e.EventType
			}

			activityEvents = append(activityEvents, ae)
		}

		// Add logout event if session ended
		if s.LogoutAt != nil {
			logoutDesc := "Logged out"
			if s.EndReason == "inactive" {
				logoutDesc = "Session timed out"
			}
			activityEvents = append(activityEvents, activityEvent{
				Time:        *s.LogoutAt,
				TimeLabel:   s.LogoutAt.UTC().Format("3:04 PM"),
				TimeISO:     s.LogoutAt.UTC().Format(time.RFC3339),
				EventType:   "logout",
				Description: logoutDesc,
			})
		}

		// Sort events by time (oldest first), then reverse so newest are first
		slices.SortFunc(activityEvents, func(a, b activityEvent) int {
			return a.Time.Compare(b.Time)
		})
		slices.Reverse(activityEvents)

		// For active sessions, add "Last activity" at the very top (after reversal)
		if s.LogoutAt == nil {
			// Use the session's last_active_at time (updated by every heartbeat)
			lastActivityTime := s.LastActiveAt
			if lastActivityTime.IsZero() {
				lastActivityTime = s.LoginAt
			}
			// Prepend status event showing last activity
			idleEvent := activityEvent{
				Time:        lastActivityTime,
				TimeLabel:   lastActivityTime.UTC().Format("3:04 PM"),
				TimeISO:     lastActivityTime.UTC().Format(time.RFC3339),
				EventType:   "idle",
				Description: "Last activity",
			}
			activityEvents = append([]activityEvent{idleEvent}, activityEvents...)
		}

		blocks = append(blocks, sessionBlock{
			Date:          date,
			LoginTime:     loginTime,
			LoginTimeISO:  loginTimeISO,
			LogoutTime:    logoutTime,
			LogoutTimeISO: logoutTimeISO,
			Duration:      duration,
			EndReason:     endReason,
			Events:        activityEvents,
		})
	}

	return blocks
}

// formatDuration formats seconds as a human-readable duration.
func formatDuration(secs int64) string {
	if secs < 60 {
		return fmt.Sprintf("%d sec", secs)
	}
	mins := secs / 60
	if mins < 60 {
		return fmt.Sprintf("%d min", mins)
	}
	hours := mins / 60
	remainingMins := mins % 60
	if remainingMins == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, remainingMins)
}
