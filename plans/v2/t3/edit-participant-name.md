# Edit Participant Name

Status: done

## Goal

Let participants fix typos in their own name, and let calendar owners clean up names for others (e.g. "Sam"/"sam"/"SAM" duplicates). Same uniqueness guarantee as joining — no two participants on a calendar can share a name.

From `docs/v2-priorities.md` (Tier 3):
> Edit participant name — Self-rename (typo fix) + owner-rename (cleanup). Same unique-name check as join. No rename limit — rate-limit server-side only if abuse shows up.

## Scope

**In scope:**
- A participant can rename themselves (the one identified by their local `participant_id` for anon joiners, or by `user_id` for linked/signed-in joiners).
- A calendar owner can rename any participant on their calendar.
- Uniqueness re-checked on rename — same per-calendar unique index the join flow already trips.
- Same length/validation as the join flow (`limits.MaxParticipantNameLength`, non-empty, trimmed).

**Out of scope:**
- Rename rate limiting — only add if abuse appears.
- Rename history / audit log.
- Admin-role rename (admins-other-than-owner don't exist yet; see `plans/v2/t1/admin-controls.md`).
- Changing participant colour (separate feature if ever needed).

## Design Decisions

**Authorization: mirror SetAvailability for self-rename, mirror RemoveParticipant for owner-rename — in a single endpoint.** The handler accepts the rename if *either* (a) the caller is the calendar owner, or (b) the caller provided a `participant_id` in the request that matches the target participant. This matches the existing "honor system" used by `PUT /calendars/{id}/availability` for anon joiners (the client asserts which participant it is; no server-side session for anon users). Signed-in joiners whose participant is linked via `user_id` get the stronger check automatically, because we compare `participant.user_id` to the session user.

**Endpoint shape: `PATCH /calendars/{id}/participants/{participantId}`.** PATCH because only the name changes. Keeps the resource hierarchy consistent with `DELETE /calendars/{id}/participants/{participantId}` from admin controls.

**No rename endpoint on a "me" route.** A dedicated `PATCH /calendars/{id}/participants/me` would require a reliable "me" resolver for anon users — we don't have one (participant_id lives in client state). The unified PATCH with body-provided participant_id keeps parity with availability writes.

**Unique-name conflict returns 409 `duplicate_name`.** Same error code the join flow emits, so the frontend can reuse its handling.

**No-op rename (same name) is a 200, not a 304.** Simpler; the user won't hit this often.

**Trim whitespace, reject empty/whitespace-only.** Same rules as join. Don't silently coerce case — "sam" and "Sam" are different names (by design, owners can clean those up).

**Linked user → name stays linked.** Renaming the participant doesn't change `user_id`. A signed-in user's display name on `users` table is separate from their participant name on this calendar.

**Finalised calendars: reject rename.** `AddParticipant` already returns `ErrCalendarFinalised`; allowing rename but not join leaves identity half-locked. Simpler rule: once finalised, the participant set and their names are frozen. If someone has a typo on finalisation day, the owner unfinalises, the rename lands, re-finalise.

**Name trimming: trim on both join and rename.** `AddParticipant` currently doesn't trim, so someone joined as `"Sam "` couldn't rename to `"Sam"` (409) without owner cleanup. Fix both paths in this change — trim before length check and before the uniqueness query. One-line amend to the join handler.

**Case sensitivity premise depends on the existing index.** The design (owners can clean up `"sam"`/`"Sam"` duplicates) assumes the unique index on `participants(calendar_id, name)` is case-sensitive. **Verify** by reading `internal/store/migrations/` before implementing — if it's `CITEXT` or `LOWER()`-functional, this plan needs to change (either drop the cleanup framing or introduce a case-insensitive rename-specific check).

**Broadcast over SSE.** Other viewers see the rename live — consistent with other mutation endpoints that publish to `realtime.Hub`.

## API

### New endpoint

| Method | Path | Auth | Description |
|---|---|---|---|
| PATCH | `/calendars/{id}/participants/{participantId}` | optionalAuth | Rename a participant. Body: `{"name": "New Name", "participant_id": "..."}` where `participant_id` is the caller's own participant (used for self-rename auth). |

### Authorization rules

Allowed if any of:
1. Caller is the calendar owner (session user id == `calendar.owner_id`).
2. `participant_id` in the body equals `{participantId}` in the path (self-rename via local participant identity).
3. Caller is signed in and the target participant's `user_id` equals the session user id (linked self-rename).

Otherwise 403.

### Responses

- 200 + updated participant on success
- 400 `invalid_json`, `invalid_request` (name empty), `name_too_long`
- 401 if a passphrase-gated calendar has no valid access cookie (via `requireCalendarAccess`, as with other mutations)
- 403 `forbidden` if none of the rules above hold
- 404 `not_found` if calendar or participant doesn't exist, or participant doesn't belong to this calendar
- 409 `duplicate_name` if another participant on this calendar already has the new name

## Backend Implementation

### Phase 1: Store

Add to `internal/store/participant.go`:

```go
// UpdateParticipantName renames a participant scoped to a calendar.
// Returns ErrNotFound if the participant doesn't exist on that calendar,
// ErrDuplicateName on unique-index conflict.
func (s *Store) UpdateParticipantName(ctx context.Context, calendarID, participantID, name string) (model.Participant, error)
```

Implementation: `UPDATE participants SET name = $1 WHERE id = $2 AND calendar_id = $3 RETURNING id, calendar_id, name, colour, user_id`. Map unique-violation → `ErrDuplicateName` (reuse `isDuplicateName`). `sql.ErrNoRows` → `ErrNotFound`.

### Phase 2: Service

Add to `CalendarService` / `internal/service/service.go`:

```go
RenameParticipant(ctx context.Context, calendarID, targetParticipantID string,
    callerUserID *string, callerParticipantID string, name string) (model.Participant, error)
```

Logic:
1. Load calendar (`GetCalendarOnly`) → on miss, `ErrNotFound`. If `calendar.FinalisedDate != nil` → `ErrCalendarFinalised` (handler maps to 409 `already_finalised`, matching `AddParticipant`).
2. Authorization (check in this order, short-circuit on first match):
   - **Owner:** `callerUserID != nil && *callerUserID == calendar.OwnerID` → allowed.
   - **Self via participant_id:** `callerParticipantID != "" && callerParticipantID == targetParticipantID` → allowed (matches the existing honor-system trust boundary of `PUT /availability` — the client asserts its own identity).
   - **Linked self:** `callerUserID != nil` → load target participant; if `target.UserID != nil && *target.UserID == *callerUserID` → allowed.
   - Otherwise `ErrForbidden`.
3. Trim the name, validate (non-empty, within `limits.MaxParticipantNameLength`). Return typed validation error → 400.
4. Call `store.UpdateParticipantName`.
5. On success, publish a participant-updated event to the hub (same pattern as other mutations).

Reuse existing `ErrForbidden`. Add a sentinel `ErrInvalidName` if one doesn't exist, or bubble up a `limits.ValidationError`-style value — follow whatever the AddParticipant flow already does when surfacing length errors (the handler currently inlines validation; we can do the same here to avoid spreading validation across service).

### Phase 3: Handler

Add to `internal/handler/participant.go`:

```go
func (h *Handler) RenameParticipant(w http.ResponseWriter, r *http.Request)
```

- `MaxBytesReader` + passphrase gate via `requireCalendarAccess`.
- Path params: `id`, `participantId`.
- Body: `{ "name": string, "participant_id": string }` (participant_id optional — if caller is the owner, they don't need it).
- Inline validation: trim, non-empty, length. Map to 400.
- Look up session user via `UserFromContext` (optionalAuth).
- Call `service.RenameParticipant`.
- Map errors: `ErrNotFound` → 404, `ErrForbidden` → 403, `ErrDuplicateName` → 409 `duplicate_name`, `ErrCalendarFinalised` → 409 `already_finalised`, else 500.
- On success, 200 + participant JSON.

### Phase 4: Route

Add to `internal/server/server.go` `routes()`:

```go
s.mux.HandleFunc("PATCH /calendars/{id}/participants/{participantId}", s.optionalAuth(s.handler.RenameParticipant))
```

`optionalAuth` (not `requireAuth`) because anon participants need to self-rename.

### Phase 5: SSE event

Emit a rename event on the hub so other tabs refresh participant lists in-place. Shape: reuse whatever the participant-delete / availability-set flow emits, or add a thin `participant.renamed` event if the hub is typed per-event. Follow the existing pattern — do not invent a new event schema if one fits.

## Frontend Implementation

### Phase 1: API client

Add to `src/lib/api.ts`:

```ts
renameParticipant(
  calendarId: string,
  participantId: string,
  name: string,
  callerParticipantId: string | null,
): Promise<Participant>
```

`PATCH /calendars/{id}/participants/{participantId}` with body `{ name, participant_id: callerParticipantId ?? "" }`. Include passphrase cookie as other mutations do. Map 409 → a typed `DuplicateNameError` the UI can catch.

### Phase 2: Self-rename UI

**"Which participant am I?" source of truth.** Before building the UI, audit where the anon participant_id is currently stored (likely localStorage keyed by calendar id — check the join flow). Whatever the answer, the rename button reads from that same source so a returning anon visitor can still self-rename after a reload. If it turns out to be in-memory-only, durability needs a fix in a sibling patch or this feature silently breaks for returning anon users.

In `ParticipantList` (or wherever the current user's row is rendered): when the row represents the caller (matched by stored `participant_id` for anon, or `user_id` for signed-in), show a small pencil affordance next to their own name. Clicking it switches the name to an inline input.

**Interaction spec (every bullet is load-bearing — these are real footguns):**

- Input autofocuses on enter-edit; selects all text.
- **Enter** → save.
- **Escape** → set a `cancelled` flag, then blur. The `blur` handler must check the flag and skip saving. Without this, Escape triggers blur which triggers save of the reverted value (noisy no-op, but worse: on mobile it races with optimistic state).
- **Blur without Escape** → save. On touch this means tapping outside commits the edit — acceptable given the "typo fix" framing. If product pushback, add a visible ✕ cancel button.
- **Touch target** ≥ 44×44px for the pencil button (mobile-first per AGENTS.md). Wrap the icon in a padded button, don't rely on the icon hitbox.
- **Focus management:** on exit (save or cancel), return focus to the pencil button.
- **Max length** enforced on the input (`maxLength={limits.MAX_PARTICIPANT_NAME_LENGTH}`) so the server's length error is unreachable via the normal path.
- **On save:** optimistic update + revert + inline error on failure. For 409, keep edit mode open with the input still focused so the user can fix the name without re-clicking the pencil. Reuse the exact copy the join flow shows for `duplicate_name` (grep the join error handling — don't invent new copy).

**SSE race during edit:** while a row is in edit mode, ignore incoming SSE participant-renamed events targeting that same participant; reconcile on save-success or cancel. Otherwise a late event replays stale state into the live input.

Same interaction pattern the rename-calendar title already uses (see `plans/v2/t1/admin-controls.md` Phase 4) — steal the component if it's reusable, but verify it already handles the Escape-vs-blur and focus-return behaviours; if not, fix them there too.

### Phase 3: Owner-rename UI

When `isOwner`, show the same pencil affordance on every participant row (not just the caller's). Shares the rename handler from Phase 2; the only difference is which `participantId` it targets. Owner doesn't need to send `participant_id` in the body, but the client can always send its own anyway — the server accepts either path.

### Phase 4: SSE handling

When a participant-renamed event arrives over the existing SSE stream, update the participant in local state. If the renamed participant is the caller, also update any "you are X" UI (e.g. the greeting on the calendar page).

### Phase 5: Tests

- `api.test.ts` — `renameParticipant` hits the right URL/body; 409 surfaces as `DuplicateNameError`.
- `ParticipantList` — pencil shows on own row always; shows on all rows when `isOwner`; hidden otherwise. Editing flow dispatches the right call.
- Inline-edit component — Escape does not save; blur saves unless Escape was pressed first; focus returns to the pencil after exit; 409 keeps edit mode open with focus preserved.
- SSE — a rename event for a non-self participant updates that row; a rename event for the caller updates the "you are X" greeting; a rename event arriving mid-edit for the row being edited is ignored until the edit resolves.
- `CalendarPageClient` — after a successful rename, displayed name updates; on 409, error surfaces inline and old name restored.

## Testing

### Backend

- Store integration: rename succeeds; rename to a taken name → `ErrDuplicateName`; rename a participant on the wrong calendar → `ErrNotFound`; rename preserves `colour` and `user_id`.
- Service unit: owner rename allowed; self rename via participant_id allowed; self rename via linked user_id allowed; stranger rename → `ErrForbidden`.
- Handler: 200 success (owner path and self path), 400 empty/too-long, 403 stranger, 404 wrong calendar, 409 duplicate.
- Integration (`server_integration_test.go`): end-to-end owner rename and self rename; verify SSE event fires (match pattern used by existing integration tests).

### Frontend

- See Phase 5 above.

## Resolved questions

- **Participant colour on rename?** Stays. Changing colour on rename would read as "someone left and a new person joined" across the cell grid.
- **Signed-in users renaming to match their `users.name`?** Allowed. Separate fields by design — display name on users vs per-calendar participant name.
- **Rename after finalisation?** Rejected (409 `already_finalised`). Keeps the finalised state fully frozen and matches `AddParticipant`.

## Pre-implementation checks

- Read `internal/store/migrations/` to confirm the `participants(calendar_id, name)` uniqueness index is **case-sensitive**. If it's `CITEXT` or `LOWER()`-functional, the "owners clean up `sam`/`Sam` duplicates" framing is dead — revisit scope.
- Locate where anon `participant_id` is stored client-side (likely localStorage). Confirm it survives reload; if not, flag as a prerequisite.
- Grep the join flow's `duplicate_name` UI copy and reuse it verbatim.

## Build Order

1. Backend Phase 1 (store) + tests.
2. Backend Phase 2 (service) + tests.
3. Backend Phase 3–4 (handler + route) + handler tests.
4. Backend Phase 5 (SSE event) + integration test.
5. Frontend Phase 1 (API client) + test.
6. Frontend Phase 2 (self-rename) + test.
7. Frontend Phase 3 (owner-rename) — mostly a UI gate flip.
8. Frontend Phase 4 (SSE wiring).
9. Manual QA on mobile: anon self-rename, signed-in self-rename, owner rename someone else, duplicate-name collision, two-tab live update.

## Log

- 2026-04-21 — Created plan.
- 2026-04-21 — Review pass: rejected rename-after-finalise; added trim-on-both-paths; called out case-sensitivity verification; tightened inline-edit spec (Escape-vs-blur, focus return, touch targets, SSE-race-during-edit); expanded frontend tests.
- 2026-04-21 14:42 — Shipped: store.UpdateParticipantName, service.RenameParticipant (owner/self-pid/linked-user auth, trim, finalised rejection, invalid-name), PATCH /calendars/{id}/participants/{participantId} route, realtime EventParticipantRenamed, trim-on-join in service.AddParticipant, renameParticipant API client, inline-edit UI in ParticipantList (pencil on own row, or every row for owner; Enter/blur save; Escape cancel; 409 keeps edit open; focus returns to pencil), wired into CalendarPageClient with optimistic update + revert. Store/service/handler/api/component tests added. `make test` + integration tests green, oxlint clean.
