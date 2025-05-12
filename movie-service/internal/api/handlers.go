// movie-service/internal/api/handlers.go
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv" // <--- РАСКОММЕНТИРОВАН для GetMovies
	"time"

	"movie-service/internal/domain"
	"movie-service/internal/store"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/lib/pq" // Для pq.StringArray
)

// MovieHandler содержит зависимости для HTTP обработчиков MovieService
type MovieHandler struct {
	store     store.MovieStore
	logger    *slog.Logger
	validator *validator.Validate
}

// NewMovieHandler создает новый экземпляр MovieHandler.
func NewMovieHandler(s store.MovieStore, l *slog.Logger, v *validator.Validate) *MovieHandler {
	return &MovieHandler{
		store:     s,
		logger:    l,
		validator: v,
	}
}

// --- Вспомогательные функции ---
func (h *MovieHandler) respondJSON(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			h.logger.ErrorContext(r.Context(), "Failed to encode JSON response", slog.String("error", err.Error()), slog.String("path", r.URL.Path))
		}
	}
}

func (h *MovieHandler) respondError(w http.ResponseWriter, r *http.Request, status int, message string) {
	h.respondJSON(w, r, status, map[string]string{"error": message})
}

// --- Обработчики ---

// CreateMovie обрабатывает запрос на создание нового фильма.
func (h *MovieHandler) CreateMovie(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var submittedUserIDStr string
	submittedUserIDStr = uuid.Nil.String() // Используем nil UUID ("00000000-0000-0000-0000-000000000000")

	h.logger.InfoContext(ctx, "HTTP CreateMovie request received", slog.String("path", r.URL.Path))

	var req domain.CreateMovieRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.ErrorContext(ctx, "Failed to decode movie creation request body", slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	if err := h.validator.StructCtx(ctx, req); err != nil {
		h.logger.ErrorContext(ctx, "Movie creation request validation failed", slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusBadRequest, "Validation failed: "+err.Error())
		return
	}

	h.logger.DebugContext(ctx, "Decoded request payload for movie", slog.Any("request_data", req))

	newMovie := &domain.Movie{
		ID:                uuid.NewString(),
		Title:             req.Title,
		Description:       req.Description,
		ReleaseYear:       req.ReleaseYear,
		Director:          req.Director,
		Genres:            pq.StringArray(req.Genres),
		Cast:              pq.StringArray(req.Cast),
		PosterURL:         req.PosterURL,
		TrailerURL:        req.TrailerURL,
		SubmittedByUserID: submittedUserIDStr,
		Status:            domain.StatusPendingApproval,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}

	h.logger.DebugContext(ctx, "Movie object before storing", slog.Any("movie_to_store", newMovie))

	if err := h.store.Create(ctx, newMovie); err != nil {
		h.logger.ErrorContext(ctx, "Failed to create movie in store", slog.String("error", err.Error()))
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			h.respondError(w, r, http.StatusConflict, "Movie with this title or other unique field might already exist.")
		} else if errors.Is(err, store.ErrMovieAlreadyExists) {
			h.respondError(w, r, http.StatusConflict, "Movie with this title might already exist (store error).")
		} else {
			h.respondError(w, r, http.StatusInternalServerError, "Failed to create movie")
		}
		return
	}
	h.respondJSON(w, r, http.StatusCreated, newMovie)
}

// GetMovies возвращает список одобренных фильмов с пагинацией и фильтрацией.
func (h *MovieHandler) GetMovies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	queryParams := r.URL.Query()
	h.logger.InfoContext(ctx, "GetMovies endpoint hit", slog.String("query", queryParams.Encode()))

	// Параметры пагинации
	page, _ := strconv.Atoi(queryParams.Get("page"))
	if page <= 0 {
		page = 1 // Страница по умолчанию
	}
	pageSize, _ := strconv.Atoi(queryParams.Get("limit")) // Используем 'limit' как page_size
	if pageSize <= 0 {
		pageSize = 10 // Размер страницы по умолчанию
	} else if pageSize > 100 {
		pageSize = 100 // Максимальный размер страницы
	}

	// Параметры фильтрации и сортировки
	params := store.MovieListParams{
		Page:        page,
		PageSize:    pageSize,
		Genre:       queryParams.Get("genre"),
		SearchQuery: queryParams.Get("search"),
		SortBy:      queryParams.Get("sort_by"),
		Status:      domain.StatusApproved, // Для публичного списка всегда только одобренные фильмы
	}
	if yearStr := queryParams.Get("year"); yearStr != "" {
		if yearVal, err := strconv.Atoi(yearStr); err == nil {
			params.Year = yearVal
		}
	}

	movies, totalCount, err := h.store.List(ctx, params)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to list movies from store", slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusInternalServerError, "Failed to retrieve movies")
		return
	}

	// Формируем ответ с пагинацией
	response := struct {
		Movies     []*domain.Movie `json:"movies"`
		TotalCount int             `json:"total_count"`
		Page       int             `json:"page"`
		PageSize   int             `json:"page_size"`
	}{
		Movies:     movies,
		TotalCount: totalCount,
		Page:       params.Page,
		PageSize:   params.PageSize,
	}

	h.logger.InfoContext(ctx, "Movies list retrieved successfully", slog.Int("count_returned", len(movies)), slog.Int("total_available", totalCount))
	h.respondJSON(w, r, http.StatusOK, response)
}

