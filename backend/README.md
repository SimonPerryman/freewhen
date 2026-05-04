# FreeWhen backend

Go API server for FreeWhen. See the root `README.md` for the full stack overview and `AGENTS.md` for project conventions.

## Run

```bash
make dev-backend         # from repo root, :8080
make dev-backend-watch   # auto-restart on .go/.sql changes (requires air)
```

Requires `DATABASE_URL` in `.env` at the repo root.

## Layout

- `cmd/` — entry point
- `internal/handler/` — HTTP handlers, one file per resource
- `internal/model/` — domain types
- `internal/store/` — database queries
- `internal/store/migrations/` — embedded SQL migrations, run on boot via `store.Migrate()`
- `internal/server/` — server setup and routing
- `internal/config/` — env vars read once at startup

## Conventions

- Standard library where possible, minimal dependencies
- Structured logging with `log/slog` — JSON to stdout in every environment
- **Never log PII** (email, phone, IP). `make lint-pii` enforces this.
- Every response carries an `X-Request-ID` header
- Config via env vars: `DATABASE_URL`, `PORT`, `CORS_ORIGIN`, `LOG_LEVEL`, `SECURE_COOKIES`. Branch on explicit vars, never on `NODE_ENV` or generic env labels (12-factor).
- Health check: `GET /health` (200 ok / 503 with DB ping)

## Tests

```bash
make test-backend                                   # unit
make test-integration                               # against embedded Postgres
cd backend && go test ./internal/store/ -run Name   # single test
```

Integration tests download an embedded Postgres to `~/.embedded-postgres-go/` on first run. Override with `TEST_DATABASE_URL=...` to use an existing database.

## Deployment

Built into a container (`Dockerfile`) and deployed to Cloud Run via GitHub Actions on push to `main`. Service config (env vars, secrets, scaling, IAM) is managed by Terraform in `infra/` — do not edit in the GCP console.
