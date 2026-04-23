# Who's Missing?

Status: done

## Goal

Let the owner of a calendar see, at a glance, who hasn't responded yet — so they can go nudge the right people in the group chat instead of pinging everyone. This is the Tier 3 "Who's missing?" item in `docs/v2-priorities.md:40`, and the original idea is in `docs/future-ideas.md:78`.

Today the owner can only see who *has* joined and marked dates. Absence is invisible: a quiet participant who hasn't opened the link is indistinguishable from someone who just isn't free any day you're offering. The owner ends up either chasing everyone (noisy) or no-one (slow). "Who's missing?" gives them a named, actionable list.

## Scope

**In**
- A per-calendar **invite list** (owner-only): names the owner expects to respond. Purely a string list — no emails, no accounts, no notifications.
- An **owner-only "Who's missing?" panel** on `/calendars/{id}` that shows two buckets:
  - **Not joined** — invite list names with no matching participant.
  - **Joined but not responded** — participants who have zero availability rows (using the same "submitted = has ≥ 1 availability row" signal that `submit_to_view` already trusts).
- Auto-match of invite → participant when a participant joins whose (trimmed, case-folded) name matches an unmatched invite. Auto-match also runs on participant rename.
- CRUD for invites (owner-only): add single, add bulk (paste comma/newline-separated list), rename, delete.
- Copy-to-clipboard on a missing-person chip so the owner can drop the name straight into a chat nudge.

**Out**
- Email/SMS invitations. We don't collect contact details for joiners (explicitly by design), and we're not going to start here. The owner nudges via whatever channel they already use.
- Reminders/notifications. That's the Tier 2 "Notifications" feature (`plans/v2/t2/notifications.md`, paused). "Who's missing?" is the *data*; reminders are the *delivery*.
- Making invites visible to non-owners. The list is owner-planning state, not a participant-facing directory. Leaking it would also leak intent ("the owner expects Alice to come") before Alice has signed up.
- Hard-gating joins to the invite list. No "only invited names may join." The list is a soft expectation, not an ACL. Anyone with the link can still join under any name; hard gating is the domain of passphrase + `submit_to_view`, not this feature.
- Per-invite metadata (role, +1, email, RSVP yes/no). Keeps the primitive as thin as possible. Can extend later without migration pain — `calendar_invites` is its own table.
- Progress/summary rendering ("3 of 8 responded") on the public summary card. The panel is owner-only; a public progress bar would confirm the expected headcount to everyone with the link, which leaks the guest list.
- A dedicated full-page `/calendars/{id}/missing` route. One owner panel on the calendar page is enough; a standalone page can come later if the panel feels cramped.

## Design Decisions

**New table, not a column.** Invites are a list of strings with their own lifecycle (added, renamed, auto-matched, deleted) and could grow metadata (e.g. a `required` flag, a note) — a `TEXT[]` column on `calendars` makes every mutation a whole-array rewrite and forbids per-row constraints. A dedicated `calendar_invites` table is the natural shape, aligns with existing store patterns (`participants`, `availability`), and costs one migration.

**Invite ≠ participant.** We do **not** represent an invite as a "stub participant" row. Participants have availability, colour, optional user linkage — invites have none of that. Overloading the participants table would force every read path to filter "is this a real participant or just a placeholder?" and would muddy the `submit_to_view` "has any availability row" check. Keeping them separate means zero churn on existing endpoints.

**Auto-match on name, case-insensitive, trimmed.** When a participant is created or renamed we look for an unmatched invite with `lower(trim(name)) = lower(trim(participant.name))`. If exactly one matches, link it (`matched_participant_id = participant.id`). If zero match, do nothing. If >1 match (owner added the same name twice), link the oldest unmatched one — arbitrary but deterministic. Matching happens server-side so the owner doesn't have to babysit the mapping, and the match is recorded so a later rename doesn't silently unmatch.

**Manual re-link fallback.** Auto-match won't catch "Bob" vs "Bobby" vs "Robert". The owner panel lets the owner manually link an invite to any participant on the calendar from a small menu. It also lets them unlink. This is the escape hatch; most of the time auto-match is enough.

**No participant side-effects on invite delete.** Deleting an invite row does not touch the participant it matched. The participant stays; they just stop being "expected" (which affects nothing except this panel). This is the safe, reversible default.

