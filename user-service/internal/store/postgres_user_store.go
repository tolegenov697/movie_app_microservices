// user-service/internal/store/postgres_user_store.go
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"user-service/internal/domain"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq" // <--- ДОБАВЬТЕ ЭТОТ ИМПОРТ для ошибок PostgreSQL
	// _ "github.com/lib/pq" // Если вы уже импортировали его с _, оставьте так
)

// PostgresUserStore реализует UserStore для PostgreSQL.
type PostgresUserStore struct {
	db     *sqlx.DB
	logger *slog.Logger
}

// NewPostgresUserStore (остается без изменений)
func NewPostgresUserStore(dbURL string, logger *slog.Logger) (*PostgresUserStore, error) {
	if dbURL == "" {
		return nil, errors.New("DB connection string (dbURL) cannot be empty")
	}
	logger.Info("Connecting to PostgreSQL database...")
	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		logger.Error("Failed to connect to PostgreSQL", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		logger.Error("Failed to ping PostgreSQL database", slog.String("error", err.Error()))
		db.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}
	logger.Info("Successfully connected to PostgreSQL database.")
	return &PostgresUserStore{db: db, logger: logger}, nil
}

// Close (остается без изменений)
func (s *PostgresUserStore) Close() error {
	s.logger.Info("Closing PostgreSQL database connection...")
	return s.db.Close()
}

// Create создает нового пользователя в базе данных.
func (s *PostgresUserStore) Create(ctx context.Context, user *domain.User) error {
	query := `INSERT INTO users (id, username, email, password_hash, role, created_at, updated_at)
              VALUES ($1, $2, $3, $4, $5, $6, $7)`

	// Устанавливаем CreatedAt и UpdatedAt только если они не установлены (хотя ID генерируется в хендлере)
	// Для Create обычно всегда устанавливаем новые временные метки.
	user.CreatedAt = time.Now().UTC()
	user.UpdatedAt = user.CreatedAt

	s.logger.DebugContext(ctx, "Executing Create user query", slog.String("userID", user.ID), slog.String("email", user.Email), slog.String("username", user.Username))
	_, err := s.db.ExecContext(ctx, query, user.ID, user.Username, user.Email, user.PasswordHash, user.Role, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		// Проверяем на специфическую ошибку PostgreSQL для нарушения уникальности
		var pqErr *pq.Error
		if errors.As(err, &pqErr) { // Пытаемся привести ошибку к *pq.Error
			// Код '23505' соответствует unique_violation в PostgreSQL
			if pqErr.Code == "23505" {
				s.logger.WarnContext(ctx, "User already exists (unique constraint violation in DB)",
					slog.String("userID", user.ID),
					slog.String("email", user.Email),
					slog.String("username", user.Username),
					slog.String("pg_error_code", string(pqErr.Code)),
					slog.String("constraint_name", pqErr.Constraint)) // Имя нарушенного ограничения (например, users_username_key или users_email_key)
				return ErrUserAlreadyExists // Возвращаем нашу кастомную ошибку
			}
		}
		// Если это не ошибка уникальности, или не удалось привести к pq.Error, логируем как общую ошибку
		s.logger.ErrorContext(ctx, "Failed to create user in DB", slog.String("error", err.Error()))
		return fmt.Errorf("failed to create user: %w", err)
	}
	s.logger.InfoContext(ctx, "User created successfully in DB", slog.String("userID", user.ID))
	return nil
}

// GetByID (остается без изменений)
func (s *PostgresUserStore) GetByID(ctx context.Context, userID string) (*domain.User, error) {
	query := `SELECT id, username, email, password_hash, role, created_at, updated_at 
              FROM users WHERE id = $1`
	var user domain.User
	s.logger.DebugContext(ctx, "Executing GetByID query", slog.String("userID", userID))
	err := s.db.GetContext(ctx, &user, query, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.WarnContext(ctx, "User not found by ID in DB", slog.String("userID", userID))
			return nil, ErrUserNotFound
		}
		s.logger.ErrorContext(ctx, "Failed to get user by ID from DB", slog.String("userID", userID), slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}
	s.logger.InfoContext(ctx, "User found by ID in DB", slog.String("userID", user.ID))
	return &user, nil
}

// GetByEmail (остается без изменений)
func (s *PostgresUserStore) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `SELECT id, username, email, password_hash, role, created_at, updated_at 
              FROM users WHERE email = $1`
	var user domain.User
	s.logger.DebugContext(ctx, "Executing GetByEmail query", slog.String("email", email))
	err := s.db.GetContext(ctx, &user, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.WarnContext(ctx, "User not found by email in DB", slog.String("email", email))
			return nil, ErrUserNotFound
		}
		s.logger.ErrorContext(ctx, "Failed to get user by email from DB", slog.String("email", email), slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	s.logger.InfoContext(ctx, "User found by email in DB", slog.String("userID", user.ID), slog.String("email", user.Email))
	return &user, nil
}

// Update (остается без изменений, но также может потребовать обработки ошибок уникальности)
func (s *PostgresUserStore) Update(ctx context.Context, user *domain.User) error {
	query := `UPDATE users SET username = $1, email = $2, password_hash = $3, role = $4, updated_at = $5
              WHERE id = $6`
	user.UpdatedAt = time.Now().UTC()
	s.logger.DebugContext(ctx, "Executing Update user query", slog.String("userID", user.ID))
	result, err := s.db.ExecContext(ctx, query, user.Username, user.Email, user.PasswordHash, user.Role, user.UpdatedAt, user.ID)
	if err != nil {
		// TODO: Добавить обработку pq.Error для unique_violation (код 23505) и здесь, если нужно
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			s.logger.WarnContext(ctx, "Update failed: username or email already exists (DB constraint)", slog.String("userID", user.ID), slog.String("constraint", pqErr.Constraint))
			return ErrUserAlreadyExists
		}
		s.logger.ErrorContext(ctx, "Failed to update user in DB", slog.String("userID", user.ID), slog.String("error", err.Error()))
		return fmt.Errorf("failed to update user: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to get rows affected after update", slog.String("userID", user.ID), slog.String("error", err.Error()))
		return fmt.Errorf("failed to check update result: %w", err)
	}
	if rowsAffected == 0 {
		s.logger.WarnContext(ctx, "No user found to update in DB or no changes made", slog.String("userID", user.ID))
		return ErrUserNotFound
	}
	s.logger.InfoContext(ctx, "User updated successfully in DB", slog.String("userID", user.ID))
	return nil
}
