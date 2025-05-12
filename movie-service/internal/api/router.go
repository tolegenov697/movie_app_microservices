// movie-service/internal/api/router.go
package api

import (
	"github.com/gorilla/mux"
	"net/http"
)

func NewRouter(handler *MovieHandler) *mux.Router {
	router := mux.NewRouter()
	// router.StrictSlash(true) // Если используете, убедитесь, что это не вызывает проблем

	// Саб-роутер для /api префикса
	apiRouter := router.PathPrefix("/api").Subrouter()

	// Эндпоинты для фильмов
	moviesRouter := apiRouter.PathPrefix("/movies").Subrouter()
	moviesRouter.HandleFunc("", handler.CreateMovie).Methods(http.MethodPost)
	moviesRouter.HandleFunc("", handler.GetMovies).Methods(http.MethodGet)
	moviesRouter.HandleFunc("/{movieId}", handler.GetMovieByID).Methods(http.MethodGet)
	// ... другие маршруты для фильмов ...

	// Эндпоинты для администрирования/модерации фильмов
	// Путь будет /api/movies/admin/...
	adminMoviesRouter := moviesRouter.PathPrefix("/admin").Subrouter()
	adminMoviesRouter.HandleFunc("/pending", handler.GetPendingMovies).Methods(http.MethodGet)
	adminMoviesRouter.HandleFunc("/{movieId}/approve", handler.ApproveMovie).Methods(http.MethodPost) // Маршрут для одобрения
	adminMoviesRouter.HandleFunc("/{movieId}/reject", handler.RejectMovie).Methods(http.MethodPost)

	return router
}
