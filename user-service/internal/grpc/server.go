package grpc

import (
	"context"
	"errors"
	"log/slog" // Для логирования

	"user-service/internal/domain"          // Ваша доменная модель User
	"user-service/internal/genproto/userpb" // Сгенерированный gRPC код
	"user-service/internal/store"           // Ваш интерфейс UserStore

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server реализует интерфейс userpb.UserServiceServer
type Server struct {
	userpb.UnimplementedUserServiceServer                 // Обязательно для прямой совместимости
	store                                 store.UserStore // Зависимость от хранилища пользователей
	logger                                *slog.Logger
}

// NewServer создает новый экземпляр gRPC сервера для UserService.
func NewServer(userStore store.UserStore, logger *slog.Logger) *Server {
	return &Server{
		store:  userStore,
		logger: logger,
	}
}

// domainUserToProto преобразует доменную модель пользователя в protobuf сообщение
func domainUserToProto(user *domain.User) *userpb.UserResponse {
	if user == nil {
		return nil
	}
	return &userpb.UserResponse{
		Id:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: timestamppb.New(user.CreatedAt),
		UpdatedAt: timestamppb.New(user.UpdatedAt),
		// Поле Role можно добавить, если оно есть в UserResponse proto
	}
}

// GetUser реализует gRPC метод GetUser.
func (s *Server) GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.UserResponse, error) {
	s.logger.InfoContext(ctx, "gRPC GetUser called", slog.String("user_id", req.GetUserId()))

	if req.GetUserId() == "" {
		s.logger.WarnContext(ctx, "gRPC GetUser called with empty user_id")
		return nil, status.Errorf(codes.InvalidArgument, "user_id cannot be empty")
	}

	user, err := s.store.GetByID(ctx, req.GetUserId())
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			s.logger.WarnContext(ctx, "User not found by ID", slog.String("user_id", req.GetUserId()), slog.String("error", err.Error()))
			return nil, status.Errorf(codes.NotFound, "user not found with ID %s", req.GetUserId())
		}
		s.logger.ErrorContext(ctx, "Failed to get user by ID from store", slog.String("user_id", req.GetUserId()), slog.String("error", err.Error()))
		return nil, status.Errorf(codes.Internal, "failed to retrieve user details: %v", err)
	}

	s.logger.InfoContext(ctx, "User retrieved successfully via gRPC", slog.String("user_id", user.ID), slog.String("username", user.Username))
	return domainUserToProto(user), nil
}

// TODO: Реализовать другие gRPC методы, если они будут определены в user.proto
// (например, для регистрации, если решите делать ее через gRPC)
