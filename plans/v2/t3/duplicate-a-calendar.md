# Duplicate a Calendar

Status: done

## Goal

Let a signed-in owner create a new calendar that reuses the bones of an existing one — "Tennis last month worked well, same group, new dates" — in a single tap, without having to re-type the name, re-pick an emoji, re-set visibility, or re-share fresh with the group. This is the Tier 3 "Duplicate a calendar" item in `docs/v2-priorities.md:38`.

The scenario we're optimising for: a recurring-ish activity (weekly tennis, monthly book club, quarterly team offsite) where the *group* is stable but each *event* is a new availability question. Today the owner has to create a new calendar from scratch and then re-send the link to everyone; the group loses the continuity of "this is the same crew" and the owner does busy work.

## Scope

**In**
- `POST /calendars/{id}/duplicate` — owner-only endpoint that creates a new calendar derived from the source.
- Copy: `name` (with "(copy)" suffix unless the client overrides), `emoji`, `visibility`. Owner is auto-joined as participant (same as `POST /calendars`).
- Optional `include_participants: bool` flag (default `true`). When true, every non-owner participant row on the source is recreated on the new calendar as a **stub participant** — same `name`, new `id`, `user_id` preserved if present. No availability copied.
- Do **not** copy: availability, finalised state (`finalised_date`/`finalised_at`), passphrase, `access_epoch`, notes, `created_at`, `updated_at`.
- Enforce the per-owner calendar cap (`limits.MaxCalendarsPerUser = 200`) — duplication is just another create.
- Enforce the per-calendar participant cap (`limits.MaxParticipantsPerCalendar = 100`) on the copied participant set.
- Frontend: "Duplicate" action in the owner-only calendar page menu, and on each owner row in `/calendars` (MyCalendarsList) as a secondary action. Both navigate to the new calendar on success.
- `include_participants` exposed as a checkbox in a small confirm sheet before duplication runs.

**Out**
- Copying availability. The whole point of duplicating is "new event"; carrying over stale availability would confuse everyone.
- Copying finalised/passphrase/access state. A duplicate is a fresh coordination; it starts open and unfinalised. Passphrase, if set on the source, is a privacy choice the owner can re-apply manually.
- Notifying copied participants. We can't (we don't have contact details for anonymous joiners) and we shouldn't (silent multi-calendar notifications are a worse product than a fresh link in the group chat). The owner re-shares the link.
- Duplicating as a non-owner. "Fork" semantics are out of scope; the only sanctioned copy path is owner-initiated.
- Recurring calendars / templates. This is the *manual one-shot* version of that idea; if the pattern takes off, the natural follow-up is scheduled auto-duplicate. Not this plan.
- Copying from a finalised or locked source is allowed — duplication is a pure read of the source row. We don't need to treat finalised sources specially.

## Design Decisions

**Server-side copy, single endpoint.** The alternative — frontend reads the source calendar and POSTs a new one plus N `AddParticipant` calls — is racy (a participant could join mid-duplicate and end up half-copied), slow on mobile (N+1 network roundtrips), and leaks the stub-participant model to the client. A server endpoint lets us do the whole thing in one transaction and keep the client logic to "tap duplicate → navigate."

**Stub participants, not claims.** Copied non-owner participants are inserted as plain rows: same `name`, same `user_id` where present, new `id`. They look to the source participants exactly like a fresh calendar someone already put them on — because, functionally, that's what it is. Signed-in participants who revisit the duplicate via the shared link will auto-match on `user_id` via the existing linking behaviour. Anonymous participants will need to re-find "their" row by name (this is the same behaviour as joining any open calendar today); we won't try to be clever about deduping by name.

**Owner is deduped explicitly.** `CreateCalendar` already auto-joins the owner via `AddParticipant` (see `handler/calendar.go:59`). When copying participants we must skip any source participant whose `user_id` matches the duplicator's user ID, otherwise the owner ends up as two participants on the new calendar. Match on `user_id`, not name — name collisions between owner and someone else are fine to preserve.

