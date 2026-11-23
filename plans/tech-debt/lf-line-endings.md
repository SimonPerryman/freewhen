# Force LF line endings via .gitattributes

Status: done

## Goal

With `core.autocrlf=true` (Windows default), working trees check out CRLF. gofmt (via golangci-lint) and `oxfmt --check` reject CRLF, so `make lint` fails on files nobody touched. Committed files are already LF. Make the repo's line endings independent of each contributor's git config.

## Plan

- Add `.gitattributes` with `* text=auto eol=lf` plus explicit binary entries.
- Verify a fresh checkout is LF, `git add --renormalize .` is a no-op, and `make lint` / `make test` pass.
- Note in AGENTS.md how to refresh an existing CRLF checkout.

## Log

- 2026-11-23 21:30 — Created plan
- 2026-11-23 21:35 — Added `.gitattributes`; fresh checkout in a worktree is LF, renormalize is a no-op, `make lint` and `make test` pass with autocrlf=true. No oxfmt/golangci config changes needed.
