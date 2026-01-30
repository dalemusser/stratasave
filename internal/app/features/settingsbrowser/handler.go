package settingsbrowser

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	errorsfeature "github.com/dalemusser/stratasave/internal/app/features/errors"
	"github.com/dalemusser/stratasave/internal/app/system/timeouts"
	"github.com/dalemusser/stratasave/internal/app/system/timezones"
	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/waffle/pantry/templates"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

const (
	defaultUserLimit = 20
)

// Handler handles settings browser HTTP requests.
type Handler struct {
	db     *mongo.Database
	store  *Store
	errLog *errorsfeature.ErrorLogger
	logger *zap.Logger
	apiKey string
}

// NewHandler creates a new settings browser handler.
func NewHandler(db *mongo.Database, errLog *errorsfeature.ErrorLogger, apiKey string, logger *zap.Logger) *Handler {
	return &Handler{
		db:     db,
		store:  NewStore(db, logger),
		errLog: errLog,
		logger: logger,
		apiKey: apiKey,
	}
}

// ServeList renders the main browser page.
func (h *Handler) ServeList(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	// Load games
	games, err := h.store.ListGames(ctx)
	if err != nil {
		h.errLog.Log(r, "failed to list games", err)
		http.Error(w, "Failed to load games", http.StatusInternalServerError)
		return
	}

	// Parse query params
	selectedGame := r.URL.Query().Get("game")
	selectedUser := r.URL.Query().Get("user")
	userSearch := r.URL.Query().Get("search")
	pageStr := r.URL.Query().Get("page")

	// Default to first game if none selected
	if selectedGame == "" && len(games) > 0 {
		selectedGame = games[0]
	}

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	// Load timezone groups
	tzGroups, _ := timezones.Groups()

	data := ListVM{
		BaseVM:         viewdata.NewBaseVM(r, h.db, "Settings Browser", "/dashboard"),
		TimezoneGroups: tzGroups,
		Games:          games,
		SelectedGame:   selectedGame,
		SelectedUser:   selectedUser,
		UserSearch:     userSearch,
		UserPage:       page,
	}

	// If game selected, load users
	if selectedGame != "" {
		users, total, err := h.store.ListUsers(ctx, selectedGame, userSearch, page, defaultUserLimit)
		if err != nil {
			h.logger.Warn("failed to list users", zap.Error(err))
		} else {
			data.Users = users
			data.UserTotal = total

			// Calculate pagination
			data.UserRangeStart = (page-1)*defaultUserLimit + 1
			data.UserRangeEnd = data.UserRangeStart + len(users) - 1
			if data.UserRangeEnd > int(total) {
				data.UserRangeEnd = int(total)
			}
			if total == 0 {
				data.UserRangeStart = 0
				data.UserRangeEnd = 0
			}

			data.UserHasPrev = page > 1
			data.UserHasNext = int64(page*defaultUserLimit) < total
			data.UserPrevPage = page - 1
			data.UserNextPage = page + 1
		}

		// If user selected, load setting
		if selectedUser != "" {
			setting, err := h.store.GetSetting(ctx, selectedGame, selectedUser)
			if err != nil {
				h.logger.Warn("failed to get setting", zap.Error(err))
			} else if setting != nil {
				jsonBytes, _ := json.MarshalIndent(setting.SettingsData, "", "  ")
				data.Setting = &SettingVM{
					ID:           setting.ID.Hex(),
					UserID:       setting.UserID,
					Game:         setting.Game,
					Timestamp:    setting.Timestamp,
					SettingsData: string(jsonBytes),
				}
			}
		}
	}

	// Check if HTMX request targeting specific elements
	if r.Header.Get("HX-Request") == "true" {
		target := r.Header.Get("HX-Target")
		switch target {
		case "users-section":
			templates.RenderSnippet(w, "settingsbrowser/users_partial", UsersPartialVM{
				SelectedGame:   selectedGame,
				SelectedUser:   selectedUser,
				UserSearch:     userSearch,
				Users:          data.Users,
				UserTotal:      data.UserTotal,
				UserPage:       page,
				UserHasPrev:    data.UserHasPrev,
				UserHasNext:    data.UserHasNext,
				UserRangeStart: data.UserRangeStart,
				UserRangeEnd:   data.UserRangeEnd,
				UserPrevPage:   data.UserPrevPage,
				UserNextPage:   data.UserNextPage,
			})
			return
		case "setting-section":
			templates.RenderSnippet(w, "settingsbrowser/setting_partial", SettingPartialVM{
				BaseVM:       data.BaseVM,
				SelectedGame: selectedGame,
				SelectedUser: selectedUser,
				Setting:      data.Setting,
			})
			return
		}
	}

	templates.Render(w, r, "settingsbrowser/list", data)
}

