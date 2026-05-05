# Calendars list — overflow menu for row actions

Status: in-progress

## Goal

The `/calendars` page currently shows a bare `Duplicate` button on every owned
calendar row. It competes visually with the calendar name, reads as a primary
CTA when it shouldn't, and leaves nowhere to put future row-level actions
(Leave, Rename, Delete, Share). Replace it with a discreet overflow menu
(`⋯`) that collapses secondary actions and adapts to the viewer's role.

## Plan

### UX shape

- Whole card remains the primary tap target (navigates to the calendar).
- Top-right of each row: icon-only `⋯` button (~36px hit area on mobile).
  - `stopPropagation` + `preventDefault` so tapping it doesn't follow the link.
  - `aria-haspopup="menu"`, `aria-expanded`, focus-visible styling.
- Menu opens as a popover on desktop, bottom sheet on mobile (match existing
  `DuplicateCalendarSheet` pattern for ≤640px).

### Menu items by role

- **Owner**: Duplicate, Rename, Delete (destructive, red).
- **Member**: Duplicate, Leave (destructive, red).
- Don't render disabled items — omit. No "Leave (you're the owner)" greys.
- Non-owners currently don't see Duplicate at all; keep that behaviour for
  this pass (revisit separately if we want members to fork a calendar).

### Confirmation + feedback

- Destructive actions (Leave, Delete) use the existing Toast + Undo pattern
  (`useToast()` from `@/lib/toast`) per AGENTS.md — no `window.confirm`.
- Optimistically remove the row; restore on Undo or on server error.

### Backend dependencies

- **Leave**: no dedicated endpoint yet. `DELETE /calendars/{id}/participants/{participantId}`
  exists (`server.go:128`) — confirm it permits self-removal for non-owners
  and wire `removeParticipant(calendarId, myParticipantId)` from the client.
  If auth currently restricts this to owners, add a self-leave path on the
  handler. Out of scope to design a new endpoint here; capture as a follow-up
  if the existing one doesn't cover it.
- **Delete calendar**: no endpoint exists. Ship the menu item behind a flag
  or omit until backend lands. Track as a separate plan (`plans/v2/.../delete-calendar.md`)
  rather than expanding scope here.
- **Rename**: `renameCalendar` already exists in `lib/api.ts`. Reuse the
  rename flow from `CalendarSettingsSheet` or open a small inline prompt.

### Files to touch

- `frontend/src/components/MyCalendarsList/MyCalendarsList.tsx` — replace
  the inline `Duplicate` button with `<RowMenu role={cal.myRole} …/>`.
- `frontend/src/components/MyCalendarsList/MyCalendarsList.module.css` —
  remove `.duplicateButton`, add `.menuTrigger` styles.
- New `frontend/src/components/RowMenu/RowMenu.tsx` (or reuse a primitive
  if one already exists — check before adding).
- Tests: extend `MyCalendarsList` test coverage for menu open, owner vs
  member items, destructive confirmation toast.

### Phasing

1. UI scaffold — overflow menu replaces the Duplicate button, only Duplicate
   wired up. Visual win immediately, no backend work.
2. Add Rename for owners (uses existing API).
3. Add Leave for members once backend self-removal is confirmed/wired.
4. Add Delete for owners once backend endpoint exists (separate plan).

## Log

- 2026-05-04 22:40 — Created plan
- 2026-05-04 22:50 — Implemented phases 1 + 2: added `RowMenu` (mobile bottom-sheet, desktop popover) and `RenameCalendarSheet`; replaced the inline Duplicate button on `MyCalendarsList` with a `⋯` menu exposing Duplicate + Rename for owners. Non-owners see no menu yet (members will get Duplicate/Leave once phase 3 lands).
- 2026-05-04 22:50 — Phase 3 (Leave) deferred: `DELETE /calendars/{id}/participants/{participantId}` is owner-only (`backend/internal/handler/calendar.go:355`) and `myCalendarDTO` doesn't expose `myParticipantId`. Needs a backend self-leave path + DTO field; capture as a follow-up plan rather than expanding scope here.
- 2026-05-04 22:50 — Phase 4 (Delete) deferred to a separate plan per original scoping — no backend endpoint exists.
