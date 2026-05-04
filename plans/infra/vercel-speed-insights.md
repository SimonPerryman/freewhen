# Vercel Speed Insights

Status: done

## Goal

Enable Vercel Speed Insights to capture real-user Core Web Vitals (LCP, INP, CLS) for the production frontend. Free on Hobby tier; useful baseline before further mobile-first perf work.

## Plan

1. Add `@vercel/speed-insights` dependency in `frontend/`.
2. Import `SpeedInsights` from `@vercel/speed-insights/next` in `frontend/src/app/layout.tsx` and render `<SpeedInsights />` inside `<body>` after `{children}`.
3. Run `make lint` and `make test`.
4. Open PR, merge to main. Vercel auto-deploys; verify data appears in the Vercel Speed Insights tab.

## Log

- 2026-05-04 — Created plan.
- 2026-05-04 — Added `@vercel/speed-insights` and mounted `<SpeedInsights />` in `frontend/src/app/layout.tsx`. Lint + tests green.
