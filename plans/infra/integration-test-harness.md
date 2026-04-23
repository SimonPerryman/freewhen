# Integration Test Harness

Status: done

## Goal

Make `make test-integration` runnable by default — locally and in CI — without requiring contributors to install Postgres, run Docker, or provision a Neon branch. The integration suite already exists (`//go:build integration` in `backend/internal/store/` and `backend/internal/server/`) but silently no-ops when `TEST_DATABASE_URL` is unset, so DB-touching code paths ship untested.

## Approach

Use [`fergusstrange/embedded-postgres`](https://github.com/fergusstrange/embedded-postgres) — a Go library that downloads a real Postgres binary on first run and starts/stops it in-process. Zero external dependencies (no Docker, no local install), real Postgres (not a mock/sqlite), ~1s startup.

If `TEST_DATABASE_URL` is already set (CI with a service container, Neon branch, etc.), use it as-is and skip the embedded server. This keeps the existing behaviour intact and lets CI pick whichever approach is cheapest.

## Plan

### 1. Add the dependency

- `cd backend && go get github.com/fergusstrange/embedded-postgres`
- First run downloads the binary to `~/.embedded-postgres-go/` (cached between runs). Size is ~60MB per platform — acceptable for a one-time download.

### 2. Shared test harness helper

Create `backend/internal/testdb/testdb.go` (new package, `//go:build integration`) exposing:

```go
// Start returns a DSN for a running Postgres instance and a cleanup func.
// If TEST_DATABASE_URL is set, returns that DSN with a no-op cleanup.
// Otherwise starts an embedded Postgres on a free port.
func Start(t *testing.T) (dsn string)
```

Implementation notes:
- Use `t.Helper()` + `t.Cleanup()` for teardown — callers don't manage lifecycle.
- Pick a free port via `net.Listen("tcp", ":0")` then close, to avoid clashes when multiple packages run in parallel under `go test ./...`.
- Start once per `testing.M` via `sync.Once` rather than per-test — embedded-postgres startup is ~1s and we don't need a fresh cluster between tests (the existing `DELETE FROM` sweep in each setup already gives isolation). Expose as `testdb.Start(t)` which returns the shared DSN.
- Use a unique database name (`freewhen_test`) inside the shared cluster; migrations run via the existing `store.Migrate` path.
- Data dir under `t.TempDir()` → auto-cleanup.
- Log level: quiet (embedded-postgres is chatty by default).

### 3. Wire into existing test setups

- `backend/internal/store/store_test.go::setupTestStore` — replace the direct `os.Getenv("TEST_DATABASE_URL")` + `t.Fatal` with `dsn := testdb.Start(t)`.
- `backend/internal/server/server_integration_test.go::setupTestServer` — same swap.
- The `t.Fatal` on unset env var goes away; the tests now always run under the `integration` tag.

### 4. Makefile

- `make test-integration` already runs `go test -tags=integration ./...` — no change needed, but it now actually executes tests.
- Update `make test` to include integration tests? **No** — keep `test` fast (unit-only) and leave integration behind `test-integration`. Update CI to run both.

### 5. CI

- Extend `.github/workflows/ci.yml`'s `ci` job:
  - Add a `Test (integration)` step running `make test-integration` after the existing `Test` step.
  - Cache `~/.embedded-postgres-go/` via `actions/cache` keyed on the embedded-postgres version, so CI doesn't redownload Postgres every run.
- Alternative considered: GH Actions `services: postgres` container with `TEST_DATABASE_URL` pointed at it. Rejected — embedded-postgres in CI keeps local and CI paths identical, removing a class of "works locally, fails in CI" bugs. The ~60MB download is cached.

### 6. Documentation

- `AGENTS.md` → add a short note under **Development**: "`make test-integration` runs against an embedded Postgres — no setup required. Override with `TEST_DATABASE_URL=...` to point at an existing database."
- `docs/infra-notes.md` → mention the embedded-postgres cache location for anyone debugging CI.

## Out of scope

- Parallelising integration tests (`t.Parallel()`) — current suite is <20 tests, sequential is fine.
- Per-test transactions for isolation — the `DELETE FROM` sweep is adequate for V1.
- Running integration tests against prod-equivalent Postgres major version matrix — embedded-postgres defaults to a recent stable; Neon tracks similarly.

## Risks

- **Embedded-postgres binary availability on ARM CI runners** — the project uses `ubuntu-24.04-arm`. Confirm `embedded-postgres` has an arm64 Linux build (it does, via `zonkyio/embedded-postgres-binaries`, but verify during implementation).
- **First-run download flakiness** — mitigated by the cache step; fallback is to pre-seed in a setup action.
- **Port conflicts under parallel `go test`** — mitigated by the free-port picker and per-`testing.M` singleton.

## Log

- 2026-04-21 08:43 — Created plan
- 2026-04-21 08:50 — Added `backend/internal/testdb` package using `fergusstrange/embedded-postgres`; starts once per test binary via `sync.Once` on a free port, honours `TEST_DATABASE_URL` when set.
- 2026-04-21 08:50 — Wired `store_test.go` and `server_integration_test.go` setups to `testdb.Start(t)`; replaced all five direct `os.Getenv("TEST_DATABASE_URL")` call sites in the server test.
- 2026-04-21 08:50 — Fixed latent bug in `TestFinaliseFlow` (reused `model.Calendar` var with `omitempty` field left the stale `FinalisedDate` pointer after unfinalise) — exposed now that the suite actually runs.
- 2026-04-21 08:51 — Added CI step `Test (integration)` with `actions/cache` on `~/.embedded-postgres-go`; documented in `AGENTS.md` and `docs/infra-notes.md`.
