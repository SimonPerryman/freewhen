# Project Scaffolding

Status: done

## Goal

Set up the full project structure before any product work begins — Go backend, Next.js frontend, dev tooling, and test infrastructure.

## Plan

1. Create Go backend with `cmd/server/main.go`, `internal/{handler,model,store,server}/`, `migrations/`
2. Create Next.js frontend with App Router, TypeScript, `src/{app,components,lib}/`
3. Add root files: `.env.example`, `.gitignore`, `Makefile`, `README.md`, `AGENTS.md`
4. Add scaffolding doc to `docs/`
5. Add test setup — `go test` for backend, Vitest for frontend
6. Add Makefile targets for test, lint, fmt, build, dev

## Log

- 2026-04-16 16:00 — Scaffolded Go backend, Next.js frontend, and root files
- 2026-04-16 16:05 — Committed scaffolding
- 2026-04-16 16:07 — Added test infrastructure: Vitest + jsdom for frontend, go test with build tags for backend, Makefile targets
