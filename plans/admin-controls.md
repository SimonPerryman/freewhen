# Admin Controls

Status: done
Depends on: `plans/participant-user-linking.md`

## Goal

Give calendar owners essential moderation powers: remove participants, rename calendars, and (later) promote others to admin. This is the first feature that leverages the `owner_id` relationship added in auth, and it completes the "creator has control" story.

## Scope

From `future-ideas.md`:
- Owner can remove participants and rename the calendar
- Owner can promote other participants to admin (same remove/rename powers)
- Admins cannot remove or demote the owner — owner role is protected

**V2 scope (this plan):** Owner removes participants, owner renames calendar. Admin promotion is deferred — it requires a participant-to-user linking model that doesn't exist yet (participants are anonymous), and the need is low until calendars have many collaborators.

## Design Decisions

**Owner-only, not role-based (for now).** The `owner_id` on calendars already identifies who has control. Adding an `admins` table or role column is premature — there's no way to link anonymous participants to authenticated users yet. When participant-user linking lands (optional sign-in for joiners), admin promotion becomes straightforward.

**Authorization pattern.** Each admin endpoint checks: (1) valid session via `requireAuth` middleware, (2) `calendar.owner_id == user.id` in the handler. No new middleware needed — the ownership check is a simple query, not a cross-cutting concern.

**Rename is a PATCH, not PUT.** Only the name changes. The emoji stays as-is (it's set at creation and isn't user-facing as editable yet). If we add more editable fields later, PATCH is the right verb.

**Remove participant cascades availability.** The DB already has `ON DELETE CASCADE` from participants to availability, so deleting the participant row cleans up everything. The frontend should also remove the participant from local state optimistically.

**No soft delete.** Removed participants are gone. The use case is moderation (spam, wrong person), not undo. If undo becomes needed, it's a separate feature.

**Owner cannot remove themselves.** The owner is always a user, never a participant, so this isn't an issue — participants and users are separate entities.

## API

### New endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| PATCH | `/calendars/{id}` | Owner | Rename calendar. Body: `{"name": "New Name"}` |
| DELETE | `/calendars/{id}/participants/{participantId}` | Owner | Remove a participant (cascades to their availability) |

### Response behaviour

**PATCH `/calendars/{id}`:**
- 200 with updated calendar object on success
- 400 if name is empty or body is invalid
- 401 if not authenticated
- 403 if authenticated but not the owner
- 404 if calendar not found

**DELETE `/calendars/{id}/participants/{participantId}`:**
- 204 on success (no body)
- 401 if not authenticated
- 403 if authenticated but not the owner
- 404 if calendar or participant not found

### Why 403 not 404?

Leaking the existence of a calendar via 403 isn't a concern — calendar IDs are in shareable links, so anyone with the link already knows the calendar exists. Using 403 gives the frontend a clear signal to show "you don't have permission" vs "this doesn't exist".

## Backend Implementation

### Phase 1: Store layer

Add to `internal/store/calendar.go`:
- `UpdateCalendarName(ctx, calendarID, name string) (model.Calendar, error)` — `UPDATE calendars SET name = $1 WHERE id = $2 RETURNING ...`
- `GetCalendarOwner(ctx, calendarID string) (string, error)` — `SELECT owner_id FROM calendars WHERE id = $1` (lightweight ownership check without loading full calendar)

Add to `internal/store/participant.go`:
- `DeleteParticipant(ctx, calendarID, participantID string) error` — `DELETE FROM participants WHERE id = $1 AND calendar_id = $2`. Returns `ErrNotFound` if no rows affected.

### Phase 2: Service layer

Add to `CalendarService` interface and `internal/service/service.go`:
- `RenameCalendar(ctx, calendarID, userID, name string) (model.Calendar, error)` — checks ownership, validates name, calls store
- `RemoveParticipant(ctx, calendarID, userID, participantID string) error` — checks ownership, calls store

Service methods handle the ownership check: load owner_id, compare to userID, return a typed `ErrForbidden` if mismatched. This keeps authorization logic in one place (service), not scattered across handlers.

### Phase 3: Handlers

Add to `internal/handler/calendar.go`:
- `RenameCalendar(w, r)` — reads user from context, parses body, calls service, returns updated calendar or error
- `RemoveParticipant(w, r)` — reads user from context, extracts path params, calls service, returns 204 or error

Error mapping: `ErrForbidden` → 403, `ErrNotFound` → 404, validation → 400.

