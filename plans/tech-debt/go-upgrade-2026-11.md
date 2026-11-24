# Go toolchain and module upgrade (2026-11)

Status: done

## Goal

Bring the backend onto the latest stable Go and current module versions, and
check for known vulnerabilities, so we aren't carrying drift into later work.

### How the Go version is pinned

The `go` (and optional `toolchain`) lines in `backend/go.mod` are Go's
equivalent of `.nvmrc`:

- `go 1.27.4` is the minimum Go version the module needs. With the default
  `GOTOOLCHAIN=auto`, an older local `go` command downloads and switches to
  that version automatically, so contributors don't have to install it by hand.
- A `toolchain goX.Y.Z` line, when present, names a *preferred* newer toolchain
  for working on the module without raising the minimum. We don't need one:
  this is an application, not a library, so the minimum and the preferred
  version are the same.
- CI's `actions/setup-go` reads the same file (`go-version-file:
  backend/go.mod`), so bumping the directive bumps CI too. The Docker builder
  image (`backend/Dockerfile`) is the one place that has to be changed by hand.

## Plan

1. Check the latest stable Go (`https://go.dev/dl/?mode=json`) and bump the
   `go` directive plus the Dockerfile builder image.
2. `go list -m -u all`, then `go get -u ./...` and `go mod tidy` for
   minor/patch updates.
3. Look for major-version successors of direct deps; read their migration
   guides and take them only if the code change is small.
4. Run golangci-lint and govulncheck.
5. `make lint`, `make test`, `make test-integration`.

## Versions

| Module | Old | New |
| --- | --- | --- |
| Go (`go` directive) | 1.26.0 | 1.27.4 |
| Docker builder | `golang:1.26-alpine` | `golang:1.27-alpine` |
| github.com/jackc/pgx/v5 | v5.9.2 | v5.13.0 |
| github.com/posthog/posthog-go | v1.12.1 | v2 (`/v2` v2.1.1) |
| golang.org/x/oauth2 | v0.36.0 | v0.38.0 |
| golang.org/x/sys | v0.21.0 | v0.50.0 |
| github.com/fergusstrange/embedded-postgres | v1.34.0 | v1.34.0 (v2 skipped) |
| github.com/joho/godotenv | v1.5.1 | v1.5.1 (latest) |
| indirect: cloud.google.com/go/compute/metadata | v0.3.0 | v0.10.0 |
| indirect: github.com/goccy/go-json | v0.10.5 | v0.12.0 |
| indirect: github.com/jackc/puddle/v2 | v2.2.2 | v2.2.4 |
| indirect: github.com/lib/pq | v1.10.9 | v1.12.3 |
| indirect: golang.org/x/sync | v0.17.0 | v0.25.0 |
| indirect: golang.org/x/text | v0.29.0 | v0.44.0 |
| indirect (new, via posthog-go) | — | andybalholm/brotli v1.3.0, klauspost/compress v1.20.5 |

### Major versions

- **posthog-go v2 — taken.** We only use feature-flag evaluation
  (`NewWithConfig`, `IsFeatureEnabled`, `GetFeatureFlag`, `Close`), whose API is
  unchanged. The v2 breaking changes are all on the event-capture path, which we
  don't use. Code change: the import path, and `Config.PersonalApiKey` (now
  deprecated) → `Config.SecretKey`, which accepts the same `phx_` key.
- **embedded-postgres v2 — skipped.** Not a small change: v2 runs Postgres
  under a separate supervisor executable downloaded from GitHub releases at
  runtime, uses Windows restricted tokens, changes `RuntimePath` semantics,
  drops the v1 archive cache and the implicit lib/pq registration, and defaults
  to Postgres 18. `internal/embeddedpg` (orphan reaping, the Windows Job
  Object, lock files beside `RuntimePath`) is built around v1's process model,
  so moving deserves its own plan and test pass. Test-only dependency, so no
  production impact from staying on v1.

## Findings

- golangci-lint: one new issue after the bump (SA1019, deprecated
  `PersonalApiKey`), fixed. Clean afterwards.
- govulncheck (`golang.org/x/vuln` v1.10.0): No vulnerabilities found.

## Log

- 2026-11-24 10:00 — Created plan. Latest stable Go is 1.27.4.
- 2026-11-24 10:10 — Bumped `go` directive to 1.27.4 and Dockerfile builder to
  `golang:1.27-alpine`; `go get -u ./...` + `go mod tidy`.
- 2026-11-24 10:20 — Read migration guides for posthog-go v2 and
  embedded-postgres v2. Took posthog-go v2 (import path + `SecretKey`); skipped
  embedded-postgres v2 (supervisor/process-model rewrite).
- 2026-11-24 10:30 — golangci-lint clean, govulncheck clean, `make lint`,
  `make test` (frontend under Node 24) and `make test-integration` pass.
