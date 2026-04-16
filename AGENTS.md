# FreeWhen

Group availability coordination tool. Go backend + Next.js frontend + Postgres.

## Architecture

- `backend/` - Go API server using standard library `net/http`
- `frontend/` - Next.js App Router with TypeScript
- `docs/` - Project documentation (read these for context)

## Backend Conventions

- Standard library where possible, minimal dependencies
- Handlers in `internal/handler/`, one file per resource
- Models in `internal/model/`
- Database queries in `internal/store/`
- Server setup and routing in `internal/server/`
- SQL migrations in `migrations/`

## Frontend Conventions

- App Router (`src/app/`)
- Components in `src/components/`
- API client and utilities in `src/lib/`
- Lint with oxlint, format with oxfmt

## Database

- Hosted Postgres (no local Docker)
- Connection string via `DATABASE_URL` env var

## Development

- `make dev-backend` / `make dev-frontend` to run
- `make lint` to lint both stacks
- `make fmt` to format frontend
