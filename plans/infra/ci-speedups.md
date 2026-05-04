# CI speedups: happy-dom, golangci config, parallel make targets

Status: done

## Goal

The `ci` job on `main` was taking ~2 minutes end-to-end, with `make test` alone
spending ~30s. Bring that down without splitting into more jobs (extra jobs pay
a ~30–60s setup tax that wipes out the savings).

After investigation the three biggest levers were:

1. **Vitest's jsdom environment** — 13s of environment startup + 5s setup, on top of ~5s of actual tests. Switching to happy-dom roughly halves the environment cost.
2. **golangci-lint with no config** — 27s running the default linter set, which includes slow analyses we don't need.
3. **Sequential `make test-backend && make test-frontend` and `lint-backend && lint-frontend && lint-pii`** — these are independent commands that can run on the same runner in parallel.

## Plan

1. Replace `jsdom` with `happy-dom` in `frontend/vitest.config.ts` and the dev
   dependencies. Fix the two test-level environment differences this surfaces:
   - `style.color` is not normalised from `#fff` to `rgb(255, 255, 255)` by
     happy-dom (it returns whatever you set). Adjust the assertion in
     `CalendarGrid.test.tsx` to compare hex against hex.
   - `navigator.clipboard` is exposed via a getter under happy-dom, so
     `Object.assign(navigator, { clipboard: ... })` throws. Use
     `Object.defineProperty` in `WhosMissingPanel.test.tsx`.
2. Add `backend/.golangci.yml` enabling a curated linter set (`errcheck`,
   `govet` minus `fieldalignment`, `ineffassign`, `staticcheck`, `unused`).
   Cuts golangci-lint runtime from ~27s to ~3s locally with caches warm.
3. Parallelise within the existing CI steps using `make -j`:
   - `make lint` runs `lint-backend`, `lint-frontend`, `lint-pii` with `-j3`.
   - `make test` runs `test-backend` and `test-frontend` with `-j2`.

   No new jobs, no extra runner setup tax. `lint` and `test` remain separate
   CI steps (lint should still gate test).

## Out of scope

- **Splitting CI into parallel jobs.** Considered and rejected: each new job
  pays the runner-setup cost. The `ci.yml:37-39` comment already captures this
  reasoning.
- **Switching to Bun.** The remaining cost in `make test` is happy-dom
  environment setup and Vitest worker startup, neither of which Bun would
  meaningfully accelerate. Tools like `oxlint`, `oxfmt`, and Vitest already use
  native binaries / worker threads. AGENTS.md already classifies Bun as an
  "optional speedup only" — that framing stands.
- **pnpm everywhere.** Already done. The only `npm` reference in CI is
  `npm install --global vercel@latest` in the deploy-frontend job; switching it
  to pnpm would save no measurable time and the Vercel CLI install path is
  conventional with npm.

## Expected impact

| Step | Before | After |
|---|---|---|
| `golangci-lint` | ~27s | ~5–8s |
| `make test` (frontend dominates) | ~30s | ~15–18s |
| `make lint` (sub-second work, parallelised) | ~1s | ~1s |
| **Total `ci` job** | **~2m** | **~1m30s** |

## Log

- 2026-05-04 15:20 — Investigated CI timings on main run #8. Identified the three levers above (jsdom environment, golangci defaults, sequential make targets).
- 2026-05-04 15:21 — Swapped jsdom for happy-dom; removed `jsdom` from `frontend/package.json`, added `happy-dom`. Vitest local: environment 13.4s → 7.0s.
- 2026-05-04 15:22 — Fixed `CalendarGrid.test.tsx` colour assertion and `WhosMissingPanel.test.tsx` clipboard stub for happy-dom semantics. All 219 tests pass.
- 2026-05-04 15:23 — Added `backend/.golangci.yml` with curated linter set. Local run drops from ~27s (CI baseline) to ~3s with warm cache.
- 2026-05-04 15:24 — Parallelised `make lint` and `make test` with `-j` recursive make. Verified both still pass.
- 2026-05-04 15:30 — Wrote this plan in retrospect; opened PR.
