// review-service/internal/clients/user_service_client.go
package clients

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	// ВАЖНО: Путь к сгенерированному proto-коду для UserService.
	// Этот путь должен соответствовать тому, как вы решили проблему доступа
	// ReviewService к сгенерированным файлам UserService.
	//
	// ВАРИАНТ 1 (Если вы скопировали genproto/userpb из user-service в review-service):
	"review-service/internal/genproto/userpb" // Используйте этот путь, если папка userpb находится здесь
	//
	// ВАРИАНТ 2 (Если используете Go Workspaces и user-service доступен):
	// "user-service/internal/genproto/userpb" // Используйте этот путь, если workspace настроен
	//
	// Выберите ОДИН из вариантов импорта userpb или адаптируйте путь под вашу структуру.
	// Если вы еще не решили, как это сделать, рекомендую пока ВАРИАНТ 1 (скопировать папку).

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes" // Для кодов ошибок gRPC
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status" // Для кодов ошибок gRPC
)

// UserServiceClient определяет методы для взаимодействия с UserService.
// Этот интерфейс должен совпадать с тем, что определен в review-service/internal/api/handlers.go
// и возвращать конкретные типы userpb.
type UserServiceClient interface {
	GetUser(ctx context.Context, userID string) (*userpb.UserResponse, error)
}

// userServiceGRPCClient реализует UserServiceClient с использованием gRPC.
type userServiceGRPCClient struct {
	client userpb.UserServiceClient // Сгенерированный gRPC клиент
	logger *slog.Logger
	conn   *grpc.ClientConn // Сохраняем соединение, чтобы его можно было закрыть
}

// NewUserServiceGRPCClient создает новый gRPC клиент для UserService.
// userServiceAddr - адрес gRPC сервера UserService (например, "localhost:9091").
func NewUserServiceGRPCClient(ctx context.Context, userServiceAddr string, logger *slog.Logger) (UserServiceClient, error) {
	logger.Info("Attempting to connect to UserService gRPC", slog.String("address", userServiceAddr))

	// Устанавливаем соединение с gRPC сервером UserService
	// Для простоты используем небезопасное соединение. В продакшене нужно использовать TLS.
	// grpc.WithBlock() блокирует до тех пор, пока соединение не будет установлено или не истечет таймаут.
	// Таймаут для DialContext можно установить через сам контекст.
	dialCtx, dialCancel := context.WithTimeout(ctx, 5*time.Second) // Таймаут на установку соединения
	defer dialCancel()

	conn, err := grpc.DialContext(dialCtx, userServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		logger.Error("Failed to connect to UserService gRPC", slog.String("address", userServiceAddr), slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to connect to user service at %s: %w", userServiceAddr, err)
	}
	logger.Info("Successfully connected to UserService gRPC", slog.String("address", userServiceAddr))

	// Создаем gRPC клиент на основе соединения
	grpcClient := userpb.NewUserServiceClient(conn)

	return &userServiceGRPCClient{
		client: grpcClient,
		logger: logger,
		conn:   conn, // Сохраняем соединение
	}, nil
}

// GetUser вызывает gRPC метод GetUser на UserService.
func (c *userServiceGRPCClient) GetUser(ctx context.Context, userID string) (*userpb.UserResponse, error) {
	c.logger.InfoContext(ctx, "Calling UserService.GetUser gRPC method", slog.String("user_id", userID))

	if userID == "" {
		c.logger.WarnContext(ctx, "GetUser called with empty userID")
		return nil, status.Errorf(codes.InvalidArgument, "userID cannot be empty")
	}

	req := &userpb.GetUserRequest{
		UserId: userID,
	}

	// Устанавливаем таймаут для самого gRPC вызова
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	res, err := c.client.GetUser(callCtx, req)
	if err != nil {
		// Логируем ошибку с деталями gRPC статуса, если возможно
		st, ok := status.FromError(err)
		if ok {
			c.logger.ErrorContext(ctx, "UserService.GetUser gRPC call failed with status",
				slog.String("user_id", userID),
				slog.String("code", st.Code().String()),
				slog.String("message", st.Message()))
		} else {
			c.logger.ErrorContext(ctx, "UserService.GetUser gRPC call failed",
				slog.String("user_id", userID),
				slog.String("error", err.Error()))
		}
		return nil, fmt.Errorf("grpc GetUser failed for userID %s: %w", userID, err)
	}

	c.logger.InfoContext(ctx, "UserService.GetUser gRPC call successful", slog.String("user_id", userID), slog.String("username_returned", res.GetUsername()))
	return res, nil
}

// Close закрывает gRPC соединение.
// Этот метод можно добавить, чтобы корректно закрывать соединение при завершении работы сервиса.
func (c *userServiceGRPCClient) Close() error {
	if c.conn != nil {
		c.logger.Info("Closing gRPC connection to UserService")
		return c.conn.Close()
	}
	return nil
}