// GetMovieByID получает фильм по ID (теперь должен работать с PostgreSQL)
func (h *MovieHandler) GetMovieByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	movieID := vars["movieId"]
	ctx := r.Context()
	h.logger.InfoContext(ctx, "GetMovieByID endpoint hit", slog.String("movieID", movieID))

	movie, err := h.store.GetByID(ctx, movieID)
	if err != nil {
		if errors.Is(err, store.ErrMovieNotFound) {
			h.respondError(w, r, http.StatusNotFound, "Movie not found")
		} else {
			h.logger.ErrorContext(ctx, "Error finding movie by ID", slog.String("movieID", movieID), slog.String("error", err.Error()))
			h.respondError(w, r, http.StatusInternalServerError, "Error finding movie")
		}
		return
	}

	// Для публичного эндпоинта показываем только одобренные фильмы
	if movie.Status != domain.StatusApproved {
		h.logger.WarnContext(ctx, "Attempt to access non-approved movie publicly via GetMovieByID", slog.String("movieID", movieID), slog.String("status", string(movie.Status)))
		h.respondError(w, r, http.StatusNotFound, "Movie not found") // Скрываем факт существования неодобренных
		return
	}

	h.respondJSON(w, r, http.StatusOK, movie)
}

// GetPendingMovies - ЗАГЛУШКА (но можно реализовать аналогично GetMovies)
func (h *MovieHandler) GetPendingMovies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h.logger.InfoContext(ctx, "GetPendingMovies endpoint hit (STUB)")
	// TODO: Реализовать получение списка фильмов со статусом pending_approval
	// params := store.MovieListParams{ Status: domain.StatusPendingApproval, ... }
	// movies, _, err := h.store.List(ctx, params)
	// if err != nil { ... }
	h.respondJSON(w, r, http.StatusOK, []domain.Movie{})
}

// ApproveMovie использует h.store.UpdateStatus
func (h *MovieHandler) ApproveMovie(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	movieID := vars["movieId"]
	ctx := r.Context()
	h.logger.InfoContext(ctx, "ApproveMovie endpoint hit", slog.String("movieID", movieID))

	_, err := h.store.GetByID(ctx, movieID)
	if err != nil {
		if errors.Is(err, store.ErrMovieNotFound) {
			h.respondError(w, r, http.StatusNotFound, "Movie not found, cannot approve")
		} else {
			h.logger.ErrorContext(ctx, "Error finding movie for approval", slog.String("movieID", movieID), slog.String("error", err.Error()))
			h.respondError(w, r, http.StatusInternalServerError, "Error finding movie before approval")
		}
		return
	}

	if err := h.store.UpdateStatus(ctx, movieID, domain.StatusApproved); err != nil {
		h.logger.ErrorContext(ctx, "Failed to update movie status for approval", slog.String("movieID", movieID), slog.String("error", err.Error()))
		if errors.Is(err, store.ErrMovieNotFound) {
			h.respondError(w, r, http.StatusNotFound, "Movie not found, cannot approve (update status failed)")
		} else {
			h.respondError(w, r, http.StatusInternalServerError, "Failed to approve movie")
		}
		return
	}

	h.logger.InfoContext(ctx, "Movie approved successfully", slog.String("movieID", movieID))
	h.respondJSON(w, r, http.StatusOK, map[string]string{"message": "Movie approved successfully"})
}

// RejectMovie - ЗАГЛУШКА (но можно сделать аналогично ApproveMovie)
func (h *MovieHandler) RejectMovie(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	movieID := vars["movieId"]
	ctx := r.Context()
	h.logger.InfoContext(ctx, "RejectMovie endpoint hit", slog.String("movieID", movieID))
	// TODO: Реализовать вызов h.store.UpdateStatus(ctx, movieID, domain.StatusRejected)
	h.respondJSON(w, r, http.StatusOK, map[string]string{"message": "Movie rejected successfully (stub response)"})
}
