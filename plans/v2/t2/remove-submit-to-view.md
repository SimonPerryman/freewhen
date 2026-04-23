# Remove submit-to-view privacy

Status: done

## Goal

Rip out the `submit_to_view` calendar privacy mode shipped in `b3aa5fc`
(2026-04-20, plan `plans/v2/t2/submit-to-view-privacy.md`). The feature works,
but we're still pre-launch with no user signal, nobody pulled for it,
and the "Hide others until you respond" toggle already feels heavy
enough on the create form that the v2-priorities doc was proposing to
hide it behind an Advanced options disclosure. That's the same shape
of decision we just made on calendar colour themes: when the instinct
is "hide the weight," the consistent call is to cut. See
`docs/cut-features.md` for the decision log; this plan is the
mechanical removal and mirrors `plans/ui/remove-calendar-themes.md`.

Not a revert. Migration 008 has nine migrations stacked on top of it,
so removal is a new forward-only migration.

## Scope

**In**
- Drop the `visibility` column from `calendars` via a new migration
  (`017_drop_calendar_visibility.sql`).
- Remove backend visibility plumbing: model field, store read/write
  (including the duplicate path), `UpdateCalendarVisibility`,
  `ParticipantHasAvailability`, `CountCalendarParticipants` if only
  used by the hidden path, `GetCalendarView` reveal matrix, PATCH
  handler field, `?as=` query param on GET, tests.
- Collapse `GET /calendars/{id}` back to always returning full details
  — no hidden shape, no `participantCount` field.
- Remove frontend visibility plumbing: the "Hide others until you
  respond" toggle on the create form, the owner control in
  `CalendarSettingsSheet`, the hidden-state banner and gating in
  `CalendarPageClient`, the optimistic-reveal flow, `?as=` on the
  API client, the `visibility` and `participantCount` fields on
  types, the `calendar_visibility_set` and
  `availability_gate_unlocked` analytics events.
- Update `docs/api.md` — remove `visibility` from POST/PATCH
  `/calendars` and `?as=` / hidden response shape from GET
  `/calendars/{id}`.
- Update `docs/v2-priorities.md` — restore the row to its pre-ship
  state with a `(previously shipped and cut — see
  docs/cut-features.md)` suffix, matching how themes was handled.
- Mark `plans/v2/t2/submit-to-view-privacy.md` status as `cut` and add a
  log entry pointing at this plan and `docs/cut-features.md`.
- Add a second entry to `docs/cut-features.md` with rationale and
  restoration notes.

**Out**
- Reverting `b3aa5fc` directly. It's entangled with nine later
  migrations and with `participant_user_linking` touches that cannot
  be cleanly reverted.
- Rolling back the `UpdateCalendar` generalisation of
  `RenameCalendar`. The PATCH handler can keep its shape and just
  stop accepting `visibility`; re-narrowing to a rename-only
  endpoint is namespace churn for no benefit.
- Any broader audit of other features that might also be scope-cut.
- Preserving existing `visibility` values. Everything is still dev
  data; nothing to migrate.

## Design decisions

- **Forward-only migration, not a drop of 008.** Same reasoning as
  themes: migrations are immutable once merged. New
  `017_drop_calendar_visibility.sql` does `ALTER TABLE calendars
  DROP COLUMN visibility;`. Fresh environments pay a no-op add-then-
  drop round-trip, which is cheaper than rewriting history.
- **Keep the PATCH handler generalised.** The submit-to-view plan
  collapsed `RenameCalendar` into `UpdateCalendar`. That generalisation
  has since absorbed other fields (finalised, passphrase, etc.) and
  re-narrowing it now would be pointless churn. Just drop
  `visibility` from the accepted body.
- **Delete the plan? No — mark it `cut`.**
  `plans/v2/t2/submit-to-view-privacy.md` stays on disk with
  `Status: cut` and a log entry pointing at this removal plan and
  `docs/cut-features.md`. Same treatment as themes.
