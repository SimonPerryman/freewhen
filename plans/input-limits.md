# Input Limits

Status: done

## Goal

Close the set of unbounded inputs on the public API. Today a caller can submit 80k dates in one availability update, add thousands of participants to a single calendar, create unlimited calendars per user, or send a 1MB calendar name — each of these is only bounded by the generic 1MB body cap, not by anything meaningful to the domain.

The server already has a few shape-level checks (visibility enum, date format, email format) but no count or length caps. This plan adds those caps in one pass so the database, the `/users/me/calendars` list, and the realtime/SSE fan-out can't be cheaply blown up.

## Scope

**In**
- Server-side enforcement of every limit below. 400 for bad shape, 409 for "would exceed a per-resource cap" (already the convention for duplicate-name / already-joined / finalised).
- A single `internal/limits/limits.go` package holding every cap as a named constant. Handlers and service import from here — nothing is inlined.
- Frontend mirror of the same constants in `src/lib/limits.ts` (hand-copied — no build-time generation; both sides change together in one PR).
- Client-side UX only for limits a normal user will hit: `maxLength` on name inputs with a small "47 / 50" counter when within 10 of the cap; a disabled "Add participant" state with a tooltip when the calendar is full.
- Tests: handler-level for each new 400/409 path; service test for the participant-count and calendars-per-user caps; one component test per frontend affordance.

**Out**
- Rate limiting (per-IP / per-user). Different problem — needs Redis or similar infra, and the biggest abuse-by-volume vector (magic-link email send) is already the reason we'd reach for it. Separate plan.
- Calendar archival / deletion to recover headroom once the calendars-per-user cap is hit. If anyone hits 200, they can delete one; adding a proper archive UI is the Tier-3 "expiry / auto-archive" item mentioned in `plans/my-calendars.md`.
- Per-participant availability-row cap as a stored check (e.g. DB constraint). The per-request cap is enough; availability is replace-semantics (`DELETE then INSERT`), so the stored row count can't exceed the per-request cap anyway.
- Magic-link request throttling. Abuse-grade, but out of scope here — same reason as rate limiting.

## Limits

| # | Limit | Value | Where enforced | Why this value |
|---|---|---|---|---|
| 1 | Calendar name length | 100 chars | handler (`CreateCalendar`, `UpdateCalendar`) | Fits a phone screen comfortably; 99th-percentile real names are <40. |
| 2 | Participant name length | 50 chars | handler (`AddParticipant`) | Real names, nicknames, "Alex (desk 3)" — 50 is generous without being silly. |
| 3 | Dates per availability submission | 366 | handler (`SetAvailability`) | One year of daily availability. If someone needs two years they can split the submission; the UI never shows more than ~3 months at once. |
| 4 | Individual date range | `today - 1y` .. `today + 5y` | handler (`SetAvailability` per-date, `FinaliseCalendar`) | Rejects year-9999 nonsense while leaving obvious legit planning windows intact. |
| 5 | Participants per calendar | 100 | service (`AddParticipant`) | Matches the implicit assumption in `plans/my-calendars.md` ("tens of participants"); larger events aren't our use case. |
| 6 | Calendars per user (owner) | 200 | service (`CreateCalendar`) | 2× the `/users/me/calendars` page cap. Generous for power users, finite for abuse. Joining calendars doesn't count — only ownership is rate-limitable. |

Each value is a named constant in `internal/limits/limits.go` so bumping one is a one-line change.

## Design Decisions

**One `limits` package, not constants scattered at call sites.** The list above is small enough that future-me will forget where half of them live if they're inlined. A single file (`internal/limits/limits.go`) with `MaxCalendarNameLength`, `MaxParticipantNameLength`, `MaxDatesPerSubmission`, `MaxParticipantsPerCalendar`, `MaxCalendarsPerUser`, and the date-range helpers means bumping a cap is a one-line change and new handlers have an obvious place to look.

**Handler vs service split follows the existing pattern.** Format and length checks live in the handler (same layer as the existing `time.Parse`, `IsValidVisibility`, `isValidEmail` checks). Count-based caps that require a DB read live in the service (same layer as the existing finalised/duplicate-name checks that flow back as `Err*` sentinels). New sentinel errors:

