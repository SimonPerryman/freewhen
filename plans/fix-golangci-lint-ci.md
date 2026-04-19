# Fix golangci-lint CI failure

Status: done

## Goal

CI is broken because golangci-lint v1.64.8 (built with Go 1.24) can't lint code targeting Go 1.25.0. Upgrade to golangci-lint v2.x which supports Go 1.25.

## Plan

1. Update `.github/workflows/ci.yml`: change `golangci/golangci-lint-action@v6` to `@v7` (v7+ supports golangci-lint v2 only)
2. Pin golangci-lint version to `v2.4` or later (first version with Go 1.25 support)
3. Verify `make lint-backend` command (`golangci-lint run ./...`) is still valid in v2 — confirmed it is

## Log

- 2026-04-19 — Created plan
- 2026-04-19 — Updating action from v6 to v7