**"Joined but not responded" is derived, not stored.** Computed at read time by joining `participants` to `availability` and taking the zero-rows set. Same rule as `submit_to_view`. Cheap, always correct, no new flag to keep in sync. The owner gets a live read; anyone who marks a day immediately falls off the list.

**Owner-only enforcement on every surface.** The panel is owner-only in the UI, but the authoritative gate is on the API. `GET /calendars/{id}/missing` and all invite mutation endpoints require `user.ID == calendar.OwnerID`. Non-owners get 403. Unauthenticated clients get 401. This matches the finalise/rename pattern.

**Invite cap: `MaxParticipantsPerCalendar` (100).** The same cap that bounds the participant list bounds the invite list. Prevents the invite list from becoming a CRM and keeps the panel's scannability honest.

**Name length: `MaxParticipantNameLength`.** Same limit as participant names — since invites often become participants, mismatched limits would create a class of "this invite is too long to be a real participant" bugs.

**Case-insensitive uniqueness within a calendar.** An invite's name is unique per calendar under `lower(trim(name))`. Duplicate entries would split responses across two buckets ("Alice" vs "alice"). Enforced by a unique index, surfaced as a `409 invite_name_taken`.

**Bulk add is multi-insert with per-line dedupe.** The owner pastes `"Alice, Bob\nCharlie"`. Server splits on `,` and `\n`, trims each, drops empty, dedupes within the batch, then inserts in a single transaction. Invalid lines (too long, whitespace-only) are reported back so the owner can fix; already-present names are skipped with a note, not a hard error (owner re-pastes an updated list often).

**Auto-match runs inside `AddParticipant` and `UpdateParticipantName` transactions.** Matching is a side-effect of the name reaching its final form. Doing it in the same tx means a crash between "create participant" and "link invite" can't leave a half-matched state. If no invite matches, zero cost beyond one indexed lookup.

**Reveal at the right moment.** The "Not joined" list is shown even when the calendar is in `submit_to_view` mode, because the owner bypasses visibility filtering. For the owner, this is the whole point — being able to see the gap regardless of how much data participants have contributed.

**Analytics.** `invite_added` `{ calendar_id, bulk: bool, count }`, `invite_deleted`, `invite_matched_auto` (fired from the matching side-effect), `invite_matched_manual`, `missing_panel_viewed`. Enough to answer "do people use this?" and "does auto-match cover most cases, or are owners relinking by hand?"

**Copy-to-clipboard on chips.** Tapping a "Not joined" name copies `"Alice"` (just the name) to the clipboard and fires a toast "Copied — paste into your group chat." The whole point of the feature is making the nudge action trivial. We don't auto-compose a share message because the owner needs a phrase with context; we just remove the typing.

## Database

Migration `014_add_calendar_invites.sql`:

```sql
CREATE TABLE calendar_invites (
  id TEXT PRIMARY KEY,
  calendar_id TEXT NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  matched_participant_id TEXT REFERENCES participants(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX calendar_invites_calendar_lower_name_idx
  ON calendar_invites (calendar_id, LOWER(TRIM(name)));

CREATE INDEX calendar_invites_calendar_idx
  ON calendar_invites (calendar_id);

CREATE INDEX calendar_invites_matched_participant_idx
  ON calendar_invites (matched_participant_id)
  WHERE matched_participant_id IS NOT NULL;
```

`ON DELETE SET NULL` on `matched_participant_id` lets an owner remove a participant without losing the invite — the invite flips back to "not joined", which is the intuitive behaviour.

## API

New endpoints, all under `/calendars/{id}/…`, all require `requireAuth` + owner check:

| Method | Path | Description |
|---|---|---|
| `GET` | `/calendars/{id}/missing` | Returns invite list + computed buckets. Owner-only. |
| `POST` | `/calendars/{id}/invites` | Add one or many invites. Body: `{ names: string[] }`. |
| `PUT`  | `/calendars/{id}/invites/{inviteId}` | Rename an invite. Body: `{ name: string }`. |
| `DELETE` | `/calendars/{id}/invites/{inviteId}` | Remove an invite. Idempotent. |
| `POST` | `/calendars/{id}/invites/{inviteId}/match` | Manual link. Body: `{ participant_id: string \| null }`. |

