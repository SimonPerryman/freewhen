# Tech Stack

## Architecture

Go backend + Next.js frontend + Postgres. Two deployable pieces with clean separation.

## Backend — Go

API server handling all business logic and data access.

- Standard library `net/http`, minimal dependencies
- Single binary, cheap to run, easy to deploy
- Structured logging with `log/slog` (JSON in prod, text in dev)
- Every response includes an `X-Request-ID` header

## Frontend — Next.js (React)

App Router with TypeScript. Mobile-first calendar UI.

- Node 24 LTS via `.nvmrc`, pnpm via Corepack
- Bun is an optional speedup only (e.g. `bunx vitest`), not a runtime dependency
- Lint with oxlint, format with oxfmt

## Database — Postgres (Neon)

Managed Postgres on Neon (free tier). Connection via `DATABASE_URL` env var. Migrations are embedded in the Go binary and run on boot.

## Hosting

| Layer | Provider | Notes |
|-------|----------|-------|
| Frontend | Vercel | Auto-deploy from git, free hobby tier |
| Backend | Google Cloud Run | Scale-to-zero, deploy via CI on push to main |
| Database | Neon | Managed Postgres, free tier, pooled connections |

## Auth

| Method | Provider | Status |
|--------|----------|--------|
| Google OAuth | Google Cloud OAuth | Implemented |
| Email magic link | Resend | Implemented |

- Creator must sign in to create a calendar. Joiners stay anonymous.
- Server-side sessions with HTTP-only cookies stored in Postgres (no JWTs).
- Backend served on `api.` subdomain for same-site cookies.

## Product Analytics — PostHog

Product events, session replay, and error capture via one SDK. EU cloud region. Free tier (1M events/mo) is comfortable at current scale.

- Gated on `NEXT_PUBLIC_POSTHOG_KEY` — unset means the SDK no-ops (12-factor, same code path everywhere).
- Centralised init in `frontend/src/lib/analytics.ts`.
- See `plans/observability-and-analytics.md`.

## Transactional Email — Resend

Used for magic link authentication emails. 3,000 emails/month free tier.

- Simple HTTP API, no SDK needed
- Domain verification via DNS records

## API Style

RESTful JSON API. Snake_case wire format, frontend client converts to/from camelCase. See `docs/api.md` for endpoint reference.

## CI

GitHub Actions. Backend: `golangci-lint` + `go test`. Frontend: `oxlint` + `vitest`. Deploy to Cloud Run on push to main.
