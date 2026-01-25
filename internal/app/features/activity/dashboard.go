// internal/app/features/activity/dashboard.go
package activity

// Terminology: User Identifiers
//   - UserID / userID / user_id: The MongoDB ObjectID (_id) that uniquely identifies a user record
//   - LoginID / loginID / login_id: The human-readable string users type to log in

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dalemusser/stratasave/internal/app/system/timeouts"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/waffle/pantry/query"
	"github.com/dalemusser/waffle/pantry/templates"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const pageSize = 25

// ServeDashboard renders the real-time activity dashboard.
// GET /activity
func (h *Handler) ServeDashboard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	// Parse query parameters
	statusFilter := query.Get(r, "status")
	if statusFilter == "" {
		statusFilter = "all"
	}
	searchQuery := query.Get(r, "search")
	sortBy := query.Get(r, "sort")
	if sortBy == "" {
		sortBy = "name"
	}
	sortDir := query.Get(r, "dir")
	if sortDir == "" {
		sortDir = "asc"
	}
	page := 1
	if p := query.Get(r, "page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}

	now := time.Now().UTC()

	// Fetch all users with activity status
	allUsers, err := h.fetchAllUsersWithActivity(ctx, now)
	if err != nil {
		h.ErrLog.Log(r, "failed to fetch users", err)
		http.Error(w, "A database error occurred", http.StatusInternalServerError)
		return
	}

	// Count statuses (before filtering)
	var onlineCount, idleCount, offlineCount int
	for _, u := range allUsers {
		switch u.Status {
		case StatusOnline:
			onlineCount++
		case StatusIdle:
			idleCount++
		case StatusOffline:
			offlineCount++
		}
	}

	// Filter by search query
	filteredUsers := filterUsersBySearch(allUsers, searchQuery)

	// Filter by status
	filteredUsers = filterUsersByStatus(filteredUsers, statusFilter)

	// Sort users
	sortUsers(filteredUsers, sortBy, sortDir)

	// Paginate
	total := len(filteredUsers)
	startIdx := (page - 1) * pageSize
	endIdx := startIdx + pageSize
	if startIdx > total {
		startIdx = total
	}
	if endIdx > total {
		endIdx = total
	}
	pagedUsers := filteredUsers[startIdx:endIdx]

	// Calculate pagination info
	rangeStart := 0
	rangeEnd := 0
	if total > 0 {
		rangeStart = startIdx + 1
		rangeEnd = endIdx
	}

	data := dashboardData{
		BaseVM:       viewdata.NewBaseVM(r, h.DB, "Activity Dashboard", "/"),
		StatusFilter: statusFilter,
		SearchQuery:  searchQuery,
		SortBy:       sortBy,
		SortDir:      sortDir,
		Page:         page,
		Total:        total,
		RangeStart:   rangeStart,
		RangeEnd:     rangeEnd,
		HasPrev:      page > 1,
		HasNext:      endIdx < total,
		PrevPage:     page - 1,
		NextPage:     page + 1,
		TotalUsers:   len(allUsers),
		OnlineCount:  onlineCount,
		IdleCount:    idleCount,
		OfflineCount: offlineCount,
		Users:        pagedUsers,
	}

	templates.Render(w, r, "activity_dashboard", data)
}