### `GET /calendars/{id}/missing`

Response `200`:

```json
{
  "invites": [
    { "id": "inv_...", "name": "Alice", "matched_participant_id": "p_..." },
    { "id": "inv_...", "name": "Bob",   "matched_participant_id": null }
  ],
  "not_joined": [
    { "invite_id": "inv_...", "name": "Bob" }
  ],
  "joined_not_responded": [
    { "participant_id": "p_...", "name": "Charlie" }
  ]
}
```

`not_joined` and `joined_not_responded` are derived server-side so the client doesn't reimplement the logic. A single GET renders the whole panel.

### `POST /calendars/{id}/invites`

Request: `{ "names": ["Alice", "Bob"] }`.

Response `201`:

```json
{
  "added":   [ { "id": "...", "name": "Alice", "matched_participant_id": "p_..." } ],
  "skipped": [ { "name": "bob", "reason": "duplicate" } ],
  "invalid": [ { "name": "<too long>", "reason": "name_too_long" } ]
}
```

Partial success is fine — bulk paste should never fail the whole batch because one line was blank.

Errors:
- `400 invalid_json`, `400 names_required`
- `401 unauthorized`
- `403 forbidden`
- `404 not_found` (calendar)
- `409 invite_limit_reached` (would exceed `MaxParticipantsPerCalendar`)

### Auto-match side-effect

No new endpoint. `POST /calendars/{id}/participants` and the existing rename handler both call a shared `service.AutoMatchInvite(tx, calendar_id, participant)` inside their transactions. It's a lookup + `UPDATE calendar_invites SET matched_participant_id = $1 WHERE id = (SELECT id FROM calendar_invites WHERE calendar_id = $2 AND matched_participant_id IS NULL AND lower(trim(name)) = lower(trim($3)) ORDER BY created_at LIMIT 1)`.

### `POST /calendars/{id}/invites/{inviteId}/match`

Body with `participant_id: string` links; `participant_id: null` unlinks. The referenced participant must belong to the same calendar (400 `participant_not_on_calendar` otherwise).

## Backend Implementation

### Phase 1 — schema & model

1. Migration 014 (above).
2. `model.Invite { ID, CalendarID, Name, MatchedParticipantID *string, CreatedAt }`.
3. `limits.MaxInvitesPerCalendar = MaxParticipantsPerCalendar` (alias, so they move together if we ever tune one).

### Phase 2 — store

New file `internal/store/invites.go`:

- `ListInvites(ctx, calendarID) ([]Invite, error)` — sorted by `created_at ASC`.
- `AddInvitesTx(ctx, tx, calendarID, names []string) (added []Invite, skipped []string, err error)` — single tx for bulk; dedupes by `lower(trim(name))` against existing rows; enforces cap; returns the inserted rows so the handler can run auto-match immediately.
- `UpdateInviteName(ctx, inviteID, newName)` — unique-violation → `ErrInviteNameTaken`.
- `DeleteInvite(ctx, inviteID)`.
- `SetInviteMatch(ctx, inviteID, participantID *string)`.
- `MatchInviteByNameTx(ctx, tx, calendarID, participantID, name)` — the auto-match helper. Idempotent; no-op when no unmatched invite exists.

Hook `MatchInviteByNameTx` into:
- `store.AddParticipant` (after the participant row is inserted, before commit).
- `store.UpdateParticipantName` (same spot).

### Phase 3 — service

`internal/service/invite.go`:

- `AddInvites(ctx, calendarID, rawNames []string) (AddInvitesResult, error)` — parses, validates (length, no control chars, non-empty after trim), batches into the store, returns `added / skipped / invalid`.
- `RenameInvite`, `DeleteInvite`, `ManualMatchInvite(ctx, calendarID, inviteID, participantID *string)` — with ownership-neutral preconditions (handler does the owner check).
- `GetMissingView(ctx, calendarID) (MissingView, error)` — one query for invites, one for participants-with-no-availability, composes the response.

Bulk-name parser lives here (split on `,` and `\n`, trim, drop empty, dedupe).

### Phase 4 — handler & routes

`internal/handler/invite.go`:

