# Project Documentation: Go Microservices Platform

## 1. Introduction

This document provides a comprehensive overview of the Go-based microservices project. The system is designed to manage users, movies, and reviews, with each domain handled by a dedicated microservice. Communication between services is facilitated by gRPC, and each service exposes a public RESTful HTTP API for client interactions. Data persistence is achieved using PostgreSQL.

## 2. System Architecture

The system comprises three core microservices: User Service, Movie Service, and Review Service.

![System Architecture Diagram (Conceptual)](https://placehold.co/800x400/ FFF/333?text=Conceptual+System+Architecture+Diagram)
*(This is a placeholder. You should replace this with an actual diagram of your architecture, perhaps showing how services interact, API gateways (if any), and databases.)*

### 2.1. Microservices

#### 2.1.1. User Service
* **Responsibilities:**
    * Manages user lifecycle: registration, authentication (login), profile management.
    * Handles user data storage and retrieval.
    * Provides user information to other services (e.g., Review Service) via gRPC.
* **Technologies:**
    * Language: Go
    * Communication:
        * Exposes RESTful HTTP API for client applications.
        * Exposes gRPC API for inter-service communication.
    * Database: PostgreSQL (dedicated `user_service_db`)
    * Authentication: JWT-based.
* **Key Components (based on provided code):**
    * `cmd/userservice/main.go`: Entry point, initializes HTTP and gRPC servers, database connection, token manager.
    * `internal/api/handlers.go`: HTTP request handlers for user operations.
    * `internal/grpc/server.go`: gRPC server implementation for user-related RPCs.
    * `internal/store/postgres_user_store.go`: PostgreSQL storage implementation for users.
    * `pkg/auth/`: JWT generation and password hashing utilities.

#### 2.1.2. Movie Service
* **Responsibilities:**
    * Manages movie catalog: creating, reading, updating, and deleting movie information.
    * Handles movie approval workflow (e.g., pending, approved, rejected statuses).
    * Provides movie information to other services (e.g., Review Service) via gRPC.
* **Technologies:**
    * Language: Go
    * Communication:
        * Exposes RESTful HTTP API for client applications and administrative tasks.
        * Exposes gRPC API for inter-service communication.
    * Database: PostgreSQL (dedicated `movie_service_db`)
* **Key Components (based on provided code):**
    * `cmd/movieservice/main.go`: Entry point, initializes HTTP and gRPC servers, database connection.
    * `internal/api/handlers.go`: HTTP request handlers for movie operations.
    * `internal/grpc/server.go`: gRPC server implementation for movie-related RPCs.
    * `internal/store/postgres_movie_store.go`: PostgreSQL storage implementation for movies.

#### 2.1.3. Review Service
* **Responsibilities:**
    * Manages user reviews for movies.
    * Allows users to create, read, update, and delete reviews.
    * Calculates and provides aggregated ratings for movies.
    * Interacts with User Service (to validate UserID, get user details) and Movie Service (to validate MovieID, get movie details) via gRPC.
* **Technologies:**
    * Language: Go
    * Communication:
        * Exposes RESTful HTTP API for client applications.
        * Acts as a gRPC client to User Service and Movie Service.
    * Database: PostgreSQL (dedicated `review_service_db`)
* **Key Components (based on provided code):**
    * `cmd/reviewservice/main.go`: Entry point, initializes HTTP server, database connection, and gRPC clients for User and Movie services.
    * `internal/api/handlers.go`: HTTP request handlers for review operations.
    * `internal/clients/`: gRPC client implementations for User and Movie services.
    * `internal/store/postgres_review_store.go`: PostgreSQL storage implementation for reviews.

### 2.2. Communication Between Microservices
* **Protocol:** gRPC is used for synchronous inter-service communication.
* **Protocol Buffers:** Messages are defined using Protocol Buffers (`.proto` files) to ensure language-agnostic and efficient data exchange.
    * `user.proto`: Defines messages and service for User Service.
    * `movie.proto`: Defines messages and service for Movie Service.
* **Interactions:**
    * Review Service calls User Service (e.g., to get user details for a review).
    * Review Service calls Movie Service (e.g., to check if a movie exists or get movie details).

### 2.3. Data Persistence
* **Database:** PostgreSQL is used as the database for all microservices.
* **Schema Design:** Each microservice has its own dedicated database (or could use separate schemas within a single database instance for isolation).
    * `user_service_db`: Stores user information (ID, username, email, password hash, role, timestamps).
    * `movie_service_db`: Stores movie information (ID, title, description, release year, director, genres, cast, poster/trailer URLs, submitter ID, status, timestamps).
    * `review_service_db`: Stores review information (ID, movie ID, user ID, rating, comment, timestamps).
* **CRUD Operations:** Each service implements CRUD operations to interact with its respective database.

### 2.4. Concurrency and Error Handling
* **Concurrency:** Goroutines and channels are utilized for concurrent operations, such as handling multiple HTTP/gRPC requests simultaneously and managing background tasks.
* **Error Handling:**
    * Services use custom error types (e.g., `store.ErrUserNotFound`, `store.ErrMovieAlreadyExists`).
    * Structured logging (slog) is implemented for informative error messages and debugging.
    * HTTP handlers return appropriate status codes and error messages in JSON format.
    * gRPC services return standard gRPC error codes.

## 3. API Endpoint Documentation

The following sections outline the primary HTTP RESTful API endpoints exposed by each microservice.

*(Note: This is based on the provided handler code. For full API documentation, consider generating Swagger/OpenAPI specifications from your code or annotations.)*

### 3.1. User Service (Port: 8080)

* **Authentication:** Uses JWT. A token is returned upon successful login and expected in the `Authorization` header (Bearer token) for protected endpoints.

| Method | Path                 | Description                                  | Request Body (JSON)                                     | Response (JSON)                                                                 | Auth Required |
| :----- | :------------------- | :------------------------------------------- | :------------------------------------------------------ | :------------------------------------------------------------------------------ | :------------ |
| `POST` | `/register`          | Registers a new user.                        | `domain.RegisterRequest` (username, email, password)    | `domain.User` (ID, username, email, role, timestamps) - without password hash | No            |
| `POST` | `/login`             | Logs in an existing user.                    | `domain.LoginRequest` (email, password)                 | `domain.LoginResponse` (User object, JWT token)                                 | No            |
| `GET`  | `/profile`           | Retrieves the authenticated user's profile.  | N/A                                                     | `domain.User` (ID, username, email, role, timestamps)                           | Yes           |
| `PUT`  | `/profile`           | Updates the authenticated user's profile.    | `domain.UpdateProfileRequest` (optional username, email) | `domain.User` (ID, username, email, role, timestamps)                           | Yes           |

### 3.2. Movie Service (Port: 8081)

* **Authentication:** Some endpoints (admin-related) would typically require authentication (e.g., admin role). This is not fully detailed in the provided snippets but implied by paths like `/admin/...`.

| Method | Path                                      | Description                                                                 | Request Body (JSON)                                                                                             | Response (JSON)                                                                                                                               | Auth Required |
| :----- | :---------------------------------------- | :-------------------------------------------------------------------------- | :-------------------------------------------------------------------------------------------------------------- | :-------------------------------------------------------------------------------------------------------------------------------------------- | :------------ |
| `POST` | `/movies`                                 | Creates a new movie (initially in `pending_approval` status).             | `domain.CreateMovieRequest` (title, description, year, director, genres, cast, posterURL, trailerURL)         | `domain.Movie` (full movie object)                                                                                                            | (Likely Yes)  |
| `GET`  | `/movies`                                 | Retrieves a list of approved movies. Supports pagination and filtering.     | Query Params: `page`, `limit`, `genre`, `search`, `sort_by`, `year`                                             | `{ movies: [domain.Movie], total_count, page, page_size }`                                                                                    | No            |
| `GET`  | `/movies/{movieId}`                       | Retrieves a specific approved movie by its ID.                              | Path Param: `movieId`                                                                                           | `domain.Movie` (full movie object)                                                                                                            | No            |
| `GET`  | `/admin/movies/pending`                   | (STUB) Retrieves a list of movies pending approval.                         | Query Params: (similar to `/movies`)                                                                            | `[]domain.Movie` (or paginated response)                                                                                                      | Yes (Admin)   |
| `POST` | `/admin/movies/{movieId}/approve`         | Approves a movie, changing its status to `approved`.                        | Path Param: `movieId`                                                                                           | `{ message: "Movie approved successfully" }`                                                                                                  | Yes (Admin)   |
| `POST` | `/admin/movies/{movieId}/reject`          | (STUB) Rejects a movie, changing its status to `rejected`.                  | Path Param: `movieId`                                                                                           | `{ message: "Movie rejected successfully (stub response)" }`                                                                                  | Yes (Admin)   |

### 3.3. Review Service (Port: 8082)

* **Authentication:** Creating a review implies an authenticated user. The provided code hardcodes a `userID` for `CreateReview` but would typically get this from an auth token.

| Method | Path                               | Description                                                              | Request Body (JSON)                                            | Response (JSON)                                                                                                                                     | Auth Required |
| :----- | :--------------------------------- | :----------------------------------------------------------------------- | :------------------------------------------------------------- | :-------------------------------------------------------------------------------------------------------------------------------------------------- | :------------ |
| `POST` | `/reviews`                         | Creates a new review for a movie.                                        | `domain.CreateReviewRequest` (movieID, rating, comment)        | `domain.Review` (full review object)                                                                                                                | Yes           |
| `GET`  | `/movies/{movieId}/reviews`        | Retrieves reviews for a specific movie. Supports pagination and sorting. | Path Param: `movieId`. Query Params: `page`, `limit`, `sort_by`  | `{ reviews: [domain.Review (enriched with username, movieTitle)], total_count, page, page_size }`                                                  | No            |
| `GET`  | `/movies/{movieId}/rating`         | Retrieves the aggregated rating for a specific movie.                    | Path Param: `movieId`                                          | `domain.AggregatedRating` (average_rating, rating_count)                                                                                            | No            |
| `GET`  | `/users/{userId}/reviews`          | Retrieves reviews submitted by a specific user.                          | Path Param: `userId`. Query Params: `page`, `limit`, `sort_by`   | `{ reviews: [domain.Review (enriched with username, movieTitle)], total_count, page, page_size }`                                                  | No (or Yes for own reviews) |
| `PUT`  | `/reviews/{reviewId}`              | (STUB) Updates an existing review.                                       | Path Param: `reviewId`. Request Body: (similar to create)      | `{ message: "UpdateReview not implemented" }`                                                                                                       | Yes (Owner)   |
| `DELETE`| `/reviews/{reviewId}`             | (STUB) Deletes an existing review.                                       | Path Param: `reviewId`                                         | `{ message: "DeleteReview not implemented" }`                                                                                                       | Yes (Owner or Admin) |

## 4. gRPC API Documentation (Conceptual)

Each service (`user-service`, `movie-service`) also exposes a gRPC server for inter-service communication. The Review Service acts as a gRPC client to these services.

### 4.1. User Service (gRPC Port: 9091)
* **Proto File:** `userpb/user.proto`
* **Services & RPCs (example):**
    * `service UserService { rpc GetUser (GetUserRequest) returns (UserResponse); }`
    * `GetUserRequest`: Contains `user_id`.
    * `UserResponse`: Contains user details like `id`, `username`, `email`, `role`.

### 4.2. Movie Service (gRPC Port: 9092)
* **Proto File:** `moviepb/movie.proto`
* **Services & RPCs (example):**
    * `service MovieInterService { rpc CheckMovieExists (CheckMovieExistsRequest) returns (CheckMovieExistsResponse); rpc GetMovieInfo (GetMovieInfoRequest) returns (MovieInfo); }`
    * `CheckMovieExistsRequest`: Contains `movie_id`.
    * `CheckMovieExistsResponse`: Contains a boolean `exists`.
    * `GetMovieInfoRequest`: Contains `movie_id`.
    * `MovieInfo`: Contains movie details like `id`, `title`.

## 5. Setup and Running the Project Locally

### 5.1. Prerequisites
* Go (version 1.23 or as specified in `go.mod`) installed.
* PostgreSQL server installed and running.
* `protoc` compiler and Go gRPC plugins (for regenerating protobuf code if `.proto` files are modified).

### 5.2. Database Setup
1.  **Create Databases:**
    * For User Service: `user_service_db`
    * For Movie Service: `movie_service_db`
    * For Review Service: `review_service_db`
    ```sql
    CREATE DATABASE user_service_db;
    CREATE DATABASE movie_service_db;
    CREATE DATABASE review_service_db;
    ```
2.  **Create Users & Grant Privileges:**
    Create a PostgreSQL user (e.g., `user_service_user1` with password `gogogogo` as per your example connection strings, though **using unique, strong passwords for each service is highly recommended in production**) and grant necessary permissions to the respective databases.
    ```sql
    CREATE USER my_project_user WITH PASSWORD 'your_strong_password';
    GRANT ALL PRIVILEGES ON DATABASE user_service_db TO my_project_user;
    GRANT ALL PRIVILEGES ON DATABASE movie_service_db TO my_project_user;
    GRANT ALL PRIVILEGES ON DATABASE review_service_db TO my_project_user;
    ```
    *(Adjust username, password, and permissions as needed. Ideally, each service should have its own user with least privilege access to its own database.)*
3.  **Run Migrations (if any):**
    * You will need to create the necessary tables within each database. This is typically done using a database migration tool (e.g., `golang-migrate/migrate`, `sql-migrate`, `gormigrate`) or by executing SQL scripts. The table schemas are implied by your `domain` and `store` packages.
    * **Example Table (Users - for `user_service_db`):**
        ```sql
        CREATE TABLE IF NOT EXISTS users (
            id UUID PRIMARY KEY,
            username VARCHAR(255) UNIQUE NOT NULL,
            email VARCHAR(255) UNIQUE NOT NULL,
            password_hash VARCHAR(255) NOT NULL,
            role VARCHAR(50) NOT NULL DEFAULT 'user',
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );
        ```
    * **Example Table (Movies - for `movie_service_db`):**
        ```sql
        CREATE TABLE IF NOT EXISTS movies (
            id UUID PRIMARY KEY,
            title VARCHAR(255) NOT NULL,
            description TEXT,
            release_year INT,
            director VARCHAR(255),
            genres TEXT[], -- Or a separate genres table and a join table
            cast TEXT[],   -- Or a separate cast table and a join table
            poster_url VARCHAR(255),
            trailer_url VARCHAR(255),
            submitted_by_user_id UUID, -- Can be NULL if submitted by system/admin
            status VARCHAR(50) NOT NULL DEFAULT 'pending_approval', -- e.g., pending_approval, approved, rejected
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            CONSTRAINT uq_movie_title UNIQUE (title) -- Example constraint
        );
        ```
    * **Example Table (Reviews - for `review_service_db`):**
        ```sql
        CREATE TABLE IF NOT EXISTS reviews (
            id UUID PRIMARY KEY,
            movie_id UUID NOT NULL, -- This would typically be a foreign key to movies.id in a real setup if they shared a DB or via application logic
            user_id UUID NOT NULL,  -- This would typically be a foreign key to users.id
            rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5), -- Or 1-10 depending on your scale
            comment TEXT,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            CONSTRAINT uq_user_movie_review UNIQUE (user_id, movie_id) -- A user can review a movie only once
        );
        ```

### 5.3. Environment Configuration
Set the following environment variables for each service (e.g., in your shell, a `.env` file loaded by your application, or run configuration):

* **User Service:**
    * `USER_SERVICE_DATABASE_URL`: e.g., `postgres://my_project_user:your_strong_password@localhost:5432/user_service_db?sslmode=disable`
    * `JWT_SECRET_KEY`: A long, random, secret string for signing JWTs.
* **Movie Service:**
    * `MOVIE_SERVICE_DATABASE_URL`: e.g., `postgres://my_project_user:your_strong_password@localhost:5432/movie_service_db?sslmode=disable`
* **Review Service:**
    * `REVIEW_SERVICE_DATABASE_URL`: e.g., `postgres://my_project_user:your_strong_password@localhost:5432/review_service_db?sslmode=disable`
    * `USER_SERVICE_GRPC_ADDR`: e.g., `localhost:9091` (Address of the User Service gRPC server)
    * `MOVIE_SERVICE_GRPC_ADDR`: e.g., `localhost:9092` (Address of the Movie Service gRPC server)

**Important Security Note:** The default database connection strings in your `main.go` files expose credentials. **Always** use environment variables for sensitive information and ensure default values are not production credentials. The provided `extractPassword` function is a good step for logging, but credentials should not be in code.

### 5.4. Running the Services
Navigate to the root directory of each microservice and run:

1.  **User Service:**
    ```bash
    cd user-service
    go mod tidy # To ensure all dependencies are downloaded
    go run ./cmd/userservice/main.go
    ```
    *(Listens on HTTP Port 8080 and gRPC Port 9091 by default)*

2.  **Movie Service:**
    ```bash
    cd movie-service
    go mod tidy
    go run ./cmd/movieservice/main.go
    ```
    *(Listens on HTTP Port 8081 and gRPC Port 9092 by default)*

3.  **Review Service:**
    ```bash
    cd review-service
    go mod tidy
    go run ./cmd/reviewservice/main.go
    ```
    *(Listens on HTTP Port 8082 by default)*

Ensure services are started in an order that respects dependencies if one service immediately tries to connect to another on startup (though gRPC clients often handle transient connection issues with backoff/retry, which is good practice to implement). In this case, User and Movie services can be started first or concurrently, followed by Review Service.

## 6. Dependencies

Key dependencies used across the services (from `go.mod` files):

* `github.com/go-playground/validator/v10`: For request validation.
* `github.com/google/uuid`: For generating UUIDs.
* `github.com/gorilla/mux`: HTTP router (used in Movie and Review services, User service might use it or stdlib).
* `github.com/jmoiron/sqlx`: SQL extensions for Go, making database interactions easier.
* `github.com/lib/pq`: PostgreSQL driver for Go.
* `google.golang.org/grpc`: gRPC library for Go.
* `google.golang.org/protobuf`: Protocol Buffers library for Go.
* `github.com/golang-jwt/jwt/v5`: (User Service) For JWT handling.
* `golang.org/x/crypto`: (User Service) For password hashing (bcrypt).

## 7. Testing and Benchmarking

* **Unit Tests:** Each microservice should have comprehensive unit tests validating its functionality. Use the standard Go `testing` package. Test handlers, store methods, and any business logic.
    * Example: For `movie-service`, create tests in `internal/api/handlers_test.go` and `internal/store/postgres_movie_store_test.go`.
* **Benchmarking:** Conduct benchmarking for critical components (e.g., database queries, frequently used API endpoints) using `testing.B` to identify performance bottlenecks.

## 8. Further Development & TODOs (from code comments)

* **Movie Service:**
    * Implement `GetPendingMovies` handler.
    * Implement `RejectMovie` handler.
* **Review Service:**
    * Implement `UpdateReview` handler.
    * Implement `DeleteReview` handler.
    * Secure `CreateReview` by getting `userID` from authenticated context instead of hardcoding.
* **User Service:**
    * Add uniqueness check for new email in `UpdateUserProfile` if it's different from the current one.
* **General:**
    * Implement robust authentication and authorization for all relevant endpoints (especially admin actions and user-specific data modification).
    * Implement database migrations systematically.
    * Consider adding an API Gateway to provide a single entry point for clients.
    * Enhance gRPC client connections with retry mechanisms and timeouts.
    * Implement distributed tracing for better observability in a microservices environment.
    * Containerize services using Docker for easier deployment and orchestration (e.g., with Kubernetes).

# 📦 Project Structure: `movie_app_microservices`

This monorepo is a Go Workspace containing three microservices: Movie, User, and Review. Each service follows a clean architecture with dedicated domains, APIs, gRPC interfaces, and storage layers.

```bash
movie_app_microservices/ ← Root of Git repository and Go Workspace
├── .git/ ← Git folder
├── .gitignore ← Git ignore rules
├── go.work ← Go Workspace file linking all modules
├── go.work.sum ← Checksums for workspace

├── movie-service/ ← 🎬 MovieService module
│ ├── cmd/
│ │ └── movieservice/
│ │ └── main.go ← Entry point for MovieService
│ ├── internal/
│ │ ├── api/ ← HTTP API (handlers, router)
│ │ ├── domain/ ← Domain models (e.g., Movie, CreateMovieRequest)
│ │ ├── store/ ← Store interface and implementation (e.g., PostgresMovieStore)
│ │ ├── grpc/ ← gRPC server implementation
│ │ └── genproto/ ← Generated gRPC code (e.g., moviepb/)
│ ├── pkg/ ← Optional shared utilities
│ ├── proto/ ← Source .proto files for gRPC
│ └── go.mod ← Go module definition

├── user-service/ ← 👤 UserService module
│ ├── cmd/
│ │ └── userservice/
│ │ └── main.go ← Entry point for UserService
│ ├── internal/
│ │ ├── api/ ← HTTP API (handlers, router, middleware)
│ │ ├── domain/ ← Domain models (e.g., User, RegisterRequest)
│ │ ├── store/ ← Store interface and implementation (e.g., PostgresUserStore)
│ │ ├── grpc/ ← gRPC server implementation
│ │ └── genproto/ ← Generated gRPC code (e.g., userpb/)
│ ├── pkg/
│ │ └── auth/ ← Utilities for password hashing and JWT handling
│ ├── proto/ ← Source .proto files for gRPC
│ └── go.mod ← Go module definition

├── review-service/ ← 📝 ReviewService module
│ ├── cmd/
│ │ └── reviewservice/
│ │ └── main.go ← Entry point for ReviewService
│ ├── internal/
│ │ ├── api/ ← HTTP API (handlers, router)
│ │ ├── domain/ ← Domain models (e.g., Review, CreateReviewRequest)
│ │ ├── store/ ← Store interface and implementation (e.g., PostgresReviewStore)
│ │ ├── clients/ ← gRPC clients to other services (UserService, MovieService)
│ │ └── genproto/ ← Copied/generated gRPC code from other services (userpb/, moviepb/)
│ ├── proto/ ← (Optional) .proto files if exposing its own gRPC API
│ └── go.mod ← Go module definition

└── README.md ← Project documentation
```


---

Would you like a version with collapsible sections or emoji-free formatting as well?
