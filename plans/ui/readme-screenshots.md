# README screenshots

Status: done

## Goal

The repo is public; give visitors a look at the app before they clone it. Add a short "Screenshots" section to `README.md` with real, current screenshots.

## Plan

- Run the e2e backend (`backend/cmd/e2e-server`, embedded Postgres) and a production build of the frontend locally.
- Seed obviously fake demo data through the API (`/test/login` with `example.com` emails): "Summer BBQ" with Alex, Sam, Priya and Jordan marking dates (a few notes and preferred votes), a finalised calendar, so "My calendars" has more than one row.
- Capture Playwright screenshots, light and dark (`<picture>` with a `prefers-color-scheme: dark` source), at 375px (mobile first) plus one desktop shot: landing, calendar grid, day sheet, finalised view, my calendars. Wait for fades before capturing.
- Save optimised PNGs (each well under 300 KB) to `docs/screenshots/`; add a heading, one line and a small image table to `README.md`.
- The capture script is throwaway (kept out of the repo); re-shoot by hand when the UI changes.

## Log

- 2026-11-24 09:40 — Created plan
- 2026-11-24 09:46 — Seeded demo data (Summer BBQ, Book club: December finalised on 3 Dec, Climbing trip) via `/test/login` + API against e2e-server and a production `next build`; captured light-mode shots at 375px (2x) and 1280px (2x)
- 2026-11-24 09:55 — Picked landing, calendar, day sheet, finalised, my calendars (mobile) + calendar (desktop); 108–230 KB each, no re-compression needed. Dark variants skipped to keep the README tidy
- 2026-11-24 10:05 — README section added; `make lint` and `make test` pass; servers, embedded Postgres and its temp dir cleaned up
- Spotted while capturing: on 375px the Summary card's "Sam prefers" pill (Sun 13 Dec row) overflows the card's right edge; the day sheet autofocuses the note field (pops the keyboard on phones)
- 2026-11-24 10:40 — Follow-up: dark-mode variants requested. Rebased on main (Next 16.4.6, Go 1.27.4; installed the matching Playwright Chromium). Renamed the demo calendar "Summer BBQ" to "Christmas drinks" (its dates are in December), re-seeded and retook the light set
- 2026-11-24 10:55 — Captured dark set (`colorScheme: "dark"`; the app honours prefers-color-scheme). The dark-mode grain overlay (`body::after` noise) pushed dark PNGs to ~1 MB, so both sets are shot with `reducedMotion: "reduce"`, which the app already uses to hide the grain (invisible at 2.5% opacity). All 12 files 100–255 KB
- 2026-11-24 11:05 — README images wrapped in `<picture>` with `-dark.png` sources; `make lint` and `make test` pass; servers, embedded Postgres and temp dir cleaned up
- 2026-11-24 14:45 — Refreshed after the UI fixes stack (#122–#126) merged. The rendered README had an empty third cell in the second table row, and the shots showed old UI: the plain "Quick fill", month-first nav dates next to day-first ones, and the day sheet with the note field focused. Retook all 12 on `main` in `en-GB` (dates now follow the browser locale). Added a sixth mobile shot, Settings with the emoji picker, to fill the empty cell. Gave the demo calendars themed emojis and added a fifth calendar, "Five-a-side", so My calendars doesn't end in a blank half screen. All 14 files are 114–255 KB.
- Spotted while capturing: the landing page's decorative mini-grid hard-codes "NOV 23 — NOV 29", month-first, regardless of locale. Left as is; it's app code, not docs.
