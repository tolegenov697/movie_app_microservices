package store

import (
	"context"
	"errors"
	"log" // Используем стандартный log для мока, можно заменить на slog если передавать его
	"review-service/internal/domain"
	"sync" // Для безопасного доступа к картам из горутин
	"time" // Для CreatedAt/UpdatedAt
)

// Кастомные ошибки
var (
	ErrReviewNotFound  = errors.New("review not found")
	ErrDuplicateReview = errors.New("user has already reviewed this movie")
)

// ListReviewsParams параметры для получения списка отзывов
type ListReviewsParams struct {
	Page     int
	PageSize int
	SortBy   string // Например, "created_at_desc", "rating_desc"
}

// ReviewStore определяет интерфейс для операций с данными отзывов.
type ReviewStore interface {
	Create(ctx context.Context, review *domain.Review) error
	GetByID(ctx context.Context, reviewID string) (*domain.Review, error)
	Update(ctx context.Context, review *domain.Review) error
	Delete(ctx context.Context, reviewID string, userID string) error
	GetReviewsByMovieID(ctx context.Context, movieID string, params ListReviewsParams) ([]*domain.Review, int, error)
	GetReviewsByUserID(ctx context.Context, userID string, params ListReviewsParams) ([]*domain.Review, int, error)
	GetAggregatedRatingByMovieID(ctx context.Context, movieID string) (*domain.AggregatedRating, error)
}

// MockReviewStore для начальной разработки и тестов
type MockReviewStore struct {
	mu             sync.RWMutex
	reviews        map[string]*domain.Review   // Ключ: reviewID
	reviewsByMovie map[string][]*domain.Review // Ключ: movieID, значение: слайс указателей на отзывы
	nextReviewIdx  map[string]map[string]bool  // Для проверки ErrDuplicateReview: map[movieID]map[userID]bool
}

// NewMockReviewStore создает новый экземпляр MockReviewStore
func NewMockReviewStore() *MockReviewStore {
	return &MockReviewStore{
		reviews:        make(map[string]*domain.Review),
		reviewsByMovie: make(map[string][]*domain.Review),
		nextReviewIdx:  make(map[string]map[string]bool),
	}
}

func (m *MockReviewStore) Create(ctx context.Context, review *domain.Review) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	//log.Printf("[MOCK REVIEW STORE] Attempting to create review: ID='%s' for MovieID='%s' by UserID='%s'\n", review.ID, review.MovieID, review.UserID)

	// Проверка на дубликат отзыва (один пользователь - один отзыв на фильм)
	//if m.nextReviewIdx[review.MovieID] != nil && m.nextReviewIdx[review.MovieID][review.UserID] {
	//	log.Printf("[MOCK REVIEW STORE] Duplicate review detected for MovieID='%s' by UserID='%s'\n", review.MovieID, review.UserID)
	//	return ErrDuplicateReview
	//}

	if _, exists := m.reviews[review.ID]; exists {
		// Этого не должно случиться, если ID генерируется как UUID
		log.Printf("[MOCK REVIEW STORE] Review with ID='%s' already exists (should be unique UUID)\n", review.ID)
		return errors.New("review with this ID already exists")
	}

	reviewCopy := *review // Сохраняем копию
	m.reviews[review.ID] = &reviewCopy
	m.reviewsByMovie[review.MovieID] = append(m.reviewsByMovie[review.MovieID], &reviewCopy)

	if m.nextReviewIdx[review.MovieID] == nil {
		m.nextReviewIdx[review.MovieID] = make(map[string]bool)
	}
	m.nextReviewIdx[review.MovieID][review.UserID] = true

	log.Printf("[MOCK REVIEW STORE] Created review: ID='%s'\n", review.ID)
	return nil
}

func (m *MockReviewStore) GetReviewsByMovieID(ctx context.Context, movieID string, params ListReviewsParams) ([]*domain.Review, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	log.Printf("[MOCK REVIEW STORE] GetReviewsByMovieID called for MovieID='%s', Params: %+v\n", movieID, params)

	movieReviews, ok := m.reviewsByMovie[movieID]
	if !ok || len(movieReviews) == 0 {
		return []*domain.Review{}, 0, nil
	}

	// Копируем, чтобы избежать изменения оригиналов при сортировке или других операциях
	reviewsCopy := make([]*domain.Review, len(movieReviews))
	for i, revPtr := range movieReviews {
		temp := *revPtr // Создаем копию значения
		reviewsCopy[i] = &temp
	}

	// TODO: Реализовать сортировку на основе params.SortBy, если нужно
	// Пример: if params.SortBy == "created_at_desc" { sort.Slice(...) }

	// Пагинация
	totalCount := len(reviewsCopy)
	start := (params.Page - 1) * params.PageSize
	end := start + params.PageSize

	if start < 0 {
		start = 0
	}
	if start >= totalCount {
		return []*domain.Review{}, totalCount, nil // Запрошенная страница за пределами данных
	}
	if end > totalCount {
		end = totalCount
	}

	return reviewsCopy[start:end], totalCount, nil
}

