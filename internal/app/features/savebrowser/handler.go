package savebrowser

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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

const (
	defaultPlayerLimit = 20
)

// Handler handles save browser HTTP requests.
type Handler struct {
	db           *mongo.Database
	store        *Store
	errLog       *errorsfeature.ErrorLogger
	logger       *zap.Logger
	defaultLimit int
	apiKey       string
}

// NewHandler creates a new save browser handler.
func NewHandler(db *mongo.Database, errLog *errorsfeature.ErrorLogger, defaultLimit int, apiKey string, logger *zap.Logger) *Handler {
	if defaultLimit <= 0 {
		defaultLimit = 10
	}
	return &Handler{
		db:           db,
		store:        NewStore(db, logger),
		errLog:       errLog,
		logger:       logger,
		defaultLimit: defaultLimit,
		apiKey:       apiKey,
	}
}

// ServeList renders the main browser page with game header, players table, and saves.
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
	playerSearch := r.URL.Query().Get("search")
	limitStr := r.URL.Query().Get("limit")
	afterID := r.URL.Query().Get("after")
	beforeID := r.URL.Query().Get("before")
	pageStr := r.URL.Query().Get("page")

	// Default to first game if none selected
	if selectedGame == "" && len(games) > 0 {
		selectedGame = games[0]
	}

	limit := h.defaultLimit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
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
		BaseVM:         viewdata.NewBaseVM(r, h.db, "State Browser", "/dashboard"),
		TimezoneGroups: tzGroups,
		Games:          games,
		SelectedGame:   selectedGame,
		SelectedUser:   selectedUser,
		PlayerSearch:   playerSearch,
		PlayerPage:     page,
		SaveLimit:      limit,
		DefaultLimit:   h.defaultLimit,
	}

	// If game selected, load players with counts
	if selectedGame != "" {
		users, total, err := h.store.ListUsersWithCounts(ctx, selectedGame, playerSearch, page, defaultPlayerLimit)
		if err != nil {
			h.logger.Warn("failed to list users with counts", zap.Error(err))
		} else {
			data.Players = make([]PlayerRowVM, len(users))
			for i, u := range users {
				data.Players[i] = PlayerRowVM{
					UserID:    u.UserID,
					SaveCount: u.SaveCount,
				}
			}
			data.PlayerTotal = total

			// Calculate pagination
			data.PlayerRangeStart = (page-1)*defaultPlayerLimit + 1
			data.PlayerRangeEnd = data.PlayerRangeStart + len(users) - 1
			if data.PlayerRangeEnd > int(total) {
				data.PlayerRangeEnd = int(total)
			}
			if total == 0 {
				data.PlayerRangeStart = 0
				data.PlayerRangeEnd = 0
			}

			data.PlayerHasPrev = page > 1
			data.PlayerHasNext = int64(page*defaultPlayerLimit) < total
			data.PlayerPrevPage = page - 1
			data.PlayerNextPage = page + 1
		}

		// If user selected, load saves
		if selectedUser != "" {
			saves, hasPrev, hasNext, err := h.store.ListSaves(ctx, selectedGame, selectedUser, limit, afterID, beforeID)
			if err != nil {
				h.logger.Warn("failed to list saves", zap.Error(err))
			} else {
				data.Saves = make([]SaveRowVM, len(saves))
				for i, s := range saves {
					jsonBytes, _ := json.MarshalIndent(s.SaveData, "", "  ")
					data.Saves[i] = SaveRowVM{
						ID:        s.ID.Hex(),
						UserID:    s.UserID,
						Game:      s.Game,
						Timestamp: s.Timestamp,
						SaveData:  string(jsonBytes),
					}
				}
				data.HasPrev = hasPrev
				data.HasNext = hasNext

				// Set cursors for pagination
				if len(saves) > 0 {
					data.PrevCursor = saves[0].ID.Hex()
					data.NextCursor = saves[len(saves)-1].ID.Hex()
				}

				// Get total count
				total, err := h.store.CountSaves(ctx, selectedGame, selectedUser)
				if err == nil {
					data.SaveTotal = total
				}
			}
		}
	}

	// Check if HTMX request targeting specific elements
	if r.Header.Get("HX-Request") == "true" {
		target := r.Header.Get("HX-Target")
		switch target {
		case "players-section":
			templates.RenderSnippet(w, "savebrowser/players_partial", PlayersPartialVM{
				SelectedGame:     selectedGame,
				SelectedUser:     selectedUser,
				PlayerSearch:     playerSearch,
				Players:          data.Players,
				PlayerTotal:      data.PlayerTotal,
				PlayerPage:       page,
				PlayerHasPrev:    data.PlayerHasPrev,
				PlayerHasNext:    data.PlayerHasNext,
				PlayerRangeStart: data.PlayerRangeStart,
				PlayerRangeEnd:   data.PlayerRangeEnd,
				PlayerPrevPage:   data.PlayerPrevPage,
				PlayerNextPage:   data.PlayerNextPage,
				Limit:            limit,
			})
			return
		case "saves-section":
			templates.RenderSnippet(w, "savebrowser/saves_partial", SavesPartialVM{
				BaseVM:       data.BaseVM,
				SelectedGame: selectedGame,
				SelectedUser: selectedUser,
				Saves:        data.Saves,
				Total:        data.SaveTotal,
				Limit:        limit,
				HasPrev:      data.HasPrev,
				HasNext:      data.HasNext,
				PrevCursor:   data.PrevCursor,
				NextCursor:   data.NextCursor,
			})
			return
		}
	}

	templates.Render(w, r, "savebrowser/list", data)
}