// ServeOnlineTable renders just the users table for HTMX refresh.
// GET /activity/online-table
func (h *Handler) ServeOnlineTable(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	// Parse query parameters
	statusFilter := query.Get(r, "status")
	if statusFilter == "" {
		statusFilter = "all"
	}
	searchQuery := query.Get(r, "search")
	sortBy := query.Get(r, "sort")
	if sortBy == "" {
		sortBy = "name"
	}
	sortDir := query.Get(r, "dir")
	if sortDir == "" {
		sortDir = "asc"
	}
	page := 1
	if p := query.Get(r, "page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}

	now := time.Now().UTC()

	// Fetch all users with activity status
	allUsers, err := h.fetchAllUsersWithActivity(ctx, now)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Count statuses (before filtering)
	var onlineCount, idleCount, offlineCount int
	for _, u := range allUsers {
		switch u.Status {
		case StatusOnline:
			onlineCount++
		case StatusIdle:
			idleCount++
		case StatusOffline:
			offlineCount++
		}
	}

	// Filter by search query
	filteredUsers := filterUsersBySearch(allUsers, searchQuery)

	// Filter by status
	filteredUsers = filterUsersByStatus(filteredUsers, statusFilter)

	// Sort users
	sortUsers(filteredUsers, sortBy, sortDir)

	// Paginate
	total := len(filteredUsers)
	startIdx := (page - 1) * pageSize
	endIdx := startIdx + pageSize
	if startIdx > total {
		startIdx = total
	}
	if endIdx > total {
		endIdx = total
	}
	pagedUsers := filteredUsers[startIdx:endIdx]

	// Calculate pagination info
	rangeStart := 0
	rangeEnd := 0
	if total > 0 {
		rangeStart = startIdx + 1
		rangeEnd = endIdx
	}

	data := dashboardData{
		BaseVM:       viewdata.NewBaseVM(r, h.DB, "Activity Dashboard", "/"),
		StatusFilter: statusFilter,
		SearchQuery:  searchQuery,
		SortBy:       sortBy,
		SortDir:      sortDir,
		Page:         page,
		Total:        total,
		RangeStart:   rangeStart,
		RangeEnd:     rangeEnd,
		HasPrev:      page > 1,
		HasNext:      endIdx < total,
		PrevPage:     page - 1,
		NextPage:     page + 1,
		TotalUsers:   len(allUsers),
		OnlineCount:  onlineCount,
		IdleCount:    idleCount,
		OfflineCount: offlineCount,
		Users:        pagedUsers,
	}

	templates.Render(w, r, "activity_online_table", data)
}

// fetchAllUsersWithActivity gets all active users with their activity status.
func (h *Handler) fetchAllUsersWithActivity(ctx context.Context, now time.Time) ([]userRow, error) {
	// Query all active users
	cur, err := h.DB.Collection("users").Find(ctx, bson.M{"status": "active"})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	type userInfo struct {
		UserID  primitive.ObjectID
		Name    string
		LoginID string
		Email   string
		Role    string
	}
	var users []userInfo

	for cur.Next(ctx) {
		var doc struct {
			ID       primitive.ObjectID `bson:"_id"`
			FullName string             `bson:"full_name"`
			LoginID  string             `bson:"login_id"`
			Email    string             `bson:"email"`
			Role     string             `bson:"role"`
		}
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		users = append(users, userInfo{
			UserID:  doc.ID,
			Name:    doc.FullName,
			LoginID: doc.LoginID,
			Email:   doc.Email,
			Role:    doc.Role,
		})
	}

	if len(users) == 0 {
		return nil, nil
	}

	// Get user IDs for session lookup
	var userIDs []primitive.ObjectID
	for _, u := range users {
		userIDs = append(userIDs, u.UserID)
	}

	// Get active sessions for these users
	sessionMap, err := h.getActiveSessionsForUsers(ctx, userIDs, now)
	if err != nil {
		return nil, err
	}

	// Get today's activity for these users
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	todayActivity, err := h.getTodayActivityForUsers(ctx, userIDs, todayStart, now)
	if err != nil {
		return nil, err
	}

	// Build user rows
	var result []userRow
	for _, u := range users {
		// Determine status
		status := StatusOffline
		statusLabel := "Offline"
		var lastActive *time.Time
		currentPage := ""

		if sess, ok := sessionMap[u.UserID]; ok {
			lastActive = &sess.LastActiveAt
			currentPage = sess.CurrentPage
			timeSince := now.Sub(sess.LastActiveAt)
			if timeSince < OnlineThreshold {
				status = StatusOnline
				statusLabel = "Active"
			} else if timeSince < IdleThreshold {
				status = StatusIdle
				statusLabel = "Idle"
			}
		}

		// Get current activity
		currentActivity := ""
		if status != StatusOffline {
			if currentPage != "" {
				currentActivity = formatPageName(currentPage)
			} else {
				currentActivity = "Dashboard"
			}
		}

		// Get time today
		timeTodayMins := 0
		if mins, ok := todayActivity[u.UserID]; ok {
			timeTodayMins = mins
		}

		// Format time today
		timeTodayStr := "0 min"
		if timeTodayMins > 0 {
			if timeTodayMins >= 60 {
				timeTodayStr = fmt.Sprintf("%dh %dm", timeTodayMins/60, timeTodayMins%60)
			} else {
				timeTodayStr = fmt.Sprintf("%d min", timeTodayMins)
			}
		}

		result = append(result, userRow{
			ID:              u.UserID.Hex(),
			Name:            u.Name,
			LoginID:         u.LoginID,
			Email:           u.Email,
			Role:            u.Role,
			Status:          status,
			StatusLabel:     statusLabel,
			CurrentActivity: currentActivity,
			TimeTodayMins:   timeTodayMins,
			TimeTodayStr:    timeTodayStr,
			LastActiveAt:    lastActive,
		})
	}

	return result, nil
}

// sessionInfo holds minimal session data for status calculation.
type sessionInfo struct {
	LastActiveAt time.Time
	CurrentPage  string
}

// getActiveSessionsForUsers gets the most recent active session for each user.
func (h *Handler) getActiveSessionsForUsers(ctx context.Context, userIDs []primitive.ObjectID, now time.Time) (map[primitive.ObjectID]sessionInfo, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	pipeline := []bson.M{
		{"$match": bson.M{
			"user_id":   bson.M{"$in": userIDs},
			"logout_at": nil,
		}},
		{"$sort": bson.M{"last_activity": -1}},
		{"$group": bson.M{
			"_id":           "$user_id",
			"last_activity": bson.M{"$first": "$last_activity"},
			"current_page":  bson.M{"$first": "$current_page"},
		}},
	}

	cur, err := h.DB.Collection("sessions").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	result := make(map[primitive.ObjectID]sessionInfo)
	for cur.Next(ctx) {
		var doc struct {
			ID           primitive.ObjectID `bson:"_id"`
			LastActivity time.Time          `bson:"last_activity"`
			CurrentPage  string             `bson:"current_page"`
		}
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		result[doc.ID] = sessionInfo{LastActiveAt: doc.LastActivity, CurrentPage: doc.CurrentPage}
	}

	return result, nil
}

// getTodayActivityForUsers calculates total active minutes for each user today.
func (h *Handler) getTodayActivityForUsers(ctx context.Context, userIDs []primitive.ObjectID, todayStart, now time.Time) (map[primitive.ObjectID]int, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	pipeline := []bson.M{
		{"$match": bson.M{
			"user_id":  bson.M{"$in": userIDs},
			"login_at": bson.M{"$gte": todayStart},
		}},
		{"$project": bson.M{
			"user_id": 1,
			"duration_mins": bson.M{
				"$cond": bson.M{
					"if": bson.M{"$ne": bson.A{"$logout_at", nil}},
					"then": bson.M{"$divide": bson.A{
						bson.M{"$subtract": bson.A{"$logout_at", "$login_at"}},
						60000, // ms to minutes
					}},
					"else": bson.M{"$divide": bson.A{
						bson.M{"$subtract": bson.A{now, "$login_at"}},
						60000,
					}},
				},
			},
		}},
		{"$group": bson.M{
			"_id":        "$user_id",
			"total_mins": bson.M{"$sum": "$duration_mins"},
		}},
	}

	cur, err := h.DB.Collection("sessions").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	result := make(map[primitive.ObjectID]int)
	for cur.Next(ctx) {
		var doc struct {
			ID        primitive.ObjectID `bson:"_id"`
			TotalMins float64            `bson:"total_mins"`
		}
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		result[doc.ID] = int(doc.TotalMins)
	}

	return result, nil
}

// filterUsersBySearch filters users by name (case-insensitive prefix match).
func filterUsersBySearch(users []userRow, searchQuery string) []userRow {
	if searchQuery == "" {
		return users
	}

	query := strings.ToLower(searchQuery)
	var filtered []userRow
	for _, u := range users {
		if strings.HasPrefix(strings.ToLower(u.Name), query) ||
			strings.HasPrefix(strings.ToLower(u.LoginID), query) {
			filtered = append(filtered, u)
		}
	}
	return filtered
}

// filterUsersByStatus filters users by their online status.
func filterUsersByStatus(users []userRow, statusFilter string) []userRow {
	if statusFilter == "all" || statusFilter == "" {
		return users
	}

	var filtered []userRow
	for _, u := range users {
		switch statusFilter {
		case "online":
			if u.Status == StatusOnline {
				filtered = append(filtered, u)
			}
		case "idle":
			if u.Status == StatusIdle {
				filtered = append(filtered, u)
			}
		case "offline":
			if u.Status == StatusOffline {
				filtered = append(filtered, u)
			}
		}
	}
	return filtered
}

// sortUsers sorts users by the specified field and direction.
func sortUsers(users []userRow, sortBy, sortDir string) {
	sort.Slice(users, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "time":
			// Sort by time today (default descending - longest first)
			if users[i].TimeTodayMins != users[j].TimeTodayMins {
				less = users[i].TimeTodayMins > users[j].TimeTodayMins
			} else {
				less = strings.ToLower(users[i].Name) < strings.ToLower(users[j].Name)
			}
			if sortDir == "asc" {
				return !less
			}
			return less
		default: // "name"
			less = strings.ToLower(users[i].Name) < strings.ToLower(users[j].Name)
		}

		if sortBy != "time" && sortDir == "desc" {
			return !less
		}
		return less
	})
}