- `ErrCalendarFull` (participants per calendar)
- `ErrCalendarLimitReached` (calendars per user)

Both surface as 409 in the handler — same convention as `ErrDuplicateName` / `ErrUserAlreadyJoined`. A 400 would be defensible for "limit reached" but 409 reads more naturally: the request is well-formed, it just conflicts with the current resource state.

**Participant cap uses the count we already compute.** `AddParticipant` already calls `participantCount` to pick a colour (`backend/internal/store/participant.go:42`). The cap check piggybacks on that read — zero new round-trips. A race where two clients both see count=99 and both insert is theoretically possible, but the outcome (101 participants) is benign and self-correcting on the next add.

**Calendars-per-user needs a new count query plus an index.** No existing call path counts `WHERE owner_id = $1`, so add `store.CountCalendarsForOwner(ctx, userID) (int, error)` and call it from `service.CreateCalendar` before the insert. One extra SELECT on the create path is fine — calendar creation isn't a hot path. Add `idx_calendars_owner_id` in the same migration: write cost is trivial (`owner_id` never changes after insert), disk footprint is tiny, and it keeps the count query O(log n) instead of depending on total calendar row count. Matches the indexing pattern already used for `participants(user_id)` and `calendars(updated_at)`.

**Date-range check runs per-date, not just on the request.** `SetAvailability` iterates dates anyway for format-parsing (`availability.go:31`). The range check drops into the same loop — O(n) already, no cost change. `FinaliseCalendar` gets the same helper applied to its single date.

**No DB-level CHECK constraints.** Tempting for length caps, but two downsides: (1) error message comes back as a generic constraint violation that the handler has to parse into a user-friendly 400, and (2) changing the cap needs a migration. Handler-level checks are cheaper to evolve and produce nicer errors. The trust boundary is the handler; the DB doesn't need belt-and-braces here.

**Add a machine-readable `code` to error responses.** Today `respondError` returns `{"error": "<human message>"}` only (`backend/internal/handler/respond.go:14`). The frontend needs to branch on "was this a full-calendar 409 or a duplicate-name 409?" — without a stable code, it has to match on English text, which breaks silently if we reword. Extend the helper to `respondError(w, status, code, message)` emitting `{"error": "...", "code": "calendar_full"}`. Existing call sites get a trivial migration: pass a short snake_case code alongside the message. New error codes introduced by this plan: `calendar_full`, `calendar_limit_reached`, `name_too_long`, `too_many_dates`, `date_out_of_range`. Existing codes (backfilled at the same time): `invalid_json`, `not_found`, `already_finalised`, `duplicate_name`, `already_joined`, `forbidden`, `unauthorized`, etc. — one-word-ish, stable, lowercase.

**Frontend mirrors constants by hand.** A shared constants package across Go and TypeScript would need codegen or a shared schema — overkill for six numbers. The frontend imports from `src/lib/limits.ts`; when a server limit changes, both files change in the same commit. Drift risk is low because the server is authoritative: a drifted client just shows a slightly wrong counter, it doesn't break anything.

**Client affordances, not client enforcement.** `maxLength` on an input is a hint, not a guard — paste can exceed it in some browsers, and a hostile client ignores it entirely. The server always runs the same check. Client UX only exists so legitimate users don't hit a 400 as their first signal that something's wrong.

**No UI for the availability-dates cap.** The calendar grid shows ~2 months at a time (<62 cells). Legitimate usage can't hit 366 through the UI, so spending pixels on "you've selected too many dates" is wasted. Server rejects; client doesn't pre-empt.

**No UI for the calendars-per-user cap.** Same reasoning — 200 is way past normal. On the create-calendar form, if the server returns 409 `calendar_limit_reached`, show a friendly error ("You've reached the 200-calendar limit. Delete an old calendar to create a new one."). No proactive counter on the home page.

## Backend Implementation

### Phase 1 — `internal/limits/` package

New file `backend/internal/limits/limits.go`:

```go
package limits

import "time"

const (
    MaxCalendarNameLength        = 100
    MaxParticipantNameLength     = 50
    MaxDatesPerSubmission        = 366
    MaxParticipantsPerCalendar   = 100
    MaxCalendarsPerUser          = 200
)

// DateWithinRange reports whether d is within the acceptable availability/
// finalisation window: today - 1 year .. today + 5 years. Caller passes the
// reference "now" so tests are deterministic.
func DateWithinRange(d, now time.Time) bool {
    min := now.AddDate(-1, 0, 0)
    max := now.AddDate(5, 0, 0)
    return !d.Before(min) && !d.After(max)
}
```

