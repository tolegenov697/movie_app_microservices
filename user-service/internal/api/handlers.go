// user-service/internal/api/handlers.go
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"user-service/internal/domain"
	"user-service/internal/store"
	"user-service/pkg/auth" // Наш пакет для хеширования и JWT

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// HTTPHandler (структура и NewHTTPHandler остаются прежними)
type HTTPHandler struct {
	store        store.UserStore
	logger       *slog.Logger
	validator    *validator.Validate
	tokenManager auth.TokenManager
}

func NewHTTPHandler(s store.UserStore, l *slog.Logger, v *validator.Validate, tm auth.TokenManager) *HTTPHandler {
	return &HTTPHandler{
		store:        s,
		logger:       l,
		validator:    v,
		tokenManager: tm,
	}
}

// --- Вспомогательные функции (respondJSON, respondError - остаются прежними) ---
func (h *HTTPHandler) respondJSON(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			h.logger.ErrorContext(r.Context(), "Failed to encode JSON response", slog.String("error", err.Error()), slog.String("path", r.URL.Path))
		}
	}
}

func (h *HTTPHandler) respondError(w http.ResponseWriter, r *http.Request, status int, message string) {
	h.respondJSON(w, r, status, map[string]string{"error": message})
}

// RegisterUser (остается прежним)
func (h *HTTPHandler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h.logger.InfoContext(ctx, "HTTP RegisterUser request received", slog.String("path", r.URL.Path))

	var req domain.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.ErrorContext(ctx, "Failed to decode registration request body", slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	if err := h.validator.StructCtx(ctx, req); err != nil {
		h.logger.ErrorContext(ctx, "Registration request validation failed", slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusBadRequest, "Validation failed: "+err.Error())
		return
	}

	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to hash password", slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusInternalServerError, "Error processing registration")
		return
	}

	newUser := &domain.User{
		ID:           uuid.NewString(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hashedPassword,
		Role:         "user",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := h.store.Create(ctx, newUser); err != nil {
		h.logger.ErrorContext(ctx, "Failed to create user in store", slog.String("error", err.Error()))
		if errors.Is(err, store.ErrUserAlreadyExists) {
			h.respondError(w, r, http.StatusConflict, "User with this email or username already exists")
		} else {
			h.respondError(w, r, http.StatusInternalServerError, "Failed to register user")
		}
		return
	}

	userResponse := &domain.User{
		ID:        newUser.ID,
		Username:  newUser.Username,
		Email:     newUser.Email,
		Role:      newUser.Role,
		CreatedAt: newUser.CreatedAt,
		UpdatedAt: newUser.UpdatedAt,
	}

	h.logger.InfoContext(ctx, "User registered successfully", slog.String("userID", newUser.ID), slog.String("username", newUser.Username))
	h.respondJSON(w, r, http.StatusCreated, userResponse)
}

// LoginUser (остается прежним)
func (h *HTTPHandler) LoginUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h.logger.InfoContext(ctx, "HTTP LoginUser request received", slog.String("path", r.URL.Path))

	var req domain.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.ErrorContext(ctx, "Failed to decode login request body", slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	if err := h.validator.StructCtx(ctx, req); err != nil {
		h.logger.ErrorContext(ctx, "Login request validation failed", slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusBadRequest, "Validation failed: "+err.Error())
		return
	}

	user, err := h.store.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			h.logger.WarnContext(ctx, "Login attempt for non-existent email", slog.String("email", req.Email))
			h.respondError(w, r, http.StatusUnauthorized, "Invalid email or password")
		} else {
			h.logger.ErrorContext(ctx, "Failed to get user by email from store", slog.String("email", req.Email), slog.String("error", err.Error()))
			h.respondError(w, r, http.StatusInternalServerError, "Login failed")
		}
		return
	}

	if !auth.CheckPasswordHash(req.Password, user.PasswordHash) {
		h.logger.WarnContext(ctx, "Invalid password attempt", slog.String("email", req.Email), slog.String("userID", user.ID))
		h.respondError(w, r, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	tokenString, err := h.tokenManager.Generate(user.ID, user.Role)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to generate JWT token", slog.String("userID", user.ID), slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusInternalServerError, "Login failed (token generation)")
		return
	}

	loginResponse := domain.LoginResponse{
		User: &domain.User{
			ID:        user.ID,
			Username:  user.Username,
			Email:     user.Email,
			Role:      user.Role,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		},
		Token: tokenString,
	}

	h.logger.InfoContext(ctx, "User logged in successfully", slog.String("userID", user.ID), slog.String("email", user.Email))
	h.respondJSON(w, r, http.StatusOK, loginResponse)
}

// GetUserProfile (остается прежним)
func (h *HTTPHandler) GetUserProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctx.Value(UserIDKey).(string)
	if !ok || userID == "" {
		h.logger.ErrorContext(ctx, "UserID not found in request context after AuthMiddleware")
		h.respondError(w, r, http.StatusInternalServerError, "Error processing user identity")
		return
	}
	h.logger.InfoContext(ctx, "HTTP GetUserProfile request received", slog.String("userID", userID))

	user, err := h.store.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			h.logger.WarnContext(ctx, "User from valid token not found in store", slog.String("userID", userID))
			h.respondError(w, r, http.StatusNotFound, "User associated with token not found")
		} else {
			h.logger.ErrorContext(ctx, "Failed to get user by ID from store for profile", slog.String("userID", userID), slog.String("error", err.Error()))
			h.respondError(w, r, http.StatusInternalServerError, "Failed to retrieve user profile")
		}
		return
	}
	userResponse := &domain.User{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
	h.respondJSON(w, r, http.StatusOK, userResponse)
}

