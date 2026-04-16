# Calendar View — Join + Mark Availability

Status: done

## Goal

Build the `/calendars/[id]` page where users join a calendar by entering their name, then toggle days to mark their availability. This is step 7 of `v1-build-order.md`.

## Context

- Backend is complete: `GET /calendars/{id}`, `POST /calendars/{id}/participants`, `PUT /calendars/{id}/availability` all work.
- Frontend API client in `src/lib/api.ts` has typed functions for all endpoints (`getCalendar`, `addParticipant`, `setAvailability`).
- `SetAvailabilityRequest` sends `{ participantId, dates[] }` — the backend does a replace-all (deletes old rows, inserts new).
- Participant names are unique per calendar (`store.ErrDuplicateName` / 409 from the API).
- Participants get a colour from a 10-colour palette, assigned by the backend.
- Step 6 (calendar creation page) redirects to `/calendars/{id}` on success — this page needs to exist.
- Styling uses CSS modules, no Tailwind. Existing patterns in `src/app/page.module.css`.
- Copy-link button was deferred from step 6 to here.

## Plan

### 1. Route + page shell — `src/app/calendars/[id]/page.tsx`

- Dynamic route using App Router `[id]` param.
- Client component (`"use client"`) — needs state, effects, localStorage.
- On mount, call `getCalendar(id)` to fetch calendar data.
- Three states: loading, error (calendar not found / network), and loaded.
- Display the calendar name (or "Untitled Calendar" if empty) as the heading.

### 2. Join flow

- Check localStorage for a saved participant ID for this calendar (`freewhen:<calendarId>` key).
- If found and the ID exists in the fetched participants list, skip join — user is already a participant.
- If not found (or stale), show a name input + "Join" button.
- On submit: call `addParticipant(calendarId, { name: name.trim() })`.
  - On success: save `participantId` to localStorage, update local state.
  - On 409 (duplicate name): show inline error "That name is already taken".
  - On other error: show generic error message.
- Name input is required (non-empty after trim).

### 3. Availability grid

- **Rolling 5-week window** starting from the Monday of the current week. Navigation via back/forward arrows that shift by one week. Back arrow disabled when the window already starts at the current week (no past navigation).
- **Timezone: local.** All date arithmetic uses the browser's local date (`new Date()` → `toLocaleDateString` / manual `YYYY-MM-DD` formatting). Documented in Decisions — backend stores bare dates, so local is the only sane choice for "today".
- Each day cell is clickable once the user has joined.
- Clicking a day toggles it: selected (user is free) or unselected.
- **Full-set PUT, not window-set.** The user's local selected-dates set includes dates outside the visible window. Every `setAvailability` call sends the complete set — not just what's on screen. On load, prune dates strictly before today from the fetched availability before seeding local state (avoids accumulating stale past dates; the backend rows remain but are never sent back).
- **Seeding the user's set.** On load, filter `CalendarWithDetails.availability` by the current user's `participantId` to initialise the local selected-dates `Set<string>`. Other participants' dates are held separately for display only.
- **Optimistic UI.** Toggle updates local state immediately so the grid feels instant. The PUT fires in the background.
- **Serialized saves with retry banner.** Rapid clicks queue — the next `setAvailability` call waits for the current one to resolve before firing with the latest state. This avoids out-of-order replace-all races. On failure: keep the user's latest intended state in the UI (don't revert — they may have kept clicking), show a persistent "Save failed — Retry" banner. Clicking retry re-fires the PUT with the current local state. Banner dismisses on success.
- Show the user's own selected dates with a distinct style (filled background using their assigned colour).
- Days before today are greyed out and not selectable (UI-only — backend does not enforce this).

### 4. Participant list + other people's availability

- Show a list of participants (name + colour dot) below or beside the grid.
- On the grid, show small colour indicators for other participants who are free on each day (e.g. coloured dots or segments).
- Highlight the current user in the participant list.
- This is a lightweight preview — step 8 (visual overlap) will make it richer.
- **No live polling.** Other participants' changes are visible on page load/refresh only. This is a known V1 limitation, not a bug.

### 5. Copy-link button

- A button that copies the current page URL to the clipboard (`navigator.clipboard.writeText`).
- Feature-detect `navigator.clipboard` — hide the button if unavailable (non-secure context). Fine for V1 since dev is localhost and prod will be HTTPS.
- Brief "Copied!" feedback on click (swap text for ~2 seconds).
- Placed near the heading, prominent enough to share.

