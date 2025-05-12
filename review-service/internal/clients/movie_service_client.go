// review-service/internal/clients/movie_service_client.go
package clients

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	// ВАЖНО: Убедитесь, что этот импорт правильный.
	// Пакет moviepb должен быть доступен для ReviewService.
	// Это означает, что вы либо скопировали сгенерированные файлы moviepb
	// из MovieService в ReviewService (например, в review-service/internal/genproto/moviepb),
	// либо используете Go Workspaces.
	"review-service/internal/genproto/moviepb" // Если скопировали в review-service
	// "movie-service/internal/genproto/moviepb" // Если используете Go Workspace и MovieService доступен так

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes" // Для кодов ошибок gRPC
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status" // Для кодов ошибок gRPC
)

// MovieServiceClient определяет методы для взаимодействия с MovieInterService.
// Этот интерфейс должен совпадать с тем, что определен в review-service/internal/api/handlers.go
type MovieServiceClient interface {
	CheckMovieExists(ctx context.Context, movieID string) (bool, error)
	GetMovieInfo(ctx context.Context, movieID string) (*moviepb.MovieInfo, error)
	Close() error // Добавляем метод для закрытия соединения
}

// movieServiceGRPCClient реализует MovieServiceClient с использованием gRPC.
type movieServiceGRPCClient struct {
	client moviepb.MovieInterServiceClient // Сгенерированный gRPC клиент для MovieInterService
	logger *slog.Logger
	conn   *grpc.ClientConn // Сохраняем соединение, чтобы его можно было закрыть
}

// NewMovieServiceGRPCClient создает новый gRPC клиент для MovieService.
// movieServiceAddr - адрес gRPC сервера MovieService (например, "localhost:9092").
func NewMovieServiceGRPCClient(ctx context.Context, movieServiceAddr string, logger *slog.Logger) (MovieServiceClient, error) {
	logger.Info("Attempting to connect to MovieService gRPC", slog.String("address", movieServiceAddr))

	dialCtx, dialCancel := context.WithTimeout(ctx, 5*time.Second) // Таймаут на установку соединения
	defer dialCancel()

	conn, err := grpc.DialContext(dialCtx, movieServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()), // Для разработки; в продакшене используйте TLS
		grpc.WithBlock(), // Блокировать до установления соединения
	)
	if err != nil {
		logger.Error("Failed to connect to MovieService gRPC", slog.String("address", movieServiceAddr), slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to connect to movie service at %s: %w", movieServiceAddr, err)
	}
	logger.Info("Successfully connected to MovieService gRPC", slog.String("address", movieServiceAddr))

	grpcClient := moviepb.NewMovieInterServiceClient(conn) // Используем NewMovieInterServiceClient

	return &movieServiceGRPCClient{
		client: grpcClient,
		logger: logger,
		conn:   conn,
	}, nil
}

// CheckMovieExists вызывает gRPC метод CheckMovieExists на MovieService.
func (c *movieServiceGRPCClient) CheckMovieExists(ctx context.Context, movieID string) (bool, error) {
	c.logger.InfoContext(ctx, "Calling MovieService.CheckMovieExists gRPC method", slog.String("movie_id", movieID))

	if movieID == "" {
		c.logger.WarnContext(ctx, "CheckMovieExists called with empty movieID")
		return false, status.Errorf(codes.InvalidArgument, "movieID cannot be empty")
	}

	req := &moviepb.CheckMovieExistsRequest{
		MovieId: movieID,
	}

	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second) // Таймаут на сам вызов
	defer cancel()

	res, err := c.client.CheckMovieExists(callCtx, req)
	if err != nil {
		st, _ := status.FromError(err)
		c.logger.ErrorContext(ctx, "MovieService.CheckMovieExists gRPC call failed",
			slog.String("movie_id", movieID),
			slog.String("code", st.Code().String()),
			slog.String("message", st.Message()))
		return false, fmt.Errorf("grpc CheckMovieExists failed for movieID %s: %w", movieID, err)
	}

	c.logger.InfoContext(ctx, "MovieService.CheckMovieExists gRPC call successful", slog.String("movie_id", movieID), slog.Bool("exists", res.GetExists()))
	return res.GetExists(), nil
}

// GetMovieInfo вызывает gRPC метод GetMovieInfo на MovieService.
func (c *movieServiceGRPCClient) GetMovieInfo(ctx context.Context, movieID string) (*moviepb.MovieInfo, error) {
	c.logger.InfoContext(ctx, "Calling MovieService.GetMovieInfo gRPC method", slog.String("movie_id", movieID))

	if movieID == "" {
		c.logger.WarnContext(ctx, "GetMovieInfo called with empty movieID")
		return nil, status.Errorf(codes.InvalidArgument, "movieID cannot be empty")
	}

	req := &moviepb.GetMovieInfoRequest{
		MovieId: movieID,
	}

	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	res, err := c.client.GetMovieInfo(callCtx, req)
	if err != nil {
		st, _ := status.FromError(err)
		c.logger.ErrorContext(ctx, "MovieService.GetMovieInfo gRPC call failed",
			slog.String("movie_id", movieID),
			slog.String("code", st.Code().String()),
			slog.String("message", st.Message()))
		return nil, fmt.Errorf("grpc GetMovieInfo failed for movieID %s: %w", movieID, err)
	}

	if res.GetMovieInfo() == nil { // Добавим проверку, если MovieInfo может быть nil (например, фильм не найден)
		c.logger.WarnContext(ctx, "MovieService.GetMovieInfo returned nil MovieInfo", slog.String("movie_id", movieID))
		// Это может быть эквивалентно NotFound, если GetMovieInfo в MovieService так себя ведет
		return nil, status.Errorf(codes.NotFound, "movie info not found for ID %s", movieID)
	}

	c.logger.InfoContext(ctx, "MovieService.GetMovieInfo gRPC call successful",
		slog.String("movie_id", movieID),
		slog.String("title_returned", res.GetMovieInfo().GetTitle()))
	return res.GetMovieInfo(), nil
}

// Close закрывает gRPC соединение.
func (c *movieServiceGRPCClient) Close() error {
	if c.conn != nil {
		c.logger.Info("Closing gRPC connection to MovieService")
		return c.conn.Close()
	}
	return nil
}
