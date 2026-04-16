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

## Plans

Every new piece of work must have a plan in `plans/`. Create the plan before starting implementation.

- One markdown file per piece of work (e.g. `plans/add-calendar-api.md`)
- Each plan must include a **Log** section with concise, timestamped entries recording what changed and when
- Log entries use the format `YYYY-MM-DD HH:MM — <what changed>`
- Update the log as work progresses — it should tell the story of what actually happened, not just what was planned
- Mark the plan status as `done` when complete

### Plan Template

```markdown
# <Title>

Status: pending | in-progress | done

## Goal

<What this work achieves and why>

## Plan

<Steps to implement>

## Log

- YYYY-MM-DD HH:MM — Created plan
```

## Development

- `make dev-backend` / `make dev-frontend` to run (backend :8080, frontend :3000)
- `make lint` to lint both stacks (golangci-lint + oxlint)
- `make fmt` to format frontend (oxfmt)
- `make test` to run all tests
- `make test-backend` / `make test-frontend` to run tests per stack
- `make test-integration` to run Go integration tests (requires `-tags=integration`)
- Frontend uses vitest with jsdom; `@` alias maps to `src/`
- Single test file: `cd frontend && npx vitest run path/to/file`
- Single Go test: `cd backend && go test ./internal/store/ -run TestName`