func (m *MockReviewStore) GetByID(ctx context.Context, reviewID string) (*domain.Review, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if review, ok := m.reviews[reviewID]; ok {
		reviewCopy := *review // Возвращаем копию
		return &reviewCopy, nil
	}
	return nil, ErrReviewNotFound
}

func (m *MockReviewStore) Update(ctx context.Context, review *domain.Review) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	log.Printf("[MOCK REVIEW STORE] Update called for review ID %s (NOT FULLY IMPLEMENTED)\n", review.ID)
	if _, ok := m.reviews[review.ID]; !ok {
		return ErrReviewNotFound
	}
	// Просто обновляем время, в реальной реализации нужно обновлять поля
	reviewCopy := *review
	reviewCopy.UpdatedAt = time.Now().UTC()
	m.reviews[review.ID] = &reviewCopy

	// Обновление в m.reviewsByMovie более сложное, нужно найти и заменить элемент
	// Для простоты мока пока не реализуем полное обновление в reviewsByMovie
	return nil
}

func (m *MockReviewStore) Delete(ctx context.Context, reviewID string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	log.Printf("[MOCK REVIEW STORE] Delete called for review ID %s by UserID %s (NOT FULLY IMPLEMENTED)\n", reviewID, userID)

	reviewToDelete, ok := m.reviews[reviewID]
	if !ok {
		return ErrReviewNotFound
	}
	// TODO: Добавить проверку userID, если нужно, чтобы пользователь удалял только свои отзывы
	// if reviewToDelete.UserID != userID { return errors.New("user not authorized to delete this review")}

	delete(m.reviews, reviewID)

	// Удаление из m.reviewsByMovie
	movieID := reviewToDelete.MovieID
	if reviewsForMovie, movieFound := m.reviewsByMovie[movieID]; movieFound {
		newReviewsForMovie := []*domain.Review{}
		for _, rev := range reviewsForMovie {
			if rev.ID != reviewID {
				newReviewsForMovie = append(newReviewsForMovie, rev)
			}
		}
		if len(newReviewsForMovie) > 0 {
			m.reviewsByMovie[movieID] = newReviewsForMovie
		} else {
			delete(m.reviewsByMovie, movieID) // Удаляем ключ, если для фильма не осталось отзывов
		}
	}

	// Удаление из индекса для проверки дубликатов
	if m.nextReviewIdx[movieID] != nil {
		delete(m.nextReviewIdx[movieID], userID)
		if len(m.nextReviewIdx[movieID]) == 0 {
			delete(m.nextReviewIdx, movieID)
		}
	}
	return nil
}

func (m *MockReviewStore) GetReviewsByUserID(ctx context.Context, userID string, params ListReviewsParams) ([]*domain.Review, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	log.Printf("[MOCK REVIEW STORE] GetReviewsByUserID called for UserID='%s', Params: %+v\n", userID, params)

	var userReviews []*domain.Review
	for _, review := range m.reviews { // Перебираем все отзывы
		if review.UserID == userID {
			reviewCopy := *review
			userReviews = append(userReviews, &reviewCopy)
		}
	}
	// TODO: Добавить сортировку и пагинацию аналогично GetReviewsByMovieID
	return userReviews, len(userReviews), nil
}

func (m *MockReviewStore) GetAggregatedRatingByMovieID(ctx context.Context, movieID string) (*domain.AggregatedRating, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	log.Printf("[MOCK REVIEW STORE] GetAggregatedRatingByMovieID called for MovieID='%s'\n", movieID)

	movieReviews, ok := m.reviewsByMovie[movieID]
	if !ok || len(movieReviews) == 0 {
		return &domain.AggregatedRating{MovieID: movieID, AverageRating: 0, RatingCount: 0}, nil
	}

	var sumRating int32
	var ratingCount int64
	for _, reviewPtr := range movieReviews {
		sumRating += reviewPtr.Rating
		ratingCount++
	}

	var avgRating float64
	if ratingCount > 0 {
		avgRating = float64(sumRating) / float64(ratingCount)
	}

	return &domain.AggregatedRating{MovieID: movieID, AverageRating: avgRating, RatingCount: ratingCount}, nil
}
