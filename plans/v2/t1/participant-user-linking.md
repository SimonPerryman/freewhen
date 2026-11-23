# Participant-User Linking

Status: done

## Goal

Allow signed-in users to link their participant record to their user account when they join a calendar. This gives participants cross-device identity ("I joined on my phone, now I can edit availability on my laptop") and is the foundation for admin promotion, "my calendars", and notifications.

Anonymous joining stays the default — signing in is optional, and joining without an account works exactly as it does today.

## What changes

Today, participants are anonymous — a name + colour with no connection to authenticated users. After this work:

1. Participants have an optional `user_id` column
2. When a signed-in user joins a calendar, their participant is automatically linked to their user account
3. When an anonymous user signs in later, they can claim their existing participant (matched via localStorage participant ID + session)
4. A signed-in user visiting a calendar they already joined (on another device) is automatically reconnected to their participant — no re-join needed

## Design Decisions

**Optional `user_id`, not required.** Most joiners are anonymous. The column is nullable. The unique constraint `(calendar_id, user_id)` prevents one user from having two participant records in the same calendar. Anonymous participants (NULL user_id) are unaffected by this constraint.

**Auto-link on join if signed in.** If the user has a session cookie when they `POST /calendars/{id}/participants`, the backend sets `user_id` automatically. The frontend doesn't need to send the user ID — the backend reads it from the session. This prevents spoofing.

**Claim flow for "signed in after joining."** A user joins anonymously (gets participant ID in localStorage). Later they sign in. The frontend calls a new `POST /calendars/{id}/participants/{participantId}/claim` endpoint with their session cookie. The backend verifies the participant exists and has no `user_id` yet, then sets it. This is a one-time operation.

**Auto-reconnect on page load.** When a signed-in user loads a calendar page, the backend response includes participant records with `user_id`. The frontend checks if any participant's `user_id` matches the current user — if so, it selects that participant automatically. This handles the "joined on phone, viewing on laptop" case without the user doing anything. localStorage is still written as a fallback for anonymous users and as a fast-path before auth is checked.

**Name from user account.** When a signed-in user joins, the participant name defaults to their account name (from Google/magic link). They can override it in the join form. The join form pre-fills with the user's name when signed in.

**Owner auto-joins.** The calendar creator is already signed in (required for creation). They should automatically become a participant in their own calendar. Today the owner has to manually join — after this change, `POST /calendars` also creates a participant linked to the owner. This removes a friction point.

## Database

Migration `005_participant_user_linking.sql`:

```sql
ALTER TABLE participants ADD COLUMN user_id TEXT REFERENCES users(id) ON DELETE SET NULL;
CREATE UNIQUE INDEX idx_participants_calendar_user ON participants(calendar_id, user_id) WHERE user_id IS NOT NULL;
```

`ON DELETE SET NULL` — if a user account is deleted, their participant records become anonymous rather than disappearing. Their availability data is preserved.

The partial unique index (`WHERE user_id IS NOT NULL`) allows multiple anonymous participants (NULL user_id) in the same calendar while preventing one user from joining twice.

## API Changes

### Modified endpoints

| Method | Path | Change |
|--------|------|--------|
| POST | `/calendars/{id}/participants` | If session cookie present, sets `user_id` on the participant. Returns 409 if user already has a participant in this calendar. |
| POST | `/calendars` | Also creates a participant for the owner (linked to their user ID). Returns the participant in the response. |
| GET | `/calendars/{id}` | Participant objects now include `userId` field (nullable). |

### New endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/calendars/{id}/participants/{participantId}/claim` | Required | Link an anonymous participant to the authenticated user. Returns 409 if participant already linked or user already has a participant in this calendar. Returns 403 if participant not found or doesn't belong to this calendar. |

### Response changes

Participant JSON gains an optional field:
```json
{
  "id": "abc123",
  "calendar_id": "xyz789",
  "name": "Simon",
  "colour": "#E63946",
  "user_id": "usr456"   // null for anonymous participants
}
```

## Backend Implementation

### Phase 1: Migration + store

1. **Migration** — `005_participant_user_linking.sql` as above
2. **Model** — add `UserID *string` (pointer for nullable) to `model.Participant`, update JSON tag: `json:"user_id,omitempty"`
3. **Store updates:**
   - `AddParticipant(ctx, calendarID, name, userID *string)` — accepts optional user ID
   - `ClaimParticipant(ctx, calendarID, participantID, userID string) error` — sets user_id where it's currently NULL, returns error if already claimed or user already in calendar
   - Update `getParticipants` and `GetParticipant` queries to include `user_id`

### Phase 2: Middleware

Add `optionalAuth` middleware to `internal/server/middleware.go` — runs the same session check as `requireAuth` but doesn't 401 on failure, just continues without attaching a user to the context. This was planned in the auth design but never built.

Apply `optionalAuth` to `POST /calendars/{id}/participants` so the handler can read the user if present.

### Phase 3: Service + handler updates

