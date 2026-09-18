# Task Management API

A robust REST API for managing tasks, built with Go and the Gin framework. It features JWT authentication, structured logging, idempotency for task creation, database transaction integrity, and more.

## Objective

This API was built to demonstrate REST API development, authentication & authorization, database design, query handling, clean architecture, and best practices in back end development.

## Tech Stack

- **Language:** Go (1.26)
- **Framework:** Gin (github.com/gin-gonic/gin)
- **Database:** PostgreSQL
- **ORM:** GORM (gorm.io/gorm) + gorm.io/driver/postgres
- **DB Driver:** github.com/lib/pq
- **Authentication:** JWT (github.com/golang-jwt/jwt/v5)
- **Password Hashing:** bcrypt (golang.org/x/crypto/bcrypt)

## Architecture

The project follows a Clean Architecture + DDD approach. Source dependencies point inward: `handler → usecase → domain`. Infrastructure details (GORM, JWT, Gin) are adapters that implement ports owned by the inner layers.

```text
cmd/api/main.go            Composition root: config, migrations, dependency wiring, router
internal/
├── domain/                Core business layer (pure Go — no framework/DB imports)
│   ├── project/           Project bounded context: Project aggregate + Member child entity,
│   │                      domain errors, Repository port
│   ├── task/              Task bounded context: Task aggregate + TaskLog child entity,
│   │                      TaskStatus value object, domain errors, Repository port
│   └── user/              User bounded context: User entity, Username value object,
│                          domain errors, Repository port
├── usecase/               Application layer — one service per bounded context
│   ├── project/           Project orchestration: membership management (owner-only)
│   ├── task/              Task orchestration: ownership, partial updates, assignment
│   └── user/              Auth orchestration; owns the TokenIssuer port (JWT adapter injected)
├── handler/               HTTP delivery: DTOs and request parsing. Defines the driving
│                          ports (TaskUsecase, ProjectUsecase, AuthUsecase) at the consumer side
├── middleware/            Auth (JWT), Logger, global ErrorHandler, Idempotency
│                          (with its own narrow IdempotencyStore port)
├── repository/            Persistence adapters (GORM). Separate persistence models;
│                          translates GORM errors into domain errors
└── database/migration/    Versioned SQL migration engine (up/down/status/reset)
pkg/                       Shared kernel: logger, response, custom errors, utils
```

Key rules applied:

- **Dependency Inversion:** each interface lives where it is consumed/owned — `task.Repository`, `project.Repository`, and `user.Repository` in the domain, `TokenIssuer` in the application layer, `IdempotencyStore` in the middleware, and the usecase ports in the handler. Cross-context needs use narrow consumer-side ports (`UserChecker`, `MembershipChecker`). `cmd/api` wires the concrete adapters into them.
- **Business logic lives in the aggregate:** invariants such as "every task lives in a project" and "every reassignment produces an audit log" are enforced by the domain, not by handlers or SQL.
- **Error translation:** infrastructure errors (`gorm.ErrRecordNotFound`, `gorm.ErrDuplicatedKey`) never leave the repository — they become domain errors and are mapped to HTTP status codes in the application layer.

## Features Implemented

1. **Authentication:** Register, Login, JWT authentication.
2. **Projects:** Create projects, list your projects, project detail, and owner-only membership management (add/remove/list members). The owner is automatically the first member and cannot be removed.
3. **Task CRUD:** Create (inside a project), List (with pagination and filters), Detail, Update, Delete.
4. **Idempotency:** `POST /tasks` accepts an optional `Idempotency-Key` header (must be a valid UUID, otherwise `400`). Keys are scoped per user. Replaying a key within a 24h window returns the stored response with the original status code without executing the handler again. Concurrent duplicates are serialized by a per-key mutex, so exactly one task is ever created.
5. **Structured Error Handling:** Uses a consistent JSON format with `status`, `code`, `message`, and `timestamp`. Differentiates 4xx and 5xx errors. Global recovery middleware prevents panics.
6. **Database Transaction:** `POST /tasks/:id/assign` assigns a task to another member of the same project. Any project member may assign. It updates the assignee, creates a task log, and mocks a notification within a single database transaction. If any step fails, it rolls back.
7. **Logging & Observability:** Custom logging middleware using `log/slog` outputs structured JSON logs including `request_id`, `method`, `path`, `status_code`, and `latency`.
8. **Race Condition Testing:** Unit tests in `internal/middleware/idempotency_test.go` prove it: sequential replay returns identical responses without re-executing the handler, and 50 concurrent goroutines sending the same key execute the handler exactly once (run with `go test -race`).

