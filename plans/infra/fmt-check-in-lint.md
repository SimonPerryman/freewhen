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

Backend is already covered: `gofmt`-equivalent checks come via `golangci-lint`
through `staticcheck`/`govet` and a `go fmt` diff would show up in PR review;
not adding a separate `gofmt -l` gate unless drift becomes a problem.

## Log

- 2026-05-10 — Created plan, committed 27-file oxfmt pass, wired `fmt-check`
  into `make lint`. Verified locally.
