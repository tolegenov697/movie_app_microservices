// review-service/internal/store/postgres_review_store.go
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"review-service/internal/domain"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq" // Для обработки ошибок PostgreSQL
)

// PostgresReviewStore реализует ReviewStore для PostgreSQL.
type PostgresReviewStore struct {
	db     *sqlx.DB
	logger *slog.Logger
}

// NewPostgresReviewStore создает новый экземпляр PostgresReviewStore.
// Важно: db *sqlx.DB должен быть уже подключен и передан сюда.
func NewPostgresReviewStore(db *sqlx.DB, logger *slog.Logger) (*PostgresReviewStore, error) {
	if db == nil {
		return nil, errors.New("database connection (db) cannot be nil for PostgresReviewStore")
	}
	return &PostgresReviewStore{db: db, logger: logger}, nil
}

// Create создает новый отзыв в базе данных.
func (s *PostgresReviewStore) Create(ctx context.Context, review *domain.Review) error {
	query := `INSERT INTO reviews (id, movie_id, user_id, rating, comment, created_at, updated_at)
              VALUES ($1, $2, $3, $4, $5, $6, $7)`

	review.CreatedAt = time.Now().UTC()
	review.UpdatedAt = review.CreatedAt

	s.logger.DebugContext(ctx, "Executing Create review query",
		slog.String("reviewID", review.ID),
		slog.String("movieID", review.MovieID),
		slog.String("userID", review.UserID))

	_, err := s.db.ExecContext(ctx, query,
		review.ID, review.MovieID, review.UserID, review.Rating, review.Comment,
		review.CreatedAt, review.UpdatedAt,
	)

	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" { // unique_violation
			if pqErr.Constraint == "uq_user_movie_review" {
				s.logger.WarnContext(ctx, "User has already reviewed this movie (DB constraint)",
					slog.String("movieID", review.MovieID), slog.String("userID", review.UserID))
				return ErrDuplicateReview
			}
			s.logger.WarnContext(ctx, "Review creation failed due to unique constraint",
				slog.String("constraint", pqErr.Constraint), slog.String("error", pqErr.Error()))
			return fmt.Errorf("failed to create review due to unique constraint %s: %w", pqErr.Constraint, err)
		}
		s.logger.ErrorContext(ctx, "Failed to create review in DB", slog.String("error", err.Error()))
		return fmt.Errorf("failed to create review: %w", err)
	}
	s.logger.InfoContext(ctx, "Review created successfully in DB", slog.String("reviewID", review.ID))
	return nil
}

// GetByID находит отзыв по его ID.
func (s *PostgresReviewStore) GetByID(ctx context.Context, reviewID string) (*domain.Review, error) {
	query := `SELECT id, movie_id, user_id, rating, comment, created_at, updated_at FROM reviews WHERE id = $1`
	var review domain.Review

	s.logger.DebugContext(ctx, "Executing GetReviewByID query", slog.String("reviewID", reviewID))
	err := s.db.GetContext(ctx, &review, query, reviewID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.WarnContext(ctx, "Review not found by ID in DB", slog.String("reviewID", reviewID))
			return nil, ErrReviewNotFound
		}
		s.logger.ErrorContext(ctx, "Failed to get review by ID from DB", slog.String("reviewID", reviewID), slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get review by ID: %w", err)
	}
	s.logger.InfoContext(ctx, "Review found by ID in DB", slog.String("reviewID", review.ID))
	return &review, nil
}

// GetReviewsByMovieID получает все отзывы для указанного фильма.
func (s *PostgresReviewStore) GetReviewsByMovieID(ctx context.Context, movieID string, params ListReviewsParams) ([]*domain.Review, int, error) {
	var reviews []*domain.Review
	var totalCount int

	countQuery := `SELECT COUNT(*) FROM reviews WHERE movie_id = $1`
	selectQuery := `SELECT id, movie_id, user_id, rating, comment, created_at, updated_at 
                    FROM reviews WHERE movie_id = $1`

	s.logger.DebugContext(ctx, "Executing GetReviewsByMovieID count query", slog.String("movieID", movieID))
	err := s.db.GetContext(ctx, &totalCount, countQuery, movieID)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to count reviews by movieID in DB", slog.String("movieID", movieID), slog.String("error", err.Error()))
		return nil, 0, fmt.Errorf("failed to count reviews by movieID: %w", err)
	}

	if totalCount == 0 {
		return []*domain.Review{}, 0, nil
	}

	orderBy := "created_at DESC"
	if params.SortBy != "" {
		if params.SortBy == "rating_desc" {
			orderBy = "rating DESC, created_at DESC"
		} else if params.SortBy == "rating_asc" {
			orderBy = "rating ASC, created_at DESC"
		}
	}
	selectQuery += " ORDER BY " + orderBy
	selectQuery += fmt.Sprintf(" LIMIT %d OFFSET %d", params.PageSize, (params.Page-1)*params.PageSize)

	s.logger.DebugContext(ctx, "Executing GetReviewsByMovieID select query", slog.String("movieID", movieID), slog.String("query", selectQuery))
	err = s.db.SelectContext(ctx, &reviews, selectQuery, movieID)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list reviews by movieID from DB", slog.String("movieID", movieID), slog.String("error", err.Error()))
		return nil, 0, fmt.Errorf("failed to list reviews by movieID: %w", err)
	}
	return reviews, totalCount, nil
}