// ServeUsers handles GET /console/api/settings/users - HTMX partial for users table.
func (h *Handler) ServeUsers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	game := r.URL.Query().Get("game")
	search := r.URL.Query().Get("search")
	selectedUser := r.URL.Query().Get("user")
	pageStr := r.URL.Query().Get("page")

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	data := UsersPartialVM{
		SelectedGame: game,
		SelectedUser: selectedUser,
		UserSearch:   search,
		UserPage:     page,
	}

	if game == "" {
		templates.RenderSnippet(w, "settingsbrowser/users_partial", data)
		return
	}

	users, total, err := h.store.ListUsers(ctx, game, search, page, defaultUserLimit)
	if err != nil {
		h.logger.Warn("failed to list users", zap.Error(err))
		templates.RenderSnippet(w, "settingsbrowser/users_partial", data)
		return
	}

	data.Users = users
	data.UserTotal = total

	// Calculate pagination
	data.UserRangeStart = (page-1)*defaultUserLimit + 1
	data.UserRangeEnd = data.UserRangeStart + len(users) - 1
	if data.UserRangeEnd > int(total) {
		data.UserRangeEnd = int(total)
	}
	if total == 0 {
		data.UserRangeStart = 0
		data.UserRangeEnd = 0
	}

	data.UserHasPrev = page > 1
	data.UserHasNext = int64(page*defaultUserLimit) < total
	data.UserPrevPage = page - 1
	data.UserNextPage = page + 1

	templates.RenderSnippet(w, "settingsbrowser/users_partial", data)
}

// ServeGamePicker handles GET /console/api/settings/game-picker - game selector modal.
func (h *Handler) ServeGamePicker(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	selectedGame := r.URL.Query().Get("selected")
	query := r.URL.Query().Get("q")

	// Load games
	games, err := h.store.ListGames(ctx)
	if err != nil {
		h.logger.Warn("failed to list games", zap.Error(err))
		games = []string{}
	}

	// Filter games by query if provided
	var filteredGames []GamePickerItem
	queryLower := strings.ToLower(query)
	for _, g := range games {
		if query == "" || strings.Contains(strings.ToLower(g), queryLower) {
			filteredGames = append(filteredGames, GamePickerItem{
				Name:     g,
				Selected: g == selectedGame,
			})
		}
	}

	data := GamePickerVM{
		Games:      filteredGames,
		SelectedID: selectedGame,
		Query:      query,
	}

	// If HTMX request targeting just the list, render only the list portion
	if r.Header.Get("HX-Target") == "game-list" {
		templates.RenderSnippet(w, "settingsbrowser/game_picker_list", data)
		return
	}

	templates.RenderSnippet(w, "settingsbrowser/game_picker", data)
}

// ServeSetting handles GET /console/api/settings/data - HTMX partial for setting view.
func (h *Handler) ServeSetting(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	game := r.URL.Query().Get("game")
	user := r.URL.Query().Get("user")

	data := SettingPartialVM{
		BaseVM:       viewdata.NewBaseVM(r, h.db, "", ""),
		SelectedGame: game,
		SelectedUser: user,
	}

	if game == "" || user == "" {
		templates.RenderSnippet(w, "settingsbrowser/setting_partial", data)
		return
	}

	setting, err := h.store.GetSetting(ctx, game, user)
	if err != nil {
		h.logger.Warn("failed to get setting", zap.Error(err))
		templates.RenderSnippet(w, "settingsbrowser/setting_partial", data)
		return
	}

	if setting != nil {
		jsonBytes, _ := json.MarshalIndent(setting.SettingsData, "", "  ")
		data.Setting = &SettingVM{
			ID:           setting.ID.Hex(),
			UserID:       setting.UserID,
			Game:         setting.Game,
			Timestamp:    setting.Timestamp,
			SettingsData: string(jsonBytes),
		}
	}

	templates.RenderSnippet(w, "settingsbrowser/setting_partial", data)
}

// HandleDeleteSetting handles POST /console/api/settings/{game}/user/{userID}/delete.
func (h *Handler) HandleDeleteSetting(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Short())
	defer cancel()

	game := chi.URLParam(r, "game")
	userID := chi.URLParam(r, "userID")

	if err := h.store.DeleteSetting(ctx, game, userID); err != nil {
		h.errLog.Log(r, "failed to delete setting", err)
		http.Error(w, "Failed to delete setting", http.StatusInternalServerError)
		return
	}

	h.logger.Info("setting deleted",
		zap.String("game", game),
		zap.String("user_id", userID),
	)

	// Return success - the client will refresh the list
	w.Header().Set("HX-Trigger", "setting-deleted")
	w.WriteHeader(http.StatusOK)
}

// HandleCreateSetting handles POST /console/api/settings/create - create test setting.
func (h *Handler) HandleCreateSetting(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Short())
	defer cancel()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	game := r.FormValue("game")
	userID := r.FormValue("user_id")
	dataStr := r.FormValue("data")

	if game == "" || userID == "" {
		http.Error(w, "Game and User ID are required", http.StatusBadRequest)
		return
	}

	// Parse JSON data
	var data bson.M
	if dataStr != "" {
		if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
			http.Error(w, "Invalid JSON data", http.StatusBadRequest)
			return
		}
	} else {
		data = bson.M{}
	}

	if err := h.store.CreateSetting(ctx, game, userID, data); err != nil {
		h.errLog.Log(r, "failed to create setting", err)
		http.Error(w, "Failed to create setting", http.StatusInternalServerError)
		return
	}

	h.logger.Info("setting created",
		zap.String("game", game),
		zap.String("user_id", userID),
	)

	// Trigger refresh and close modal
	w.Header().Set("HX-Trigger", "setting-created")
	w.WriteHeader(http.StatusOK)
}
