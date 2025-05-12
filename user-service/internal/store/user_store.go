// user-service/internal/store/user_store.go
package store

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"user-service/internal/domain"
)

// Кастомные ошибки хранилища
var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user with this email or username already exists")
)

// UserStore определяет интерфейс для операций с данными пользователей.
type UserStore interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, userID string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
}

// MockUserStore для начальной разработки и тестов
type MockUserStore struct {
	mu           sync.RWMutex
	users        map[string]*domain.User // Ключ: UserID
	usersByEmail map[string]*domain.User // Ключ: Email
}

// NewMockUserStore создает новый экземпляр MockUserStore
func NewMockUserStore() *MockUserStore {
	m := &MockUserStore{
		users:        make(map[string]*domain.User),
		usersByEmail: make(map[string]*domain.User),
	}

	// --- ДОБАВЛЯЕМ ПРЕДОПРЕДЕЛЕННОГО ПОЛЬЗОВАТЕЛЯ ---
	predefinedUserID := "user-from-auth-token-123"
	predefinedUser := &domain.User{
		ID:           predefinedUserID,
		Username:     "HardcodedReviewer", // Имя, которое мы ожидаем увидеть
		Email:        "reviewer@example.com",
		PasswordHash: "somehash", // Не используется для GetUser
		Role:         "user",
		CreatedAt:    time.Now().Add(-48 * time.Hour),
		UpdatedAt:    time.Now().Add(-48 * time.Hour),
	}
	m.users[predefinedUserID] = predefinedUser
	m.usersByEmail[predefinedUser.Email] = predefinedUser
	log.Printf("[MOCK USER STORE] Predefined user added: ID='%s', Username='%s'\n", predefinedUserID, predefinedUser.Username)
	// --- КОНЕЦ ДОБАВЛЕНИЯ ---

	return m
}

func (m *MockUserStore) Create(ctx context.Context, user *domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("[MOCK USER STORE] Attempting to create user: ID='%s', Email='%s', Username='%s'\n", user.ID, user.Email, user.Username)

	for _, existingUser := range m.users {
		if existingUser.ID != user.ID && (existingUser.Email == user.Email || existingUser.Username == user.Username) {
			log.Printf("[MOCK USER STORE] User already exists with Email='%s' or Username='%s'\n", user.Email, user.Username)
			return ErrUserAlreadyExists
		}
	}

	userCopy := *user
	userCopy.CreatedAt = time.Now().UTC()
	userCopy.UpdatedAt = time.Now().UTC()
	m.users[user.ID] = &userCopy
	m.usersByEmail[user.Email] = &userCopy

	log.Printf("[MOCK USER STORE] Created user: ID='%s'\n", user.ID)
	return nil
}

func (m *MockUserStore) GetByID(ctx context.Context, userID string) (*domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	log.Printf("[MOCK USER STORE] Getting user by ID: %s\n", userID)
	if user, ok := m.users[userID]; ok {
		userCopy := *user
		return &userCopy, nil
	}
	log.Printf("[MOCK USER STORE] User not found by ID: %s\n", userID) // Добавим лог, если не найден
	return nil, ErrUserNotFound
}

func (m *MockUserStore) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	log.Printf("[MOCK USER STORE] Getting user by email: %s\n", email)
	if user, ok := m.usersByEmail[email]; ok {
		userCopy := *user
		return &userCopy, nil
	}
	return nil, ErrUserNotFound
}

func (m *MockUserStore) Update(ctx context.Context, user *domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	log.Printf("[MOCK USER STORE] Updating user ID %s\n", user.ID)

	existingUser, ok := m.users[user.ID]
	if !ok {
		return ErrUserNotFound
	}

	if user.Username != "" {
		for _, u := range m.users {
			if u.ID != user.ID && u.Username == user.Username {
				return ErrUserAlreadyExists
			}
		}
		existingUser.Username = user.Username
	}
	if user.Email != "" && existingUser.Email != user.Email {
		if _, emailExists := m.usersByEmail[user.Email]; emailExists {
			// Проверяем, не принадлежит ли этот email самому обновляемому пользователю
			// (этот сценарий не должен вызывать ошибку, если email не меняется)
			if m.usersByEmail[user.Email].ID != user.ID {
				return ErrUserAlreadyExists
			}
		}
		delete(m.usersByEmail, existingUser.Email)
		existingUser.Email = user.Email
		m.usersByEmail[existingUser.Email] = existingUser
	}
	if user.PasswordHash != "" {
		existingUser.PasswordHash = user.PasswordHash
	}
	if user.Role != "" {
		existingUser.Role = user.Role
	}

	existingUser.UpdatedAt = time.Now().UTC()
	m.users[user.ID] = existingUser
	return nil
}
