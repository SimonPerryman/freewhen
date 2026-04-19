# Cleanup Pass

Status: done

## Goal

Bring the codebase up to the standards we'd expect for a small-but-real project: tidier layout, structured logging, minimal observability, and CI. Scope deliberately stays small — no new features, no speculative abstractions.

## Runtime & tooling decisions (context)

- **Runtime**: Node 24 LTS — committed in dev *and* prod (Vercel).
- **Package manager**: pnpm — via Corepack, pinned in `packageManager` field.
- **Bun**: not a runtime, not a package manager. May be invoked opportunistically as a fast TS executor (e.g. `bunx vitest`, `bun run scripts/seed.ts`) if individual commands become a bottleneck. This is an opt-in speedup, not a dependency — the project must keep working without Bun installed.

## Plan

### 1. Frontend: switch npm → pnpm

Do this first so every subsequent step uses pnpm.

- Enable Corepack (ships with Node): `corepack enable`.
- Add `"packageManager": "pnpm@<version>"` to `frontend/package.json` — Corepack auto-installs the right version for every contributor.
- Delete `package-lock.json`, run `pnpm install` to generate `pnpm-lock.yaml`, commit it.
- Bump `@types/node` from `^20` to `^24` in `frontend/package.json` so types match the committed runtime.
- Update `Makefile`:
  - `dev-frontend`: `cd frontend && pnpm dev`
  - `lint-frontend`: `cd frontend && pnpm exec oxlint`
  - `fmt`: `cd frontend && pnpm exec oxfmt --write src/`
  - `test-frontend`: `cd frontend && pnpm test`
  - `build-frontend`: `cd frontend && pnpm build`
  - `build-backend`: `cd backend && go build ./...`
- Why pnpm: content-addressable store (less disk), stricter dep resolution (fewer phantom-dep bugs), faster installs — especially noticeable in CI.

### 2. Node version pinning

- Add `frontend/.nvmrc` with `24` (or the exact LTS minor at time of implementation).
- Add `"engines": { "node": ">=24 <25" }` to `frontend/package.json` so `pnpm install` warns on mismatch and we don't silently accept Node 25+.

### 3. Frontend: folder-per-component

Each component gets its own directory. Keep the existing dotted filenames (`CalendarGrid.tsx`, `CalendarGrid.test.tsx`, `CalendarGrid.module.css`) so this step is a pure move — no rename churn in imports or CSS module references. `ParticipantList` is missing a test — add `ParticipantList.test.tsx` as part of this step.

```
src/components/
  CalendarGrid/
    CalendarGrid.tsx
    CalendarGrid.test.tsx
    CalendarGrid.module.css
  CopyLinkButton/
  JoinForm/
  ParticipantList/
  SummaryView/
```

Update imports to explicit paths like `@/components/CalendarGrid/CalendarGrid`. No `index.ts` re-exports — explicit paths are greppable and avoid barrel-file bloat. Keep `lib/` flat — files there are single-purpose and would just gain a wrapper folder.

### 4. Backend: structured logging with `log/slog`

- Add a `logger` field to `Server` (pass `*slog.Logger` into `New`).
- Thread it through handlers (keep DI simple — constructor param, not global).
- Add a request-logging middleware: method, path, status, duration, request ID.
- Generate a request ID per request (`crypto/rand`, hex-encoded, 8 bytes). Attach to `context.Context` and include in log lines. Return as `X-Request-ID` response header.
- In `cmd/server`, build a JSON handler in prod, text handler when `ENV=dev`.

### 5. Backend: `GET /health`

- Returns `200 {"status":"ok"}` with a quick `db.PingContext` check (503 if DB unreachable).
- Have the request-logging middleware skip when `r.URL.Path == "/health"` so probe traffic doesn't pollute logs. Simpler than splitting the mux — health still gets CORS headers if needed, just no log line.

### 6. Backend: central config

- New `internal/config/config.go` that reads env once (`DATABASE_URL`, `PORT`, `ENV`, `CORS_ORIGIN`).
- `Load()` returns a `Config` struct with validation (missing `DATABASE_URL` → fatal).
- `cmd/server` calls `Load()` at startup; no more `os.Getenv` scattered around.

### 7. Frontend: tiny log wrapper

