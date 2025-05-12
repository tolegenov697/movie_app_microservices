// movie-service/cmd/movieservice/main.go
package main

import (
	"context"
	"fmt"
	"github.com/jmoiron/sqlx"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	httpAPI "movie-service/internal/api"      // HTTP API
	"movie-service/internal/genproto/moviepb" // Сгенерированный gRPC код
	grpcServer "movie-service/internal/grpc"  // Наш gRPC сервер
	"movie-service/internal/store"
)

// getDBConnectionString возвращает строку подключения к БД для MovieService.
// ВАЖНО: Замените значение по умолчанию на вашу реальную строку подключения!
func getDBConnectionString() string {
	dbURL := os.Getenv("MOVIE_SERVICE_DATABASE_URL")
	if dbURL == "" {
		// ЗАМЕНИТЕ ЭТУ СТРОКУ НА ВАШУ РЕАЛЬНУЮ СТРОКУ ПОДКЛЮЧЕНИЯ К POSTGRESQL
		// Укажите пользователя и базу данных, которые вы настроили для MovieService.
		// Это может быть movie_service_user@movie_service_db или user_service_user1@user_service_db
		// или user_service_user1@movie_service_db, в зависимости от вашего выбора.
		dbURL = "postgres://user_service_user1:gogogogo@localhost:5432/movie_service_db?sslmode=disable" // Пример
		slog.Warn("MOVIE_SERVICE_DATABASE_URL environment variable not set, using default connection string. Ensure this is correct for your environment.")
	}
	return dbURL
}

// connectToDB инициализирует соединение с базой данных
func connectToDB(dbURL string, logger *slog.Logger) (*sqlx.DB, error) {
	logger.Info("Attempting to connect to MovieService database", slog.String("dbURL_used", strings.Replace(dbURL, extractPassword(dbURL), "********", 1)))

	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		logger.Error("Failed to connect to MovieService PostgreSQL", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		logger.Error("Failed to ping MovieService PostgreSQL database", slog.String("error", err.Error()))
		db.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}
	logger.Info("Successfully connected to MovieService PostgreSQL database.")
	return db, nil
}

// extractPassword - вспомогательная функция для логирования URL без пароля (упрощенная)
func extractPassword(dbURL string) string {
	parts := strings.Split(dbURL, ":")
	if len(parts) > 2 {
		passAndHost := strings.Split(parts[2], "@")
		if len(passAndHost) > 0 {
			return passAndHost[0]
		}
	}
	return ""
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	validate := validator.New()

	httpPort := "8081"
	grpcPort := "9092"

	// --- Инициализация хранилища PostgreSQL для MovieService ---
	dbURL := getDBConnectionString()
	db, err := connectToDB(dbURL, logger) // Используем новую функцию для подключения
	if err != nil {
		logger.Error("MovieService failed to initialize database connection", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		logger.Info("Closing MovieService PostgreSQL database connection...")
		if err := db.Close(); err != nil {
			logger.Error("Failed to close MovieService PostgreSQL connection", slog.String("error", err.Error()))
		}
	}()

	movieStorage, err := store.NewPostgresMovieStore(db, logger) // Передаем *sqlx.DB
	if err != nil {
		logger.Error("Failed to initialize PostgreSQL movie store", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("PostgreSQL MovieStore initialized for MovieService.")

	// --- Настройка и запуск gRPC сервера ---
	grpcServiceImplementation := grpcServer.NewServer(movieStorage, logger) // Передаем PostgresMovieStore
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		logger.Error("Failed to listen for MovieService gRPC", slog.String("port", grpcPort), slog.String("error", err.Error()))
		os.Exit(1)
	}
	grpcSrv := grpc.NewServer()
	moviepb.RegisterMovieInterServiceServer(grpcSrv, grpcServiceImplementation)
	reflection.Register(grpcSrv)

	go func() {
		logger.Info("MovieService gRPC server starting", slog.String("port", grpcPort))
		if err := grpcSrv.Serve(lis); err != nil {
			logger.Error("MovieService gRPC server Serve() failed", slog.String("error", err.Error()))
		}
	}()

	// --- Настройка и запуск HTTP сервера ---
	movieAPIHandler := httpAPI.NewMovieHandler(movieStorage, logger, validate) // Передаем PostgresMovieStore
	httpRouter := httpAPI.NewRouter(movieAPIHandler)
	httpSrv := &http.Server{
		Addr:         ":" + httpPort,
		Handler:      httpRouter,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("MovieService HTTP server starting", slog.String("port", httpPort))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("MovieService HTTP server ListenAndServe() failed", slog.String("error", err.Error()))
		}
	}()

	// Ожидание сигнала для graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("MovieService shutting down...")

	ctxHttp, cancelHttp := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelHttp()
	if err := httpSrv.Shutdown(ctxHttp); err != nil {
		logger.Error("MovieService HTTP Server Shutdown Failed", slog.String("error", err.Error()))
	} else {
		logger.Info("MovieService HTTP Server gracefully stopped.")
	}

	grpcSrv.GracefulStop()
	logger.Info("MovieService gRPC server gracefully stopped.")
}