- **Revert `docs/v2-priorities.md` rather than leaving it as
  ✅ Shipped.** The table is a forward-looking priority list; a
  ship-then-cut roundtrip is a cut-features doc concern. Restore the
  original row text (Tier 2, first row) so the priority list reads as
  if it never shipped, with a parenthetical
  `(previously shipped and cut — see docs/cut-features.md)`. This
  keeps the priority list scannable without erasing history.
- **No data preservation.** Existing `visibility` values on dev
  calendars are thrown away with the column drop. We're pre-launch;
  there are no production values to worry about.
- **Drop `participantCount` from the API entirely.** It was only
  added to support the hidden-state banner ("5 people have joined —
  mark a day to see them"). Without the hidden state, the frontend
  can count `participants.length` locally. Keeping the field as
  dead weight on every GET is exactly the kind of carrying cost
  this plan exists to remove.
- **Keep `ParticipantHasAvailability` only if something else uses
  it.** It was introduced for the reveal matrix. If no other caller
  exists after the service plumbing is removed, delete the store
  method and its test. Mechanical call: grep before ripping. Same
  decision for `CountCalendarParticipants`.
- **The "Advanced options disclosure" v2-priorities row goes away
  too.** It was premised on a home for this toggle. Without the
  toggle, the row is proposing a drawer with nothing in it. Remove
  the row from `docs/v2-priorities.md`. Future toggles that want a
  drawer can propose one when they exist.

## Plan

1. **Backend — remove visibility column and reveal plumbing.**
   - Add `backend/internal/store/migrations/017_drop_calendar_visibility.sql`:
     `ALTER TABLE calendars DROP COLUMN visibility;`
   - Remove `Visibility string` from `model.Calendar`; delete the
     visibility const block / `IsValidVisibility` helper.
   - Remove `visibility` from `store.CreateCalendar`, `GetCalendar`,
     `GetCalendarOnly`, `UpdateCalendar`, and the duplicate path;
     delete `UpdateCalendarVisibility`.
   - Delete `GetCalendarView` and the `Viewer` struct; callers switch
     back to `GetCalendar` returning `CalendarWithDetails` directly.
   - Delete `ParticipantHasAvailability` and `CountCalendarParticipants`
     if unused after the above; keep (with a note) if something else
     has picked them up.
   - Remove `visibility` from POST `/calendars` body validation.
   - Remove `visibility` from PATCH `/calendars/{id}` body (keep the
     generalised update handler; it still takes `name` and other
     fields).
   - Remove `?as=` handling from GET `/calendars/{id}`; remove the
     hidden-shape branch and the `participantCount` field from the
     response.
   - Strip visibility/reveal assertions from `handler_test.go`,
     `service_test.go`, `calendar_test.go`, and any integration
     tests that exercise the hidden shape.
   - `make test-backend` and `make test-integration`.

2. **Frontend — remove visibility UI.**
   - `frontend/src/app/page.tsx` — remove `hideUntilRespond` state,
     the toggle row (`styles.toggleRow`, `styles.toggleLabel`,
     `styles.toggleHelp`), and the `visibility` payload on
     `createCalendar`. Remove the `calendar_visibility_set`
     analytics capture.
   - `frontend/src/app/page.module.css` — drop the toggle-row styles
     if nothing else uses them.
   - `frontend/src/app/page.test.tsx` — remove the visibility toggle
     assertions.
   - `CalendarSettingsSheet.tsx` — remove the visibility control,
     its label, and its `onVisibilityChange` wiring; callers stop
     passing the prop.
   - `CalendarPageClient.tsx` — remove the `revealed` computation,
     the hidden-state banner, the optimistic-reveal flow, the
     `?as=` plumbing on `getCalendar`, and the
     `availability_gate_unlocked` analytics event. The grid goes
     back to always rendering full participant data.
   - `CalendarPageClient.test.tsx` — remove hidden/revealed cases.
   - `InvitePrompt.tsx` — if any visibility-aware copy exists,
     collapse back to the single unconditional variant.
   - `MyCalendarsList.tsx` + `.test.tsx` — if the list surfaces a
     visibility badge anywhere, remove it.
   - `frontend/src/lib/api.ts` — remove `visibility` from
     `createCalendar`, remove `as` param from `getCalendar`, remove
     `visibility` from the `updateCalendar` patch type.
   - `frontend/src/lib/types.ts` — remove `visibility` from
     `Calendar`; remove `participantCount` from
     `CalendarWithDetails`.
   - `make test-frontend` and `make lint`.

3. **Docs.**
   - `docs/api.md` — remove `visibility` from POST/PATCH
     `/calendars` and remove the `?as=` query + hidden response
     shape + `participantCount` field from GET `/calendars/{id}`.
   - `docs/v2-priorities.md` — restore the Tier 2 "Submit to view
     privacy" row to its pre-ship state with a
     `(previously shipped and cut — see docs/cut-features.md)`
     suffix. Delete the Tier 3 "Advanced options disclosure on
     create" row (it was premised on a home for this toggle and
     no longer has a reason to exist).
   - `plans/v2/t2/submit-to-view-privacy.md` — set `Status: cut`, add a
     log entry pointing at this plan and `docs/cut-features.md`.

