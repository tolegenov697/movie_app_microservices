package store

import (
	"context"
	"errors"
	"log" // Можно заменить на slog, если передавать его в MockMovieStore
	"movie-service/internal/domain"
	"sort"
	"strings"
	"sync" // Для защиты доступа к in-memory карте
	"time"
)

var (
	ErrMovieNotFound      = errors.New("movie not found")
	ErrMovieAlreadyExists = errors.New("movie with these identifying features already exists")
)

type MovieListParams struct {
	Page        int
	PageSize    int
	Genre       string
	Year        int
	SearchQuery string
	SortBy      string
	Status      domain.MovieStatus
}

type MovieStore interface {
	Create(ctx context.Context, movie *domain.Movie) error
	GetByID(ctx context.Context, id string) (*domain.Movie, error)
	Update(ctx context.Context, movie *domain.Movie) error // Пока не реализован в Mock
	Delete(ctx context.Context, id string) error           // Пока не реализован в Mock
	List(ctx context.Context, params MovieListParams) ([]*domain.Movie, int, error)
	UpdateStatus(ctx context.Context, id string, status domain.MovieStatus) error
}

type MockMovieStore struct {
	mu               sync.RWMutex
	movies           map[string]*domain.Movie // Фильмы, созданные во время выполнения
	predefinedMovies map[string]*domain.Movie // Предопределенные фильмы для тестов
}

func NewMockMovieStore() *MockMovieStore {
	predefined := map[string]*domain.Movie{
		"existing-approved-id": {ID: "existing-approved-id", Title: "Одобренный тестовый фильм 1", Description: "Описание одобренного фильма 1", ReleaseYear: 2022, Genres: []string{"Sci-Fi", "Action"}, Status: domain.StatusApproved, CreatedAt: time.Now().Add(-72 * time.Hour), UpdatedAt: time.Now().Add(-72 * time.Hour), SubmittedByUserID: "user1"},
		"another-approved-id":  {ID: "another-approved-id", Title: "Другой одобренный фильм 2", Description: "Описание одобренного фильма 2", ReleaseYear: 2023, Genres: []string{"Comedy"}, Status: domain.StatusApproved, CreatedAt: time.Now().Add(-48 * time.Hour), UpdatedAt: time.Now().Add(-48 * time.Hour), SubmittedByUserID: "user2"},
		"yet-another-approved": {ID: "yet-another-approved", Title: "Еще один фильм (одобрен) 3", Description: "Описание фильма 3", ReleaseYear: 2022, Genres: []string{"Drama", "Thriller"}, Status: domain.StatusApproved, CreatedAt: time.Now().Add(-96 * time.Hour), UpdatedAt: time.Now().Add(-96 * time.Hour), SubmittedByUserID: "user1"},
		"pending-movie-id":     {ID: "pending-movie-id", Title: "Тестовый фильм на модерации 4", Description: "Описание фильма 4", ReleaseYear: 2024, Genres: []string{"Drama"}, Status: domain.StatusPendingApproval, CreatedAt: time.Now().Add(-24 * time.Hour), UpdatedAt: time.Now().Add(-24 * time.Hour), SubmittedByUserID: "user3"},
		"early-bird-approved":  {ID: "early-bird-approved", Title: "Ранняя пташка (одобрен) 5", Description: "Описание фильма 5", ReleaseYear: 2021, Genres: []string{"Adventure", "Sci-Fi"}, Status: domain.StatusApproved, CreatedAt: time.Now().Add(-120 * time.Hour), UpdatedAt: time.Now().Add(-120 * time.Hour), SubmittedByUserID: "user2"},
	}
	return &MockMovieStore{
		movies:           make(map[string]*domain.Movie),
		predefinedMovies: predefined,
	}
}

func (m *MockMovieStore) Create(ctx context.Context, movie *domain.Movie) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	log.Printf("[MOCK STORE] Creating movie: Title='%s', ID='%s'\n", movie.Title, movie.ID)

	// Проверяем и в runtime созданных, и в предопределенных (хотя ID должен быть уникальным)
	if _, exists := m.movies[movie.ID]; exists {
		return ErrMovieAlreadyExists
	}
	if _, exists := m.predefinedMovies[movie.ID]; exists {
		return ErrMovieAlreadyExists
	}
	// Клонируем фильм перед сохранением, чтобы избежать изменения оригинала извне через указатель
	movieCopy := *movie
	m.movies[movie.ID] = &movieCopy
	return nil
}

func (m *MockMovieStore) GetByID(ctx context.Context, id string) (*domain.Movie, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	log.Printf("[MOCK STORE] Getting movie by ID: %s\n", id)

	if movie, ok := m.movies[id]; ok {
		movieCopy := *movie // Возвращаем копию
		return &movieCopy, nil
	}
	if movie, ok := m.predefinedMovies[id]; ok {
		movieCopy := *movie // Возвращаем копию
		return &movieCopy, nil
	}
	return nil, ErrMovieNotFound
}

