# Submit-to-View Privacy

Status: cut

## Goal

Let the calendar owner set a privacy mode that hides other participants' names and availability from a joiner until that joiner has marked at least one date. The aim is to prevent anchoring bias — people defaulting to "yeah I'm free whenever the group is free" instead of honestly reporting their own availability.

## Scope

From `docs/future-ideas.md` "Progressive disclosure" list, this plan implements mode **3. Submit to view** only:

- New per-calendar setting `visibility` with values `open` (current behaviour, default) and `submit_to_view`.
- When `submit_to_view`, any viewer who has not yet marked at least one date sees no participant names and no availability — only their own selections and the calendar metadata (name, emoji, finalised state).
- Owner sees everything regardless of mode (needed for moderation and to see responses as they come in).
- Once a participant marks at least one date, they see everything for the rest of that session and on all future visits (state is server-side: it's derived from whether the participant has any availability rows).

**Deferred** (same column can host later):
- `join_to_view` (hides before the viewer has created a participant)
- `submit_first_then_reveal` (same as submit-to-view but stricter — no partial reveal; effectively identical for our grid UI, not worth the distinction until there's a user request)
- `host_only_reveal` (only the owner ever sees availability — different product shape, plan separately)

## Design Decisions

**One column, enum-like string.** `calendars.visibility TEXT NOT NULL DEFAULT 'open'` with a CHECK constraint restricting to known values. Easier to extend than a boolean, and maps cleanly onto the five-mode design from `future-ideas.md`.

**Enforce on the server, not the client.** The filter must happen in `GET /calendars/{id}` — we can't trust a client to hide data it already has. Frontend gating is a UX nicety, but the authoritative hide happens in the handler.

**"Has submitted" = has any availability row.** Simplest possible signal. If someone marks a day and then unmarks it, they drop back to hidden — acceptable, and probably correct: they haven't committed anything. An alternative (a `has_submitted` flag on participants) adds state without buying anything.

**Owner always bypasses.** The owner is authenticated (owner_id is a user, not a participant). If `user.id == calendar.owner_id`, return the full view. This matches the finalise/rename pattern already used in `handler/calendar.go`.

**Identifying the viewer for anonymous users.** The client already holds `participantId` in localStorage and passes it on `POST /calendars/{id}/availability`. For the GET, accept an optional `?as={participantId}` query param — if present and the participant belongs to this calendar and has availability rows, reveal the full view. Treating the participant ID as a bearer-style identifier is consistent with how `setAvailability` already works.

**No enumeration risk from `?as=`.** Participant IDs are random; the `?as=` param only changes what the *same* viewer sees. Passing a random other participant's ID doesn't leak anything — the handler only checks "does this participant (a) belong to this calendar and (b) have any availability?" and responds full/hidden accordingly. It never echoes the participant back. Worst case: a guesser confirms a participant ID exists, but they already know all participant IDs once anyone submits anything.

**Hidden response shape.** Return the calendar object intact, and return:
- `participants: []`
- `availability: []`
- `participantCount: <int>` — so the UI can show "5 people have joined, mark a day to see them".

Omitting participants entirely (rather than returning stub objects with redacted names) keeps the response type narrow and removes the temptation to leak anything via colour, id, or user linkage.

**Setting the mode.** Two surfaces:
1. A dropdown on the calendar creation form (`/calendars/new`) — default Open.
2. A PATCH on `/calendars/{id}` extended to accept `visibility` alongside `name` — owner only.

No separate settings panel yet; the rename PATCH already exists, extending it is cheaper than a new endpoint.

**Finalised calendars.** Visibility still applies. If a viewer hasn't submitted, they see the finalised date (it's on the calendar metadata) but not who voted for it or who's in the group. This is the desired behaviour — you shouldn't need to know who else is coming to know the date is locked in.

## API

### Schema

Migration `008_add_calendar_visibility.sql`:
```sql
ALTER TABLE calendars ADD COLUMN visibility TEXT NOT NULL DEFAULT 'open'
  CHECK (visibility IN ('open', 'submit_to_view'));
```

### Model change

`model.Calendar` gains:
```go
Visibility string `json:"visibility"`
```

Serialized as `"open"` or `"submit_to_view"`.

### Endpoint changes

**POST `/calendars`** — accept optional `visibility` in body. Validate against the allowed set. Default `open` if omitted.

**PATCH `/calendars/{id}`** (existing rename endpoint) — accept optional `visibility`. Validate. Owner only (already enforced).

**GET `/calendars/{id}`** — accept optional `?as={participantId}` query param. Compute `reveal`:
- `reveal = true` if `visibility == "open"`, OR
- the session user is the owner, OR
- the session user is a linked participant with availability rows, OR
- `as` is a participant of this calendar and has availability rows.

If `reveal`, return full `CalendarWithDetails`. Otherwise, return `{calendar, participants: [], availability: [], participantCount}`.

Response shape change is additive (new `participantCount` field always present, participants/availability arrays may be empty). Existing clients keep working.

## Backend Implementation

### Phase 1 — Schema & model

- Add migration `008_add_calendar_visibility.sql`.
- Add `Visibility` field to `model.Calendar`.
- Update `store/calendar.go` SELECT/INSERT/UPDATE to include `visibility`.
- Add a `const` block for valid values and an `IsValidVisibility(string) bool` helper, either in `model` or `store`.

### Phase 2 — Store layer

- `CreateCalendar(ctx, name, ownerID, visibility string)` — extend signature; default applied at service layer, store takes the literal.
- `UpdateCalendarVisibility(ctx, id, visibility string)` — mirrors `UpdateCalendarName`.
- `CountParticipants(ctx, calendarID string) (int, error)` — needed for the `participantCount` field when hiding.
- `ParticipantHasAvailability(ctx, participantID string) (bool, error)` — `SELECT EXISTS(SELECT 1 FROM availability WHERE participant_id = $1)`.

### Phase 3 — Service layer

- `CreateCalendar` accepts `visibility`. Validates or falls back to `open`.
- `UpdateCalendarVisibility(ctx, calendarID, userID, visibility)` — owner check + validation.
- New `GetCalendarView(ctx, calendarID string, viewer Viewer) (CalendarView, error)` where `Viewer` carries `{userID *string, participantID *string}` (either, both, or neither). Returns a struct with either a full `CalendarWithDetails` plus `Revealed bool`, or the hidden shape. Keeping the "should I reveal" decision in the service means the handler stays thin and the logic is unit-testable.

### Phase 4 — Handlers

- `CreateCalendar` — parse optional `visibility`, validate, pass through.
- `RenameCalendar` → generalise to `UpdateCalendar` (still PATCH `/calendars/{id}`). Accept either or both of `name`/`visibility`. Empty body = 400. Keeps backwards compatibility — existing PATCH with only `name` still works.
- `GetCalendar` — read `?as=` query param, look up session user, call `GetCalendarView`, serialise. Always include `participantCount` in the response (even when revealed — it's cheap and simplifies the frontend types).

### Phase 5 — Routes

No new routes; all changes ride on existing paths.

## Frontend Implementation

### Phase 1 — Types & API client

- Extend `Calendar` type in `src/lib/types.ts` with `visibility: "open" | "submit_to_view"`.
- Extend `CalendarWithDetails` with `participantCount: number`.
- `createCalendar(name, visibility?)` — new optional arg.
- `updateCalendar(id, patch: {name?, visibility?})` — replace current `renameCalendar` or keep `renameCalendar` as a wrapper for backwards compatibility of callers.
- `getCalendar(id, as?)` — optional `as` participant ID; if the client has a stored participant, always pass it.

### Phase 2 — Creation form

Add a visibility control to `/calendars/new` — simplest form: a radio group or a single "Hide others' availability until you respond" toggle, labelled in plain language. Default off (= `open`). Show a short explainer sentence ("Prevents people from just matching everyone else's dates.").

### Phase 3 — Calendar page gating

In `CalendarPageClient`:
- Compute `revealed = calendar.visibility === 'open' || isOwner || hasSubmitted`, where `hasSubmitted = selectedDates.size > 0 || /* server already revealed this response */`. Simplest: if the server returned non-empty `participants`, we're revealed.
- Optimistic reveal: the moment the user toggles their first date, swap to revealed UI *optimistically*. Refetch on the save completion to populate real participant data. This avoids a blank-to-populated flash that would feel buggy.
- When hidden:
  - Hide `ParticipantList`, `SummaryView`, `BestDay`, `InvitePrompt`.
  - `CalendarGrid` renders, but each cell only shows the current user's own selection — no heatmap, no initials, no "N free" counts. The existing `freeByDate` already filters out "not yet toggled" correctly; we just need to pass an empty `participants` array.
  - Show a banner above the grid: "Mark a day to see when the other N people are free." Use `participantCount` for N; if 0, show "Mark a day — you're the first to respond."

### Phase 4 — Owner settings control

On the calendar page, when `isOwner`, add a small control near the rename pencil to toggle visibility. A dropdown or an overflow menu — single concept for now (Open / Submit to view). Changing it calls `updateCalendar` and optimistically updates local state.

### Phase 5 — Analytics

Add events:
- `calendar_visibility_set` `{ calendar_id, visibility }` — on creation + on change
- `availability_gate_unlocked` `{ calendar_id }` — fired once per session when a hidden viewer marks their first date

Lets us measure whether the mode actually gets used and how often it gates a real response.

## Testing

### Backend

- Store: `ParticipantHasAvailability` true/false, `CountParticipants`, `UpdateCalendarVisibility` happy path + invalid value.
- Service: `GetCalendarView` with matrix — visibility open, submit_to_view × viewer owner / submitted participant / unsubmitted participant / anonymous.
- Handler: `GET /calendars/{id}` with `?as=` returning hidden vs revealed; POST/PATCH validation errors for bad visibility values; PATCH with only visibility, only name, both, neither (400).
- Integration: create a submit_to_view calendar, add two participants, first has no availability → GET as them returns hidden; they mark a day → GET returns revealed.

### Frontend

- Component test: `CalendarPageClient` with `visibility: submit_to_view` and no `selectedDates` renders the hidden-state banner and not `ParticipantList`/`SummaryView`.
- Component test: toggling a date flips to revealed UI without waiting for the refetch (optimistic).
- Component test: owner always sees the full view regardless of visibility.
- Component test: creation form submits the chosen visibility.

## Build Order

1. Backend Phase 1–2 (schema, model, store) — isolatable, fully testable.
2. Backend Phase 3 (service, including `GetCalendarView`) — the core decision.
3. Backend Phase 4 (handler changes) — wires everything.
4. Backend tests (unit + integration).
5. Frontend Phase 1 (types + API client).
6. Frontend Phase 3 (page gating) — the meaty bit; test against a manually-created submit_to_view calendar.
7. Frontend Phase 2 (creation form).
8. Frontend Phase 4 (owner toggle).
9. Frontend Phase 5 (analytics).
10. Manual QA on mobile: create submit_to_view, join as second user in another browser, verify hidden state, mark a day, verify reveal.

## Resolved Questions

- **Hide names too, but show the count.** Hidden state returns `participants: []`, `availability: []`, `participantCount: N`. UI copy emphasises the count ("5 people have joined — mark a day to see who's free") so the viewer knows the calendar is active and is enticed to respond.
- **No live push on visibility change.** If the owner flips `submit_to_view` → `open` while another user has the page open, that user stays on the hidden view until their next refetch. Pushing the change over WebSockets is out of scope; it's covered by the separate "Live updates" Tier 2 feature.

## Log

- 2026-04-20 14:00 — Created plan.
- 2026-04-20 14:20 — Confirmed with user: hide names but surface participant count; no live push on visibility change (refetch model is fine, WebSockets deferred to the Live Updates plan).
- 2026-04-20 15:30 — Backend implemented: migration 008, `visibility` column with CHECK, `ParticipantHasAvailability`/`CountCalendarParticipants` store methods, `GetCalendarView` service with reveal matrix, PATCH `/calendars/{id}` accepts `visibility`, GET accepts `?as=`. Unit tests cover open/submit_to_view × owner/submitted/unsubmitted/anonymous, plus invalid-visibility handler responses.
- 2026-04-20 15:40 — Frontend implemented: `visibility` toggle on creation form, owner toggle on calendar page, hidden-state banner with participant count, optimistic reveal on first toggle + refetch with `?as=`, localStorage stores participant colour for hidden-mode grid render. Analytics events `calendar_visibility_set` and `availability_gate_unlocked` wired up. Status → done.
- 2026-04-23 — Cut. Feature removed per `plans/remove-submit-to-view.md`; see `docs/cut-features.md` for rationale and restoration notes. Status → cut.
