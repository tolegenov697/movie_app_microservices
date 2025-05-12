// user-service/pkg/auth/password.go
package auth

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

// HashPassword генерирует bcrypt хеш для заданного пароля.
func HashPassword(password string) (string, error) {
	// bcrypt.DefaultCost (обычно 10) - это хороший баланс между безопасностью и производительностью.
	// Вы можете увеличить его, если требуется более высокая безопасность (но это замедлит хеширование).
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hashedPassword), nil
}

// CheckPasswordHash сравнивает предоставленный пароль с существующим хешем.
// Возвращает true, если пароль совпадает с хешем, иначе false.
func CheckPasswordHash(password, hashedPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	// bcrypt.CompareHashAndPassword возвращает nil при совпадении,
	// и ошибку bcrypt.ErrMismatchedHashAndPassword (или другую) при несовпадении.
	return err == nil
}