// UpdateUserProfile обновляет профиль текущего аутентифицированного пользователя.
func (h *HTTPHandler) UpdateUserProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctx.Value(UserIDKey).(string)
	if !ok || userID == "" {
		h.logger.ErrorContext(ctx, "UserID not found in request context for update profile")
		h.respondError(w, r, http.StatusInternalServerError, "Error processing user identity")
		return
	}
	h.logger.InfoContext(ctx, "HTTP UpdateUserProfile request received", slog.String("userID", userID))

	var req domain.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.ErrorContext(ctx, "Failed to decode update profile request body", slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	// Валидируем только те поля, которые были переданы (из-за использования указателей в UpdateProfileRequest)
	if err := h.validator.StructCtx(ctx, req); err != nil {
		h.logger.ErrorContext(ctx, "Update profile request validation failed", slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusBadRequest, "Validation failed: "+err.Error())
		return
	}

	// Получаем текущего пользователя из хранилища
	currentUser, err := h.store.GetByID(ctx, userID)
	if err != nil {
		// Это не должно произойти, если токен валиден и пользователь не был удален
		h.logger.ErrorContext(ctx, "User to update not found in store, though token was valid", slog.String("userID", userID), slog.String("error", err.Error()))
		h.respondError(w, r, http.StatusNotFound, "User not found")
		return
	}

	// Обновляем поля, если они были предоставлены в запросе
	updated := false
	if req.Username != nil {
		currentUser.Username = *req.Username
		updated = true
	}
	if req.Email != nil {
		// TODO: Добавить проверку на уникальность нового email, если он меняется
		// и если он отличается от текущего email пользователя.
		// userWithNewEmail, err := h.store.GetByEmail(ctx, *req.Email)
		// if err == nil && userWithNewEmail.ID != userID {
		//    h.respondError(w, r, http.StatusConflict, "Email already in use by another account")
		//    return
		// }
		// if err != nil && !errors.Is(err, store.ErrUserNotFound) {
		//    h.logger.ErrorContext(ctx, "Error checking new email uniqueness", slog.String("email", *req.Email), slog.String("error", err.Error()))
		//    h.respondError(w, r, http.StatusInternalServerError, "Failed to update profile")
		//    return
		// }
		currentUser.Email = *req.Email
		updated = true
	}

	if updated {
		currentUser.UpdatedAt = time.Now().UTC()
		if err := h.store.Update(ctx, currentUser); err != nil {
			// Обработка возможных ошибок от store.Update, например, если новый email/username уже занят (если store это проверяет)
			if errors.Is(err, store.ErrUserAlreadyExists) { // Предполагаем, что store.Update может вернуть эту ошибку
				h.respondError(w, r, http.StatusConflict, "Username or email may already be in use.")
			} else {
				h.logger.ErrorContext(ctx, "Failed to update user profile in store", slog.String("userID", userID), slog.String("error", err.Error()))
				h.respondError(w, r, http.StatusInternalServerError, "Failed to update profile")
			}
			return
		}
	}

	userResponse := &domain.User{
		ID:        currentUser.ID,
		Username:  currentUser.Username,
		Email:     currentUser.Email,
		Role:      currentUser.Role,
		CreatedAt: currentUser.CreatedAt,
		UpdatedAt: currentUser.UpdatedAt,
	}
	h.respondJSON(w, r, http.StatusOK, userResponse)
}