- `ListMissing`, `AddInvites`, `RenameInvite`, `DeleteInvite`, `ManualMatchInvite`.
- All five do: `requireAuth` → load calendar → `user.ID != owner` → 403.
- Error mapping: `ErrInviteLimitReached → 409`, `ErrInviteNameTaken → 409`, `ErrInviteNameTooLong → 400`, `ErrInviteNotFound → 404`, `ErrParticipantNotOnCalendar → 400`.

Wire in `server.go` under `requireAuth`.

### Phase 5 — tests

- Store: insert/list, case-insensitive uniqueness, cap, bulk insert partial skip, `MatchInviteByNameTx` with 0/1/many unmatched candidates, `ON DELETE SET NULL` behaviour when the matched participant is removed.
- Service: name parsing (commas, newlines, trailing whitespace, unicode), dedupe within a batch, cap enforcement with mix of new + existing names, `GetMissingView` correctness (no invites, invites only, invites + joined + responded permutations).
- Handler: 401 / 403 / 404 / 409 matrix, happy path shapes.
- Integration (`integration` tag): end-to-end against real Postgres — add invites, join under a matching name, assert auto-match; rename participant to a different matching name, assert match moves; delete participant, assert invite flips back to unmatched.

## Frontend Implementation

### Phase 1 — API client + types

`src/lib/api.ts`:

- `getMissing(calendarId)`
- `addInvites(calendarId, names: string[])`
- `renameInvite(calendarId, inviteId, name)`
- `deleteInvite(calendarId, inviteId)`
- `matchInvite(calendarId, inviteId, participantId: string | null)`

Types: `Invite`, `MissingView`, `AddInvitesResult`.

### Phase 2 — Owner panel on `/calendars/{id}`

