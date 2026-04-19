# FreeWhen

Group availability coordination tool. Go backend + Next.js frontend + Postgres.

## Principles

- **Mobile first** — all UI must be designed for small screens first, then enhanced for larger viewports. Use min-width breakpoints, test on 375px as the baseline, and never ship a component that doesn't work on a phone.

## Architecture

- `backend/` - Go API server using standard library `net/http`
- `frontend/` - Next.js App Router with TypeScript
- `docs/` - Project documentation (read these for context)
  - `docs/api.md` - API endpoint reference
  - `docs/infra-notes.md` - Hosting, deploy, and first-time setup

## Runtime & Tooling

- **Node 24 LTS** via `.nvmrc` — engines pinned to `>=24 <25` in `package.json`
- **pnpm** via Corepack — version pinned in `packageManager` field
- **Bun** is an optional speedup only (e.g. `bunx vitest`), not a runtime or package manager dependency
- Backend uses structured logging with `log/slog` (JSON in prod, text in dev)
- Every response includes an `X-Request-ID` header

## Backend Conventions

- Standard library where possible, minimal dependencies
- Handlers in `internal/handler/`, one file per resource
- Models in `internal/model/`
- Database queries in `internal/store/`
- Server setup and routing in `internal/server/`
- Config in `internal/config/` — env vars read once at startup (`DATABASE_URL`, `PORT`, `ENV`, `CORS_ORIGIN`)
- SQL migrations embedded in `internal/store/migrations/` — run on boot via `store.Migrate()`
- Health check: `GET /health` (200 ok / 503 unhealthy with DB ping)

## Frontend Conventions

- App Router (`src/app/`)
- Components in `src/components/<ComponentName>/<ComponentName>.tsx` (folder-per-component, no barrel files)
- API client and utilities in `src/lib/` (flat)
- Log wrapper in `src/lib/log.ts` — use `log.info`/`log.warn`/`log.error` instead of `console.*`
- Error boundary in `src/app/error.tsx`, not-found page in `src/app/not-found.tsx`
- Lint with oxlint, format with oxfmt

## Database

- Hosted Postgres (no local Docker)
- Connection string via `DATABASE_URL` env var

## Deployment

- **Frontend:** Vercel (auto-deploy from git)
- **Backend:** Google Cloud Run (deploy via `gcloud`, CI deploys on push to main)
- **Database:** Neon (managed Postgres)
- Migrations run on boot — no separate migration step needed
- See `docs/infra-notes.md` for first-time setup checklist

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
- Single test file: `cd frontend && pnpm exec vitest run path/to/file`
- Single Go test: `cd backend && go test ./internal/store/ -run TestName`