### Phase 4: Routes

Add to `internal/server/server.go` `routes()`:
```go
s.mux.HandleFunc("PATCH /calendars/{id}", s.requireAuth(s.handler.RenameCalendar))
s.mux.HandleFunc("DELETE /calendars/{id}/participants/{participantId}", s.requireAuth(s.handler.RemoveParticipant))
```

## Frontend Implementation

### Phase 1: API client

Add to `src/lib/api.ts`:
- `renameCalendar(id: string, name: string): Promise<Calendar>` — `PATCH /calendars/{id}`
- `removeParticipant(calendarId: string, participantId: string): Promise<void>` — `DELETE /calendars/{id}/participants/{participantId}`

### Phase 2: Owner detection

The calendar page already computes `is_owner` (line 87 of `CalendarPageClient.tsx`):
```ts
is_owner: !!user && data.calendar.ownerId === user.id
```

Lift this into component state so it can gate UI elements:
```ts
const isOwner = !!user && calendar.calendar.ownerId === user.id;
```

### Phase 3: Remove participant UI

Add a remove button to `ParticipantList` — only visible when `isOwner` is true. Each participant row gets a small "×" or trash icon button. Clicking it:
1. Shows a brief confirmation (e.g. "Remove {name}?" with confirm/cancel)
2. On confirm, calls `removeParticipant(calendarId, participantId)`
3. Optimistically removes from local state
4. Shows error toast if the API call fails and reverts

The owner sees this on every participant. Participants cannot remove other participants (no button rendered).

### Phase 4: Rename calendar UI

Add an edit affordance to the calendar title in the header — only visible when `isOwner` is true. Options:
- A small pencil icon next to the title
- Clicking it switches the `<h1>` to an input field (inline edit)
- On blur or Enter, calls `renameCalendar(calendarId, newName)`
- On Escape, reverts to the original name
- Shows the updated name optimistically

Keep it simple — no modal, no separate settings page. Inline editing is the most natural pattern for renaming.

### Phase 5: Settings panel (stretch)

A lightweight "Calendar Settings" section below the calendar, visible only to the owner. For now it just contains the rename field (moved from inline if we prefer) and the participant list with remove controls. This becomes the home for future owner-only settings (privacy mode, finalise date, etc.).

**Decision: defer this.** Inline rename + remove buttons on the participant list are sufficient for V2. A settings panel adds UI complexity that isn't justified yet.

## Testing

### Backend
- Unit tests for store methods (rename, delete participant, get owner)
- Unit tests for service methods (ownership check passes, ownership check fails with ErrForbidden, not found)
- Unit tests for handlers (200/204 success, 400 validation, 401 unauthed, 403 not owner, 404 not found)
- Integration test: create calendar as user A, try to rename as user B (403), rename as user A (200)
- Integration test: create calendar, add participant, remove participant as owner (204), verify availability is gone

### Frontend
- Component test: ParticipantList renders remove buttons when `isOwner` is true, hides when false
- Component test: calendar title is editable when `isOwner` is true, static when false

## Build Order

1. Backend Phase 1 (store) — can test in isolation
2. Backend Phase 2 (service) — ownership logic
3. Backend Phase 3–4 (handlers + routes) — working API
4. Backend tests
5. Frontend Phase 1 (API client)
6. Frontend Phase 2–3 (owner detection + remove participant)
7. Frontend Phase 4 (rename calendar)
8. Frontend tests
9. Manual QA on mobile

## Log

- 2026-04-19 21:30 — Created plan
- 2026-04-20 14:10 — Implemented end-to-end. Backend: `UpdateCalendarName`/`DeleteParticipant` store methods, service wrappers, `RenameCalendar`/`RemoveParticipant` handlers (owner check via existing `GetCalendarOnly` pattern, matching `FinaliseCalendar`), routes `PATCH /calendars/{id}` and `DELETE /calendars/{id}/participants/{participantId}`. Frontend: `renameCalendar`/`removeParticipant` API methods, inline rename on calendar title (pencil icon → input, Enter/blur saves, Esc reverts) with optimistic update + revert on failure, owner-only × button per row in `ParticipantList` with `confirm()` + optimistic remove. Tests: store integration, handler unit (success/401/403/404/400), `ParticipantList` component tests for canRemove behaviour, `CalendarPageClient` tests for rename and ownership-gated controls. Admin promotion deferred per scope.
