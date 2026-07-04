# Workout Tracker API

[![CI](https://github.com/Aejkatappaja/workout_tracker/actions/workflows/ci.yml/badge.svg)](https://github.com/Aejkatappaja/workout_tracker/actions/workflows/ci.yml)

REST API in Go for tracking workouts and exercises. Bearer token auth, CRUD on workouts with exercise entries, PostgreSQL storage.

## Features

- **Token auth**: register, log in, get a bearer token (SHA-256 hashed at rest, scoped, expirable).
- **Owner-scoped CRUD**: users only touch their own workouts; cross-user access returns `403` (no IDOR).
- **Structured workouts**: each workout holds ordered exercise entries tracking either reps or duration (mutually exclusive, DB-enforced).
- **Embedded migrations**: Goose migrations run automatically on startup.
- **Interface-based stores**: handlers depend on store interfaces, making them unit-testable without a database.
- **Interactive docs**: OpenAPI 3.1 spec served with a Scalar UI at `/docs`.
- **Tested and linted**: unit + integration tests, `go vet`, `gofmt` and `golangci-lint` enforced in CI.

## Stack

- **Go 1.24** with [Chi](https://github.com/go-chi/chi) router
- **PostgreSQL** with [pgx](https://github.com/jackc/pgx) driver
- **[Goose](https://github.com/pressly/goose)** for migrations (embedded, applied on startup)
- **Docker Compose** for local dev (app DB + test DB)

## API

| Method | Route | Auth | Description |
|--------|-------|------|-------------|
| `POST` | `/users` | Public | Register |
| `POST` | `/tokens/authentication` | Public | Login (returns a bearer token) |
| `GET` | `/workouts/{id}` | Bearer | Get a workout with its entries |
| `POST` | `/workouts` | Bearer | Create a workout |
| `PUT` | `/workouts/{id}` | Bearer | Update a workout |
| `DELETE` | `/workouts/{id}` | Bearer | Delete a workout |
| `GET` | `/health` | Public | Health check |
| `GET` | `/docs` | Public | Interactive API docs (Scalar) |
| `GET` | `/openapi.yaml` | Public | OpenAPI 3.1 spec |

Workout routes are owner-scoped: a user can only read, update, or delete their own workouts (403 otherwise).

Interactive docs (Scalar, with a "try it" console) are served at **http://localhost:8080/docs**, backed by the OpenAPI spec at `/openapi.yaml`.

## Data model

Workouts contain exercise entries. Each entry tracks either **reps** or **duration**, mutually exclusive, enforced by a `CHECK` constraint.

- `users` -> `tokens` (bearer, SHA-256 hashed, scoped, expirable)
- `users` -> `workouts` -> `workout_entries` (sets, reps/duration, weight, order)

## Run

```bash
docker compose up -d   # app DB on :5432, test DB on :5433
go run .                # migrations run on startup, server listens on :8080
```

Configuration:

- `DATABASE_URL` overrides the connection string (defaults to the local Docker DB).
- `-port` flag sets the listen port (defaults to `8080`).

## Examples

### curl

```bash
# 1. register
curl -X POST localhost:8080/users \
  -d '{"username":"neo","email":"neo@matrix.io","password":"whiterabbit"}'

# 2. login -> { "auth_token": { "token": "<TOKEN>", "expiry": ... } }
#    capture the token straight into a shell variable
TOKEN=$(curl -s -X POST localhost:8080/tokens/authentication \
  -d '{"username":"neo","password":"whiterabbit"}' | jq -r .auth_token.token)

# 3. create a workout (owner is taken from the token, not the body)
curl -X POST localhost:8080/workouts -H "Authorization: Bearer $TOKEN" -d '{
  "title": "push day",
  "duration_minutes": 60,
  "entries": [
    {"exercise_name": "bench press", "sets": 3, "reps": 10, "order_index": 1},
    {"exercise_name": "plank", "sets": 3, "duration_seconds": 60, "order_index": 2}
  ]
}'

# 4. read it back
curl localhost:8080/workouts/1 -H "Authorization: Bearer $TOKEN"

# a different user's token gets 403, a missing id gets 404, no token gets 401
```

### Hurl file

[`api.hurl`](api.hurl) is the full request flow with assertions, runnable from the shell with
[Hurl](https://hurl.dev). It captures the bearer token and workout id between requests and asserts
every status, including the `403` IDOR case.

```bash
hurl --test api.hurl
```

### Smoke test (curl only)

No extra tooling: [`scripts/smoke.sh`](scripts/smoke.sh) drives the same flow with `curl` + `jq`
and asserts every status code. Random credentials make it safe to re-run without a DB reset.

```bash
./scripts/smoke.sh                 # defaults to http://localhost:8080
./scripts/smoke.sh http://host:port
```

## Test

```bash
docker compose up -d   # test DB must be running on :5433
go test ./...
```

## Project structure

```
internal/
├── api/          # HTTP handlers
├── app/          # App config, DI wiring
├── docs/         # OpenAPI spec + Scalar UI
├── middleware/   # Auth middleware (bearer token)
├── routes/       # Route definitions
├── store/        # PostgreSQL repositories (interface-based)
├── tokens/       # Token generation + hashing
└── utils/        # Request/response helpers
migrations/       # Goose SQL migrations
scripts/          # smoke.sh end-to-end check
api.hurl          # Hurl request flow with assertions
```
