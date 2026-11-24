# Frontend dependency refresh (2026-11)

Status: done

## Goal

Close every open Dependabot alert on `frontend/pnpm-lock.yaml` (all
transitive: dompurify, protobufjs + @protobufjs/utf8, vite, @vitest/mocker,
tinypool, fflate, @opentelemetry/core, source-map-js) and take the pending
frontend upgrades, so `pnpm audit` is clean and `pnpm outdated` only lists
majors we've chosen to skip.

## Plan

1. Map each alert to the direct dependency that pulls it in (`pnpm why`).
2. Fix by bumping the parent; fall back to `pnpm.overrides` only when no
   parent release allows a patched version.
3. Take all minor/patch updates (`@types/node` stays on 24.x to match the
   Node 24 runtime).
4. Read changelogs for majors; take the small ones, list the rest.
5. Bump the pinned `packageManager` pnpm within 10.x.
6. Verify: oxlint, typecheck, oxfmt check, vitest, `next build`, backend
   lint/test (unchanged), CI e2e.

### Alert → parent

| Alert package | Pulled in by | Fix |
| --- | --- | --- |
| dompurify, fflate, protobufjs, @protobufjs/utf8, @opentelemetry/core | posthog-js | posthog-js 1.369.3 → 1.454.2 (dropped the OTLP exporter chain; dompurify ^3.4.18, fflate 0.4.9) |
| tinypool | oxfmt | oxfmt 0.40 → 0.80 (tinypool 2.2.0) |
| source-map-js | @vitest/coverage-v8 → magicast | coverage-v8 4.1.5 → 5.1.1 |
| @vitest/mocker | vitest | vitest 4.1.11 → 5.1.1 |
| vite | peer of @vitejs/plugin-react and vitest | added `vite ^8.3.8` as a direct devDependency |

`vite` is a peer dependency of `@vitejs/plugin-react`; pnpm had auto-installed
it and kept the stale 8.0.8 resolution even after the parents were bumped
(every parent accepts `^8`). Declaring the peer explicitly is the intended
way to satisfy it and keeps it visible to `pnpm outdated`, so no override is
needed.

### Majors

- Taken: vitest 4 → 5 + @vitest/coverage-v8 4 → 5 (suite passes unchanged),
  @testing-library/jest-dom 6 → 8 (7 adds a `@testing-library/dom` peer,
  already present; 8 only drops vitest < 5), TypeScript 5.9 → 6.0
  (typecheck and `next build` clean with the existing tsconfig).
- Skipped: TypeScript 7 (native port; ships no classic compiler API, which
  `next build`'s type-check step relies on), `@types/node` 26 (runtime is
  Node 24).

## Log

- 2026-11-24 09:55 — Created plan. Baseline: 39 open Dependabot alerts;
  `pnpm audit --prod` 28 (7 low, 16 moderate, 5 high); full `pnpm audit` 40
  (8 low, 23 moderate, 7 high, 2 critical). Tests 505/505.
- 2026-11-24 09:57 — Minor/patch bumps (next 16.4.6, react 19.3.1,
  posthog-js 1.454.2, oxlint 1.95, playwright 1.65.1, @types/node 24.19.2,
  vitest 4.1.12, oxfmt 0.80, …). Audit down to 8 (vite + fflate stale in
  lockfile); `pnpm update --depth Infinity` fixed fflate, vite needed the
  explicit peer. Audit clean.
- 2026-11-24 09:58 — vitest 5 + coverage-v8 5 + jest-dom 8: 505/505, no
  code changes. The ECONNREFUSED :8080 stderr noise during tests predates
  this change (same count on vitest 4).
- 2026-11-24 10:00 — TypeScript 6.0.3: typecheck + `next build` clean.
  TypeScript 7 skipped (see Majors). pnpm 10.30.3 → 10.34.9.
- 2026-11-24 10:01 — Both audits report no known vulnerabilities; oxlint,
  typecheck, oxfmt check, 505 tests, `next build`, backend lint + tests pass.
- 2026-11-24 10:06 — PR #116 opened; CI `ci` and `e2e` (Playwright 1.65)
  green.
