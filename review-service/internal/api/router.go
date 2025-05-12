package api

import (
	"github.com/gorilla/mux"
	"net/http"
)

// NewReviewRouter создает и настраивает маршрутизатор для ReviewService
func NewReviewRouter(handler *ReviewHandler) *mux.Router {
	router := mux.NewRouter()
	// router.StrictSlash(true) // Раскомментируйте, если хотите, чтобы /path и /path/ обрабатывались одинаково

	// Саб-роутер для всех эндпоинтов API с префиксом /api
	apiRouter := router.PathPrefix("/api").Subrouter()

	// Маршруты для отзывов, с префиксом /api/reviews
	reviewsRouter := apiRouter.PathPrefix("/reviews").Subrouter()
	reviewsRouter.HandleFunc("", handler.CreateReview).Methods(http.MethodPost)                      // POST /api/reviews - Создать отзыв
	reviewsRouter.HandleFunc("/movie/{movieId}", handler.GetReviewsForMovie).Methods(http.MethodGet) // GET /api/reviews/movie/{movieId} - Получить отзывы для фильма
	reviewsRouter.HandleFunc("/user/{userId}", handler.GetReviewsByUserID).Methods(http.MethodGet)   // GET /api/reviews/user/{userId} - Получить отзывы пользователя (TODO: implement handler)
	reviewsRouter.HandleFunc("/{reviewId}", handler.UpdateReview).Methods(http.MethodPut)            // PUT /api/reviews/{reviewId} - Обновить отзыв (TODO: implement handler)
	reviewsRouter.HandleFunc("/{reviewId}", handler.DeleteReview).Methods(http.MethodDelete)         // DELETE /api/reviews/{reviewId} - Удалить отзыв (TODO: implement handler)

	// Маршрут для получения агрегированного рейтинга фильма.
	// Этот эндпоинт логически связан с отзывами, поэтому может быть здесь.
	// Альтернативно, MovieService мог бы делать gRPC вызов к ReviewService для получения этих данных.
	apiRouter.HandleFunc("/movies/{movieId}/rating", handler.GetMovieAggregatedRating).Methods(http.MethodGet) // GET /api/movies/{movieId}/rating (TODO: implement handler)

	// TODO: В будущем здесь можно будет добавить middleware для аутентификации, логирования запросов и т.д.
	// Например:
	// loggedRouter := LoggingMiddleware(router)
	// authRouter := AuthenticationMiddleware(loggedRouter) // или применять middleware к конкретным саб-роутерам
	// return authRouter

	return router
}