Unit tests for `DateWithinRange` (boundary dates, today, past, far future).

### Phase 2 — handler-level length and count checks

1. `handler/calendar.go:CreateCalendar` — reject `len(name) > MaxCalendarNameLength` with 400.
2. `handler/calendar.go:UpdateCalendar` — same check when `req.Name != nil`.
3. `handler/participant.go:AddParticipant` — reject `len(name) > MaxParticipantNameLength` with 400.
4. `handler/availability.go:SetAvailability`:
   - Reject `len(req.Dates) > MaxDatesPerSubmission` with 400.
   - In the existing `time.Parse` loop, also call `DateWithinRange`; reject out-of-range with 400 "date out of range: <d>".
5. `handler/calendar.go:FinaliseCalendar` — apply `DateWithinRange` after the existing `time.Parse`.

Tests (`handler_test.go`): one 400 case per check, plus one pass-through success case per endpoint to confirm we didn't break the happy path.

### Phase 3 — error-code response shape

1. Extend `respondError(w, status, code, message)` in `handler/respond.go` to emit `{"error": "...", "code": "..."}`.
2. Update every existing call site to pass a snake_case code (listed in Design Decisions). Search with `grep -rn "respondError(" backend/internal/handler/` — should be ~30 sites.
3. Handler tests: one assertion per existing error path that the code field is present and correct. Add a helper `assertErrorCode(t, resp, "not_found")` to keep tests terse.

This phase ships before Phase 4 because the new service caps need the code field to exist.

### Phase 4 — service-level count caps + migration

1. Migration `010_add_input_limit_indexes.sql`:
   ```sql
   CREATE INDEX idx_calendars_owner_id ON calendars(owner_id);
   ```
   (`participants(calendar_id)` already exists via the FK; no second index needed for the participant count.)
2. New sentinel errors in `store/errors.go`: `ErrCalendarFull`, `ErrCalendarLimitReached`.
3. New store method `CountCalendarsForOwner(ctx, ownerID) (int, error)` — single `SELECT COUNT(*) FROM calendars WHERE owner_id = $1`, served by the new index.
4. `service.CreateCalendar` — before calling the store, `CountCalendarsForOwner`; if `>= MaxCalendarsPerUser`, return `ErrCalendarLimitReached`.
5. `service.AddParticipant` — add the cap check in the service (not the store). Service calls `store.CountCalendarParticipants` (already exists), returns `ErrCalendarFull` if `>= MaxParticipantsPerCalendar`. The store-level `participantCount` call inside `AddParticipant` stays — it's for colour selection and is cheap.
6. Handler mapping: `CreateCalendar` → 409 + code `calendar_limit_reached`. `AddParticipant` → 409 + code `calendar_full`.
7. Service tests: mock the store counts, assert the right error flows back for each cap.
8. Handler tests: assert 409 status and the error code string for each cap.

## Frontend Implementation

### Phase 1 — `src/lib/limits.ts`

```ts
export const MAX_CALENDAR_NAME_LENGTH = 100;
export const MAX_PARTICIPANT_NAME_LENGTH = 50;
export const MAX_PARTICIPANTS_PER_CALENDAR = 100;
```

Only the limits the UI actually shows. Others stay server-only.

### Phase 2 — name-length affordances

1. `src/app/page.tsx` calendar-name input: `maxLength={MAX_CALENDAR_NAME_LENGTH}`. No counter — 100 is far past what anyone will type.
2. `src/components/JoinForm/JoinForm.tsx` participant-name input: `maxLength={MAX_PARTICIPANT_NAME_LENGTH}`. When `name.length >= 40`, render a small muted counter `{name.length} / 50` below the input. Mobile-first: right-aligned under the input, not inline with it.
3. Wherever the calendar is renamed (owner UI — check `CalendarPageClient.tsx`), same `maxLength` + counter treatment as participant name.

### Phase 3 — full-calendar state

