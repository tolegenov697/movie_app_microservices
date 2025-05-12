package domain

import (
	"time"
)

// User представляет модель пользователя в вашем приложении
type User struct {
	ID           string    `json:"id" db:"id"` // UUID
	Username     string    `json:"username" db:"username"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`     // Не отдаем хеш пароля в JSON
	Role         string    `json:"role,omitempty" db:"role"` // Например, "user", "admin"
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// RegisterRequest для регистрации нового пользователя (HTTP)
type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6,max=100"`
}

// LoginRequest для входа пользователя (HTTP)
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"` // Или Username
	Password string `json:"password" validate:"required"`
}

// LoginResponse для ответа при успешном входе (HTTP)
type LoginResponse struct {
	User  *User  `json:"user"` // Можно возвращать User DTO без хеша
	Token string `json:"token"`
}

// UpdateProfileRequest для обновления профиля (HTTP)
type UpdateProfileRequest struct {
	Username *string `json:"username,omitempty" validate:"omitempty,min=3,max=50"`
	Email    *string `json:"email,omitempty" validate:"omitempty,email"`
	// Не позволяем менять пароль этим эндпоинтом, для этого нужен отдельный
}