// GetReviewsByUserID получает все отзывы, оставленные пользователем.
func (s *PostgresReviewStore) GetReviewsByUserID(ctx context.Context, userID string, params ListReviewsParams) ([]*domain.Review, int, error) {
	var reviews []*domain.Review
	var totalCount int

	countQuery := `SELECT COUNT(*) FROM reviews WHERE user_id = $1`
	selectQuery := `SELECT id, movie_id, user_id, rating, comment, created_at, updated_at 
                    FROM reviews WHERE user_id = $1`

	s.logger.DebugContext(ctx, "Executing GetReviewsByUserID count query", slog.String("userID", userID))
	err := s.db.GetContext(ctx, &totalCount, countQuery, userID)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to count reviews by userID in DB", slog.String("userID", userID), slog.String("error", err.Error()))
		return nil, 0, fmt.Errorf("failed to count reviews by userID: %w", err)
	}

	if totalCount == 0 {
		return []*domain.Review{}, 0, nil
	}

	orderBy := "created_at DESC"
	if params.SortBy != "" {
		if params.SortBy == "rating_desc" {
			orderBy = "rating DESC, created_at DESC"
		}
	}
	selectQuery += " ORDER BY " + orderBy
	selectQuery += fmt.Sprintf(" LIMIT %d OFFSET %d", params.PageSize, (params.Page-1)*params.PageSize)

	s.logger.DebugContext(ctx, "Executing GetReviewsByUserID select query", slog.String("userID", userID), slog.String("query", selectQuery))
	err = s.db.SelectContext(ctx, &reviews, selectQuery, userID)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list reviews by userID from DB", slog.String("userID", userID), slog.String("error", err.Error()))
		return nil, 0, fmt.Errorf("failed to list reviews by userID: %w", err)
	}
	return reviews, totalCount, nil
}

// GetAggregatedRatingByMovieID рассчитывает средний рейтинг и количество оценок для фильма.
func (s *PostgresReviewStore) GetAggregatedRatingByMovieID(ctx context.Context, movieID string) (*domain.AggregatedRating, error) {
	query := `SELECT COALESCE(AVG(rating), 0) as average_rating, COUNT(rating) as rating_count 
              FROM reviews WHERE movie_id = $1`

	var aggRating domain.AggregatedRating
	aggRating.MovieID = movieID

	s.logger.DebugContext(ctx, "Executing GetAggregatedRatingByMovieID query", slog.String("movieID", movieID))
	err := s.db.GetContext(ctx, &aggRating, query, movieID)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to get aggregated rating from DB", slog.String("movieID", movieID), slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get aggregated rating for movieID %s: %w", movieID, err)
	}
	s.logger.InfoContext(ctx, "Aggregated rating calculated for movie", slog.String("movieID", movieID), slog.Float64("avg", aggRating.AverageRating), slog.Int64("count", aggRating.RatingCount))
	return &aggRating, nil
}

// Update обновляет существующий отзыв.
func (s *PostgresReviewStore) Update(ctx context.Context, review *domain.Review) error {
	query := `UPDATE reviews SET rating = $1, comment = $2, updated_at = $3 WHERE id = $4 AND user_id = $5`
	review.UpdatedAt = time.Now().UTC()

	s.logger.DebugContext(ctx, "Executing Update review query", slog.String("reviewID", review.ID), slog.String("userID", review.UserID))
	result, err := s.db.ExecContext(ctx, query, review.Rating, review.Comment, review.UpdatedAt, review.ID, review.UserID)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to update review in DB", slog.String("reviewID", review.ID), slog.String("error", err.Error()))
		return fmt.Errorf("failed to update review: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to get rows affected after review update", slog.String("reviewID", review.ID), slog.String("error", err.Error()))
		return fmt.Errorf("failed to check review update result: %w", err)
	}
	if rowsAffected == 0 {
		s.logger.WarnContext(ctx, "No review found to update or user not authorized", slog.String("reviewID", review.ID), slog.String("userID", review.UserID))
		return ErrReviewNotFound // Или более специфичная ошибка, если нужно различать "не найдено" и "не авторизован"
	}
	s.logger.InfoContext(ctx, "Review updated successfully in DB", slog.String("reviewID", review.ID))
	return nil
}

// Delete удаляет отзыв.
func (s *PostgresReviewStore) Delete(ctx context.Context, reviewID string, userID string) error {
	query := `DELETE FROM reviews WHERE id = $1 AND user_id = $2`

	s.logger.DebugContext(ctx, "Executing Delete review query", slog.String("reviewID", reviewID), slog.String("userID", userID))
	result, err := s.db.ExecContext(ctx, query, reviewID, userID)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to delete review from DB", slog.String("reviewID", reviewID), slog.String("error", err.Error()))
		return fmt.Errorf("failed to delete review: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to get rows affected after review delete", slog.String("reviewID", reviewID), slog.String("error", err.Error()))
		return fmt.Errorf("failed to check review delete result: %w", err)
	}
	if rowsAffected == 0 {
		s.logger.WarnContext(ctx, "No review found to delete or user not authorized", slog.String("reviewID", reviewID), slog.String("userID", userID))
		return ErrReviewNotFound // Или более специфичная ошибка
	}
	s.logger.InfoContext(ctx, "Review deleted successfully from DB", slog.String("reviewID", reviewID))
	return nil
}
