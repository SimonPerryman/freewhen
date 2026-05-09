# Surface "preferred" and notes without a wizard

Status: done

## Goal

Two of the most useful participant features — marking a date as **preferred** and leaving a **note** — live exclusively inside `DayDetailSheet`, which only opens when you tap a cell you've *already* marked free. First-time participants tap once, see no further affordance, and leave without ever discovering either feature. Owners hit the same dead-end.

We considered an interactive onboarding wizard and rejected it: wizards are usually a band-aid for unintuitive UI, and most users dismiss them. Instead, surface the second-tap interaction in three lightweight, persistent places. No modal, no tour, no extra route.

A static HTML demo of the proposed treatment lives at `demo-discoverability.html` (project root, not shipped) — open it for the visual reference.

## Plan

Three changes, all in `frontend/src/components/CalendarGrid/`. They compose: each one stands alone but they're cheaper to ship together because they share styling and state.

### 1. Always-visible instruction strip above the grid

A single line of helper text rendered above the day-of-week row, styled as a soft terracotta-bordered hint (matches the demo: `border-left: 2px solid var(--accent)`, subtle tinted background, ~12.5px DM Sans).

Copy: **"Tap once to mark a date free. Tap again to prefer it or add a note."**

- New element inside `CalendarGrid.tsx`, rendered just above the existing day-of-week row, only when `currentParticipantId` is set (owner-as-participant included; pure-spectator owners don't need it).
- Copy stored as a plain string in the component — no i18n yet, matches existing patterns in the file.
- Styling lives in `CalendarGrid.module.css` as a new `.instructionStrip` class.
- Mobile-first: full-width inside the grid container, single line wraps gracefully at 375px (verify).

### 2. Chevron affordance on own selected cells

Selected cells currently look terminal. Add a small chevron (corner-lower-right, ~10px, white at 70% opacity over the terracotta fill) to every cell where `isMyOwnFree(date)` is true. Pure CSS — no extra DOM if we use `::after` on the cell when it has the existing "mine" modifier.

- Add `::after` rules in `CalendarGrid.module.css` keyed off the existing own-free class. Use the same border-right + border-bottom rotated-square trick from the demo.
- Make sure it doesn't collide with the existing top-left vote badge (★) or top-right note dot — chevron stays in the bottom-right corner.
- Hide it on the preferred variant if it crowds the star marker; verify visually. If it crowds, drop the chevron when `preferred` is true on the assumption the cell is already obviously "rich".
- Respect `prefers-reduced-motion`: skip the entrance fade-in animation; render chevron at full opacity immediately.

### 3. One-time first-mark coachmark

The first time a participant successfully marks a cell free in a given calendar, show a small dark tooltip pointing at that cell explaining the second-tap. Dismiss on any of: tapping the dismiss button, tapping anywhere outside, or making a second tap (which itself opens the detail sheet).

Persist dismissal in `localStorage` keyed by calendar + participant: e.g. `freewhen.coachmark.detailSheet.${calendarId}.${participantId}` = `"1"`. So:

- A new participant on a new calendar sees it.
- A returning user does not.
- An existing participant who joined before this change ships gets it once on their next visit (graceful introduction, no "what changed" notification needed).

Implementation:

- New component `CalendarGrid/CoachmarkPreferNote.tsx` (folder-per-component convention). Takes `anchorEl: HTMLElement | null` and `onDismiss: () => void`. Renders a positioned tooltip with arrow, using the same dark-on-cream styling as the demo (`background: var(--ink)`, cream text, terracotta accent on the verb "prefer").
- Position relative to `anchorEl.getBoundingClientRect()` against the calendar grid container. Recompute on resize / scroll (single `useEffect` with cleanup).
- Trigger: in `CalendarGrid.tsx`, after a successful toggle that *adds* a date (not removes), check `localStorage` for the dismissed flag and if absent, set local state `coachmarkAnchor = cellRef`. Show with a 220ms delay so the cell-fill animation lands first.
- Dismiss writes the flag and clears local state. Also auto-dismisses if the user opens the detail sheet (i.e. the second tap that the coachmark was teaching).
- `prefers-reduced-motion`: render coachmark with no fade/scale transition; instantly visible.
- Defensive: wrap `localStorage` calls in try/catch — Safari private mode etc.

## Out of scope

- Per-participant "tour" beyond this single coachmark. If we later need to teach more (e.g. comments thread, owner-only features), each gets its own narrowly-scoped coachmark — never a multi-step wizard.
- Server-side persistence of "have you seen this hint." `localStorage` is sufficient; if it gets cleared the user just sees the hint once more, which is harmless.
- Owner-specific hints. Owners viewing-as-participant get the same treatment. A separate plan can layer owner-only nudges (e.g. "share link is here") if/when needed.

## Verification

- Manual: at 375px viewport, confirm the instruction strip wraps cleanly, the chevron is visible without crowding the vote/note badges, and the coachmark stays inside the card on the leftmost and rightmost cells.
- Manual: in a fresh browser profile, joining a calendar and marking a cell triggers the coachmark exactly once; reloading does not re-trigger.
- Manual: opening the detail sheet auto-dismisses the coachmark.
- Tests: extend `CalendarGrid.test.tsx` to assert (a) the instruction strip renders only when `currentParticipantId` is set, (b) `::after` chevron is not asserted directly (CSS-only) but a regression test for "selecting a date adds the `mine` class" guards the hook. Add a `CoachmarkPreferNote.test.tsx` for show/dismiss/localStorage flag behaviour.
- Lint + format clean (`make lint`, `make fmt`). All frontend tests pass (`make test-frontend`).

## Log

- 2026-05-09 — Created plan; reviewed `demo-discoverability.html` and confirmed three-part approach (instruction strip, chevron affordance, one-time coachmark) over a multi-step wizard.
- 2026-05-09 — Implemented all three parts in `CalendarGrid/`. Added `instructionStrip` rendered when `currentParticipantId` is set; added `mine` class + `::after` chevron (suppressed when a vote badge already occupies bottom-right or when finalised); new `CoachmarkPreferNote.tsx` triggered on first-add tap, dismissed via Got it / outside-click / second-tap, persisted in `localStorage` keyed by `freewhen.coachmark.detailSheet.{calendarId}.{participantId}`. `CalendarGrid` now takes a `calendarId` prop wired through `CalendarPageClient`. Tests + lint + format all green.
