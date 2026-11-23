# Keep the calendar name through magic-link sign-in

Status: done

## Goal

A signed-out visitor who types a calendar name on the landing page shouldn't have to retype it after signing in. The calendar is created when they return signed in.

## Plan

- `lib/pendingCalendarName.ts`: save the trimmed name in localStorage (it must survive the link opening in another tab/app) with a 1h expiry; `take` removes it before use so reloads can't create duplicates.
- Landing: save on submit when signed out (and on a 401 from create); once signed in, consume the pending name once and create the calendar, then navigate. On failure show the error with the name left in the field.
- Callback page stays centred; return_url unchanged (landing is the default return).
- Tests: vitest for save/resume/no-duplicate/failure/stale; e2e for the full magic-link flow.

## Log

- 2026-11-23 21:15 — Created plan
- 2026-11-23 21:20 — Implemented pending-name helper and landing resume; vitest (504) and e2e magic-link flow pass
