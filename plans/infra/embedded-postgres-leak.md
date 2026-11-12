# Embedded Postgres leak

Status: done

## Goal

Test runs on a Windows dev machine leaked ~400 `postgres.exe` processes and 46
runtime dirs (~7.8 GB) in `%TEMP%` (`freewhen-epg-*` from
`internal/testdb`, `freewhen-e2e-pg-*` from `cmd/e2e-server`). Stop the leak,
and make sure that whatever still escapes (killed test binaries, `taskkill /F`
on the e2e server) is cleaned up automatically on the next run.

## Plan

1. Reproduce on `main`: run the integration suite normally, count
   `postgres.exe` processes and temp dirs before/after.
2. New package `internal/embeddedpg` shared by `testdb` and `e2e-server`:
   - `Start(prefix, db)` makes a `%TEMP%/<prefix>*` runtime dir, holds an
     exclusive OS file lock on `<dir>/owner.lock` for the instance's lifetime,
     starts embedded Postgres, waits until ready.
   - `Stop()` stops Postgres, releases the lock, removes the runtime dir.
   - `ReapStale()` runs before every start: for each `freewhen-epg-*` /
     `freewhen-e2e-pg-*` dir, if its lock can be taken the owner is dead →
     `pg_ctl stop -m immediate` (if a postmaster is alive) and delete the dir.
     Lock-less dirs (pre-fix) are deleted when their postmaster is dead, or
     stopped+deleted once older than 2h.
   - Windows only: put the owning process in a Job Object with
     `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` so the postmaster (and its
     backends) die with the owner even on `taskkill /F` / test timeout.
     No-op elsewhere (pg_ctl daemonises via setsid on Unix, so the reaper is
     the safety net there).
3. `testdb`: add `testdb.Main(m)` for `TestMain` (stops + removes on exit,
   also on SIGINT/SIGTERM); `Start` fails loudly if `Main` isn't wired.
   Add `TestMain` to the `store` and `server` integration packages.
4. `e2e-server`: use `embeddedpg`; existing SIGINT/SIGTERM path now also
   removes the dir via `Stop()`.
5. Check whether `-p 1` is really required for `-tags integration ./...`.
6. Prove: kill `go test` mid-run and `taskkill /F` the e2e server; count
   processes/dirs before and after the next run.
7. Docs: AGENTS.md Development notes, onboarding.html.

## Log

- 2026-11-12 16:46 — Created plan. Root cause found while reading
  `testdb.go`: the shared instance is started via `sync.Once` but never
  stopped and its runtime dir (~170 MB of extracted binaries + data) is never
  removed — every integration test binary leaks one Postgres cluster.
- 2026-11-12 17:00 — Reproduced on `main`: one green `go test -tags
  integration ./...` leaves 2 clusters (18 `postgres.exe`, 2 dirs, 305 MB);
  one `-v ./internal/server/` run leaves 1 more. 46 dirs ≈ 46 package runs.
  First parallel run on `main` also hung `internal/server` to the 10 min
  timeout; not reproducible in 5 later parallel runs.
- 2026-11-12 17:15 — Added `internal/embeddedpg` (Start/Stop/ReapStale, owner
  lock, Windows job object), `testdb.Main` + `TestMain` in store/server,
  rewired e2e-server. First attempt kept `owner.lock` inside RuntimePath —
  embedded-postgres `RemoveAll`s that on Start, so moved binaries to
  `<dir>/rt`.
- 2026-11-12 17:30 — Kill test: store.test `taskkill /F` mid-run → 10
  postgres.exe before, 0 after (job object), dir left behind; e2e-server
  `taskkill /F` → 11 → 0. Next start reaped both dirs ("owner exited").
  Normal runs leave 0 processes / 0 dirs. Added `embeddedpg` integration
  tests for Stop and ReapStale (live postmaster path, used on Unix).
- 2026-11-12 17:35 — `-p 1` not needed: 3 parallel runs of the full
  integration suite (3 packages booting Postgres at once) green. Makefile
  never used `-p 1`.
- 2026-11-12 17:50 — Cold-cache parallel start (fresh USERPROFILE) failed on
  Windows: concurrent downloads race renaming the archive into
  `~/.embedded-postgres-go` ("Access is denied"). Added one retry of
  `pg.Start` (cache is populated by then); 2 cold parallel runs green.
- 2026-11-12 17:55 — Gate green (build, vet, vet -tags integration, unit,
  integration with and without `-p 1`), golangci-lint clean on touched
  packages (windows + linux). Killed the 5 legacy clusters left by the `main`
  repro runs; 0 freewhen postgres.exe / 0 temp dirs remain.
