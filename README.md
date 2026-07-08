<p align="center">
  <img src="assets/go-gym.svg" alt="Cartoon Go gophers lifting weights at the GO-GYM" width="460"/>
</p>

# GO-GYM

[![CI](https://github.com/Aejkatappaja/go-gym/actions/workflows/ci.yml/badge.svg)](https://github.com/Aejkatappaja/go-gym/actions/workflows/ci.yml)

**Live demo:** [go-gym.aejkatappaja.com](https://go-gym.aejkatappaja.com). Hit "explore the demo" for a read-only seeded account.

A training log built twice over one Go backend: a documented **JSON REST API** (bearer tokens) and a **server-rendered web UI** (templ + HTMX, cookie sessions). One set of stores, two consumers.

<p align="center">
  <img src="assets/progress.png" alt="Progress page: personal-record cards and a weekly training-volume bar chart" width="720"/>
  <br/>
  <sub>Progress page: personal records and weekly volume, rendered as server-side SVG (no charting library).</sub>
</p>

## Features

- **Two front doors, one backend**: the same PostgreSQL stores serve a JSON API for programmatic clients and an HTMX web UI for the browser. One set of store interfaces, two consumers, no duplication.
- **Progress analytics, charted by hand**: personal-record cards, an estimated-1RM (Epley) progression line chart, and a weekly training-volume bar chart, all rendered as **server-side SVG with no JavaScript charting library**. Queries use window functions / `DISTINCT ON` for PRs and `date_trunc` for weekly rollups.
- **Exercise catalog with typeahead**: a normalized `exercises` table with prefix search over a Postgres **`pg_trgm` GIN index**, wired to an HTMX typeahead that dedupes case-insensitively and lets you create a new exercise with a muscle group inline.
- **Dual-transport auth**: one opaque token (SHA-256 hashed at rest, scoped, expirable) carried either as a `Bearer` header (API) or an `HttpOnly` session cookie (web).
- **Owner-scoped by construction**: workouts are user-scoped down to the SQL (`WHERE id AND user_id`); cross-user access returns `403`/`404`, never leaks.
- **Structured workouts + activity heatmap**: each workout holds ordered exercises tracking either reps or duration, exactly one, enforced by a DB `CHECK` and surfaced as inline validation; the dashboard renders a GitHub-style activity heatmap computed in a single grouped query.
- **Transactional + scheduled email**: welcome and password-reset emails, plus a **weekly training-recap** background job (sessions, volume, best lift). All delivered through a swappable mailer (Resend in production, logged to the console in dev). The reset flow uses a 1-hour, single-use, hashed token; the recap job is idempotent (a conditional per-user claim, at most one email per 7 days) and multi-replica-safe (a Postgres advisory lock serializes the scan).
- **Structured observability**: `log/slog` with JSON output in production, one access-log line per request, and a per-request id propagated through the context so every error is correlated to the request that caused it.
- **Interface-based stores**: handlers depend on store interfaces, so they unit-test without a database; embedded Goose migrations run automatically on startup.
- **Interactive docs**: OpenAPI 3.1 spec served with a Scalar UI at `/docs`.
- **Tested and linted**: unit, integration and end-to-end tests; `go vet`, `gofmt`, `golangci-lint` and `templ` drift checks enforced in CI.

## Architecture

```text
        API client ──Authorization: Bearer──┐
        Browser ──────session cookie─────────┤
                                             ▼
                              chi router + middleware
      RealIP · RequestID · request-log(slog) · metrics · Recoverer · SecurityHeaders · BodyLimit · rate-limit
                                             │
                          Authenticate (header, else cookie fallback)
                                             │
                     ┌───────────────────────┴───────────────────────┐
                     ▼                                                 ▼
             internal/api (JSON)                         internal/web (templ + HTMX)
                     └───────────────────────┬───────────────────────┘
                                             ▼
     stores (interfaces): User · Token · Workout · Exercise · Analytics · Recap
                                             │
                           PostgreSQL (pgx) · Goose migrations
                                             ▲
    weekly-recap scheduler ──────────────────┘   (background ticker, on when mail configured)
    mail.Mailer (Resend │ log)  ◀── welcome · password-reset · weekly-recap emails
```

Both surfaces share one middleware chain, one `Authenticate` step, and one set of
store interfaces; only the rendering (JSON vs HTML) and the auth-failure behaviour
(`401` vs redirect to `/login`) differ. A background scheduler reuses the same
stores and mailer to send weekly recaps.

## Stack

- **Go 1.26** with [Chi](https://github.com/go-chi/chi) router
- **PostgreSQL** via [pgx](https://github.com/jackc/pgx), migrations by [Goose](https://github.com/pressly/goose)
- **Web UI**: [templ](https://templ.guide) typed components + [HTMX](https://htmx.org), hand-written CSS and hand-rolled SVG charts, no build step (assets embedded)
- **Transactional email** via [Resend](https://resend.com) behind a swappable `Mailer` interface (logs to the console when unconfigured)
- **Observability**: `log/slog` structured logging with per-request id correlation
- **Docker Compose** for local dev (app DB + test DB)

## JSON API

| Method | Route | Auth | Description |
|--------|-------|------|-------------|
| `POST` | `/users` | Public | Register |
| `POST` | `/tokens/authentication` | Public | Login (returns a bearer token) |
| `GET` | `/workouts` | Bearer | List the caller's workouts (keyset paginated: `?limit=&cursor=`, `next_cursor` in the response) |
| `GET` | `/workouts/{id}` | Bearer | Get a workout with its entries |
| `POST` | `/workouts` | Bearer | Create a workout |
| `PUT` | `/workouts/{id}` | Bearer | Update a workout |
| `DELETE` | `/workouts/{id}` | Bearer | Delete a workout |
| `GET` | `/records` | Bearer | Personal records (best e1RM per exercise) |
| `GET` | `/exercises/{id}/progress` | Bearer | e1RM + volume progression for one exercise |
| `GET` | `/exercises` | Public | Prefix search the exercise catalog (typeahead) |
| `GET` | `/health` | Public | Health check |
| `GET` | `/metrics` | Public | Prometheus metrics (RED + Go runtime) |
| `GET` | `/docs` | Public | Interactive API docs (Scalar) |
| `GET` | `/openapi.yaml` | Public | OpenAPI 3.1 spec |

Interactive docs with a "try it" console live at **http://localhost:8080/docs**.

## Web UI

Server-rendered pages (cookie session) under `/`:

- `/login`, `/register`, and logout wired to the same auth as the API.
- `/forgot` and `/reset`: request a reset link and set a new password.
- `/app` dashboard listing your workouts (paginated with an HTMX "load more"), with an activity heatmap. Totals come from a SQL aggregate, not by loading every row.
- `/app/workouts/new` and `/app/workouts/{id}/edit`: forms with add/remove exercise rows (HTMX), an exercise typeahead, and reps/duration locked mutually exclusive.
- `/app/workouts/{id}`: detail with the exercise table, edit and delete.
- `/app/progress`: personal-record cards and a weekly training-volume bar chart.
- `/app/exercises/{id}`: an estimated-1RM progression line chart for one exercise.

## Data model

- `users` -> `tokens` (bearer, SHA-256 hashed, scoped, expirable)
- `users` -> `workouts` -> `workout_entries` (sets, reps or duration, weight, order)
- `workout_entries` -> `exercises` (normalized catalog: unique name, muscle group, `pg_trgm` index)
- `users.last_recap_sent_at` gates the weekly recap job (at most one email per 7 days)

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

## Observability

`log/slog` backs all logging: `LOG_FORMAT=json` emits structured JSON (for log aggregation), otherwise human-readable text, with the level set by `LOG_LEVEL`. A middleware assigns each request an id (`chi` RequestID) and logs one access line with method, path, status, response size and latency. Handlers log through a request-scoped logger pulled from the context, so every error carries the same `req_id` and can be traced back to the exact request:

```json
{"time":"...","level":"INFO","msg":"request","req_id":"…","method":"POST","path":"/app/workouts","status":200,"bytes":0,"duration_ms":21.3}
```

**Metrics**: `GET /metrics` exposes Prometheus RED metrics (`http_requests_total`, `http_request_duration_seconds`) plus the Go runtime and process collectors. Requests are labeled by the **chi route pattern** (`/app/workouts/{id}`), not the raw path, so ids don't blow up cardinality and unmatched paths (404s, scanners) collapse to `other`. The endpoint is public in this demo; in a real deployment you'd bind it to an internal network or put it behind auth. Point Prometheus (or Grafana Cloud) at it to graph request rate, error rate and p95 latency.

## Load test

Load testing the dashboard (`GET /app`, the heaviest read: an aggregate stats query, a page of workouts and the heatmap counts) with [`hey`](https://github.com/rakyll/hey) at concurrency 150 surfaced a real bottleneck. The connection pool used the `database/sql` default (unlimited open connections), so under load it opened one Postgres backend per in-flight query and hit the server's `max_connections` (100):

```
FATAL: sorry, too many clients already (SQLSTATE 53300)
```

Bounding the pool (`SetMaxOpenConns(25)` + idle/lifetime, in [`internal/store/database.go`](internal/store/database.go)) makes requests queue on the pool instead of exhausting the server:

| `GET /app`, c=150, 15s | Before (unbounded) | After (pool = 25) |
|---|---|---|
| Failed requests | **321** (`too many clients`) | **0** |
| Throughput | collapses (fail-fast + stalls) | **~9,900 req/s** |
| Worst latency | **13.5 s** | **76 ms** |
| p95 / p99 | — | **18 ms / 25 ms** |

Reproduce with [`scripts/loadtest.sh`](scripts/loadtest.sh) (needs `hey` and a running server).

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
- `APP_URL` (e.g. `https://go-gym.example.com`) is the public base URL used in email links. With `APP_URL` and the Resend variables all set, a background job emails each active user a weekly training recap (sessions, volume, best lift); it sends at most once per 7 days per user and skips the demo account.
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
├── middleware/   # auth (bearer header or session cookie), request logging
├── recap/        # weekly-recap email scheduler
├── routes/       # route definitions
├── store/        # PostgreSQL repositories (interface-based)
├── tokens/       # token generation + hashing
├── utils/        # request/response helpers
└── web/          # server-rendered HTMX UI (templ views, static assets, SVG charts)
migrations/       # Goose SQL migrations
scripts/          # smoke.sh end-to-end check
api.hurl          # Hurl e2e for the JSON API
web.hurl          # Hurl e2e for the browser UI (cookie + HTMX)
```

## Credits

The gopher on the login and register pages is based on the Go Gopher by [Renée French](https://reneefrench.blogspot.com/), licensed under [CC-BY-3.0](https://creativecommons.org/licenses/by/3.0/).