1. **AddParticipant** — if user is in context, pass user ID to store. If user already has a participant in this calendar, return 409.
2. **ClaimParticipant** — new service method + handler. Validates ownership, calls store.
3. **CreateCalendar** — after creating the calendar, also create a participant for the owner (name from user account, linked user ID). Return both calendar and participant in response.
4. **GetCalendar** — no code change needed, just the model update propagates the new field.

### Phase 4: Routes

```go
// Update existing — wrap with optionalAuth
s.mux.HandleFunc("POST /calendars/{id}/participants", s.optionalAuth(s.handler.AddParticipant))

// New
s.mux.HandleFunc("POST /calendars/{id}/participants/{participantId}/claim", s.requireAuth(s.handler.ClaimParticipant))
```

## Frontend Implementation

### Phase 1: Types + API client

1. Update `Participant` type — add `userId?: string`
2. Update `Calendar` type or `CalendarWithDetails` — the create response now includes the owner's participant
3. Add `claimParticipant(calendarId, participantId): Promise<Participant>` to API client

### Phase 2: Auto-link on join

Update `CalendarPageClient` join flow:
- When user is signed in and joins, the backend auto-links. No frontend change needed beyond the backend doing it.
- Pre-fill the join form name with `user.name` when signed in (the join form already accepts `defaultName`).

### Phase 3: Auto-reconnect

Update `CalendarPageClient` load flow:
- After fetching calendar data, if user is signed in, check if any participant has `userId === user.id`
- If found, select that participant (same as the current localStorage path)
- This takes priority over localStorage — the user_id match is authoritative

### Phase 4: Claim flow

After the calendar loads, if:
- User is signed in, AND
- No participant has `userId === user.id`, AND
- localStorage has a participant ID for this calendar, AND
- That participant exists in the response with no `userId`

Then automatically call `claimParticipant`. This silently links the anonymous participant to the user account. On success, update local state. On failure (already claimed), clear localStorage and show the join form.

### Phase 5: Owner auto-join

Update the create calendar flow:
- The `POST /calendars` response now includes the owner's participant
- After creation, store the participant in localStorage and redirect to the calendar page
- The calendar page load will find the participant via user_id match

## Edge Cases

**User joins calendar A anonymously, then signs in, then joins calendar B.** Calendar B auto-links. Calendar A gets claimed on next visit (Phase 4 claim flow).

**Two people share a device.** Person A joins, stored in localStorage. Person B signs in and visits. Person B's user_id doesn't match any participant, localStorage participant doesn't match their user — they see the join form. Person A's localStorage entry is stale but harmless (participant already has no user_id, claim would fail if attempted by wrong user since the backend checks the session).

**User joins on two devices anonymously, then signs in.** First device claim succeeds. Second device's participant is orphaned (different participant ID). This is acceptable — the user just has their old anonymous participant sitting there. The claim endpoint returns 409 ("you already have a participant in this calendar"), and the frontend falls through to the auto-reconnect path (finds the linked participant by user_id).

## Testing

### Backend
- Store: add participant with user_id, add participant without user_id, claim participant, claim already-claimed (error), user joins same calendar twice (error)
- Service: add participant with auth context, add participant without auth, claim flow
- Handler: 200/201 success paths, 409 duplicate user, 403 wrong participant
- Integration: full flow — create calendar (auto-joins owner), anonymous join, sign in, claim, verify user_id set

### Frontend
- Join form pre-fills name when signed in
- Auto-reconnect finds participant by user_id
- Claim flow triggers for anonymous-then-signed-in user

## Build Order

1. Backend Phase 1 (migration + store) — foundation
2. Backend Phase 2 (optionalAuth middleware)
3. Backend Phase 3 (service + handler updates)
4. Backend Phase 4 (routes)
5. Backend tests
6. Frontend Phase 1 (types + API client)
7. Frontend Phases 2–4 (auto-link, reconnect, claim)
8. Frontend Phase 5 (owner auto-join)
9. Frontend tests
10. Manual QA — test on phone: create → join → sign in on laptop → verify reconnect

## Log

- 2026-04-19 21:45 — Created plan
- 2026-04-19 22:20 — Implemented all phases. Migration 007 adds nullable user_id to participants with partial unique index. Store: AddParticipant accepts optional userID, new ClaimParticipant method, getParticipants scans user_id. New optionalAuth middleware. Service/handler: AddParticipant auto-links when signed in, new ClaimParticipant handler, CreateCalendar auto-joins owner as participant. Routes: POST /participants wrapped with optionalAuth, new POST .../claim with requireAuth. Frontend: Participant type has userId, new claimParticipant API function, CalendarPageClient resolves participant by userId match first (auto-reconnect), claim flow for anonymous-then-signed-in, pre-fills join form with user name, create flow stores owner participant in localStorage. Updated all existing backend tests (unit + integration) for new signatures. All tests pass (backend unit, frontend 100/100, oxlint clean).
- 2026-11-23 20:52 — Superseded: the claim flow no longer trusts an empty `user_id` alone; `ClaimParticipant` now requires the participant's token (`backend/internal/service/service.go`, `participant_auth.go`).
