# Grid colours and Quick fill

Status: in-progress

## Goal

Make colour mean something on the calendar grid (Refactoring UI review, PR C): only free slots carry the participant colour, not-free/no-answer slots read as empty, and the bulk tools stop crowding the top of the page.

## Plan

- Not-free/no-answer slots: transparent with a dashed `--border` outline; initials stay (muted). Free slots keep the participant colour and contrast-aware initial.
- Remove red (`#E63946`) from the participant palette in `frontend/src/lib/overlap.ts` and `backend/internal/store/participant.go`; the owner (index 0) is now blue.
- Keep the "everyone free" tint untouched.
- Move Weekends / Weekdays / Import from Google / Clear into a native `<details>` "Quick fill" disclosure (44px summary tap target). Clear renders only when the participant has dates. `CalendarGrid` takes a `quickFillExtra` slot for the Import entry.
- Tests: CalendarGrid (faint slot unfilled, disclosure, Clear hidden), overlap palette contrast case.

## Log

- 2026-11-23 21:30 — Created plan
- 2026-11-23 21:45 — Implemented dashed empty slots, palette change (frontend + backend), Quick fill disclosure; vitest 505 pass, backend unit tests pass; no storage change so integration not run
- Existing participants keep their stored colour (including red); no migration, as there are no production users.