func (m *MockMovieStore) List(ctx context.Context, params MovieListParams) ([]*domain.Movie, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	log.Printf("[MOCK STORE] Listing movies with params: %+v\n", params)

	var allMoviesSource []*domain.Movie // Собираем указатели на оригиналы для фильтрации
	for _, movie := range m.predefinedMovies {
		allMoviesSource = append(allMoviesSource, movie)
	}
	for _, movie := range m.movies {
		allMoviesSource = append(allMoviesSource, movie)
	}

	var filteredMovies []domain.Movie // Здесь будут копии отфильтрованных фильмов

	for _, moviePtr := range allMoviesSource {
		movie := *moviePtr // Работаем с копией для проверок
		keep := true
		// Фильтр по статусу
		if params.Status != "" && movie.Status != params.Status {
			keep = false
		}
		// Фильтр по жанру
		if keep && params.Genre != "" {
			foundGenre := false
			for _, g := range movie.Genres {
				if strings.EqualFold(g, params.Genre) {
					foundGenre = true
					break
				}
			}
			if !foundGenre {
				keep = false
			}
		}
		// Фильтр по году
		if keep && params.Year != 0 && movie.ReleaseYear != params.Year {
			keep = false
		}
		// Фильтр по поисковому запросу
		if keep && params.SearchQuery != "" && !strings.Contains(strings.ToLower(movie.Title), strings.ToLower(params.SearchQuery)) {
			keep = false
		}

		if keep {
			filteredMovies = append(filteredMovies, movie) // Добавляем копию в результат
		}
	}

	// Сортировка
	sort.SliceStable(filteredMovies, func(i, j int) bool {
		switch params.SortBy {
		case "title_asc":
			return strings.ToLower(filteredMovies[i].Title) < strings.ToLower(filteredMovies[j].Title)
		case "title_desc":
			return strings.ToLower(filteredMovies[i].Title) > strings.ToLower(filteredMovies[j].Title)
		case "release_year_asc":
			if filteredMovies[i].ReleaseYear == filteredMovies[j].ReleaseYear {
				return strings.ToLower(filteredMovies[i].Title) < strings.ToLower(filteredMovies[j].Title)
			}
			return filteredMovies[i].ReleaseYear < filteredMovies[j].ReleaseYear
		case "release_year_desc":
			if filteredMovies[i].ReleaseYear == filteredMovies[j].ReleaseYear {
				return strings.ToLower(filteredMovies[i].Title) < strings.ToLower(filteredMovies[j].Title)
			}
			return filteredMovies[i].ReleaseYear > filteredMovies[j].ReleaseYear
		case "created_at_asc":
			return filteredMovies[i].CreatedAt.Before(filteredMovies[j].CreatedAt)
		default: // "created_at_desc" или неизвестное значение
			return filteredMovies[i].CreatedAt.After(filteredMovies[j].CreatedAt)
		}
	})

	totalCount := len(filteredMovies)
	if params.Page == 0 {
		params.Page = 1
	}
	if params.PageSize == 0 {
		params.PageSize = 10
	}
	start := (params.Page - 1) * params.PageSize
	end := start + params.PageSize
	if start < 0 {
		start = 0
	}
	if start >= totalCount {
		return []*domain.Movie{}, totalCount, nil
	}
	if end > totalCount {
		end = totalCount
	}

	paginatedMovies := filteredMovies[start:end]
	resultMoviesPtrs := make([]*domain.Movie, len(paginatedMovies))
	for i := range paginatedMovies {
		// Создаем новую копию для каждого элемента в итоговом слайсе указателей
		movieCopy := paginatedMovies[i]
		resultMoviesPtrs[i] = &movieCopy
	}
	return resultMoviesPtrs, totalCount, nil
}

func (m *MockMovieStore) UpdateStatus(ctx context.Context, id string, status domain.MovieStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	log.Printf("[MOCK STORE] Updating status for movie ID %s to %s\n", id, status)

	// Сначала ищем в runtime созданных фильмах
	if movie, ok := m.movies[id]; ok {
		movie.Status = status
		movie.UpdatedAt = time.Now().UTC()
		return nil
	}
	// Затем в предопределенных
	if movie, ok := m.predefinedMovies[id]; ok {
		// Обновляем копию в predefinedMovies (или сам элемент, если считаем это приемлемым для мока)
		// Для простоты мока, обновим напрямую, но для реальной БД это была бы операция UPDATE
		movie.Status = status
		movie.UpdatedAt = time.Now().UTC()
		// m.predefinedMovies[id] = movie // Если movie был бы локальной копией
		return nil
	}
	return ErrMovieNotFound
}

// Заглушки для нереализованных методов интерфейса
func (m *MockMovieStore) Update(ctx context.Context, movie *domain.Movie) error {
	log.Printf("[MOCK STORE] Update method called for movie ID %s (NOT IMPLEMENTED)\n", movie.ID)
	// TODO: Реализовать обновление полей фильма, если потребуется для тестов
	return errors.New("mock update not implemented")
}

func (m *MockMovieStore) Delete(ctx context.Context, id string) error {
	log.Printf("[MOCK STORE] Delete method called for movie ID %s (NOT IMPLEMENTED)\n", id)
	// TODO: Реализовать удаление фильма из m.movies, если потребуется для тестов
	return errors.New("mock delete not implemented")
}
