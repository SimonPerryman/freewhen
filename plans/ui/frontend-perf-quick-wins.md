# Frontend perf quick wins

Status: done

## Goal

A Lighthouse mobile audit of production scored Perf 84 (LCP 4.3s on simulated
slow 4G, 3.5s of it render delay) and Best Practices 96 (401 console errors).
Trim the JS every route ships on first load, stop competing font preloads, and
stop hammering `/auth/me` so the critical path is shorter and quieter.

## Plan

1. **Lazy-load PostHog** — `src/lib/analytics.ts` stops importing `posthog-js`
   statically. `init()` schedules a dynamic `import("posthog-js")` after the
   browser is idle (`requestIdleCallback`, `setTimeout` fallback). Calls to
   `identify`/`reset`/`capture` made before the SDK loads are queued and
   replayed in order once it initialises, so no events are lost. Add
   `disable_surveys: true` (we don't use surveys; avoids an extra fetch).
   When `NEXT_PUBLIC_POSTHOG_KEY` is unset everything stays a no-op and
   nothing is queued or imported.
2. **Fonts** — `preload: false` on Instrument Serif (only used for one italic
   accent word, so it shouldn't compete with the body/display preloads).
   Evaluate dropping Fraunces' `opsz` axis; keep DM Sans as is.
3. **Deduplicate `/auth/me`** — `AuthProvider` owns the one check: it fires on
   mount, keeps a single in-flight promise, and caches the result (signed in
   *or* signed out) until `setUser`/`clearAuth`/`signOut`. `checkAuth` becomes
   a stable callback that returns the shared promise/cached result. `Header`
   and the home page stop calling `checkAuth` and just read context. A 401 is
   the expected signed-out state — resolve to `null`, no error logging; other
   failures `log.warn`.
4. **browserslist** — add a modern target (Safari 15+, Chrome last 2 years)
   to `package.json` if it trims polyfills/transpilation without dropping
   supported mobile browsers.
5. Tests: analytics queue/replay + no-op when unset; AuthProvider dedupe
   (multiple consumers → one `getMe`), 401 → signed out without console error.
6. Measure first-load JS for `/` and `/calendars/[id]` before/after
   (`next build`, gzipped sum of root main + layout + page entry chunks).

Out of scope: `CalendarPageClient.tsx`, participant/token code, and
`next.config.ts` headers (other PRs in flight).

## Log

- 2026-11-12 10:00 — Created plan. Baseline (`next build`, gz): `/` 209.6 KB,
  `/calendars/[id]` 233.1 KB first-load JS.
- 2026-11-12 10:40 — PostHog moved behind idle-time dynamic import with a
  bounded pre-load call queue; `disable_surveys: true`. Tests in
  `src/lib/analytics.test.ts`.
- 2026-11-12 10:50 — AuthProvider now owns the single `/auth/me` (shared
  in-flight promise + cached result incl. signed-out); Header and home page
  read context only. 401 resolves to signed out silently; other failures
  `log.warn`. Tests in `AuthProvider.test.tsx`.
- 2026-11-12 10:55 — Fonts: Instrument Serif `preload: false` and italic-only
  (every use is italic) — font preloads on `/` drop 4 → 2. Kept Fraunces
  `opsz`: h1–h3 use `font-optical-sizing: auto`, so dropping it changes hero
  rendering.
- 2026-11-12 10:55 — browserslist: skipped. Next 16's default target is
  already chrome/edge/firefox 111 + safari 16.4; a Safari 15+ list would widen
  transpilation, not trim it, and the polyfill chunk is `nomodule` (modern
  browsers never download it).
- 2026-11-12 11:05 — After (`next build`, gz first-load JS): `/` 209.6 → 152.6
  KB, `/calendars/[id]` 233.1 → 176.1 KB. Note: the browser itself still logs
  the one expected 401 `/auth/me` as a network console error; removing that
  needs the backend to answer signed-out with a 2xx (follow-up).