4. **Append to `docs/cut-features.md`.**
   New entry at the top, above "Calendar colour themes", following
   the shape established by that entry:
   - **Shipped:** 2026-04-20 — commit `b3aa5fc` — plan
     `plans/v2/t2/submit-to-view-privacy.md`
   - **Cut:** 2026-04-23 — commit `<TBD>` — plan
     `plans/v2/t2/remove-submit-to-view.md`
   - **Why cut:** Pre-launch feature that nobody asked for. The
     create-form toggle already felt disproportionate enough that the
     v2-priorities doc was proposing to hide it behind an Advanced
     options disclosure — the same "hide the weight" instinct that
     preceded cutting calendar colour themes. No user signal for or
     against; cheapest time to remove is now.
   - **What it did:** Per-calendar `visibility` enum (`open` vs
     `submit_to_view`) gating the `GET /calendars/{id}` response —
     unsubmitted non-owner viewers got a stripped shape
     (`participants: []`, `availability: []`, `participantCount`)
     until they marked a day. Owner always bypassed; anonymous
     viewers identified themselves via `?as={participantId}`.
     Frontend rendered a hidden-state banner and flipped to full UI
     optimistically on first toggle.
   - **Downstream decisions changed:** The PATCH handler was
     generalised from `RenameCalendar` to `UpdateCalendar` as part
     of this feature. That generalisation stays — later features
     (finalise, passphrase) already ride on it and it's no longer
     tied to visibility.
   - **To restore:** Cherry-pick `b3aa5fc` and any follow-ups
     that touched visibility code between then and the removal
     commit. Migration 008 is on disk; migration 017 drops the
     column, so a clean restore needs a new migration (018+) to
     re-add `visibility` with the CHECK constraint rather than
     resurrecting 008. Re-evaluate whether the owner toggle still
     belongs in `CalendarSettingsSheet` — the sheet has evolved
     (passphrase, duplicate) since the original plan assumed the
     toggle sat near rename.

5. **Verify.**
   - `make lint && make test && make test-integration` all green.
   - Manually: create a calendar, open it, confirm no broken imports
     in the settings sheet, no stray `?as=` in network tab, no
     hidden-state banner can render.

## Log

- 2026-04-23 — Created plan.
- 2026-04-23 — Implemented: migration 017 drops `visibility`; backend
  visibility plumbing removed (model field, store read/write,
  `UpdateCalendarVisibility`, `ParticipantHasAvailability`,
  `GetCalendarView`/`CalendarView`, `ErrInvalidVisibility`); `?as=` gone
  from GET/`events`/passphrase access paths; `participantCount` dropped
  from `GET /calendars/{id}` response. Frontend removed the create-form
  toggle, `CalendarSettingsSheet` visibility section, hidden-state
  banner/reveal plumbing, `visibility`/`participantCount` from types, and
  the `calendar_visibility_set`/`availability_gate_unlocked` events.
  Docs: `api.md` PATCH body + POST body updated; `v2-priorities.md`
  row restored and Advanced-options row removed;
  `plans/v2/t2/submit-to-view-privacy.md` set to `cut`; new entry at the top
  of `docs/cut-features.md`.
