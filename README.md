# Task Management API

A robust REST API for managing tasks, built with Go and the Gin framework. It features JWT authentication, structured logging, idempotency for task creation, database transaction integrity, and more.

## Objective

This API was built to demonstrate REST API development, authentication & authorization, database design, query handling, clean architecture, and best practices in back end development.

## Tech Stack

- **Language:** Go (1.20+)
- **Framework:** Gin (github.com/gin-gonic/gin)
- **Database:** PostgreSQL
- **ORM:** GORM (gorm.io/gorm) + gorm.io/driver/postgres
- **DB Driver:** github.com/lib/pq
- **Authentication:** JWT (github.com/golang-jwt/jwt/v5)
- **Password Hashing:** bcrypt (golang.org/x/crypto/bcrypt)

## Architecture

The project follows a Domain-Driven Clean Architecture approach, organized into layers:
- **`cmd/api`**: Application entry point. Sets up the server, database, and router.
- **`cmd/migrate`**: Standalone migration CLI tool with `up`, `down`, `status`, `reset` commands.
- **`internal/domain`**: Core domain models and repository interfaces (`User`, `Task`, etc.).
- **`internal/handler`**: HTTP delivery layer. Parses requests, interacts with repositories, and formats responses.
- **`internal/middleware`**: Middleware for Auth (JWT validation), Logging, Global Error Handling, and Idempotency.
- **`internal/repository`**: Data persistence layer. Handles interactions with the PostgreSQL database via GORM.
- **`internal/database/migration`**: Versioned SQL migration engine with tracking, rollback, and dirty-flag support.
- **`pkg`**: Shared utilities (logging, hashing, custom errors, response formatting).

## Features Implemented

1. **Authentication:** Register, Login, JWT authentication.
2. **Task CRUD:** Create, List (with pagination and filters), Detail, Update, Delete.
3. **Idempotency:** `POST /tasks` supports an `Idempotency-Key` header. Duplicate requests within 24h will return the original response without creating a duplicate task. This is protected against race conditions using a concurrency lock.
4. **Structured Error Handling:** Uses a consistent JSON format with `status`, `code`, `message`, and `timestamp`. Differentiates 4xx and 5xx errors. Global recovery middleware prevents panics.
5. **Database Transaction:** `POST /tasks/:id/assign` assigns a task to a user. It updates the assignee, creates a task log, and mocks a notification within a single database transaction. If any step fails, it rolls back.
6. **Logging & Observability:** Custom logging middleware using `log/slog` outputs structured JSON logs including `request_id`, `method`, `path`, `status_code`, and `latency`.
7. **Race Condition Testing:** Unit tests exist in `internal/middleware/idempotency_test.go` to prove no race conditions occur during concurrent duplicate requests.

## How to Run

### Prerequisites
- Go 1.20+
- PostgreSQL installed and running

### 1. Create the PostgreSQL database
```bash
createdb task_api
```

Or via `psql`:
```sql
CREATE DATABASE task_api;
```

### 2. Set environment variables (optional)
```bash
export DB_DSN="host=localhost user=postgres password=postgres dbname=task_api port=5432 sslmode=disable"
export JWT_SECRET="your-secret-key"
export PORT="8080"
```

### 3. Download dependencies
```bash
go mod tidy
```

### 4. Run database migrations
```bash
# Apply all migrations
go run cmd/migrate/main.go up

# Check migration status
go run cmd/migrate/main.go status

# Rollback last migration
go run cmd/migrate/main.go down

# Rollback ALL migrations (with confirmation)
go run cmd/migrate/main.go reset
```

### 5. Run the API server
```bash
go run cmd/api/main.go
```
> Migrations also run automatically on server start.

## Run Unit Tests

To run the unit tests (focusing on Idempotency and Race Conditions):

```bash
go test -v ./internal/middleware
```

## API Endpoints

### Auth
- **`POST /api/register`** - Register a new user
  - Body: `{"username": "user1", "password": "password"}`
- **`POST /api/login`** - Login to get JWT
  - Body: `{"username": "user1", "password": "password"}`
  - Returns: `{"data": {"token": "eyJhbG..."}}`

*(All endpoints below require header: `Authorization: Bearer <token>`)*

### Tasks
- **`POST /api/tasks`** - Create a task
  - Headers: `Idempotency-Key: <UUID>` (Optional, to ensure idempotency)
  - Body: `{"title": "Task 1", "description": "Do something"}`
- **`GET /api/tasks`** - List tasks
  - Query Params: `?limit=10&page=1&status=pending&title=Task`
- **`GET /api/tasks/:id`** - Get task details
- **`PUT /api/tasks/:id`** - Update a task
  - Body: `{"status": "completed", "title": "Updated Task"}`
- **`DELETE /api/tasks/:id`** - Delete a task
- **`POST /api/tasks/:id/assign`** - Assign task to another user (Runs in DB Transaction)
  - Body: `{"assignee_id": "<user-uuid>"}`

## Structured Error Example
```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "Resource not found",
  "timestamp": "2023-10-25T12:00:00Z"
}
```
