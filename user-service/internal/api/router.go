// user-service/internal/api/router.go
package api

import (
	"github.com/gorilla/mux"
	"net/http"
)

// NewHTTPRouter создает и настраивает HTTP маршрутизатор для UserService
func NewHTTPRouter(httpHandler *HTTPHandler) *mux.Router {
	router := mux.NewRouter()
	// router.StrictSlash(true) // Раскомментируйте, если хотите одинаковую обработку /path и /path/

	// Базовый префикс для всех API эндпоинтов пользователей
	apiUsersRouter := router.PathPrefix("/api/users").Subrouter()

	// Публичные эндпоинты (не требуют аутентификации)
	apiUsersRouter.HandleFunc("/register", httpHandler.RegisterUser).Methods(http.MethodPost)
	apiUsersRouter.HandleFunc("/login", httpHandler.LoginUser).Methods(http.MethodPost)

	// Эндпоинты, требующие аутентификации
	// Создаем саб-роутер для /me и применяем к нему AuthMiddleware
	meRouter := apiUsersRouter.PathPrefix("/me").Subrouter()
	meRouter.Use(httpHandler.AuthMiddleware)                                       // AuthMiddleware применяется ко всем маршрутам в meRouter
	meRouter.HandleFunc("", httpHandler.GetUserProfile).Methods(http.MethodGet)    // GET /api/users/me
	meRouter.HandleFunc("", httpHandler.UpdateUserProfile).Methods(http.MethodPut) // PUT /api/users/me <--- ДОБАВЛЕН ЭТОТ МАРШРУТ

	return router
}
