  # Workout Tracker API

  REST API in Go for tracking workouts and exercises. Bearer token auth, CRUD on workouts with exercise entries, PostgreSQL storage.

  ## Stack

  - **Go 1.24** with [Chi](https://github.com/go-chi/chi) router
  - **PostgreSQL** with [pgx](https://github.com/jackc/pgx) driver
  - **[Goose](https://github.com/pressly/goose)** for migrations
  - **Docker Compose** for local dev (app DB + test DB)

  ## API

  | Method | Route | Auth | Description |
  |--------|-------|------|-------------|
  | `POST` | `/users` | — | Register |
  | `POST` | `/tokens/authentication` | — | Login (get bearer token) |
  | `GET` | `/workouts/{id}` | Bearer | Get workout with entries |
  | `POST` | `/workouts` | Bearer | Create workout |
  | `PUT` | `/workouts/{id}` | Bearer | Update workout |
  | `DELETE` | `/workouts/{id}` | Bearer | Delete workout |
  | `GET` | `/health` | — | Health check |

  ## Data model

  Workouts contain exercise entries. Each entry tracks either **reps** or **duration** (mutually exclusive, enforced by a `CHECK` constraint).

  - `users` → `tokens` (bearer, SHA-256 hashed, scoped, expirable)
  - `users` → `workouts` → `workout_entries` (sets, reps/duration, weight, order)

  ## Run

  ```bash
  docker compose up -d
  go run migrations/fs.go  # apply migrations
  go run main.go           # starts on :8080

  Test

  docker compose up -d     # test_db runs on :5433
  go test ./...

  Project structure

  internal/
  ├── api/          # HTTP handlers
  ├── app/          # App config, DI wiring
  ├── middleware/    # Auth middleware (bearer token)
  ├── routes/       # Route definitions
  ├── store/        # PostgreSQL repositories (interface-based)
  ├── tokens/       # Token generation + hashing
  └── utils/        # Request/response helpers
  migrations/       # Goose SQL migrations