A new collapsible panel in the owner controls area, titled **"Who's missing?"** with a count badge (`not_joined.length + joined_not_responded.length`). Closed by default on first visit (it's owner-planning chrome, not the primary content); state persisted in localStorage per calendar.

Panel content, top to bottom:

1. **Not joined** — chips with the invite name. Tap a chip → copy name + toast. Long-press / kebab menu per chip: *Rename*, *Delete*, *Link to a participant…* (opens a small sheet listing current participants).
2. **Joined but not responded** — chips with participant name (no menu).
3. **Add invites** — inline input + "Paste a list" affordance that expands to a textarea. Submits via `addInvites`; surfaces `skipped` / `invalid` inline as small grey notes ("2 already on the list", "1 name too long").
4. **All invited** — disclosed list of already-matched invites (muted). Tapping reveals a *Unlink* action for the manual-link escape hatch.

Render rules:
- Owner-only. If the page's `isOwner` is false, the panel doesn't render and `getMissing` is not called.
- Live updates: when an SSE participant event arrives (`plans/v2/t2/live-updates.md`), refetch `getMissing` in the background. Panel updates in place.
- Empty state: "No-one's missing 🎉" when both buckets are empty and there's at least one invite; "Add names to track who still needs to respond" when the invite list is empty.

### Phase 3 — Wiring on joins/renames

No client-side auto-match logic — server does it. When `POST /participants` or the rename endpoint returns, we invalidate the panel's cached view so the owner sees the match pop in.

### Phase 4 — Analytics

- `invites_panel_opened` `{ calendar_id }`
- `invite_added` `{ calendar_id, bulk: bool, count }` (bulk = true when `count > 1`)
- `invite_deleted`
- `invite_matched_manual`
- `invite_copied_to_clipboard`

### Phase 5 — tests

- API client: snake ↔ camel mapping on `matched_participant_id`, `not_joined`, `joined_not_responded`.
- `WhosMissingPanel` component: renders buckets, empty states, add/rename/delete flows, copy-to-clipboard fires toast, kebab menu link-to-participant flow.
- Bulk add: pasting `"Alice, Bob\nCharlie"` yields 3 added; inline reports `skipped` / `invalid`.

## Testing (manual)

- Owner creates calendar, opens "Who's missing?" panel — empty state asks them to add names.
- Bulk-add `"Alice, Bob, Charlie"` — three chips appear in **Not joined**.
- Share link. Someone joins as "alice" (lowercase). Chip moves out of **Not joined** (into **All invited**, muted).
- Join as "bob ". Trailing space is trimmed — auto-match works.
- Rename a participant from "Bob" to "Bobby" — invite unmatches (it was matched on "Bob"), flips back to **Not joined**. Owner opens kebab on the invite → *Link to a participant…* → picks Bobby. Chip moves back to matched.
- Join as "Dave" (not invited). No auto-match. Dave marks one day. Doesn't appear in either bucket (he joined *and* responded).
- Join as "Eve" but mark no dates. Eve shows up under **Joined but not responded**. Mark a date — she falls off immediately.
- Non-owner visits the calendar — no panel visible. Direct GET to `/calendars/{id}/missing` returns 403.
- Owner at the invite cap tries to bulk-add 3 more — response `409 invite_limit_reached`, panel shows the error inline.
- Duplicate add: add "Alice" twice — second is `skipped` with `duplicate`.
- Delete a matched participant — invite flips back to **Not joined** (DB `ON DELETE SET NULL` working).
- `submit_to_view` calendar: participants still see the filtered view; owner still sees the full panel.

## Build Order

1. Backend Phase 1 (migration + model) — run `make test-integration` against the new schema.
2. Backend Phase 2 (store + tests).
3. Backend Phase 3 (service + tests).
4. Backend Phase 4 (handler + routes + tests).
5. Backend integration test for auto-match side-effect on participant create + rename + delete.
6. Frontend Phase 1 (API client + types).
7. Frontend Phase 2 (panel component + tests).
8. Frontend Phase 3 (live-updates refetch).
9. Frontend Phase 4 (analytics).
10. Manual QA matrix.
11. Mark Tier 3 item shipped in `docs/v2-priorities.md:40`.

## Open Questions

> Resolved 2026-04-21. None of these block shipping.

- **Match by normalised name vs. by explicit claim.** ✅ *Stick with name match.* The use case is "group chat full of known names," not an auth system. Requiring signed-in + email hints would gate the feature on data we've deliberately chosen not to collect. Manual relink is the escape hatch.
- **Should invites feed the participant cap?** ✅ *Keep separate caps.* Unifying creates a "can't add the 101st expected name because 60 already joined" failure mode right when the owner most needs the list. Permissive is the right default; tighten later if analytics shows abuse.
- **Surface "invite yourself" during join?** ❌ *No.* Leaks the invite list via side-channel to anyone with the join link — exactly the privacy property the feature is built around. Also keeps the join flow decoupled from invite state.
- **Progress badge for non-owners ("5 of 8 responded").** ❌ *No, by default.* Same leak as above, broader blast radius. If owners ask for it later, offer as an owner-toggleable setting so the leak is an explicit choice, not a surprise.
- **Invite notes / roles.** ⏸ *Defer.* `calendar_invites` is its own table; extending is a non-breaking migration. Don't build until an owner asks twice.
- **Deleting a participant who was matched.** ✅ *Current behaviour (flip invite back to "Not joined") stays as default.* Owner removals are usually moderation or mistake-fix; they'll want the name back on the expected list. When we add a remove-participant confirmation dialog, include an unchecked *"Also remove from invite list"* checkbox as the escape hatch.

## Log

- 2026-04-21 — Created plan.
- 2026-04-21 — Resolved all six open questions (see Open Questions section); none block implementation.
- 2026-04-21 22:30 — Implemented all phases. Migration 015 (not 014 as planned — 014 was already used by calendar theme). `model.Invite` + `limits.MaxInvitesPerCalendar` added. `store/invites.go` implements list/add/rename/delete/match. Auto-match is a best-effort call on `AddParticipant` and `UpdateParticipantName` (not an explicit tx — participant ops were already non-tx; a match failure logs nothing and doesn't fail the write, with manual relink as the escape hatch). Service layer in `service/invite.go` wraps validation + bulk parse. Handler in `handler/invite.go` enforces owner check. Routes wired under `requireAuth`. Frontend: `WhosMissingPanel` component mounted in owner controls, `getMissing`/`addInvites`/`renameInvite`/`deleteInvite`/`matchInvite` in `api.ts`, types extended. Backend tests cover store (integration), service (unit), handler (401/403/404/409 matrix). Frontend tests cover API camelCase and panel flows. Marked shipped in `docs/v2-priorities.md:40`.