## How to Run

### Prerequisites
- Go 1.26+
- PostgreSQL installed and running

### 1. Create the PostgreSQL database
```bash
createdb task_api
```

Or via `psql`:
```sql
CREATE DATABASE task_api;
```

### 2. Configure environment variables

Copy the example file and adjust the values:
```bash
cp .env.example .env
```

| Variable | Default | Description |
|---|---|---|
| `APP_PORT` | `8080` | HTTP server port |
| `DB_HOST` / `DB_PORT` | `localhost` / `5432` | PostgreSQL address |
| `DB_USER` / `DB_PASSWORD` | — | Database credentials |
| `DB_NAME` | — | Database name |
| `DB_SSLMODE` | `disable` | PostgreSQL SSL mode |
| `DB_TIMEZONE` | `Asia/Jakarta` | Database timezone |
| `JWT_SECRET` | `supersecretkey` | JWT signing secret — use a strong random value in production |
| `DB_DSN` | — | Optional full DSN; overrides all `DB_*` variables above |

If `.env` is absent, the values are read from real environment variables.

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

Unit tests run entirely without a database or external services — dependencies are replaced with in-memory fakes/stubs. The idempotency suite proves the absence of race conditions:

```bash
# Full suite with the race detector
go test -race ./...

# Idempotency race-condition tests only
# (sequential replay + 50 concurrent duplicates -> handler runs exactly once)
go test -race -run TestIdempotency -v ./internal/middleware/
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
- **`POST /api/tasks`** - Create a task in a project (you must be a member of the project, otherwise `403`)
  - Headers: `Idempotency-Key: <UUID>` (optional; must be a valid UUID, otherwise `400`. Replaying the same key within 24h returns the original response without creating a new task)
  - Body: `{"title": "Task 1", "description": "Do something", "project_id": "<project-uuid>"}`
- **`GET /api/tasks`** - List tasks
  - Query Params: `?limit=10&page=1&status=pending&title=Task`
- **`GET /api/tasks/:id`** - Get task details
- **`PUT /api/tasks/:id`** - Update a task (partial: empty fields are left unchanged)
  - Body: `{"status": "completed", "title": "Updated Task"}` — `status` accepts `pending`, `in-progress`, or `completed`
- **`DELETE /api/tasks/:id`** - Delete a task
- **`POST /api/tasks/:id/assign`** - Assign task to another member of the same project (Runs in DB Transaction). Any project member may assign.
  - Body: `{"assignee_id": "<user-uuid>"}`
  - Errors: target user does not exist → `400`; target is not a project member → `400`; caller is not a project member → `403`

### Projects
- **`POST /api/projects`** - Create a project (you become the owner and first member)
  - Body: `{"name": "Alpha Team"}`
- **`GET /api/projects`** - List projects you are a member of
- **`GET /api/projects/:id`** - Get project details (members only)
- **`POST /api/projects/:id/members`** - Add a member (owner only)
  - Body: `{"user_id": "<user-uuid>"}` — target must be an existing user, otherwise `400`; already a member → `409`
- **`GET /api/projects/:id/members`** - List project members (members only)
- **`DELETE /api/projects/:id/members/:userId`** - Remove a member (owner only; the owner cannot be removed)

## Structured Error Example
```json
{
  "status": "error",
  "code": "NOT_FOUND",
  "message": "Resource not found",
  "timestamp": "2023-10-25T12:00:00Z"
}
```