1. On `CalendarPageClient` / `ParticipantList`, derive `isFull = participants.length >= MAX_PARTICIPANTS_PER_CALENDAR`.
2. Hide or disable the `JoinForm` when `isFull` and show a muted message: "This calendar is full (100 people)."
3. For the owner's invite prompt (`InvitePrompt.tsx`): when full, swap the share CTA for the same message.
4. Server still rejects with 409 if someone races in via the raw API — the client UI is a courtesy.

### Phase 4 — error surfacing

The API client (`src/lib/api.ts`) already throws on non-2xx. Callers of `createCalendar`, `addParticipant`, `setAvailability`, `finaliseCalendar` need to map the new 400 / 409 codes to user-friendly messages:

- `calendar_limit_reached` → "You've reached the 200-calendar limit. Delete an old calendar to create a new one."
- `calendar_full` → "This calendar is full (100 people)."
- Availability / finalisation range / date-count errors → surface the server's message directly; they're rare enough not to warrant bespoke copy.

One component test per new error mapping.

## Testing

### Backend
- Limits package: `DateWithinRange` boundary cases.
- Each handler 400 path: too-long name, too-many dates, out-of-range date, finalise-date out of range.
- Service caps: 100th participant succeeds, 101st returns `ErrCalendarFull`; 200th calendar succeeds, 201st returns `ErrCalendarLimitReached`.
- Handler mapping: 409 + correct error code for each service cap.
- Integration (optional): real Postgres + real counts to confirm the service checks work end-to-end.

### Frontend
- `JoinForm` renders the counter once `name.length >= 40` and not before.
- `ParticipantList` or host component hides `JoinForm` when at the cap.
- API client surfaces `calendar_full` and `calendar_limit_reached` as the expected user-facing strings.

### Manual
- Paste a 500-char name into the calendar-name input → truncated to 100 client-side; server would reject anyway.
- Add the 100th participant → succeeds; try the 101st → server 409, client shows "full" message.
- Submit availability with 400 dates via the API directly (`curl`) → 400 with "too many dates".
- Submit availability with a date in year 2099 → 400 with "date out of range".
- Create 200 calendars (scripted) → 201st returns 409.

## Build Order

1. Backend Phase 1 (`internal/limits/`).
2. Backend Phase 3 (extend `respondError` with `code` + backfill existing sites). Ships before Phase 4 so the new caps have a stable code to emit.
3. Backend Phase 2 (handler length/count/date checks + tests).
4. Backend Phase 4 (migration + service count caps + new errors + tests).
5. Frontend Phase 1 (`src/lib/limits.ts`).
6. Frontend Phase 2 (name `maxLength` + counter).
7. Frontend Phase 3 (full-calendar state).
8. Frontend Phase 4 (error mapping against the new `code` field).
9. Manual QA matrix.

## Open Questions

None outstanding. Resolved during planning:

- Error response shape — adding a `code` field to `respondError` as Phase 3 (see Design Decisions).
- `idx_calendars_owner_id` — added in Phase 4 migration; write cost is trivial and it future-proofs the owner count query.
- Whether joined (non-owned) calendars count toward the 200 cap — no. The cap is per-owner only; a user can participate in unlimited calendars they didn't create.

## Log

- 2026-04-20 17:00 — Created plan.
- 2026-04-20 17:15 — Resolved open questions: added `idx_calendars_owner_id` to the Phase 4 migration; promoted the error-code response shape into a dedicated Phase 3 with a `{error, code}` body; clarified that the 200 cap is owner-only. Removed the speculative per-visibility-cap question.
- 2026-04-20 18:45 — Implemented all phases. Added `internal/limits` package; extended `respondError` with a machine-readable `code` and backfilled every existing handler call site; added handler-level length/count/date-range checks; added migration 010 with `idx_calendars_owner_id`; added `ErrCalendarFull` / `ErrCalendarLimitReached` sentinels and `CountCalendarsForOwner`; service now enforces the per-owner and per-calendar caps. Frontend: added `src/lib/limits.ts`, plumbed `code` through `ApiError`, added `maxLength` on the calendar-name, rename, and join inputs, added a live counter on the join input near the cap, and gated the join form behind a "calendar is full" message. New tests: limits package boundary cases, handler tests for each new 400/409 path, service tests for both caps, JoinForm counter tests, and an `ApiError.code` round-trip test.