// formatPageName converts a URL path to a readable page name.
func formatPageName(path string) string {
	pageNames := map[string]string{
		"/":                 "Dashboard",
		"/dashboard":        "Dashboard",
		"/profile":          "Profile",
		"/settings":         "Settings",
		"/about":            "About",
		"/contact":          "Contact",
		"/terms":            "Terms",
		"/privacy":          "Privacy",
		"/activity":         "Activity",
		"/activity/summary": "Activity Summary",
		"/activity/export":  "Activity Export",
		"/pages":            "Pages",
		"/announcements":    "Announcements",
		"/users":            "Users",
		"/auditlog":         "Audit Log",
		"/status":           "Status",
	}

	if name, ok := pageNames[path]; ok {
		return name
	}

	// Check for prefix matches
	prefixes := map[string]string{
		"/activity/user/":   "User Activity",
		"/activity/export/": "Activity Export",
		"/pages/":           "Pages",
		"/users/":           "Users",
	}
	for prefix, name := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return name
		}
	}

	// For unknown paths, try to make them readable
	if len(path) > 1 {
		path = path[1:]
		if idx := strings.LastIndex(path, "/"); idx > 0 {
			path = path[idx+1:]
		}
		if len(path) > 0 {
			return strings.ToUpper(path[:1]) + path[1:]
		}
	}

	return "Dashboard"
}
