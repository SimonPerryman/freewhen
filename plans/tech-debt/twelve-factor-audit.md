# 12-Factor Audit & Fixes

Status: done

## Goal

Audit the codebase against [12-factor](https://12factor.net), with particular attention to the AGENTS.md principle:

> **Config over environment detection** — branch on explicit env vars (e.g. `POSTHOG_KEY` set/unset, `ENV=production`), never on `NODE_ENV`. Dev/prod parity means the same code path runs everywhere; only config changes.

Fix the violations so the same code path runs in dev and prod, and the process handles termination signals cleanly on Cloud Run.

## Findings

What's already compliant (no action):

- **IV Backing services** — `DATABASE_URL` pluggable, read once at startup.
- **V Build/release/run** — multi-stage Dockerfile, CI builds, runtime injects env.
- **VI Processes** — backend is stateless; all persistence in Postgres.
- **VII Port binding** — backend and Next.js self-bind.
- **XI Logs** — slog to stdout, structured fields, `X-Request-ID` propagated.
- **XII Admin processes** — migrations embedded and run on boot.

Violations (verified in-tree):

| # | Factor | File:Line | Issue | Severity |
|---|--------|-----------|-------|----------|
| 1 | III / X | `frontend/src/lib/log.ts:1` | `process.env.NODE_ENV !== "production"` | blocker — directly contradicts the stated rule |
| 2 | IX | `backend/cmd/server/main.go:52` | `http.ListenAndServe` — no SIGTERM handling, no drain on Cloud Run | blocker |
| 3 | III / X | `backend/cmd/server/main.go:25-29` | Text logger in dev, JSON logger in prod — different code path | should-fix |
| 4 | III / X | `backend/internal/handler/auth.go:68` | `secureCookie: cfg.Env != "dev"` — cookie flags vary by env type | should-fix |
| 5 | III | `backend/internal/config/config.go:25-27` | Conditional `godotenv.Load` gated on `ENV != "prod"` | nit |

## Plan

### 1. Frontend: remove `NODE_ENV` branching (blocker)

`frontend/src/lib/log.ts` currently silences `info`/`warn` in any non-`production` Node env. Replace with an explicit config var so the same code path runs everywhere and only config varies.

- Introduce `NEXT_PUBLIC_LOG_LEVEL` (values: `debug` | `info` | `warn` | `error`, default `info`).
- Wrapper reads the var once at module load and gates each method on the level.
- No `NODE_ENV` reads anywhere in `frontend/src/`.
- Document in `frontend/.env.example` (create if missing) and `docs/infra-notes.md`:
  - Local default: unset → `info` (matches today's prod behaviour).
  - Vercel prod: set to `warn` if we want to suppress info logs; otherwise leave unset.

Grep check after: `grep -rn "NODE_ENV" frontend/src` returns nothing.

### 2. Backend: graceful shutdown (blocker)

Cloud Run sends `SIGTERM` then kills after a grace window. Today the process dies mid-request.

In `backend/cmd/server/main.go`:

- Build an `&http.Server{Addr, Handler, ReadHeaderTimeout}` instead of calling `http.ListenAndServe` directly (also fixes the implicit slowloris risk from the default server).
- Run `server.ListenAndServe()` in a goroutine; treat `http.ErrServerClosed` as clean exit.
- Main goroutine waits on `signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)`.
- On signal: call `server.Shutdown(ctx)` with a 25s timeout (Cloud Run default grace is 10s but configurable; keep headroom).
- Close the `*sql.DB` after shutdown returns, not via `defer` on main (ordering matters — finish draining first).
- Log `"shutting down"` on signal and `"shutdown complete"` after `Shutdown` returns.

No behaviour change in the happy path; just correct lifecycle.

### 3. Backend: unified logging format (should-fix)

Always emit JSON to stdout. Developers can pipe through a formatter locally (`| jq`, or a helper in the Makefile) — the binary behaviour is identical.

- Delete the `cfg.Env == "dev"` branch in `main.go`; always construct `slog.NewJSONHandler(os.Stdout, ...)`.
- Add a `LOG_LEVEL` env var (`debug` | `info` | `warn` | `error`, default `info`) wired into `slog.HandlerOptions.Level`. This gives us the knob we actually want without reintroducing env-type branching.
- Add `dev-backend-pretty` target in `Makefile` that pipes through `jq` for anyone who wants text-ish output locally. Optional convenience, not required.
- Update `docs/infra-notes.md` log section.

### 4. Backend: explicit `SECURE_COOKIES` (should-fix)

Replace the `cfg.Env != "dev"` cookie-security derivation with an explicit config var.

- Add `SecureCookies bool` to `config.Config`, read `SECURE_COOKIES` env var, default **true** (fail-safe — prod stays secure even if the var is missing).
- `backend/internal/handler/auth.go:68` becomes `secureCookie: cfg.SecureCookies`.
- Local dev `.env.local` sets `SECURE_COOKIES=false` (documented in `docs/infra-notes.md`).
- Prod / staging leave it unset → defaults to true.

### 5. Backend: unconditional dotenv load (nit)

Drop the `if os.Getenv("ENV") != "prod"` gate in `config.Load()`. `godotenv.Load(".env.local", ".env")` is a no-op when files don't exist, and in the production container there is no `.env*`. One fewer branch, one less env-type check.

### 6. Retire the `ENV` var entirely (cleanup)

Once steps 1–5 land, nothing in code reads `cfg.Env`. Audit, then:

- Remove `Env` from `config.Config` and the `ENV` lookup in `config.Load`.
- Delete `ENV=dev` / `ENV=prod` lines from `.env.example`, deploy docs, and Cloud Run env config.
- Update AGENTS.md's example (`ENV=production`) to use a concrete explicit flag like `LOG_LEVEL=info` so we don't reintroduce the pattern by example.

If we find a legitimate need for a generic env label (metrics tags, Sentry environment), reintroduce it for *tagging only* — never for control flow.

### 7. Verification

- `grep -rn "NODE_ENV\|cfg\.Env\|\.Env ==\|\.Env !=" backend frontend` → no hits.
- `make lint && make test` clean.
- Manual: start backend, `curl` an endpoint, send `SIGTERM`, confirm log line `"shutdown complete"` and process exits 0.
- Manual: unset/set `SECURE_COOKIES=false` locally, confirm cookie `Secure` flag toggles; default-on in prod.

## Out of scope

- Observability/analytics work — tracked separately in `plans/v2/t2/observability-and-analytics.md`.
- Anything that would add new backing services or change deploy topology.

## Log

- 2026-04-19 — Created plan after auditing codebase against 12factor.net. Verified violations in `frontend/src/lib/log.ts:1`, `backend/cmd/server/main.go:25,52`, `backend/internal/handler/auth.go:68`, `backend/internal/config/config.go:25`.
- 2026-04-19 11:20 — Implemented all phases.
  - Frontend `log.ts` now reads `NEXT_PUBLIC_LOG_LEVEL` (debug|info|warn|error, default info); removed `NODE_ENV` branch.
  - Backend `main.go` rewritten: `*http.Server` with `ReadHeaderTimeout`, `signal.NotifyContext(SIGINT, SIGTERM)`, 25s `Shutdown`, DB `Close` after drain. Always emits JSON via `slog.NewJSONHandler` with `HandlerOptions.Level = cfg.LogLevel`.
  - Added `SECURE_COOKIES` (default `true`) and `LOG_LEVEL` (default `info`) to `config.Config`; auth handler now reads `cfg.SecureCookies`.
  - Dropped the `ENV != "prod"` gate around `godotenv.Load`.
  - Retired `ENV` entirely: removed `Env` field from `config.Config`, deleted `ENV=dev` lines from `.env.example`/`.env.local`, updated integration test config literal, refreshed AGENTS.md and `docs/infra-notes.md` (added Configuration section covering new vars + graceful-shutdown note).
  - Verification: `go vet ./...` clean, `go build ./...` clean, `make test` passes (all Go + vitest), `make lint-frontend` clean (oxlint 0 errors). `grep -rn "NODE_ENV\|cfg\.Env\|\.Env ==\|\.Env !=" backend frontend` returns no hits. (Backend `golangci-lint` not installed locally; CI will cover.)
