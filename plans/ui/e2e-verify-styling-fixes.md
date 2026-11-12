# E2E verify styling fixes

Status: done

## Goal

Fix five UI defects surfaced by a full-site end-to-end verification run:

1. The magic-link callback interstitial (`/auth/magic-link/callback`) was
   written with Tailwind utility classes, but the project has no Tailwind, so
   it rendered completely unstyled.
2. A stray list bullet renders next to each card on My calendars.
3. The finalise confirm button wraps its date across two lines at 375px.
4. The join form puts the optional email field after the Join button.
5. "N responded" counts every participant (including an owner who hasn't
   marked any dates) rather than only those who have submitted availability.

## Plan

1. Rewrite the callback page with a CSS module that reuses the login page
   patterns (centred container, heading, subheading, entrance animation with
   `prefers-reduced-motion`). Use the shared `Button` (primary, full width,
   `loading` while submitting). Error text uses `--error` and `role="alert"`.
   Grep the frontend for other Tailwind-style class strings; add a unit test
   that fails if any reappear.
2. `MyCalendarsList` `.list`: `list-style: none; margin: 0; padding: 0`, plus
   `role="list"` so Safari keeps list semantics.
3. `SummaryView` confirm button: wrap the date in a `white-space: nowrap`
   span so it never breaks mid-phrase.
4. `JoinForm`: name → email → submit. On ≥640px the name input and button no
   longer share a row; the button sits after the fields.
5. "Responded" is derived in the frontend (`participants.length` in
   `ParticipantList` and the calendar page OG description). Derive it from
   availability instead: a participant has responded once they have at least
   one availability row. No backend/API change.
6. Unit tests for each; extend e2e (magic-link-auth) to assert the Continue
   button is the shared Button and visible.

## Log

- 2026-11-12 10:00 — Created plan
- 2026-11-12 10:20 — Callback page rewritten on the login page's CSS module +
  shared Button (`loading` while verifying, error as `role="alert"` in
  `--error`). Repo-wide grep found no other Tailwind class strings (the only
  literal classNames left are real globals: `pill`, `skip-link`,
  `serif-accent`). Added `src/app/noTailwindClasses.test.ts` guard — confirmed
  it fails against the old callback page.
- 2026-11-12 10:30 — My calendars list reset (`list-style: none`, margin and
  padding 0) with `role="list"`; finalise confirm date wrapped in a `nowrap`
  span; JoinForm reordered to name → email → Join (button full width on
  mobile, auto width from 640px).
- 2026-11-12 10:40 — "Responded" was frontend-only (`participants.length`).
  Now `respondedParticipantIds(freeByDate)` feeds ParticipantList ("N of M
  responded") and `countResponded()` drives the calendar OG description. No
  API change.
- 2026-11-12 11:00 — Gate: vitest 451/451, tsc clean, oxlint clean, build ok,
  Playwright 13/13 (magic-link spec now asserts the styled Button). Checked
  screenshots at 375/1280, light/dark.
