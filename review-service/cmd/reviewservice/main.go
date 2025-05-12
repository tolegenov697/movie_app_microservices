// review-service/cmd/reviewservice/main.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	_ "net" // Для gRPC клиента, если он будет здесь инициализироваться
	"net/http"
	"os"
	"os/signal"
	"strings" // Для extractPassword
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/jmoiron/sqlx" // Для sqlx.DB
	_ "github.com/lib/pq"     // Драйвер PostgreSQL

	"review-service/internal/api"
	"review-service/internal/clients"
	"review-service/internal/store"
	// "review-service/internal/genproto/moviepb" // Импорты для gRPC клиентов, если они здесь
	// "review-service/internal/genproto/userpb"
)

// getDBConnectionString возвращает строку подключения к БД для ReviewService.
// ВАЖНО: Замените значение по умолчанию на вашу реальную строку подключения!
func getDBConnectionString() string {
	dbURL := os.Getenv("REVIEW_SERVICE_DATABASE_URL")
	if dbURL == "" {
		// ЗАМЕНИТЕ ЭТУ СТРОКУ НА ВАШУ РЕАЛЬНУЮ СТРОКУ ПОДКЛЮЧЕНИЯ К POSTGRESQL
		// Укажите пользователя и базу данных, которые вы настроили для ReviewService.
		// Например, если таблица reviews в базе movie_service_db и пользователь user_service_user1:
		dbURL = "postgres://user_service_user1:gogogogo@localhost:5432/review_service_db?sslmode=disable"
		slog.Warn("REVIEW_SERVICE_DATABASE_URL environment variable not set, using default connection string. Ensure this is correct for your environment and user has permissions on 'reviews' table.")
	}
	return dbURL
}

// connectToDB инициализирует соединение с базой данных
func connectToDB(dbURL string, logger *slog.Logger) (*sqlx.DB, error) {
	// Логируем URL без пароля для безопасности
	safeDbURL := dbURL
	atIndex := strings.Index(dbURL, "@")
	if atIndex > 0 {
		protocolAndUser := dbURL[:strings.LastIndex(dbURL[:atIndex], ":")]
		hostAndDB := dbURL[atIndex:]
		safeDbURL = protocolAndUser + ":********" + hostAndDB
	}
	logger.Info("Attempting to connect to ReviewService database", slog.String("dbURL_used", safeDbURL))

	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		logger.Error("Failed to connect to ReviewService PostgreSQL", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		logger.Error("Failed to ping ReviewService PostgreSQL database", slog.String("error", err.Error()))
		db.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}
	logger.Info("Successfully connected to ReviewService PostgreSQL database.")
	return db, nil
}

// extractPassword (эта функция больше не нужна, если логируем URL без пароля по-другому)
// func extractPassword(dbURL string) string { /* ... */ }

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	validate := validator.New()
	httpPort := "8082"

	userServiceGRPCAddr := "localhost:9091"
	movieServiceGRPCAddr := "localhost:9092"

	// --- Инициализация хранилища PostgreSQL для ReviewService ---
	dbURL := getDBConnectionString()
	db, err := connectToDB(dbURL, logger)
	if err != nil {
		logger.Error("ReviewService failed to initialize database connection", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		logger.Info("Closing ReviewService PostgreSQL database connection...")
		if err := db.Close(); err != nil {
			logger.Error("Failed to close ReviewService PostgreSQL connection", slog.String("error", err.Error()))
		}
	}()

	reviewStorage, err := store.NewPostgresReviewStore(db, logger) // Используем PostgresReviewStore
	if err != nil {
		logger.Error("Failed to initialize PostgreSQL review store", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("PostgreSQL ReviewStore initialized for ReviewService.")

	// --- Инициализация gRPC клиентов ---
	clientCtx, clientCancel := context.WithTimeout(context.Background(), 10*time.Second)

	userSvcClient, err := clients.NewUserServiceGRPCClient(clientCtx, userServiceGRPCAddr, logger)
	if err != nil {
		logger.Error("Failed to create UserService gRPC client", slog.String("error", err.Error()))
		clientCancel()
		os.Exit(1)
	}
	logger.Info("UserService gRPC client created and connected.")

	movieSvcClient, err := clients.NewMovieServiceGRPCClient(clientCtx, movieServiceGRPCAddr, logger)
	if err != nil {
		logger.Error("Failed to create MovieService gRPC client", slog.String("error", err.Error()))
		clientCancel()
		if closer, ok := userSvcClient.(interface{ Close() error }); ok {
			closer.Close()
		}
		os.Exit(1)
	}
	logger.Info("MovieService gRPC client created and connected.")
	clientCancel()

	// Создание HTTP обработчика API
	reviewAPIHandler := api.NewReviewHandler(reviewStorage, logger, validate, userSvcClient, movieSvcClient) // Передаем PostgresReviewStore
	router := api.NewReviewRouter(reviewAPIHandler)

	// Настройка и запуск HTTP-сервера
	httpSrv := &http.Server{
		Addr:         ":" + httpPort,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("Review Service HTTP server starting", slog.String("port", httpPort))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Review Service HTTP ListenAndServe() failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Ожидание сигнала для graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Review Service shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Review Service HTTP Server Shutdown Failed", slog.String("error", err.Error()))
	} else {
		logger.Info("Review Service HTTP Server gracefully stopped.")
	}

	if closer, ok := userSvcClient.(interface{ Close() error }); ok {
		closer.Close()
	}
	if closer, ok := movieSvcClient.(interface{ Close() error }); ok {
		closer.Close()
	}

	logger.Info("Review Service fully stopped.")
}
