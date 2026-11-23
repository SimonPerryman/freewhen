# Leave (member) + Delete (owner) calendar

Status: done

## Goal

Finish phases 3 + 4 of `plans/ui/calendars-list-overflow-menu.md`:

- **Member**: Leave a calendar from the row overflow menu (`⋯ → Leave`).
- **Owner**: Delete a calendar from the row overflow menu (`⋯ → Delete`).

Both are destructive, both should feel instant (toast + undo), and both must
behave coherently when other tabs / other participants still have the
calendar open.

## Principles

- **Soft delete always.** Never `DELETE FROM` a calendar or a participant
  link. Set `deleted_at` (UTC) on the row and filter on read. This keeps
  audit history, supports undo, and lets us "restore" if a member rejoins
  via a fresh invite.
- **Idempotent.** Leaving a calendar you've already left, or deleting one
  that's already deleted, returns 204 — not 404, not 409. The client
  shouldn't have to care about races.
- **Soft-deleted state is invisible.** From the perspective of a user who
  has left (or a calendar that's been deleted), the rest of the system must
  behave as if the row never existed: no listing, no notifications, no
  pushes, no email digests, no nudge cron picking it up.
- **Rejoin = clean slate.** If a member leaves and is later re-invited (or
  an owner restores via support), they must not see their old availability,
  notes, or comments resurface. Re-joining mints a *new* participant row;
  the old one stays soft-deleted and inert. (We're not building a "restore"
  UI for owners in this pass — soft delete is for safety/undo, not for
  user-facing recovery beyond the toast window.)

## Plan

### Schema

New migration `026_soft_delete_calendars_and_participants.sql`:

- `ALTER TABLE calendars ADD COLUMN deleted_at TIMESTAMPTZ NULL;`
- `ALTER TABLE participants ADD COLUMN deleted_at TIMESTAMPTZ NULL;`
- Partial indexes on `(deleted_at IS NULL)` for the hot read paths
  (`calendars_owner_active_idx`, `participants_calendar_active_idx`,
  `participants_user_active_idx`) so the live-only filters stay cheap.

**Unique constraints — must be rewritten as partial indexes on
`deleted_at IS NULL`, otherwise rejoin will fail with a constraint
violation when a soft-deleted row collides with the new one:**

- `participants (calendar_id, name)` UNIQUE (from `001_create_tables.sql`).
  Drop the table-level constraint, replace with
  `CREATE UNIQUE INDEX participants_calendar_name_active_idx
   ON participants (calendar_id, name) WHERE deleted_at IS NULL;`
- `idx_participants_calendar_user (calendar_id, user_id) WHERE user_id
   IS NOT NULL` (from `007_participant_user_linking.sql`). Replace with
  `… WHERE user_id IS NOT NULL AND deleted_at IS NULL;`
- `calendar_invites_calendar_lower_name_idx` (from
  `015_add_calendar_invites.sql`) — if invites get a `deleted_at` for
  parity, same treatment. (Out of scope for this plan unless we discover
  invites need soft-delete too; flag in implementation.)

Availability, notes, and comments are keyed on `participant_id` (a fresh
UUID per join), so a rejoin doesn't collide on those tables — but
double-check each unique index in the audit pass to be sure.

Every existing query in `internal/store/` that reads calendars or
participants must add `AND deleted_at IS NULL` (or `WHERE c.deleted_at IS
NULL` on joins). Audit each query — this is the bulk of the work and the
biggest source of leak risk.

### Backend — Leave (member)

- New endpoint: `DELETE /calendars/{id}/participants/me`
  (cleaner than reusing the owner-only `…/participants/{participantId}`
  path, and avoids needing `myParticipantId` in the DTO).
- Auth: any authenticated participant of the calendar.
- Behaviour:
  - Soft-delete *all* live participant rows for `(calendar_id, user_id)` —
    a user with multiple participant rows on the same calendar (legacy
    duplicates from the participant↔user linking work) should leave fully.
  - Idempotent: 204 whether or not any rows were affected.
  - Owner cannot leave their own calendar (must Delete instead) → 409
    `cannot_leave_owned_calendar`.
- Add `myParticipantId` is **not** needed any more (endpoint is `…/me`).
- Audit log entry: `participant.left`.

### Backend — Delete (owner)

- New endpoint: `DELETE /calendars/{id}`.
- Auth: owner only. Non-owner → 403.
- Behaviour:
  - Set `calendars.deleted_at = now()`. Don't touch participant rows,
    availability, comments — they're already filtered via the calendar's
    `deleted_at`.
  - Idempotent: 204 if already deleted.
- Cancel any in-flight side effects keyed off the calendar:
  - `last_nudge_at` cron / nudges (see migration 025) must skip
    soft-deleted calendars.
  - Notifications MVP (`plans/v2/t2/notifications-mvp.md`) — when wiring
    delivery, filter on `c.deleted_at IS NULL` at the producer. Add a
    test that a deleted calendar produces no notifications even if an
    update would otherwise fire.
- Audit log entry: `calendar.deleted`.

### Backend — stale write protection

A user can have a tab open on a calendar that's just been deleted (or one
they've just been removed from / left from another device). Today, mutations
would either succeed silently or 500. Define a single shape for "this
calendar is gone for you":

- All mutating handlers on `/calendars/{id}/…` resolve the calendar+
  participant context first. If the calendar is soft-deleted, **or** the
  caller has no live participant row, return **410 Gone** with
  `{ "error": "calendar_unavailable" }`.
  - 410 (vs 404) tells the frontend "you knew about this, but it's gone" —
    distinct from a typo'd ID.
  - Applies to: availability upserts, comment posts, nudge sends,
    rename, finalise, participant management.
- The frontend's API client maps 410 on calendar routes to a single
  `CalendarUnavailableError`. The page-level handler shows a toast
  ("This calendar is no longer available") and routes back to
  `/calendars`, evicting the local cache for that ID.
- Read endpoints (`GET /calendars/{id}`) return 404 once soft-deleted —
  the calendar genuinely doesn't exist for this user any more.

### Frontend — Leave

- Wire `RowMenu` "Leave" (member) → optimistic remove from
  `MyCalendarsList`, toast with Undo (15s window).
- Undo path: re-POST a join via the existing invite/participant flow? No —
  cleaner to issue an `Undo` token: keep the optimistic-remove local for
  15s; if Undo fires, *don't* call the API at all. If 15s elapses, fire
  `DELETE …/participants/me`. (Matches the toast pattern in AGENTS.md.)
- After confirmed leave, the row is gone from `/calendars` and visiting
  the calendar URL directly hits the 410 path → toast + redirect.

### Frontend — Delete

- Wire `RowMenu` "Delete" (owner) → optimistic remove + toast with Undo
  (same 15s deferred-fire pattern).
- After confirmed delete, calendar URL → 410 → toast + redirect.
- For other participants currently on the calendar page (different device
  / tab), their next mutation will 410 → same toast + redirect. This is
  acceptable for v1; live push of "owner deleted this" is out of scope
  (covered by `live-updates.md` when that lands).

### Architectural concerns to honour

These are the load-bearing invariants that the implementation must respect.
Calling them out explicitly so they don't get lost in handler-by-handler
work.

**TOCTOU on the live-calendar precheck.** The "is this calendar live for
me" check must run **inside the same transaction** as the mutation, not
as a separate `SELECT … then UPDATE …`. Concretely: prefer
`UPDATE … WHERE id = $1 AND deleted_at IS NULL RETURNING …` and treat
zero rows updated as 410. Same for participant-scoped writes (join
participants and require `participants.deleted_at IS NULL`). Codify in
the store layer so handlers can't accidentally split the check.

**Aggregates and counts.** `myCalendarDTO.ParticipantCount`, any "X out
of N said yes" rollups, and any analytics counts will silently include
soft-deleted rows unless every aggregate adds `deleted_at IS NULL`.
These don't fail loudly — they just under/over-report. Audit aggregates
as a distinct pass from read filters.

**Already-enqueued side effects.** Notifications, nudge cron, email
digests, web-push dispatch — filtering at *enqueue* time isn't enough.
Items already in flight will resolve against a soft-deleted calendar.
Re-check `calendar.deleted_at IS NULL` and the recipient's
`participant.deleted_at IS NULL` at **delivery** time. Same for any
push-subscription send loop.

**Migration ordering for the unique-index swap.** Dropping a UNIQUE
constraint on populated `participants` and recreating it as a partial
index needs the right order to keep uniqueness enforced throughout:

1. `CREATE UNIQUE INDEX CONCURRENTLY participants_calendar_name_active_idx
    ON participants (calendar_id, name) WHERE deleted_at IS NULL;`
2. Then drop the original constraint.

The naive `ALTER TABLE … DROP CONSTRAINT … ADD …` order leaves a window
with no uniqueness. Migrations run on boot single-threaded so the
practical risk on Neon is small, but be deliberate.

**Frontend cache staleness for passive viewers.** The 410 path catches
*writes* on a calendar deleted from under the user, but a passive viewer
just sees stale data until they refetch. v1 mitigation: refetch the
calendar on tab focus / `visibilitychange`. Live push of "this calendar
was deleted" is out of scope and lives in `live-updates.md`.

**GDPR — redaction in place, not hard purge.** Soft delete keeps PII
(names, emails, availability notes, comments) in the database. Rather
than building a hard-purge job, treat right-to-be-forgotten as a
*redaction* operation that nulls/replaces PII fields while leaving the
row and its FKs intact:

- `participants`: `name = 'Removed'`, `email = NULL`.
- `availability.note`: `NULL`.
- `comments.body`: `'[removed]'`.
- `users` (on full-account erasure): `email = NULL`, `name = 'Removed'`,
  set `redacted_at`.

Routine soft-delete (Leave / Delete-calendar) does **not** trigger
redaction — the data is already invisible to live users, and we'd
break Undo. Redaction is a separate, explicit pathway tied to an
erasure request. Out of scope to build the request flow in this plan;
in scope to make sure the schema/columns we add don't make redaction
harder later (e.g., don't add `NOT NULL` constraints to PII fields
without a default-redacted value).

**Ownership transfer assumption.** "Owner can't leave, must Delete" is
correct only as long as ownership transfer doesn't exist. If/when we
add it, the Leave-vs-Delete branch on `myRole` needs to grow a third
case ("owner with co-owners → can leave"). Note in code so the next
person sees it.

**No reliance on FK CASCADE.** Under soft-delete we never hard-delete a
calendar, so any `ON DELETE CASCADE` paths from `calendars` to children
become dead code in the normal flow. Audit FKs to confirm nothing
*today* assumes a hard-delete cascade we'd be silently breaking by
switching to soft-delete only (e.g., a cleanup job that hard-deletes
empty calendars).

### Files to touch

- `backend/internal/store/migrations/026_*.sql` — new
- `backend/internal/store/calendar.go`, `participant.go`, etc. — add
  `deleted_at IS NULL` filters; add `SoftDeleteCalendar`,
  `SoftDeleteParticipantsForUser`.
- `backend/internal/handler/calendar.go` — `DeleteCalendar`,
  `LeaveCalendar` (`…/participants/me`); 410 helper for stale writes.
- `backend/internal/server/server.go` — route registration.
- `frontend/src/lib/api.ts` — `leaveCalendar(id)`, `deleteCalendar(id)`,
  `CalendarUnavailableError`.
- `frontend/src/components/RowMenu/RowMenu.tsx` — enable Leave + Delete.
- `frontend/src/components/MyCalendarsList/MyCalendarsList.tsx` —
  optimistic-remove + Undo wiring.
- Calendar page error boundary — handle `CalendarUnavailableError`.

### Tests

- Integration: leave is idempotent; owner cannot leave; delete is owner-only
  and idempotent; deleted calendar returns 404 on GET and 410 on writes;
  participants of a deleted calendar get 410 on writes; left participant
  gets 410 on writes; nudge cron skips soft-deleted calendars; rejoining
  after leave starts with empty availability/notes (old soft-deleted rows
  not exposed).
- Unit (frontend): RowMenu shows Leave for members and Delete for owners;
  toast Undo cancels the API call; on 410 the page redirects and toasts.

## Phasing

1. Migration + read-path filters + integration tests proving soft-delete
   invisibility (no UI yet).
2. `DELETE /calendars/{id}` + 410 stale-write helper across mutating
   handlers + frontend `CalendarUnavailableError`.
3. Owner Delete in `RowMenu` with toast/undo.
4. `DELETE /calendars/{id}/participants/me` + member Leave in `RowMenu`.
5. Notifications/nudge producers updated to skip soft-deleted calendars
   (coordinate with `notifications-mvp.md`).

## Log

- 2026-05-10 — Created plan.
- 2026-05-10 17:00 — Implemented all five phases on `leave-and-delete-calendar`:
  migration 026 (deleted_at + partial unique indexes); store/service/handler
  filters across every calendar/participant query; new `DELETE /calendars/{id}`
  + `DELETE /calendars/{id}/participants/me` endpoints; 410 Gone helper for
  mutating handlers (calendars/participants/availability/invites/passphrase/
  finalise/notifications); RowMenu Leave/Delete with optimistic-remove +
  toast Undo (15s deferred fire); `CalendarUnavailableError` mapped at the
  api client; `CalendarPageClient` toasts + redirects on 410. Notifications
  re-check `IsCalendarSoftDeleted` at delivery time. New integration tests
  in `store/soft_delete_test.go` and `server/server_softdelete_integration_test.go`.
