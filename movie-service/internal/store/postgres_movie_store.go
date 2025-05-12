// movie-service/internal/store/postgres_movie_store.go
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings" // Для работы со строками в Search
	"time"

	"movie-service/internal/domain"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq" // Для обработки ошибок PostgreSQL и работы с массивами TEXT[]
	// _ "github.com/lib/pq" // Драйвер PostgreSQL уже должен быть импортирован в main.go MovieService, если там есть PostgresUserStore
)

// PostgresMovieStore реализует MovieStore для PostgreSQL.
type PostgresMovieStore struct {
	db     *sqlx.DB
	logger *slog.Logger
}

// NewPostgresMovieStore создает новый экземпляр PostgresMovieStore.
func NewPostgresMovieStore(db *sqlx.DB, logger *slog.Logger) (*PostgresMovieStore, error) {
	if db == nil {
		return nil, errors.New("database connection (db) cannot be nil")
	}
	return &PostgresMovieStore{db: db, logger: logger}, nil
}

// Create создает новый фильм в базе данных.
func (s *PostgresMovieStore) Create(ctx context.Context, movie *domain.Movie) error {
	query := `INSERT INTO movies (id, title, description, release_year, director, genres, cast_members, poster_url, trailer_url, submitted_by_user_id, status, created_at, updated_at)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	movie.CreatedAt = time.Now().UTC()
	movie.UpdatedAt = movie.CreatedAt
	if movie.Status == "" { // Статус по умолчанию, если не задан
		movie.Status = domain.StatusPendingApproval
	}

	s.logger.DebugContext(ctx, "Executing Create movie query", slog.String("movieID", movie.ID), slog.String("title", movie.Title))
	_, err := s.db.ExecContext(ctx, query,
		movie.ID, movie.Title, movie.Description, movie.ReleaseYear, movie.Director,
		pq.Array(movie.Genres), pq.Array(movie.Cast), // Используем pq.Array для TEXT[]
		movie.PosterURL, movie.TrailerURL, movie.SubmittedByUserID, movie.Status,
		movie.CreatedAt, movie.UpdatedAt,
	)

	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" { // unique_violation
			s.logger.WarnContext(ctx, "Movie already exists (unique constraint violation in DB)", slog.String("error", pqErr.Error()), slog.String("constraint", pqErr.Constraint))
			return ErrMovieAlreadyExists // Предполагаем, что эта ошибка определена в вашем пакете store
		}
		s.logger.ErrorContext(ctx, "Failed to create movie in DB", slog.String("error", err.Error()))
		return fmt.Errorf("failed to create movie: %w", err)
	}
	s.logger.InfoContext(ctx, "Movie created successfully in DB", slog.String("movieID", movie.ID))
	return nil
}

// GetByID находит фильм по его ID.
func (s *PostgresMovieStore) GetByID(ctx context.Context, id string) (*domain.Movie, error) {
	query := `SELECT id, title, description, release_year, director, genres, cast_members, poster_url, trailer_url, submitted_by_user_id, status, created_at, updated_at
              FROM movies WHERE id = $1`
	var movie domain.Movie

	s.logger.DebugContext(ctx, "Executing GetMovieByID query", slog.String("movieID", id))
	err := s.db.GetContext(ctx, &movie, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.WarnContext(ctx, "Movie not found by ID in DB", slog.String("movieID", id))
			return nil, ErrMovieNotFound // Предполагаем, что эта ошибка определена
		}
		s.logger.ErrorContext(ctx, "Failed to get movie by ID from DB", slog.String("movieID", id), slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get movie by ID: %w", err)
	}
	s.logger.InfoContext(ctx, "Movie found by ID in DB", slog.String("movieID", movie.ID))
	return &movie, nil
}

// List возвращает список фильмов на основе предоставленных параметров.
func (s *PostgresMovieStore) List(ctx context.Context, params MovieListParams) ([]*domain.Movie, int, error) {
	var movies []*domain.Movie
	var totalCount int

	// Базовый запрос для подсчета общего количества
	countQuery := `SELECT COUNT(*) FROM movies WHERE 1=1`
	// Базовый запрос для выборки данных
	selectQuery := `SELECT id, title, description, release_year, director, genres, cast_members, poster_url, trailer_url, submitted_by_user_id, status, created_at, updated_at
                    FROM movies WHERE 1=1`

	var args []interface{}
	var conditions []string
	argId := 1

	if params.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argId))
		args = append(args, params.Status)
		argId++
	}
	if params.Genre != "" {
		// Поиск по жанру в массиве (регистронезависимый)
		conditions = append(conditions, fmt.Sprintf("LOWER(genres::text)::text[] @> ARRAY[LOWER($%d::text)]", argId))
		args = append(args, params.Genre)
		argId++
	}
	if params.Year != 0 {
		conditions = append(conditions, fmt.Sprintf("release_year = $%d", argId))
		args = append(args, params.Year)
		argId++
	}
	if params.SearchQuery != "" {
		// Простой поиск по названию (регистронезависимый)
		conditions = append(conditions, fmt.Sprintf("LOWER(title) LIKE LOWER($%d)", argId))
		args = append(args, "%"+params.SearchQuery+"%")
		argId++
	}

	if len(conditions) > 0 {
		conditionStr := " AND " + strings.Join(conditions, " AND ")
		countQuery += conditionStr
		selectQuery += conditionStr
	}

	// Получаем общее количество
	s.logger.DebugContext(ctx, "Executing List movies count query", slog.String("query", countQuery), slog.Any("args", args))
	err := s.db.GetContext(ctx, &totalCount, countQuery, args...)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to count movies in DB", slog.String("error", err.Error()))
		return nil, 0, fmt.Errorf("failed to count movies: %w", err)
	}

	if totalCount == 0 {
		return []*domain.Movie{}, 0, nil
	}

	// Добавляем сортировку
	// TODO: Добавить более гибкую и безопасную сортировку
	orderBy := "created_at DESC" // Сортировка по умолчанию
	if params.SortBy != "" {
		// В реальном приложении здесь нужна валидация SortBy, чтобы избежать SQL-инъекций
		// Например, разрешать только определенные поля и направления
		// Для примера, если params.SortBy = "title_asc", то orderBy = "title ASC"
		// Этот мок пока не реализует сложную сортировку
		if params.SortBy == "release_year_desc" {
			orderBy = "release_year DESC, title ASC"
		} else if params.SortBy == "title_asc" {
			orderBy = "title ASC"
		}
	}
	selectQuery += " ORDER BY " + orderBy

	// Добавляем пагинацию
	selectQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argId, argId+1)
	args = append(args, params.PageSize, (params.Page-1)*params.PageSize)
	argId += 2

	s.logger.DebugContext(ctx, "Executing List movies select query", slog.String("query", selectQuery), slog.Any("args", args))
	err = s.db.SelectContext(ctx, &movies, selectQuery, args...)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list movies from DB", slog.String("error", err.Error()))
		return nil, 0, fmt.Errorf("failed to list movies: %w", err)
	}

	return movies, totalCount, nil
}

// UpdateStatus обновляет статус фильма.
func (s *PostgresMovieStore) UpdateStatus(ctx context.Context, id string, status domain.MovieStatus) error {
	query := `UPDATE movies SET status = $1, updated_at = $2 WHERE id = $3`
	updatedAt := time.Now().UTC()

	s.logger.DebugContext(ctx, "Executing UpdateMovieStatus query", slog.String("movieID", id), slog.String("status", string(status)))
	result, err := s.db.ExecContext(ctx, query, status, updatedAt, id)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to update movie status in DB", slog.String("movieID", id), slog.String("error", err.Error()))
		return fmt.Errorf("failed to update movie status: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		s.logger.WarnContext(ctx, "No movie found to update status in DB", slog.String("movieID", id))
		return ErrMovieNotFound
	}
	s.logger.InfoContext(ctx, "Movie status updated successfully in DB", slog.String("movieID", id), slog.String("new_status", string(status)))
	return nil
}

// TODO: Реализовать методы Update и Delete для MovieStore
func (s *PostgresMovieStore) Update(ctx context.Context, movie *domain.Movie) error {
	s.logger.WarnContext(ctx, "Update movie method not fully implemented for PostgresMovieStore")
	// Примерный запрос:
	// query := `UPDATE movies SET title=$1, description=$2, release_year=$3, director=$4, genres=$5, cast_members=$6, poster_url=$7, trailer_url=$8, status=$9, updated_at=$10 WHERE id=$11`
	// _, err := s.db.ExecContext(ctx, query, movie.Title, ..., movie.ID)
	return errors.New("update movie not implemented yet")
}

func (s *PostgresMovieStore) Delete(ctx context.Context, id string) error {
	s.logger.WarnContext(ctx, "Delete movie method not fully implemented for PostgresMovieStore")
	// Примерный запрос:
	// query := `DELETE FROM movies WHERE id=$1`
	// _, err := s.db.ExecContext(ctx, query, id)
	return errors.New("delete movie not implemented yet")
}