**No "duplicate with availability" escape hatch.** Someone will ask for it. The answer is no for v1 because:
- The most frequent intent is "new event" — stale availability is noise.
- If we ship both options we're inviting confusion ("wait, is this the old one?"). Keep the primitive clean; add a separate "recurring" feature later if the signal says so.
- If a user genuinely wants a snapshot, they can keep the original. Calendars are cheap.

**`include_participants` default = true.** The whole value proposition is "same group, new event." Defaulting to false would make duplicate no better than create-from-scratch for the modal use case. We still expose the toggle so an owner can say "actually this is just a similar calendar for a different group" without having to delete participants afterwards.

**Name suffix is a server-side default, overrideable.** If the client sends no `name` field, the server uses `source.name + " (copy)"` (clipped to `MaxCalendarNameLength`). If the client sends `name`, that wins. This keeps the happy path ("I just want a copy") zero-fields on the wire and lets the UI offer a rename-before-duplicate flow later without an API change. We don't auto-deduplicate "(copy) (copy) (copy)" — the user can rename after.

**Single transaction for atomicity.** The server does `INSERT calendars` + N `INSERT participants` in one transaction. If any participant insert fails (e.g. we hit the cap — see below), the whole thing rolls back. No partial duplicates.

**Cap enforcement: fail fast.** Check `CountCalendarsForOwner >= MaxCalendarsPerUser` before inserting (same as `service.CreateCalendar`). Check `len(sourceParticipants) > MaxParticipantsPerCalendar` before inserting participants (shouldn't happen given the source was under the cap when built, but the cap could tighten later — cheap to be defensive). No partial participant copy.

**Finalised source is fine to copy.** We intentionally don't block duplicating a finalised calendar — "the tennis we nailed down for April worked great, let's do it again in May" is the textbook use case. We just don't carry the `finalised_date` forward.

**Copy participants does not bump source `updated_at`.** Duplication is a read of the source. The new calendar has its own `updated_at = now()` by default.

**Authorization = source owner only.** Match existing mutation handlers (`UpdateCalendar`, `FinaliseCalendar`): load source with `GetCalendarOnly`, 404 if missing, 403 if `OwnerID != user.ID`. No admin/participant-can-duplicate path.

**Passphrase is dropped, not carried.** If the source has a passphrase, the duplicate does not. Reasoning: the owner probably wants the same group to frictionlessly land on the new calendar via the freshly-shared link; re-gating them behind a passphrase defeats the "save me work" framing. If they want privacy on the duplicate, they set a passphrase there. Flagged in Open Questions in case that's the wrong call.

**Response shape mirrors `POST /calendars`.** Same DTO: `Calendar` + optional `Participant` for the owner's auto-join. This lets the frontend reuse the existing create-and-redirect flow verbatim. Clients that want the participant count on the duplicate re-fetch via `GET /calendars/{id}`.

**Analytics: track source → target.** `calendar_duplicated` event with `{ source_id, target_id, copied_participant_count, include_participants }`. Lets us measure whether duplicate is used primarily with participants-on (expected) or participants-off (means the feature is really "template"), and informs whether to ever build recurring.

## Database

No migration required. All columns already exist; we're just doing `INSERT ... SELECT`-shaped copies via the existing store methods (possibly with a new transactional store method — see Backend Implementation).

## API

### New endpoint

`POST /calendars/{id}/duplicate` — `requireAuth`, owner-only.

Request body (all fields optional):

```json
{
  "name": "Tennis May",
  "include_participants": true
}
```

- `name` — optional override. If omitted or empty, server uses `<source.name> (copy)` clipped to `MaxCalendarNameLength`.
- `include_participants` — optional, defaults to `true`. When `false`, only the owner ends up on the new calendar.

Response `201 Created`, shape identical to `POST /calendars`:

```json
{
  "id": "abc12345",
  "name": "Tennis May",
  "emoji": "🎾",
  "visibility": "open",
  "created_at": "...",
  "updated_at": "...",
  "owner_id": "...",
  "participant": { "id": "...", "name": "Simon", "colour": "...", "userId": "..." }
}
```

Errors:
- `401 unauthorized` — not signed in.
- `403 forbidden` — signed in but not the source owner.
- `404 not_found` — source calendar missing.
- `409 calendar_limit_reached` — owner at `MaxCalendarsPerUser`.
- `400 name_too_long` — override name longer than `MaxCalendarNameLength`.
- `500 internal_error` — transaction failure.

No other endpoints change.

## Backend Implementation

### Phase 1 — store method

Add `store.DuplicateCalendar(ctx, sourceID, newOwnerID, overrideName string, includeParticipants bool) (model.Calendar, model.Participant, error)`:

1. Open a transaction.
2. Read source via `getCalendarRow` (re-use helper inside the tx).
3. Generate a new ID (reuse `generateID(8)` + unique-violation retry loop — we can't naively call `CreateCalendar` because we want the whole thing under one tx, so either:
    - Expose a transactional variant of `CreateCalendar` (`createCalendarTx(ctx, tx, ...)`) that the public method also uses, or
    - Inline the insert with the same retry pattern here.

    Prefer the first: `createCalendarTx` becomes the shared primitive, existing `CreateCalendar` becomes a one-line wrapper (`BEGIN / createCalendarTx / COMMIT`). Low risk, cleans up duplication.
4. Compute the name: if `overrideName` is non-empty, use it; else `source.Name + " (copy)"`, clipped to `limits.MaxCalendarNameLength`. (The clip is in the service layer; the store trusts callers.)
5. Insert the new calendar with source's `emoji`, `visibility`, new `owner_id`.
6. Insert the owner's participant row (same name the handler would supply — falls back to `user.Name`).
7. If `includeParticipants`, read `getParticipants(ctx, sourceID)` inside the tx. For each source participant whose `user_id != newOwnerID` (nil-safe), insert a new participant on the new calendar with a fresh ID, same name, same `user_id`, colour auto-assigned by existing insert logic.
8. Return the new calendar + the owner's participant row.

Unique-violation retries apply only to the calendar ID insert, not participants (participant IDs are generated fresh and collisions are vanishingly unlikely; the existing `AddParticipant` already handles that).

### Phase 2 — service method

`service.DuplicateCalendar(ctx, sourceID, newOwnerID, ownerName, overrideName string, includeParticipants bool) (model.Calendar, model.Participant, error)`:

1. Validate `overrideName` length (if provided): reject > `MaxCalendarNameLength` with `ErrInvalidName`.
2. Check `CountCalendarsForOwner(newOwnerID) >= MaxCalendarsPerUser` → `ErrCalendarLimitReached`.
3. Load source via `GetCalendarOnly`. If not found → `ErrNotFound`. (Ownership check happens in the handler — service stays composable.)
4. Compute participant count: if `includeParticipants`, count source participants minus owner and check against `MaxParticipantsPerCalendar`. If over, return `ErrCalendarFull`.
5. Call `store.DuplicateCalendar`.
6. Publish `EventCalendarCreated` (or whatever new event constant fits — probably none needed since nobody's subscribed to the *new* calendar yet).
7. Return `(calendar, ownerParticipant, nil)`.

No `TouchCalendar` bump on the new calendar — the INSERT already sets `updated_at = now()` via default.

### Phase 3 — handler

`handler.DuplicateCalendar(w http.ResponseWriter, r *http.Request)`:

1. `requireAuth` gate (wired in router).
2. Decode body: `{ name?: string, include_participants?: bool }`. Missing body is OK (treat as `{}`). Default `include_participants = true`.
3. Load source (`GetCalendarOnly`). 404 if missing.
4. Ownership check: `source.OwnerID != user.ID` → 403.
5. Call `service.DuplicateCalendar(ctx, sourceID, user.ID, user.Name, req.Name, req.IncludeParticipants)`.
6. Map errors: `ErrCalendarLimitReached → 409`, `ErrCalendarFull → 409`, `ErrInvalidName → 400`, else 500.
7. Respond 201 with the same DTO shape as `CreateCalendar` (embed owner participant).

Wire route in `server.go`:

```go
s.mux.HandleFunc("POST /calendars/{id}/duplicate", s.requireAuth(s.handler.DuplicateCalendar))
```

### Phase 4 — tests

- Store: source with 0 / 1 / N participants, `includeParticipants` true/false, override name honoured, emoji/visibility/owner carried through, availability NOT copied, finalised fields NOT copied, passphrase NOT copied, owner deduped when they were in the source participant list, transaction rollback on forced failure (inject a bad query or use a cancelled context partway through).
- Service: cap errors propagate (`MaxCalendarsPerUser`, `MaxParticipantsPerCalendar`), override-name length validation, source-not-found maps correctly.
- Handler: 401 anon, 403 non-owner, 404 missing source, 409 cap, 400 name-too-long, 201 happy path with default name and with override, `include_participants=false` yields a 1-participant new calendar.
- Integration: end-to-end POST against real Postgres; follow-up `GET /calendars/{newID}` returns expected shape; original calendar unchanged; a `GET /users/me/calendars` lists both source and duplicate with duplicate on top (new `updated_at`).

## Frontend Implementation

### Phase 1 — API client + types

1. `src/lib/api.ts`: `duplicateCalendar(id: string, opts?: { name?: string; includeParticipants?: boolean }): Promise<CalendarCreateResponse>` — same return type used by `createCalendar`. snake/camel conversion via the existing helper.
2. No new types beyond a small `DuplicateCalendarRequest` interface.

### Phase 2 — entry point on `/calendars/[id]`

In `CalendarPageClient.tsx`, when `isOwner` is true, add a "Duplicate" action. The cleanest home is the existing owner controls area that already houses Rename and Finalise. Add a button that opens a small sheet (reuse the Toast/Sheet primitives used by DayDetailSheet if appropriate; otherwise a simple modal).

Sheet contents:
- Title: "Duplicate this calendar"
- Input (pre-filled): `"<source name> (copy)"`, editable. Empty falls back to server default.
- Checkbox: "Copy participants" (default checked). Helper text: "Everyone on this calendar will be added to the new one. They'll need to re-mark availability."
- Buttons: Cancel, Duplicate (primary, disabled while submitting).

On submit:
- Call `duplicateCalendar(id, { name, includeParticipants })`.
- On success, `router.push('/calendars/' + newId)`.
- On `calendar_limit_reached`: inline error "You've reached the calendar limit."
- On other errors: inline error with the API message + retry.
- Emit `calendar_duplicated` analytics event.

### Phase 3 — entry point on `/calendars` (MyCalendarsList)

Each owner row gets a secondary action. On mobile, a small "⋯" menu button in the row (sibling to the card link, not inside it — cards are already links). Menu items:
- Duplicate → opens the same sheet as Phase 2 (extract to a shared `DuplicateCalendarSheet` component).

Keep joiner rows unchanged (no duplicate affordance for non-owners).

If we decide the menu is scope creep for this plan, Phase 3 can ship later — the in-calendar entry point from Phase 2 covers the main path. Flagged in Open Questions.

### Phase 4 — Analytics

- `calendar_duplicated` `{ source_id, copied_participant_count, include_participants }` on success.
- `calendar_duplicate_failed` `{ source_id, reason }` on error (reason = the API error code).

### Phase 5 — tests

- `DuplicateCalendarSheet` component: default name prefill, toggle checkbox, submit calls the API, navigates on success, renders inline error on failure, disables Duplicate while submitting.
- API client unit test: request body serialisation (camelCase → snake_case), response parse.
- Integration: on an owner's calendar page, clicking Duplicate → Confirm navigates to the new calendar; the new calendar renders; original calendar unchanged on back-nav.

## Testing (manual)

- As owner: open a calendar with 5 participants and some availability; duplicate with default name, `include_participants=true`. Land on new calendar; assert 5 participants present (including self), 0 availability, not finalised, same emoji and visibility.
- Same source, duplicate with `include_participants=false`. New calendar has only the owner.
- Rename during duplicate: override name is honoured.
- Duplicate a finalised calendar: new calendar is not finalised.
- Duplicate a passphrase-protected calendar: new calendar is open (no passphrase). Shared link works without unlock.
- Non-owner viewing the calendar: no Duplicate action visible.
- Anonymous user visiting `/calendars/{id}/duplicate` (shouldn't be reachable via UI): API returns 401.
- Owner at `MaxCalendarsPerUser - 1`: duplicate succeeds and moves them to the cap. Next duplicate returns 409.
- Source with 100 participants (the cap): participants copy correctly (we fit exactly). Source with 101 (shouldn't be possible, but test the error path via direct store insert): returns 409.
- `/calendars` page: owner rows show the "⋯" menu; joiner rows don't. Duplicate from the list navigates correctly.

## Build Order

1. Backend Phase 1 (store + transactional primitive refactor + tests).
2. Backend Phase 2 (service + tests).
3. Backend Phase 3 (handler + route + tests).
4. Backend integration test.
5. Frontend Phase 1 (API client).
6. Frontend Phase 2 (in-calendar entry point + `DuplicateCalendarSheet` + tests).
7. Frontend Phase 3 (MyCalendarsList entry point).
8. Frontend Phase 4 (analytics).
9. Manual QA matrix.
10. Mark Tier 3 item shipped in `docs/v2-priorities.md`.

## Open Questions

- **Passphrase carry-over.** Plan drops the passphrase on duplicate. If owners report "I duplicated my private calendar and the link is now public," we flip the default (carry passphrase forward) and add a checkbox to drop it. Cheap to reverse; defaulting to open is the friendlier bet for the "same group, new event" flow.
- **Stub participants and `user_id` carry.** For signed-in participants, copying `user_id` means they'll see the new calendar in their `/calendars` list without ever visiting it. Probably a feature (silent awareness of duplicates), possibly noisy (unexpected calendars appearing). If this becomes a complaint, add a "notify me when I'm added to a calendar" story — don't fix it by withholding the linkage here.
- **MyCalendarsList duplicate menu scope.** Could defer Phase 3 to a follow-up if we want the smallest possible first ship. The in-calendar button is the main entry point; the list affordance is a QoL nicety.
- **"(copy)" suffix and internationalisation.** Hardcoded English for now, consistent with the rest of the product. When i18n arrives, this becomes a translation key — small edit, not worth structuring the code around today.
- **Rate limiting.** Owners hitting duplicate in a tight loop could mint 200 calendars in seconds. The `MaxCalendarsPerUser = 200` cap contains the blast radius; beyond that, adding a per-user duplicate rate limiter is overkill for this feature alone. Revisit if we add more creation primitives.
- **Copying notes on availability.** N/A because we don't copy availability. If we ever add an "include availability" mode (we said no above), notes would travel with it. Flagging so future-me doesn't forget.

## Log

- 2026-04-21 — Created plan.
- 2026-04-21 — Implemented end-to-end: `POST /calendars/{id}/duplicate` endpoint (store `DuplicateCalendar` under a single tx, service-level cap enforcement + name-suffix default, owner-only handler), frontend `duplicateCalendar` API client + `DuplicateCalendarSheet` modal wired into the calendar page and an owner-only "Duplicate" button on `/calendars`. Unit tests green (backend `go test ./...`, frontend `vitest run`). Integration store tests added under the `integration` build tag.
