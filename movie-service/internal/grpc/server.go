// movie-service/internal/grpc/server.go
package grpc

import (
	"context"
	"errors"
	"log/slog"

	"movie-service/internal/domain"           // Ваша доменная модель Movie
	"movie-service/internal/genproto/moviepb" // Сгенерированный gRPC код
	"movie-service/internal/store"            // Ваш интерфейс MovieStore

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server реализует интерфейс moviepb.MovieInterServiceServer
type Server struct {
	moviepb.UnimplementedMovieInterServiceServer                  // Обязательно для прямой совместимости
	store                                        store.MovieStore // Зависимость от хранилища фильмов
	logger                                       *slog.Logger
}

// NewServer создает новый экземпляр gRPC сервера для MovieService.
func NewServer(movieStore store.MovieStore, logger *slog.Logger) *Server {
	return &Server{
		store:  movieStore,
		logger: logger,
	}
}

// domainMovieToProtoInfo преобразует доменную модель фильма в MovieInfo protobuf сообщение
func domainMovieToProtoInfo(movie *domain.Movie) *moviepb.MovieInfo {
	if movie == nil {
		return nil
	}
	return &moviepb.MovieInfo{
		Id:          movie.ID,
		Title:       movie.Title,
		ReleaseYear: int32(movie.ReleaseYear), // int в int32
		Status:      string(movie.Status),     // domain.MovieStatus в string
		// PosterUrl: movie.PosterURL, // Если добавите в MovieInfo
	}
}

// GetMovieInfo реализует gRPC метод GetMovieInfo.
func (s *Server) GetMovieInfo(ctx context.Context, req *moviepb.GetMovieInfoRequest) (*moviepb.GetMovieInfoResponse, error) {
	s.logger.InfoContext(ctx, "gRPC GetMovieInfo called", slog.String("movie_id", req.GetMovieId()))

	if req.GetMovieId() == "" {
		s.logger.WarnContext(ctx, "gRPC GetMovieInfo called with empty movie_id")
		return nil, status.Errorf(codes.InvalidArgument, "movie_id cannot be empty")
	}

	movie, err := s.store.GetByID(ctx, req.GetMovieId()) // Используем существующий MockMovieStore
	if err != nil {
		if errors.Is(err, store.ErrMovieNotFound) {
			s.logger.WarnContext(ctx, "Movie not found by ID for GetMovieInfo", slog.String("movie_id", req.GetMovieId()))
			return nil, status.Errorf(codes.NotFound, "movie not found with ID %s", req.GetMovieId())
		}
		s.logger.ErrorContext(ctx, "Failed to get movie by ID from store for GetMovieInfo", slog.String("movie_id", req.GetMovieId()), slog.String("error", err.Error()))
		return nil, status.Errorf(codes.Internal, "failed to retrieve movie details: %v", err)
	}

	s.logger.InfoContext(ctx, "Movie info retrieved successfully via gRPC", slog.String("movie_id", movie.ID))
	return &moviepb.GetMovieInfoResponse{MovieInfo: domainMovieToProtoInfo(movie)}, nil
}

// CheckMovieExists реализует gRPC метод CheckMovieExists.
func (s *Server) CheckMovieExists(ctx context.Context, req *moviepb.CheckMovieExistsRequest) (*moviepb.CheckMovieExistsResponse, error) {
	s.logger.InfoContext(ctx, "gRPC CheckMovieExists called", slog.String("movie_id", req.GetMovieId()))

	if req.GetMovieId() == "" {
		s.logger.WarnContext(ctx, "gRPC CheckMovieExists called with empty movie_id")
		return nil, status.Errorf(codes.InvalidArgument, "movie_id cannot be empty")
	}

	movie, err := s.store.GetByID(ctx, req.GetMovieId()) // Используем существующий MockMovieStore
	if err != nil {
		if errors.Is(err, store.ErrMovieNotFound) {
			s.logger.InfoContext(ctx, "Movie does not exist (checked via gRPC)", slog.String("movie_id", req.GetMovieId()))
			return &moviepb.CheckMovieExistsResponse{Exists: false}, nil
		}
		s.logger.ErrorContext(ctx, "Failed to check movie existence from store", slog.String("movie_id", req.GetMovieId()), slog.String("error", err.Error()))
		return nil, status.Errorf(codes.Internal, "failed to check movie existence: %v", err)
	}

	// Если фильм найден (movie != nil) и нет ошибки, значит он существует.
	// Мы можем также проверить статус, если это важно для CheckMovieExists.
	// Например, считать существующим только 'approved' фильм.
	// Для простоты, пока считаем любой найденный фильм существующим.
	s.logger.InfoContext(ctx, "Movie exists (checked via gRPC)", slog.String("movie_id", movie.ID))
	return &moviepb.CheckMovieExistsResponse{Exists: true}, nil
}
   