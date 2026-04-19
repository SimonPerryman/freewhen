# Finalise a Date

Status: done

## Goal

Complete the core user journey inside the app. Today the flow ends at "see overlap" — users have to leave the calendar and message each other to actually pick a date. Let the calendar owner lock in the chosen date so the calendar itself answers "when is it?".

## Design Decisions

**Owner-only for v1.** Calendar admin roles aren't built yet (Tier 1 "Admin controls"). The owner is currently the only durable identity attached to a calendar. When admin roles land, we'll decide whether to extend finalise to admins.

**Single date, not a range.** Most events are one day. Range support (weekend trips) can come later without breaking the data model — store as a single `DATE` for now, change to a range type only if/when needed.

**Finalising locks the calendar.** Once a date is locked, availability edits are rejected (PUT availability → 409) and joining is also rejected (POST participants → 409). The calendar becomes a read-only confirmation view. The owner sees an "Unfinalise" button to reopen if plans change.

  *Why lock joins too?* Simpler mental model — finalised means "done, look but don't touch". Avoids the half-state where new joiners can't actually do anything useful (their availability wouldn't move the decision). Revisit if real friction appears.

**Audit fields kept minimal.** Just `finalised_date` and `finalised_at`. Skip `finalised_by_user_id` — owner-only means it's always the owner. Add later if admins get the power.

**UX surface: button per Summary row.** The Summary section already ranks dates ("Days everyone is free", "Days most people are free"). Add a "Finalise this date" button on each row, visible only to the owner. Confirms via a small inline confirmation step (no separate modal) — picking the wrong date is recoverable via Unfinalise.

**Finalised state is the page's hero.** When finalised, replace the grid+summary chrome with a prominent confirmation: "📅 Saturday, Apr 26 — confirmed!" The grid still renders below it (read-only, with the locked date highlighted) so people can see the context that led to the decision.

## Data

Migration `006_add_calendar_finalised.sql`:

```sql
ALTER TABLE calendars
    ADD COLUMN finalised_date DATE,
    ADD COLUMN finalised_at   TIMESTAMPTZ;
```

Both nullable. `finalised_date IS NULL` means "not finalised". When set, `finalised_at` is set in the same write.

`model.Calendar` gains `FinalisedDate *string` and `FinalisedAt *time.Time` (pointer or `sql.Null*` — match what other optional fields like `Emoji` already do).

## Backend

