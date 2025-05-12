// review-service/internal/api/handlers.go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"review-service/internal/domain"
	"review-service/internal/store"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"review-service/internal/genproto/moviepb"
	"review-service/internal/genproto/userpb"
)

// UserServiceClient определяет интерфейс для клиента UserService
type UserServiceClient interface {
	GetUser(ctx context.Context, userID string) (*userpb.UserResponse, error)
}

// MovieServiceClient определяет интерфейс для клиента MovieService
type MovieServiceClient interface {
	CheckMovieExists(ctx context.Context, movieID string) (bool, error)
	GetMovieInfo(ctx context.Context, movieID string) (*moviepb.MovieInfo, error)
}

type ReviewHandler struct {
	store              store.ReviewStore
	logger             *slog.Logger
	validator          *validator.Validate
	userServiceClient  UserServiceClient
	movieServiceClient MovieServiceClient
}

func NewReviewHandler(s store.ReviewStore, l *slog.Logger, v *validator.Validate, usc UserServiceClient, msc MovieServiceClient) *ReviewHandler {
	return &ReviewHandler{
		store:              s,
		logger:             l,
		validator:          v,
		userServiceClient:  usc,
		movieServiceClient: msc,
	}
}

// --- Вспомогательные функции ---
func (h *ReviewHandler) respondJSON(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			h.logger.ErrorContext(r.Context(), "Failed to encode JSON response", slog.String("error", err.Error()), slog.String("path", r.URL.Path))
		}
	}
}
func (h *ReviewHandler) respondError(w http.ResponseWriter, r *http.Request, status int, message string) {
	h.respondJSON(w, r, status, map[string]string{"error": message})
}

// --- Обработчики ---
func (h *ReviewHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Используем валидный UUID пользователя, который существует в UserService (например, pguser1)
	// Замените на ID пользователя, которого вы создали в UserService и для которого хотите оставить отзыв.
	// Этот ID был f5fe253c-832a-4f0c-8606-af54f29e4ca8 из ваших логов для pguser1.
	userID := "3aeb2a43-616f-4d62-a5e4-958e0802a31e" // <--- ИЗМЕНЕНО НА ВАЛИДНЫЙ UUID
	h.logger.InfoContext(ctx, "User attempting to create review", slog.String("userID", userID), slog.String("path", r.URL.Path))

	var req domain.CreateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.ErrorContext(ctx, "Failed to decode request body for review", slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	if err := h.validator.StructCtx(ctx, req); err != nil {
		h.logger.ErrorContext(ctx, "Review request validation failed", slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusBadRequest, "Validation failed: "+err.Error())
		return
	}

	movieExists, err := h.movieServiceClient.CheckMovieExists(ctx, req.MovieID)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to check movie existence via gRPC",
			slog.String("movie_id", req.MovieID), slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusInternalServerError, "Could not verify movie existence")
		return
	}
	if !movieExists {
		h.logger.WarnContext(ctx, "Attempt to create review for non-existent movie", slog.String("movie_id", req.MovieID))
		h.respondError(w, r, http.StatusNotFound, "Movie not found")
		return
	}
	h.logger.InfoContext(ctx, "Movie existence check successful for movie_id: "+req.MovieID)

	review := &domain.Review{
		ID:        uuid.NewString(),
		MovieID:   req.MovieID,
		UserID:    userID, // Теперь это валидный UUID
		Rating:    req.Rating,
		Comment:   req.Comment,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := h.store.Create(ctx, review); err != nil {
		h.logger.ErrorContext(ctx, "Failed to create review in store", slog.String("error", err.Error()))
		if errors.Is(err, store.ErrDuplicateReview) {
			h.respondError(w, r, http.StatusConflict, "You have already reviewed this movie.")
		} else {
			h.respondError(w, r, http.StatusInternalServerError, "Failed to create review")
		}
		return
	}
	h.logger.InfoContext(ctx, "Review created successfully", slog.String("reviewID", review.ID), slog.String("movieID", review.MovieID))
	h.respondJSON(w, r, http.StatusCreated, review)
}