// ServePlayers handles GET /saves/players - HTMX partial for players table.
func (h *Handler) ServePlayers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	game := r.URL.Query().Get("game")
	search := r.URL.Query().Get("search")
	selectedUser := r.URL.Query().Get("user")
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := h.defaultLimit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	data := PlayersPartialVM{
		SelectedGame: game,
		SelectedUser: selectedUser,
		PlayerSearch: search,
		PlayerPage:   page,
		Limit:        limit,
	}

	if game == "" {
		templates.RenderSnippet(w, "savebrowser/players_partial", data)
		return
	}

	users, total, err := h.store.ListUsersWithCounts(ctx, game, search, page, defaultPlayerLimit)
	if err != nil {
		h.logger.Warn("failed to list users with counts", zap.Error(err))
		templates.RenderSnippet(w, "savebrowser/players_partial", data)
		return
	}

	data.Players = make([]PlayerRowVM, len(users))
	for i, u := range users {
		data.Players[i] = PlayerRowVM{
			UserID:    u.UserID,
			SaveCount: u.SaveCount,
		}
	}
	data.PlayerTotal = total

	// Calculate pagination
	data.PlayerRangeStart = (page-1)*defaultPlayerLimit + 1
	data.PlayerRangeEnd = data.PlayerRangeStart + len(users) - 1
	if data.PlayerRangeEnd > int(total) {
		data.PlayerRangeEnd = int(total)
	}
	if total == 0 {
		data.PlayerRangeStart = 0
		data.PlayerRangeEnd = 0
	}

	data.PlayerHasPrev = page > 1
	data.PlayerHasNext = int64(page*defaultPlayerLimit) < total
	data.PlayerPrevPage = page - 1
	data.PlayerNextPage = page + 1

	templates.RenderSnippet(w, "savebrowser/players_partial", data)
}

// ServeGamePicker handles GET /saves/game-picker - game selector modal.
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
		templates.RenderSnippet(w, "savebrowser/game_picker_list", data)
		return
	}

	templates.RenderSnippet(w, "savebrowser/game_picker", data)
}

