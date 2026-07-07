<p align="center">
  <img src="assets/go-gym.svg" alt="Cartoon Go gophers lifting weights at the GO-GYM" width="460"/>
</p>

# GO-GYM

[![CI](https://github.com/Aejkatappaja/go-gym/actions/workflows/ci.yml/badge.svg)](https://github.com/Aejkatappaja/go-gym/actions/workflows/ci.yml)

**Live demo:** [go-gym.aejkatappaja.com](https://go-gym.aejkatappaja.com). Hit "explore the demo" for a read-only seeded account.

A training log built twice over one Go backend: a documented **JSON REST API** (bearer tokens) and a **server-rendered web UI** (templ + HTMX, cookie sessions). One set of stores, two consumers.

## Features

- **Two front doors, one backend**: the same PostgreSQL stores serve a JSON API for programmatic clients and an HTMX web UI for the browser.
- **Dual-transport auth**: one opaque token (SHA-256 hashed at rest, scoped, expirable) carried either as a `Bearer` header (API) or an `HttpOnly` session cookie (web).
- **Password reset & welcome email**: a forgot/reset flow with a 1-hour, single-use, hashed token, plus a welcome email on signup, delivered through a swappable mailer (Resend in production, logged to the console in dev).
- **Owner-scoped by construction**: workouts are user-scoped down to the SQL (`WHERE id AND user_id`); cross-user access returns `403`/`404`, never leaks.
- **Structured workouts**: each workout holds ordered exercises tracking either reps or duration, exactly one, enforced by a DB `CHECK` and surfaced as inline validation in the UI.
- **Embedded migrations**: Goose migrations run automatically on startup.
- **Interface-based stores**: handlers depend on store interfaces, so they unit-test without a database.
- **Interactive docs**: OpenAPI 3.1 spec served with a Scalar UI at `/docs`.
- **Tested and linted**: unit, integration and end-to-end tests; `go vet`, `gofmt`, `golangci-lint` and `templ` drift checks enforced in CI.

## Architecture

```text
        API client ──Authorization: Bearer──┐
        Browser ──────session cookie─────────┤
                                             ▼
                              chi router + middleware
                RealIP · SecurityHeaders · BodyLimit(1 MiB) · rate-limit
                                             │
                          Authenticate (header, else cookie fallback)
                                             │
                     ┌───────────────────────┴───────────────────────┐
                     ▼                                                 ▼
             internal/api (JSON)                         internal/web (templ + HTMX)
                     └───────────────────────┬───────────────────────┘
                                             ▼
                    stores (interfaces): User · Token · Workout
                                             │
                           PostgreSQL (pgx) · Goose migrations

    mail.Mailer (Resend │ log)  ◀── welcome + password-reset emails
```

Both surfaces share one middleware chain, one `Authenticate` step, and one set of
store interfaces; only the rendering (JSON vs HTML) and the auth-failure behaviour
(`401` vs redirect to `/login`) differ.

## Stack

- **Go 1.25** with [Chi](https://github.com/go-chi/chi) router
- **PostgreSQL** via [pgx](https://github.com/jackc/pgx), migrations by [Goose](https://github.com/pressly/goose)
- **Web UI**: [templ](https://templ.guide) typed components + [HTMX](https://htmx.org), hand-written CSS, no build step (assets embedded)
- **Transactional email** via [Resend](https://resend.com) behind a swappable `Mailer` interface (logs to the console when unconfigured)
- **Docker Compose** for local dev (app DB + test DB)

## JSON API

| Method | Route | Auth | Description |
|--------|-------|------|-------------|
| `POST` | `/users` | Public | Register |
| `POST` | `/tokens/authentication` | Public | Login (returns a bearer token) |
| `GET` | `/workouts` | Bearer | List the caller's workouts |
| `GET` | `/workouts/{id}` | Bearer | Get a workout with its entries |
| `POST` | `/workouts` | Bearer | Create a workout |
| `PUT` | `/workouts/{id}` | Bearer | Update a workout |
| `DELETE` | `/workouts/{id}` | Bearer | Delete a workout |
| `GET` | `/health` | Public | Health check |
| `GET` | `/docs` | Public | Interactive API docs (Scalar) |
| `GET` | `/openapi.yaml` | Public | OpenAPI 3.1 spec |

Interactive docs with a "try it" console live at **http://localhost:8080/docs**.

## Web UI

Server-rendered pages (cookie session) under `/`:

- `/login`, `/register`, and logout wired to the same auth as the API.
- `/forgot` and `/reset`: request a reset link and set a new password.
- `/app` dashboard listing your workouts.
- `/app/workouts/new` and `/app/workouts/{id}/edit`: forms with add/remove exercise rows (HTMX), reps and duration locked mutually exclusive.
- `/app/workouts/{id}`: detail with the exercise table, edit and delete.

## Data model

- `users` -> `tokens` (bearer, SHA-256 hashed, scoped, expirable)
- `users` -> `workouts` -> `workout_entries` (sets, reps or duration, weight, order)

Each entry tracks **either reps or duration**, never both and never neither, enforced by a `CHECK` constraint.

## Security

- **Auth**: opaque bearer tokens (32 bytes from `crypto/rand`), stored only as a SHA-256 hash, scoped and expiring; passwords hashed with bcrypt (cost 12) and never serialized. The browser carries the same token in an `HttpOnly`, `SameSite=Lax`, `Secure` (over HTTPS) cookie.
- **Authorization**: workouts are owner-scoped down to the SQL (`WHERE id AND user_id`), so cross-user access is impossible by construction, not just by a handler check.
- **Input**: every query is parameterized; request bodies are capped at 1 MiB; exercise counts are bounded.
- **Headers**: `Content-Security-Policy` (no inline scripts), `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy`, and HSTS over HTTPS.
- **Abuse**: credential endpoints are rate-limited per IP; an unknown username on login still runs a bcrypt compare, so response time does not leak whether an account exists.
- **Password reset**: reset tokens are hashed at rest, single-use, and expire in 1 hour; a successful reset revokes every session for that user. `/forgot` returns the same response whether or not the email is registered, so it never enumerates accounts.
- **CSRF**: state-changing requests rely on `SameSite=Lax` cookies; the JSON API authenticates with a bearer header, which a browser cannot send cross-site.

In production, point `DATABASE_URL` at a connection string with `sslmode=require` (or `verify-full`) so database traffic is encrypted. The local Docker default uses `sslmode=disable`.

## Run

```bash
docker compose up -d   # app DB on :5432, test DB on :5433
go run .               # migrations run on startup; API + web UI on :8080
```

Then open **http://localhost:8080/register** for the UI, or hit the JSON API directly.

Configuration:

- `DATABASE_URL` overrides the connection string (defaults to the local Docker DB); use `sslmode=require` in production.
- `-port` / `PORT` sets the listen port (defaults to `8080`).
- `RESEND_API_KEY` + `MAIL_FROM` (e.g. `go-gym <noreply@example.com>`) enable real email via Resend; unset, the app logs emails to the console instead.
- `LOG_FORMAT=json` emits structured JSON logs (set it in production for log aggregation); anything else uses human-readable text. `LOG_LEVEL` (`debug`/`info`/`warn`/`error`, default `info`) sets the minimum level. Each request is logged once with its method, path, status, size, latency and a `req_id` that also tags any error logged while handling it.

Editing `.templ` views requires regenerating the Go (the generated files are committed):

```bash
go tool templ generate
```

## Deploy

The app compiles to a single static binary with assets, migrations and templates embedded, so the container image is tiny and self-contained. Migrations run and the read-only demo seeds on startup, so a fresh database is usable immediately.

```bash
docker build -t go-gym .
docker run -p 8080:8080 -e DATABASE_URL="postgres://user:pass@host:5432/db?sslmode=require" go-gym
```

The same Dockerfile runs on any container PaaS (Northflank, Render, Railway, Fly, Koyeb) paired with any managed Postgres (the platform's addon, Neon, Supabase). `-port` / `PORT` and `DATABASE_URL` are the only knobs; migrations and the demo seed on first boot. Graceful shutdown handles `SIGTERM`, and `/health` reports readiness by pinging the database. A [`fly.toml`](fly.toml) is included as a ready example for [Fly.io](https://fly.io).

## Examples

### curl (JSON API)

```bash
# register
curl -X POST localhost:8080/users \
  -d '{"username":"neo","email":"neo@matrix.io","password":"whiterabbit"}'

# login -> capture the token
TOKEN=$(curl -s -X POST localhost:8080/tokens/authentication \
  -d '{"username":"neo","password":"whiterabbit"}' | jq -r .auth_token.token)

# create a workout (owner comes from the token, not the body)
curl -X POST localhost:8080/workouts -H "Authorization: Bearer $TOKEN" -d '{
  "title": "push day",
  "duration_minutes": 60,
  "entries": [
    {"exercise_name": "bench press", "sets": 3, "reps": 10, "order_index": 1},
    {"exercise_name": "plank", "sets": 3, "duration_seconds": 60, "order_index": 2}
  ]
}'

# a different user's token gets 403, a missing id 404, no token 401
```

### End-to-end flows (Hurl)

Runnable from the shell with [Hurl](https://hurl.dev):

- [`api.hurl`](api.hurl) drives the JSON API (bearer token + workout id captured between requests, every status asserted, including the `403` IDOR case).
- [`web.hurl`](web.hurl) drives the browser UI end to end (cookie session + HTMX): anonymous redirect, login, dashboard, create/detail/delete, inline validation, logout.

```bash
hurl --test api.hurl
hurl --test web.hurl
```

[`scripts/smoke.sh`](scripts/smoke.sh) covers the API flow with just `curl` + `jq` (no extra tooling), using random credentials so it re-runs without a DB reset.

## Test

```bash
docker compose up -d   # test DB must be running on :5433
go test ./...
```

## Project structure

```
internal/
├── api/          # JSON handlers
├── app/          # config, DI wiring
├── docs/         # OpenAPI spec + Scalar UI
├── middleware/   # auth (bearer header or session cookie)
├── routes/       # route definitions
├── store/        # PostgreSQL repositories (interface-based)
├── tokens/       # token generation + hashing
├── utils/        # request/response helpers
└── web/          # server-rendered HTMX UI (templ views, static assets)
migrations/       # Goose SQL migrations
scripts/          # smoke.sh end-to-end check
api.hurl          # Hurl e2e for the JSON API
web.hurl          # Hurl e2e for the browser UI (cookie + HTMX)
```

## Credits

The gopher on the login and register pages is based on the Go Gopher by [Renée French](https://reneefrench.blogspot.com/), licensed under [CC-BY-3.0](https://creativecommons.org/licenses/by/3.0/).
