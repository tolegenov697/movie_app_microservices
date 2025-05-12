// movie-service/internal/domain/movie.go
package domain

import (
	"github.com/lib/pq" // <--- ДОБАВЬТЕ ЭТОТ ИМПОРТ
	"time"
)

// MovieStatus определяет возможные статусы фильма
type MovieStatus string

const (
	StatusPendingApproval MovieStatus = "pending_approval"
	StatusApproved        MovieStatus = "approved"
	StatusRejected        MovieStatus = "rejected"
)

// Movie представляет основную доменную модель фильма
type Movie struct {
	ID                string         `json:"id" db:"id"`
	Title             string         `json:"title" db:"title"`
	Description       string         `json:"description" db:"description"`
	ReleaseYear       int            `json:"release_year" db:"release_year"`
	Director          string         `json:"director" db:"director"`
	Genres            pq.StringArray `json:"genres" db:"genres"`     // <--- ИЗМЕНЕН ТИП НА pq.StringArray
	Cast              pq.StringArray `json:"cast" db:"cast_members"` // <--- ИЗМЕНЕН ТИП НА pq.StringArray (поле в Go: Cast, колонка в БД: cast_members)
	PosterURL         string         `json:"poster_url,omitempty" db:"poster_url"`
	TrailerURL        string         `json:"trailer_url,omitempty" db:"trailer_url"`
	SubmittedByUserID string         `json:"submitted_by_user_id" db:"submitted_by_user_id"`
	Status            MovieStatus    `json:"status" db:"status"`
	CreatedAt         time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at" db:"updated_at"`
}

// CreateMovieRequest определяет тело запроса для создания нового фильма
type CreateMovieRequest struct {
	Title       string   `json:"title" validate:"required,min=1,max=255"`
	Description string   `json:"description" validate:"required,min=10"`
	ReleaseYear int      `json:"release_year" validate:"required,gte=1888,lte=2100"`
	Director    string   `json:"director" validate:"required,min=2,max=100"`
	Genres      []string `json:"genres" validate:"required,min=1,dive,min=2,max=50"`     // Для JSON и валидации оставляем []string
	Cast        []string `json:"cast,omitempty" validate:"omitempty,dive,min=2,max=100"` // Для JSON и валидации оставляем []string
	PosterURL   string   `json:"poster_url,omitempty" validate:"omitempty,url"`
	TrailerURL  string   `json:"trailer_url,omitempty" validate:"omitempty,url"`
}

// UpdateMovieRequest (если вы его используете, также проверьте теги)
type UpdateMovieRequest struct {
	Title       *string  `json:"title,omitempty" validate:"omitempty,min=1,max=255"`
	Description *string  `json:"description,omitempty" validate:"omitempty,min=10"`
	ReleaseYear *int     `json:"release_year,omitempty" validate:"omitempty,gte=1888,lte=2100"`
	Director    *string  `json:"director,omitempty" validate:"omitempty,min=2,max=100"`
	Genres      []string `json:"genres,omitempty" validate:"omitempty,min=1,dive,min=2,max=50"`
	Cast        []string `json:"cast,omitempty" validate:"omitempty,dive,min=2,max=100"`
	PosterURL   *string  `json:"poster_url,omitempty" validate:"omitempty,url"`
	TrailerURL  *string  `json:"trailer_url,omitempty" validate:"omitempty,url"`
	Status      *string  `json:"status,omitempty" validate:"omitempty,oneof=pending_approval approved rejected"`
}
