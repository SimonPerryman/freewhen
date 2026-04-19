# Calendar Error States

Status: done

## Goal

When a user hits a calendar URL that returns 404 (or any load failure), the page currently renders a tiny line of red text — "Calendar not found" — with no heading, no illustration, and no affordance. It looks broken, not intentional.

Make the error state feel deliberate: a centered, well-proportioned panel with a clear heading, sub-copy, and a primary CTA. Match the visual language of `src/app/not-found.tsx` and `src/app/error.tsx` so all three empty-ish pages read as a family.

## Plan

- In `frontend/src/app/calendars/[id]/CalendarPageClient.tsx`:
  - Track the error case as a discriminated value (e.g. `"not-found" | "load-failed" | null`) rather than a generic string, so the render branch can pick copy + CTA.
  - Extract the retry-the-fetch behaviour out of the current `load()` effect into a `loadCalendar` callback we can call again from a "Try again" button on the load-failed state.
  - For `not-found`: heading "Calendar not found", sub-copy hinting the link may be wrong or expired, CTA "Back to home" (Link to `/`).
  - For `load-failed`: heading "Couldn't load this calendar", sub-copy about a connection issue, CTA "Try again" that re-invokes `loadCalendar`.
- In `frontend/src/app/calendars/[id]/page.module.css`:
  - Add styles for the error panel — centered column, heading, sub-copy, button — mirroring `not-found.tsx`'s proportions.
  - Keep the existing `.error` banner-style class untouched (used inline in other places) or replace if unused elsewhere.
- Update `frontend/src/app/calendars/[id]/CalendarPageClient.test.tsx` if existing tests assert on the old "Calendar not found" string — adjust selectors to match the new heading, and add a test for the retry flow if it's cheap.
- Manual QA at 375px: panel stays centered, button is tappable, no horizontal overflow.

## Non-goals

- Hiding the `/auth/me` 401 from the browser devtools network tab — that's a browser behaviour when unauthenticated, not something we can suppress cleanly. The AuthProvider already handles it silently at the app level.

## Log

- 2026-04-19 10:13 — Created plan
- 2026-04-19 10:14 — Implemented: `CalendarPageClient` now tracks error as a `"not-found" | "load-failed"` union and renders a centered panel with heading, sub-copy, and CTA. 404 shows a "Back to home" link; other failures show a "Try again" button that re-runs the fetch via a `loadAttempt` counter. Added `.errorPanel/.errorHeading/.errorSub/.errorCta` styles in `page.module.css`. Added 2 tests covering the not-found panel and the retry flow. All 86 frontend tests pass; lint clean.