- `src/lib/log.ts` — exports `log.info`, `log.warn`, `log.error`.
- In dev: proxies to `console`. In prod: `info`/`warn` become no-ops, `error` still fires (so when we add Sentry later it's a one-line swap).
- Replace existing `console.*` call sites.

### 8. Frontend: error boundary

- `src/app/error.tsx` (Next.js App Router convention) — renders a friendly fallback, logs via `log.error`.
- `src/app/not-found.tsx` while we're here if missing.

### 9. API contract sketch

- `docs/api.md` — markdown table of endpoints, request/response shapes. Not OpenAPI; just enough that a new contributor doesn't have to read handlers to know the surface.

### 10. Hosting & future CI notes

Capture these in `docs/infra-notes.md` so decisions aren't lost. Section 13 is where we actually commit the stack; this section captures reasoning and alternatives so they're on record.

- **Hosting committed** (see §13 for the work) — frontend: Vercel (auto-build from git, free hobby tier fits V1). Backend: Fly.io (single Go binary, deploy via `flyctl`). DB: Neon (managed Postgres, free tier, pooled connection string).
- **Alternatives considered for backend** — Render (more point-and-click, fine if `flyctl` ergonomics grate); Railway (similar tier). Revisit if Fly bills get weird or cold-start latency bites.
- **Faster CI options if minutes become a bottleneck**:
  - [Blacksmith](https://www.blacksmith.sh/pricing) — 3,000 free min/month (same as GH), ~2x faster, swap via `runs-on:` change. Good starter upgrade.
  - [WarpBuild](https://www.warpbuild.com/pricing) — free credit (not monthly), then ~$0.003/min Linux. Better for heavy Docker/integration workloads.
  - GitHub cut hosted-runner prices up to 39% on Jan 1 2026, so the gap is narrower than it was — don't swap until 3k free min is actually tight.
- **Bun as an opportunistic speedup** — if `vitest` or a local script becomes slow, `bunx vitest` / `bun run script.ts` can shave seconds without committing the whole project to Bun. Do not add Bun to CI or `package.json` scripts without a real measured reason.

### 11. Sync `AGENTS.md`

Every change above that alters workflow or commands needs AGENTS.md updated in the same change that lands it. Specifically:

- Replace `npx vitest run path/to/file` with pnpm equivalent.
- Replace `npm run dev` references (if any) with pnpm.
- Note that the committed runtime is Node 24 + pnpm, with Bun as an optional speedup for individual commands.
- Note the new `.nvmrc` and expected Node version.
- Note the new `/health` endpoint, request-ID header, and structured logging convention.
- Note the new `docs/api.md` and `docs/infra-notes.md` references.
- Note the deploy targets (Vercel frontend, Fly.io backend, Neon DB), that backend migrations run on boot, and point at `docs/infra-notes.md` for first-time setup.

Rule for future work: any change to dev commands, runtime version, or directory layout must update AGENTS.md in the same PR. Treat doc drift as a failure mode of the work, not a separate task.

### 12. CI: GitHub Actions

- `.github/workflows/ci.yml` running on PRs to `main` and pushes to `main`.
- One job, ARM runner (`runs-on: ubuntu-24.04-arm` — cheaper, comparable speed for Node/Go).
- Steps:
  - `actions/checkout@v4`
  - `pnpm/action-setup@v4` (reads version from `packageManager` field — single source of truth)
  - `actions/setup-node@v4` with `node-version-file: frontend/.nvmrc`, `cache: 'pnpm'`, and `cache-dependency-path: frontend/pnpm-lock.yaml` (lockfile isn't at repo root, so the cache key needs the explicit path)
  - `actions/setup-go@v5` with `go-version-file: backend/go.mod`, `cache: true`
  - `golangci/golangci-lint-action@v6` (installs and caches the linter — `make lint` calls `golangci-lint run` and will fail without it)
  - `cd frontend && pnpm install --frozen-lockfile`
  - `make lint`
  - `make test`
  - `make build-backend` (catches compile errors tests miss)
  - `make build-frontend` (catches TS / RSC errors that vitest misses)
- Skip `test-integration` in CI for now (needs a Postgres service container — note it as a follow-up).
- Artifacts: none uploaded. CI proves "it builds + tests pass"; §13 adds the deploy step on top.

### 13. Deployment: Vercel + Fly.io + Neon

Commit the stack and wire deploy-on-push. The one-time account/project creation is genuinely manual; everything after that is CI.

**Backend (Fly.io):**
- Add `backend/Dockerfile` — multi-stage: `golang:1.23-alpine` builder → `gcr.io/distroless/static` runtime. Copy `migrations/` into the final image so the on-boot runner can find them.
- Add `backend/fly.toml` — app name, internal port 8080, HTTP healthcheck on `/health` (from §5), primary region, `auto_stop_machines = true` for the free tier.
- **Migrations run on boot** in `cmd/server`: read `backend/migrations/*.sql` in lexicographic order, track applied filenames in a `schema_migrations(filename text primary key, applied_at timestamptz)` table, apply pending inside a transaction, fail startup on error. Keep it small — don't pull in `golang-migrate` unless the file-tracking logic starts growing cases.

**Frontend (Vercel):**
- No code changes. Vercel's git integration deploys on push; project settings: root `frontend/`, install `pnpm install --frozen-lockfile`, build `pnpm build`. The `packageManager` field (§1) drives the pnpm version.
- Preview deploys on PRs come free — keep them on, wire `NEXT_PUBLIC_API_URL` to the prod backend for now (V1 has no staging env).

**Database (Neon):**
- One project, one branch for prod. Dev uses a separate Neon project (already the case, just formalise in infra-notes).

**CI deploy job (extends §12):**
- Add a second job `deploy-backend`, `needs: [test]`, gated on `if: github.ref == 'refs/heads/main'`.
- Steps: checkout, `superfly/flyctl-actions/setup-flyctl@master`, `flyctl deploy --remote-only` with `working-directory: backend` and `FLY_API_TOKEN` from GH secrets.
- No frontend deploy job — Vercel's git integration is the CD path. CI just gates merges.

**First-time setup (`docs/infra-notes.md` checklist):**
- Create Neon project, copy `DATABASE_URL`.
- `fly launch --no-deploy` in `backend/`, then `fly secrets set DATABASE_URL=... CORS_ORIGIN=... ENV=prod`.
- Create `FLY_API_TOKEN` (`fly tokens create deploy`), paste into GH repo secrets.
- Import repo in Vercel, set root to `frontend/`, set `NEXT_PUBLIC_API_URL` to Fly app URL.
- Point a custom domain (optional); update `CORS_ORIGIN` to match.

## Out of scope (flagged, not done)

- Sentry / error reporting SDK
- Prometheus metrics
- OpenAPI spec + generated client
- Integration tests in CI (needs Postgres service container)
- Rate limiting
- Auth (there isn't any yet — V1 design is share-a-link)
- Bun as a committed runtime or package manager (revisit if Vercel's Bun runtime leaves Beta and the dev-loop speed argument becomes pressing)
- Staging environment for backend — V1 ships prod-only; Vercel PR previews point at prod API
- Rollback tooling beyond what `fly releases` + `fly deploy --image <prev>` already give us
- Preview backend deploys per PR (would need per-PR Fly apps + DB branches; not worth it until we have auth or paying users)

## Log

- 2026-04-17 — Created plan
- 2026-04-17 — Added build steps to CI (Go + Next), switched to arm64 runners, pinned Node 22 via `.nvmrc`, added hosting + Blacksmith/WarpBuild notes section
- 2026-04-17 — Added pnpm migration step (Corepack-managed), bumped Node pin to 24 LTS (Node 20 EOL Apr 30 2026, Node 22 in maintenance), renumbered sections
- 2026-04-17 — Committed to Node + pnpm (runtime and package manager). Added "Runtime & tooling decisions" context block. Added explicit AGENTS.md sync step (section 11). Moved Bun to opt-in speedup only, noted in out-of-scope. Fixed section numbering (10/11 were swapped).
- 2026-04-17 14:30 — Audit fixes: reordered sections so pnpm (1) and Node pinning (2) come before everything else; moved infra-notes (10) before AGENTS.md sync (11) and CI (12) to fix dependency ordering; added `build-frontend` to Makefile updates in pnpm step; noted missing ParticipantList test in folder-per-component step; added HH:MM to log timestamps per AGENTS.md convention.
- 2026-04-17 15:10 — Second audit pass: resolved file-naming inconsistency in section 3 (keep dotted form, e.g. `CalendarGrid.tsx`, so the move is purely structural); bumped `@types/node` to `^24` in section 1 to match runtime; tightened engines range to `>=24 <25` in section 2; spelled out `/health` middleware exemption in section 5; added `cache-dependency-path` and `golangci-lint-action` to CI in section 12.
- 2026-04-17 16:00 — Added §13 (deployment): Vercel frontend + Fly.io backend + Neon Postgres, Dockerfile, fly.toml, on-boot migration runner, `deploy-backend` CI job, first-time setup checklist. Updated §10 to reference §13 as commitment point and §11 to include deploy targets in AGENTS.md sync. Added staging/rollback/preview-deploys to out-of-scope.
- 2026-04-17 16:45 — Resolved open decisions: section 3 uses explicit import paths (no index.ts re-exports); section 5 uses middleware skip for /health (not split mux); CI build steps use Makefile targets; added `build-backend` to Makefile updates in section 1.
- 2026-04-17 — Implemented sections 4, 5, 6: added structured logging with log/slog (text in dev, JSON in prod), request-logging middleware with request ID (X-Request-ID header), GET /health endpoint with db ping, central config package reading DATABASE_URL/PORT/ENV/CORS_ORIGIN from env. Updated migrate.go to accept *slog.Logger. Updated integration tests for new signatures.
- 2026-04-17 — Implemented all remaining sections. §1+2: switched npm→pnpm, added packageManager field (pnpm@10.30.3), .nvmrc (24), engines (>=24 <25), bumped @types/node to ^24, added oxlint as devDep, updated Makefile. §3: moved components to folder-per-component layout, updated imports to explicit paths, added ParticipantList.test.tsx. §7: created src/lib/log.ts. §8: created error.tsx and not-found.tsx. §9: created docs/api.md. §10: created docs/infra-notes.md. §11: updated AGENTS.md with all new conventions. §12: created .github/workflows/ci.yml (ARM runner, test + deploy-backend jobs). §13: created backend/Dockerfile (multi-stage distroless) and backend/fly.toml. All 70 frontend tests pass, Go builds clean.
- 2026-04-19 — Fixed §12 CI regression: `pnpm/action-setup@v4` was failing with "No pnpm version is specified" on every run because the action looks for `package.json` at the repo root, and this repo keeps `packageManager` in `frontend/package.json`. Added `with: package_json_file: frontend/package.json` so the action reads the pinned version from the single source of truth as originally intended.