// ServeSaves handles GET /saves/data - HTMX partial for save list.
func (h *Handler) ServeSaves(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	game := r.URL.Query().Get("game")
	user := r.URL.Query().Get("user")
	limitStr := r.URL.Query().Get("limit")
	afterID := r.URL.Query().Get("after")
	beforeID := r.URL.Query().Get("before")

	limit := h.defaultLimit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	data := SavesPartialVM{
		BaseVM:       viewdata.NewBaseVM(r, h.db, "", ""),
		SelectedGame: game,
		SelectedUser: user,
		Limit:        limit,
	}

	if game == "" || user == "" {
		templates.RenderSnippet(w, "savebrowser/saves_partial", data)
		return
	}

	saves, hasPrev, hasNext, err := h.store.ListSaves(ctx, game, user, limit, afterID, beforeID)
	if err != nil {
		h.logger.Warn("failed to list saves", zap.Error(err))
		templates.RenderSnippet(w, "savebrowser/saves_partial", data)
		return
	}

	data.Saves = make([]SaveRowVM, len(saves))
	for i, s := range saves {
		jsonBytes, _ := json.MarshalIndent(s.SaveData, "", "  ")
		data.Saves[i] = SaveRowVM{
			ID:        s.ID.Hex(),
			UserID:    s.UserID,
			Game:      s.Game,
			Timestamp: s.Timestamp,
			SaveData:  string(jsonBytes),
		}
	}
	data.HasPrev = hasPrev
	data.HasNext = hasNext

	if len(saves) > 0 {
		data.PrevCursor = saves[0].ID.Hex()
		data.NextCursor = saves[len(saves)-1].ID.Hex()
	}

	total, err := h.store.CountSaves(ctx, game, user)
	if err == nil {
		data.Total = total
	}

	templates.RenderSnippet(w, "savebrowser/saves_partial", data)
}

// ServeFullPage handles GET /saves - renders full page for pushState navigation.
func (h *Handler) ServeFullPage(w http.ResponseWriter, r *http.Request) {
	// Just redirect to ServeList which handles full page rendering
	h.ServeList(w, r)
}

// HandleDeleteSave handles POST /saves/{game}/{id}/delete - delete a single save.
func (h *Handler) HandleDeleteSave(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Short())
	defer cancel()

	game := chi.URLParam(r, "game")
	idStr := chi.URLParam(r, "id")

	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "Invalid save ID", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteSave(ctx, game, id); err != nil {
		h.errLog.Log(r, "failed to delete save", err)
		http.Error(w, "Failed to delete save", http.StatusInternalServerError)
		return
	}

	h.logger.Info("save deleted",
		zap.String("game", game),
		zap.String("id", idStr),
	)

	// Return success - the client will refresh the list
	w.Header().Set("HX-Trigger", "save-deleted")
	w.WriteHeader(http.StatusOK)
}

// HandleCreateState handles POST /console/api/state/create - create test state.
func (h *Handler) HandleCreateState(w http.ResponseWriter, r *http.Request) {
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
	var data map[string]interface{}
	if dataStr != "" {
		if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
			http.Error(w, "Invalid JSON data", http.StatusBadRequest)
			return
		}
	} else {
		data = map[string]interface{}{}
	}

	if err := h.store.CreateState(ctx, game, userID, data); err != nil {
		h.errLog.Log(r, "failed to create state", err)
		http.Error(w, "Failed to create state", http.StatusInternalServerError)
		return
	}

	h.logger.Info("state created",
		zap.String("game", game),
		zap.String("user_id", userID),
	)

	// Trigger refresh and close modal
	w.Header().Set("HX-Trigger", "state-created")
	w.WriteHeader(http.StatusOK)
}

// HandleDeleteUserSaves handles POST /saves/{game}/user/{userID}/delete - delete all saves for user.
func (h *Handler) HandleDeleteUserSaves(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeouts.Medium())
	defer cancel()

	game := chi.URLParam(r, "game")
	userID := chi.URLParam(r, "userID")

	count, err := h.store.DeleteUserSaves(ctx, game, userID)
	if err != nil {
		h.errLog.Log(r, "failed to delete user saves", err)
		http.Error(w, "Failed to delete saves", http.StatusInternalServerError)
		return
	}

	h.logger.Info("user saves deleted",
		zap.String("game", game),
		zap.String("user_id", userID),
		zap.Int64("count", count),
	)

	// Return success - the client will refresh
	w.Header().Set("HX-Trigger", "saves-deleted")
	w.WriteHeader(http.StatusOK)
}

