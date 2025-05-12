// user-service/cmd/userservice/main.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	httpAPI "user-service/internal/api"
	"user-service/internal/genproto/userpb"
	grpcServer "user-service/internal/grpc"
	"user-service/internal/store"
	"user-service/pkg/auth"
)

// getDBConnectionString возвращает строку подключения к БД.
// ВАЖНО: Замените значение по умолчанию на вашу реальную строку подключения!
func getDBConnectionString() string {
	// Попробуйте получить из переменной окружения
	dbURL := os.Getenv("USER_SERVICE_DATABASE_URL")
	if dbURL == "" {
		// Если переменная окружения не установлена, используйте значение по умолчанию (для локальной разработки)
		// ЗАМЕНИТЕ ЭТУ СТРОКУ НА ВАШУ РЕАЛЬНУЮ СТРОКУ ПОДКЛЮЧЕНИЯ К POSTGRESQL
		dbURL = "postgres://user_service_user1:gogogogo@localhost:5432/user_service_db?sslmode=disable"
		slog.Warn("USER_SERVICE_DATABASE_URL environment variable not set, using default connection string. Ensure this is correct for your environment.")
	}
	return dbURL
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	validate := validator.New()

	grpcPort := "9091"
	httpPort := "8080"

	// --- Конфигурация для JWT ---
	jwtSecretKey := os.Getenv("JWT_SECRET_KEY")
	if jwtSecretKey == "" {
		jwtSecretKey = "your-very-secret-and-long-enough-key-for-hmac256-dev-only"
		logger.Warn("JWT_SECRET_KEY environment variable not set, using default insecure key for development.")
	}
	jwtTokenDuration := time.Hour * 24

	tokenManager, err := auth.NewTokenManager(jwtSecretKey, jwtTokenDuration)
	if err != nil {
		logger.Error("Failed to create token manager", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("Token manager initialized.")

	// --- Инициализация хранилища PostgreSQL ---
	dbURL := getDBConnectionString()
	logger.Info("Attempting to connect to database", slog.String("dbURL", dbURL)) // Логируем используемый URL

	userStorage, err := store.NewPostgresUserStore(dbURL, logger) // Используем PostgresUserStore
	if err != nil {
		logger.Error("Failed to initialize PostgreSQL user store", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if err := userStorage.Close(); err != nil {
			logger.Error("Failed to close PostgreSQL connection", slog.String("error", err.Error()))
		} else {
			logger.Info("PostgreSQL connection closed.")
		}
	}()
	logger.Info("PostgreSQL UserStore initialized.")

	// --- Настройка и запуск gRPC сервера ---
	grpcServiceImplementation := grpcServer.NewServer(userStorage, logger) // Передаем PostgresUserStore
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		logger.Error("Failed to listen for gRPC", slog.String("port", grpcPort), slog.String("error", err.Error()))
		os.Exit(1)
	}
	grpcSrv := grpc.NewServer()
	userpb.RegisterUserServiceServer(grpcSrv, grpcServiceImplementation)
	reflection.Register(grpcSrv)

	go func() {
		logger.Info("User gRPC Service starting", slog.String("port", grpcPort))
		if err := grpcSrv.Serve(lis); err != nil {
			logger.Error("User gRPC Service Serve() failed", slog.String("error", err.Error()))
		}
	}()

	// --- Настройка и запуск HTTP сервера ---
	httpAPIHandler := httpAPI.NewHTTPHandler(userStorage, logger, validate, tokenManager) // Передаем PostgresUserStore
	httpRouter := httpAPI.NewHTTPRouter(httpAPIHandler)
	httpSrv := &http.Server{
		Addr:         ":" + httpPort,
		Handler:      httpRouter,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("User HTTP Service starting", slog.String("port", httpPort))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("User HTTP Service ListenAndServe() failed", slog.String("error", err.Error()))
		}
	}()

	// Ожидание сигнала для graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("User Service shutting down...")

	ctxHttp, cancelHttp := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelHttp()
	if err := httpSrv.Shutdown(ctxHttp); err != nil {
		logger.Error("User HTTP Server Shutdown Failed", slog.String("error", err.Error()))
	} else {
		logger.Info("User HTTP Server gracefully stopped.")
	}

	grpcSrv.GracefulStop()
	logger.Info("User gRPC Service gracefully stopped.")
}
