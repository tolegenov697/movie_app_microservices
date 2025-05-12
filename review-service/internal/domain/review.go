// review-service/internal/domain/review.go
package domain

import (
	"time"
)

// Review представляет модель отзыва/оценки
type Review struct {
	ID         string    `json:"id" db:"id"`                     // UUID
	MovieID    string    `json:"movie_id" db:"movie_id"`         // Внешний ключ к MovieService
	UserID     string    `json:"user_id" db:"user_id"`           // Внешний ключ к UserService
	Rating     int32     `json:"rating" db:"rating"`             // Оценка (например, 1-10)
	Comment    string    `json:"comment,omitempty" db:"comment"` // Текстовый комментарий (может быть пустым)
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
	Username   string    `json:"username,omitempty"`    // Не хранится в БД reviews, подтягивается
	MovieTitle string    `json:"movie_title,omitempty"` // Не хранится в БД reviews, подтягивается
}

// CreateReviewRequest определяет тело запроса для создания нового отзыва.
type CreateReviewRequest struct {
	MovieID string `json:"movie_id" validate:"required,uuid"`
	Rating  int32  `json:"rating" validate:"required,gte=1,lte=10"`
	Comment string `json:"comment,omitempty" validate:"max=2000"`
}

// UpdateReviewRequest определяет тело запроса для обновления отзыва.
type UpdateReviewRequest struct {
	Rating  *int32  `json:"rating,omitempty" validate:"omitempty,gte=1,lte=10"`
	Comment *string `json:"comment,omitempty" validate:"omitempty,max=2000"`
}

// AggregatedRating содержит агрегированную информацию о рейтинге фильма
type AggregatedRating struct {
	MovieID       string  `json:"movie_id" db:"movie_id"` // db тег, если будете хранить агрегаты в отдельной таблице
	AverageRating float64 `json:"average_rating" db:"average_rating"`
	RatingCount   int64   `json:"rating_count" db:"rating_count"`
}
