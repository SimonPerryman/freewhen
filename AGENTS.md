# FreeWhen

Group availability coordination tool. Go backend + Next.js frontend + Postgres.

## Principles

- **Mobile first** — all UI must be designed for small screens first, then enhanced for larger viewports. Use min-width breakpoints, test on 375px as the baseline, and never ship a component that doesn't work on a phone.
- **Config over environment detection** ([12-factor](https://12factor.net/config)) — branch on explicit env vars (e.g. `POSTHOG_KEY` set/unset, `LOG_LEVEL=info`, `SECURE_COOKIES=false`), never on `NODE_ENV` or a generic `ENV=prod`/`ENV=dev` label. Dev/prod parity means the same code path runs everywhere; only config changes.

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
- Backend uses structured logging with `log/slog` (JSON to stdout everywhere; pipe through `jq` locally if you want pretty output)
- Every response includes an `X-Request-ID` header

## Logging

- **Never log PII** — no email addresses, phone numbers, IP addresses, or other personal identifiers in log fields. Logs go to stdout in all environments and end up in shared log sinks. The `error` field alone is almost always enough to diagnose.
- Enforced by `make lint-pii` (runs as part of `make lint`). If you genuinely need to surface a PII-shaped value for debugging, do it behind a debug-only code path, not in a structured log field named `email`/`phone`/`ip_address`/`client_ip`.

## Backend Conventions

- Standard library where possible, minimal dependencies
- Handlers in `internal/handler/`, one file per resource
- Models in `internal/model/`
- Database queries in `internal/store/`
- Server setup and routing in `internal/server/`
- Config in `internal/config/` — env vars read once at startup (`DATABASE_URL`, `PORT`, `CORS_ORIGIN`, `LOG_LEVEL`, `SECURE_COOKIES`)
- SQL migrations embedded in `internal/store/migrations/` — run on boot via `store.Migrate()`
- Health check: `GET /health` (200 ok / 503 unhealthy with DB ping)

## Frontend Conventions

- App Router (`src/app/`)
- Components in `src/components/<ComponentName>/<ComponentName>.tsx` (folder-per-component, no barrel files)
- API client and utilities in `src/lib/` (flat)
- Log wrapper in `src/lib/log.ts` — use `log.info`/`log.warn`/`log.error` instead of `console.*`
- Error boundary in `src/app/error.tsx`, not-found page in `src/app/not-found.tsx`
- Lint with oxlint, format with oxfmt
- Feedback for reversible actions: use the Toast primitive (`useToast()` from `@/lib/toast`, `<ToastProvider>` mounted in `src/app/layout.tsx`) with an `Undo` action. Don't reach for `window.confirm` on mobile — it breaks the PWA aesthetic and is the wrong shape for an undoable operation.

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
- `make test-integration` to run Go integration tests (runs against an embedded Postgres downloaded to `~/.embedded-postgres-go/` on first use — no local setup required; override with `TEST_DATABASE_URL=...` to point at an existing database)
- Frontend uses vitest with jsdom; `@` alias maps to `src/`
- Single test file: `cd frontend && pnpm exec vitest run path/to/file`
- Single Go test: `cd backend && go test ./internal/store/ -run TestName`

## Before committing

Always run `make lint` and `make test` before committing, and make sure both pass. For changes that touch backend storage, handlers, or server wiring, also run `make test-integration`. Fix failures rather than committing around them.
