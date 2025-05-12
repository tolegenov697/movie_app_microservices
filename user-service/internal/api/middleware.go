// user-service/internal/api/middleware.go
package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
)

// ContextKey используется для ключей в контексте запроса.
type ContextKey string

const (
	// UserIDKey ключ для хранения ID пользователя в контексте.
	UserIDKey ContextKey = "userID"
	// UserRoleKey ключ для хранения роли пользователя в контексте.
	UserRoleKey ContextKey = "userRole"
)

// AuthMiddleware проверяет JWT токен из заголовка Authorization.
// Если токен валиден, ID пользователя и его роль добавляются в контекст запроса.
func (h *HTTPHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			h.logger.WarnContext(r.Context(), "Authorization header missing")
			h.respondError(w, r, http.StatusUnauthorized, "Authorization header required")
			return
		}

		// Ожидаем токен в формате "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			h.logger.WarnContext(r.Context(), "Invalid Authorization header format", slog.String("header", authHeader))
			h.respondError(w, r, http.StatusUnauthorized, "Invalid Authorization header format")
			return
		}
		tokenString := parts[1]

		claims, err := h.tokenManager.Validate(tokenString)
		if err != nil {
			h.logger.WarnContext(r.Context(), "Invalid or expired token", slog.String("error", err.Error()))
			h.respondError(w, r, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		// Добавляем информацию из токена в контекст запроса
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UserRoleKey, claims.Role)

		h.logger.DebugContext(ctx, "Token validated successfully", slog.String("userID", claims.UserID), slog.String("role", claims.Role))

		// Передаем запрос дальше следующему обработчику с обновленным контекстом
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