func (h *ReviewHandler) GetReviewsForMovie(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	movieID := vars["movieId"]

	queryParams := r.URL.Query()
	h.logger.InfoContext(ctx, "Attempting to get reviews for movie", slog.String("movieID", movieID), slog.String("query", queryParams.Encode()))

	page, _ := strconv.Atoi(queryParams.Get("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(queryParams.Get("limit"))
	if limit <= 0 {
		limit = 10
	} else if limit > 50 {
		limit = 50
	}

	params := store.ListReviewsParams{
		Page:     page,
		PageSize: limit,
		SortBy:   queryParams.Get("sort_by"),
	}

	reviews, totalCount, err := h.store.GetReviewsByMovieID(ctx, movieID, params)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to get reviews by movieID from store", slog.String("movieID", movieID), slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusInternalServerError, "Failed to retrieve reviews")
		return
	}

	enrichedReviews := make([]domain.Review, 0, len(reviews))
	for _, rev := range reviews {
		enrichedRev := *rev
		userInfo, err := h.userServiceClient.GetUser(ctx, rev.UserID)
		if err != nil {
			h.logger.WarnContext(ctx, "Failed to get user info via gRPC for review",
				slog.String("userID", rev.UserID), slog.String("reviewID", rev.ID), slog.String("error", err.Error()))
		} else if userInfo != nil {
			enrichedRev.Username = userInfo.GetUsername()
		}

		movieInfo, movieErr := h.movieServiceClient.GetMovieInfo(ctx, rev.MovieID)
		if movieErr != nil {
			h.logger.WarnContext(ctx, "Failed to get movie info via gRPC for review",
				slog.String("movieID", rev.MovieID), slog.String("reviewID", rev.ID), slog.String("error", movieErr.Error()))
		} else if movieInfo != nil {
			enrichedRev.MovieTitle = movieInfo.GetTitle()
		}
		enrichedReviews = append(enrichedReviews, enrichedRev)
	}

	response := struct {
		Reviews    []domain.Review `json:"reviews"`
		TotalCount int             `json:"total_count"`
		Page       int             `json:"page"`
		PageSize   int             `json:"page_size"`
	}{
		Reviews:    enrichedReviews,
		TotalCount: totalCount,
		Page:       params.Page,
		PageSize:   params.PageSize,
	}

	h.logger.InfoContext(ctx, "Reviews for movie retrieved successfully", slog.String("movieID", movieID), slog.Int("count", len(enrichedReviews)))
	h.respondJSON(w, r, http.StatusOK, response)
}

func (h *ReviewHandler) GetMovieAggregatedRating(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	movieID := vars["movieId"]

	h.logger.InfoContext(ctx, "Attempting to get aggregated rating for movie", slog.String("movieID", movieID))

	movieExists, err := h.movieServiceClient.CheckMovieExists(ctx, movieID)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to check movie existence via gRPC for aggregated rating",
			slog.String("movie_id", movieID), slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusInternalServerError, "Could not verify movie existence")
		return
	}
	if !movieExists {
		h.logger.WarnContext(ctx, "Attempt to get aggregated rating for non-existent movie", slog.String("movie_id", movieID))
		h.respondError(w, r, http.StatusNotFound, "Movie not found")
		return
	}
	h.logger.InfoContext(ctx, "Movie existence check successful for aggregated rating, movie_id: "+movieID)

	aggRating, err := h.store.GetAggregatedRatingByMovieID(ctx, movieID)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to get aggregated rating from store", slog.String("movieID", movieID), slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusInternalServerError, "Failed to retrieve aggregated rating")
		return
	}

	h.logger.InfoContext(ctx, "Aggregated rating retrieved successfully",
		slog.String("movieID", movieID),
		slog.Float64("average_rating", aggRating.AverageRating),
		slog.Int64("rating_count", aggRating.RatingCount))
	h.respondJSON(w, r, http.StatusOK, aggRating)
}

func (h *ReviewHandler) GetReviewsByUserID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	targetUserID := vars["userId"]

	targetUserInfo, err := h.userServiceClient.GetUser(ctx, targetUserID)
	if err != nil {
		h.logger.WarnContext(ctx, "Failed to get target user info via gRPC for GetReviewsByUserID",
			slog.String("targetUserID", targetUserID), slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusNotFound, "User not found or error fetching user details.")
		return
	}

	queryParams := r.URL.Query()
	h.logger.InfoContext(ctx, "Attempting to get reviews for user", slog.String("targetUserID", targetUserID), slog.String("username", targetUserInfo.GetUsername()), slog.String("query", queryParams.Encode()))

	page, _ := strconv.Atoi(queryParams.Get("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(queryParams.Get("limit"))
	if limit <= 0 {
		limit = 10
	} else if limit > 50 {
		limit = 50
	}

	params := store.ListReviewsParams{
		Page:     page,
		PageSize: limit,
		SortBy:   queryParams.Get("sort_by"),
	}

	reviews, totalCount, err := h.store.GetReviewsByUserID(ctx, targetUserID, params)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to get reviews by userID from store", slog.String("targetUserID", targetUserID), slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusInternalServerError, "Failed to retrieve user's reviews")
		return
	}

	enrichedReviews := make([]domain.Review, 0, len(reviews))
	for _, rev := range reviews {
		enrichedRev := *rev
		enrichedRev.Username = targetUserInfo.GetUsername()

		movieInfo, movieErr := h.movieServiceClient.GetMovieInfo(ctx, rev.MovieID)
		if movieErr != nil {
			h.logger.WarnContext(ctx, "Failed to get movie info via gRPC for user's review",
				slog.String("movieID", rev.MovieID), slog.String("reviewID", rev.ID), slog.String("error", movieErr.Error()))
		} else if movieInfo != nil {
			enrichedRev.MovieTitle = movieInfo.GetTitle()
		}
		enrichedReviews = append(enrichedReviews, enrichedRev)
	}

	response := struct {
		Reviews    []domain.Review `json:"reviews"`
		TotalCount int             `json:"total_count"`
		Page       int             `json:"page"`
		PageSize   int             `json:"page_size"`
	}{
		Reviews:    enrichedReviews,
		TotalCount: totalCount,
		Page:       params.Page,
		PageSize:   params.PageSize,
	}

	h.logger.InfoContext(ctx, "Reviews for user retrieved successfully", slog.String("targetUserID", targetUserID), slog.Int("count", len(enrichedReviews)))
	h.respondJSON(w, r, http.StatusOK, response)
}

func (h *ReviewHandler) UpdateReview(w http.ResponseWriter, r *http.Request) {
	h.logger.InfoContext(r.Context(), "UpdateReview endpoint hit (TODO: implement)")
	h.respondJSON(w, r, http.StatusNotImplemented, map[string]string{"message": "UpdateReview not implemented"})
}
func (h *ReviewHandler) DeleteReview(w http.ResponseWriter, r *http.Request) {
	h.logger.InfoContext(r.Context(), "DeleteReview endpoint hit (TODO: implement)")
	h.respondJSON(w, r, http.StatusNotImplemented, map[string]string{"message": "DeleteReview not implemented"})
}
