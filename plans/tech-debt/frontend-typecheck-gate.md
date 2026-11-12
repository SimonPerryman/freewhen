# Frontend typecheck gate

Status: done

## Goal

`tsc --noEmit` fails on main (53 errors, all in test files / e2e). Nothing in
`make lint` or CI runs tsc — vitest and `next build` don't type-check test
files — so type errors in tests regress silently. Fix every error properly and
add a typecheck step to the lint gate so it stays green.

## Plan

1. Fix all `tsc --noEmit` errors with correct types: typed `vi.fn<...>()`
   mocks, complete fixtures. No `any`, no `@ts-ignore`/`@ts-expect-error`, no
   tsconfig loosening.
2. Add `typecheck` script (`tsc --noEmit`) to `frontend/package.json`.
3. Wire it into the Makefile frontend lint target and the CI frontend lint step.
4. Update AGENTS.md Development section and `docs/onboarding.html` where lint
   steps are listed.
5. Gate: typecheck, vitest, oxlint, oxfmt check (changed files), `pnpm build`.

## Log

- 2026-11-12 10:00 — Created plan. Baseline: 53 tsc errors across e2e/claim-participant (1), CalendarPageClient (2), CalendarGrid (39), JoinForm (9), MyCalendarsList (2) tests.
- 2026-11-12 15:30 — Fixed all 53 errors: typed `vi.fn<Props["onX"]>()` mocks (CalendarGrid, JoinForm via `ComponentProps`), typed `(id: string) => Promise<void>` api mocks (MyCalendarsList), added missing `updatedAt` to Calendar fixtures (CalendarPageClient), single `CalendarBody` type with optional `user_id` in e2e claim test. No production code changes needed.
- 2026-11-12 15:45 — Added `typecheck` script; `make lint-frontend` now runs `oxlint && pnpm run typecheck` (CI runs `make lint`, so covered). Updated AGENTS.md and onboarding.html. Gate: typecheck 0 errors, vitest 411/411, oxlint clean, build ok.