**New endpoints:**

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/calendars/{id}/finalise` | Owner | Lock a date. Body: `{"date": "YYYY-MM-DD"}` |
| DELETE | `/calendars/{id}/finalise` | Owner | Clear the locked date |

Both return the updated calendar.

**Modified endpoints:**

- `PUT /calendars/{id}/availability` — return `409 Conflict` with `{"error": "calendar is finalised"}` if `finalised_date IS NOT NULL`.
- `POST /calendars/{id}/participants` — same 409 behaviour.

**Files to touch:**

- `backend/internal/store/migrations/006_add_calendar_finalised.sql` — new
- `backend/internal/model/model.go` — add fields to `Calendar`
- `backend/internal/store/calendar.go` — `SetFinalisedDate(ctx, calendarID, date)`, `ClearFinalisedDate(ctx, calendarID)`; update existing `Get` queries to select the new columns
- `backend/internal/store/availability.go`, `participant.go` — guard on finalised state (or check in handler — see below)
- `backend/internal/handler/calendar.go` — `FinaliseCalendar`, `UnfinaliseCalendar`. Owner check: load calendar, compare `cal.OwnerID` to `UserFromContext(r).ID`, 403 if mismatched, 401 if no session.
- `backend/internal/handler/availability.go`, `participant.go` — add the 409 guard (either inline or via a store error like `ErrCalendarFinalised`)
- `backend/internal/server/server.go` — register the two new routes
- `backend/internal/handler/handler_test.go` and `server_integration_test.go` — cover happy path, non-owner gets 403, double-finalise replaces the date, edits/joins return 409 when finalised, unfinalise reopens

**Decision: where does the lock check live?** Prefer in the store layer with a typed error (`ErrCalendarFinalised`) so handlers map it to 409 the same way `ErrNotFound` becomes 404. Keeps the rule in one place.

## Frontend

**Types:**

- `frontend/src/lib/types.ts` — add `finalisedDate?: string` and `finalisedAt?: string` to `Calendar`. The API client snake↔camel mapping handles the rest.

**API client (`frontend/src/lib/api.ts`):**

- `finaliseCalendar(id, date)` → `POST /calendars/{id}/finalise`
- `unfinaliseCalendar(id)` → `DELETE /calendars/{id}/finalise`

**Components:**

- `SummaryView/SummaryView.tsx` — when `isOwner && !calendar.finalisedDate`, render a "Finalise" button on each summary row. Two-step confirm: first click → button changes to "Confirm Apr 26?" with a small "cancel" affordance; second click commits. On 409 from a stale state, refetch and show a toast.
- `CalendarGrid/CalendarGrid.tsx` — when `calendar.finalisedDate` is set, disable click handlers and add a `finalised` style on the matching cell (e.g. solid colour ring) so the locked date stands out. Other cells lose their hover/active affordances.
- New component `FinalisedBanner/FinalisedBanner.tsx` — hero banner shown above the grid when finalised: emoji, formatted date ("Saturday, 26 April"), and (owner-only) an "Unfinalise" button with a confirm step.
- `JoinForm/JoinForm.tsx` — when finalised, hide the form and show a short read-only message ("This calendar is finalised — Apr 26 is the date.").
- `frontend/src/app/calendars/[id]/page.tsx` — derive `isOwner = user?.id === calendar.ownerId` from `AuthProvider` and pass down. When finalised, render `FinalisedBanner` above the existing layout.

**Tests:**

- `SummaryView.test.tsx` — finalise button only visible to owner; click flow goes confirm → commit → calls API; hidden when already finalised.
- `CalendarGrid.test.tsx` — finalised date cell has the locked style and is not clickable; other cells are non-interactive in finalised state.
- `FinalisedBanner.test.tsx` — renders date, owner sees Unfinalise button, joiner does not.
- `api.test.ts` — finalise/unfinalise calls hit the right paths and decode the updated calendar.

## Docs

- `docs/api.md` — document the two new endpoints, the new 409 on availability/participant writes, and the new `finalised_date`/`finalised_at` fields on the calendar response.
- `docs/user-journeys.md` — add journey 7 "Finalise the date (owner)" with an ASCII mock of the finalised state.
- `docs/v2-priorities.md` — once shipped, update Tier 1 row.

## Out of Scope

- Admin-can-finalise (waits for admin-controls plan)
- Date ranges
- Notifying participants when a date is finalised (waits for notifications)
- Adding the finalised date to anyone's external calendar (waits for calendar integrations)

## Log

- 2026-04-19 — Created plan. Chose owner-only, single-date, lock-on-finalise (incl. joins), Summary-row button, with an Unfinalise escape hatch.
- 2026-04-19 13:15 — Implemented all phases in a single commit:
  - Migration `006_add_calendar_finalised.sql` adds `finalised_date DATE` and `finalised_at TIMESTAMPTZ` (both nullable).
  - Store: `SetFinalisedDate` / `ClearFinalisedDate`, typed `ErrCalendarFinalised`, writes from `AddParticipant` and `SetAvailability` now guard on the lock.
  - Handlers: `POST` / `DELETE /calendars/{id}/finalise` (owner-only, 401/403/404). Existing `availability` and `participants` writes now return 409 when locked.
  - Frontend: `finalisedDate`/`finalisedAt` on `Calendar`, `finaliseCalendar` / `unfinaliseCalendar` API helpers, `FinalisedBanner` component, per-row Finalise button in `SummaryView` with a two-step confirm, `CalendarGrid` disables all cells and highlights the locked date when set.
  - Wired `CalendarPageClient` to show the banner hero when finalised, hide `JoinForm` / `SummaryView` / `BestDay`, and pass `finalisedDate` through. Analytics events added for `calendar_finalised` and `calendar_unfinalised`.
  - Tests: added backend handler unit tests (happy path + unauth + non-owner + bad date + not-found for both endpoints, plus 409 mapping on availability/participants) and integration tests (`TestFinaliseFlow`, `TestFinalise_Unauthorized`, `TestFinalise_NotOwner`). Added frontend tests for API helpers, SummaryView confirm flow, CalendarGrid locked state, and FinalisedBanner.
  - Docs: `docs/api.md` gains the two endpoints, the 409s, and the new calendar fields; `docs/user-journeys.md` adds journey 7; `docs/v2-priorities.md` marks the row shipped.
