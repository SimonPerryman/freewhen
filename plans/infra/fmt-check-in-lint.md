# Wire `oxfmt --check` into `make lint`

Status: done

## Goal

Stop unformatted frontend code from landing on main. Previously `make fmt`
rewrote files but no target ever *checked* formatting, so drift accumulated
silently and only surfaced when someone ran `make fmt` locally and noticed
the diff.

## Plan

1. Commit the existing oxfmt drift on `main` as a one-shot formatting pass
   so the gate-introducing change lands on a clean tree.
2. Add a `fmt-check` Makefile target that runs `oxfmt --check src/`
   (read-only, exits non-zero on drift).
3. Add `fmt-check` to the `lint` fan-out. CI already runs `make lint`, so no
   workflow edit is required — gating happens automatically.

Mirror the same gate on the backend: enable `gofmt` and `goimports` as
formatters in `backend/.golangci.yml`. `golangci-lint run` reports
unformatted files as issues, so the existing `lint-backend` target picks
this up automatically. Extend `make fmt` to call `golangci-lint fmt` so
the local "fix it" command matches what CI checks.

## Log

- 2026-05-10 — Created plan. Committed 27-file oxfmt pass + 16-file
  gofmt/goimports pass to clear pre-existing drift. Wired `fmt-check`
  into `make lint`, enabled `gofmt`/`goimports` formatters in
  `backend/.golangci.yml`, extended `make fmt` to format the backend
  via `golangci-lint fmt`. Verified locally.