### 6. Components

Extract into `src/components/`:

- `CalendarGrid.tsx` — the month grid, receives dates, selected dates, other participants' availability, and an `onToggle` callback.
- `JoinForm.tsx` — name input + submit, receives `onJoin` callback and error state.
- `ParticipantList.tsx` — list of participants with colour dots, highlights current user.
- `CopyLinkButton.tsx` — copy URL to clipboard with feedback.

Each component gets its own `.module.css` file. Design tokens (`--border`, `--text-primary`, `--accent`, etc.) are lifted to `globals.css` `:root` (and dark-mode media query) so all components share them without re-declaring. Step 6's `page.module.css` is updated to consume from `:root` instead of scoping its own copies.

### 7. localStorage strategy

- Key: `freewhen:<calendarId>` → value: JSON `{ participantId, name }`.
- On page load, read the key. If the participant ID isn't in the fetched participants list, treat as new user (the participant may have been deleted or it's stale) — but pre-fill the join form's name input from the stored `name` so returning users don't retype.
- On successful join, write the key with both fields.

### 8. Tests

- `src/app/calendars/[id]/page.test.tsx`:
  - Renders loading state.
  - Fetches and displays calendar data.
  - Shows join form when user is not a participant.
  - Skips join form when localStorage has a valid participant ID.
  - Handles calendar not found (404).
- `src/components/CalendarGrid.test.tsx`:
  - Renders correct days for current month.
  - Toggles day selection on click.
  - Disables past dates.
- `src/components/JoinForm.test.tsx`:
  - Submits name, calls onJoin.
  - Shows error state.
  - Disables submit on empty input.
- `src/components/CopyLinkButton.test.tsx`:
  - Copies URL to clipboard, shows feedback.

### 9. Manual verification

- `make dev-backend` + `make dev-frontend`.
- Create a calendar from `/`, land on `/calendars/{id}`.
- Enter a name, join, see the grid.
- Toggle days, verify network calls to `PUT /calendars/{id}/availability`.
- Open same URL in an incognito tab, join as second user, see both users' availability.
- Refresh — returning user is recognised via localStorage.
- Copy link button works.

## Decisions

- **Grid model: rolling week window**, not snap-to-month. 5 weeks from current Monday, shift by week. Simpler than month navigation and avoids the "can't go before today" awkwardness with month boundaries.
- **Timezone: local.** Dates are derived from the user's browser time. Backend stores bare `DATE` values with no timezone. Users near midnight may see different "today" than each other — acceptable for V1.
- **Optimistic UI + serialized saves + retry banner.** Grid updates instantly on click; PUT fires in background; rapid clicks are queued (not debounced) so every intent is captured. On failure, keep the user's latest intended state (don't revert — confusing if they kept clicking) and show a persistent "Save failed — Retry" banner. Retry re-fires the PUT with current local state.
- **Full-set PUT.** Every save sends all selected dates, including those outside the visible window. On load, prune dates before today so stale past dates don't accumulate.
- **No live polling.** Other participants' data is fetched once on page load. Refresh to see updates. Explicit V1 limitation.
- **Past-date restriction is UI-only.** Backend accepts any date. Fine for V1.
- Replace-all availability on every toggle — simple, no diffing. Acceptable at V1 scale.
- Lightweight multi-user display (colour dots) — full visual overlap is step 8.
- **Design tokens lifted to globals.css `:root`.** Avoids re-declaring `--border`, `--text-primary`, etc. in every module CSS. Step 6's `page.module.css` updated to consume from `:root`.
- **Clipboard API feature-detected.** Copy-link button hidden if `navigator.clipboard` unavailable.

## Log

- 2026-04-17 — Created plan
- 2026-04-17 — Resolved ambiguities from review: rolling-week grid (not month nav), local timezone, serialized saves, optimistic UI with rollback, no live polling, localStorage stores name alongside participantId
- 2026-04-17 14:30 — Second review pass: full-set PUT (not window-set) with past-date pruning on load; seed user's selected set by filtering availability on participantId; retry banner instead of revert on save failure; lift CSS tokens to globals.css :root; feature-detect clipboard API; fixed log timestamp format
- 2026-04-17 — Implemented: lifted CSS tokens to globals.css :root; created CalendarGrid, JoinForm, ParticipantList, CopyLinkButton components; built /calendars/[id] page with join flow, localStorage persistence, optimistic toggling, serialized saves with retry banner; wrote tests (30/30 pass); lint clean
