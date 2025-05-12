// user-service/pkg/auth/token.go
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenManager предоставляет методы для генерации и валидации JWT токенов.
type TokenManager interface {
	Generate(userID string, userRole string) (string, error)
	Validate(tokenString string) (*Claims, error)
}

// jwtManager реализует TokenManager.
type jwtManager struct {
	secretKey     []byte        // Секретный ключ для подписи токенов
	tokenDuration time.Duration // Длительность жизни токена
}

// Claims определяет структуру данных, хранимых в JWT.
type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// NewTokenManager создает новый экземпляр jwtManager.
// secretKey должен быть достаточно сложным и храниться безопасно.
// tokenDuration - например, time.Hour * 24 для токена, живущего 24 часа.
func NewTokenManager(secretKey string, tokenDuration time.Duration) (TokenManager, error) {
	if secretKey == "" {
		return nil, fmt.Errorf("JWT secret key cannot be empty")
	}
	if len(secretKey) < 32 { // Рекомендуется минимальная длина для HMAC-SHA256
		// В реальном приложении здесь может быть более строгая проверка или генерация ключа
		// Для примера, мы не будем вызывать ошибку, но в проде это важно.
		// return nil, fmt.Errorf("JWT secret key is too short (recommended min 32 bytes for HS256)")
		fmt.Printf("Warning: JWT secret key is short. For production, use a key of at least 32 bytes for HS256.\n")
	}
	return &jwtManager{
		secretKey:     []byte(secretKey),
		tokenDuration: tokenDuration,
	}, nil
}

// Generate создает новый JWT токен для указанного userID и userRole.
func (m *jwtManager) Generate(userID string, userRole string) (string, error) {
	expirationTime := time.Now().Add(m.tokenDuration)
	claims := &Claims{
		UserID: userID,
		Role:   userRole,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "user-service", // Опционально: кто выдал токен
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(m.secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}
	return tokenString, nil
}

// Validate проверяет JWT токен и возвращает извлеченные из него Claims.
func (m *jwtManager) Validate(tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
   