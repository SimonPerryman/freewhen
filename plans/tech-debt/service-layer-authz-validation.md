# Service-layer authorization and validation

Status: done

## Goal

Make the service layer the single place that decides "may this caller do
this?" and "is this input valid?", so handlers shrink to decode → call
service → map error. Today the calendar-owner check is copy-pasted across
~8 handlers (each loads the calendar just to compare `OwnerID`), validation
for availability notes/preferred dates and participant renames lives in
handlers, every handler hand-rolls its JSON decode, and handlers import
`store` purely for sentinel errors and a list-row type.

External API behaviour (status codes, error codes, JSON shapes) stays the
same. The one deliberate edge-case shift: owner-only invite endpoints now
decode the body before the owner check, so a non-owner sending malformed
JSON gets `400 invalid_json` instead of `403` (both are rejections; no
data is exposed or mutated either way).

## Plan

1. **Owner authorization in the service.**
   - `service.requireOwner(ctx, calendarID, userID)` loads the calendar and
     returns `model.ErrCalendarNotFound` (wraps `ErrNotFound`) or
     `ErrForbidden`.
   - Owner-only service methods take the caller's user id and call it:
     `DuplicateCalendar`, `FinaliseCalendar`, `UnfinaliseCalendar`,
     `RenameCalendar`, `RemoveParticipant`, `RotateCalendarPassphrase`,
     `ClearCalendarPassphrase`, `GetMissingView`, `AddInvites`,
     `RenameInvite`, `DeleteInvite`, `ManualMatchInvite`, and
     `DeleteCalendar` (already did it inline).
   - Public `AuthorizeCalendarOwner` for the nudge handler, whose action
     lives in `notifications.Service`.
   - The invite "belongs to this calendar" check moves into the service too.
   - Handlers drop `requireCalendarOwner` and all inline `OwnerID` checks;
     `ErrCalendarNotFound` → `respondCalendarMissing` (404/410) so the
     participant-not-found 404 on `RemoveParticipant` stays distinct.
2. **Validation in the service.**
   - `service.ValidationError{Code, Message, Err}` carries the stable API
     error code; handlers map it with one `errors.As`.
   - `SetAvailability` takes an `AvailabilityUpdate{Dates, Notes, Preferred}`
     and owns participant-id / date-count / date-format / range / note /
     preferred rules; `ImportAvailability` shares the date rules.
   - `RenameParticipant` owns empty / too-long name rules, checked before
     the calendar is loaded (same order as the old handler).
   - Service unit tests for every rule.
3. **`decodeJSON(w, r, &v) bool`** helper; replace the hand-rolled decode
   blocks. Drop per-handler `MaxBytesReader(1<<20)` now that `limitBody`
   wraps every route (no handler had a smaller limit).
4. **Handlers stop importing `store`.**
   - Sentinel errors move to `model/errors.go`; `store/errors.go` keeps
     aliases (`store.ErrNotFound = model.ErrNotFound`) so store/service code
     and tests keep compiling and `errors.Is` is unchanged.
   - `store.CalendarListItem` → `model.CalendarListItem`.
5. Update `docs/onboarding.html` (handler/service responsibilities). No API
   change, so `docs/api.md` only gets the invalid-JSON ordering note if it
   documents ordering (it doesn't).

## Log

- 2026-11-12 13:30 — Created plan
- 2026-11-12 14:00 — Sentinel errors + `CalendarListItem` moved to `model`; store keeps aliases. `service.requireOwner` / `AuthorizeCalendarOwner` added; 13 owner-only service methods now take the caller id. Invite "belongs to calendar" check moved into the service; `DeleteInvite` swallows the concurrent-delete `ErrInviteNotFound` itself (still 204).
- 2026-11-12 14:15 — `ValidationError` + `AvailabilityUpdate`; availability/import/rename-participant rules moved to the service (validated before any load, preserving the old handler order). `AddInvites` `names_required` moved after the owner check, matching the old order.
- 2026-11-12 14:25 — `decodeJSON` / `decodeJSONMessage` replace all 16 hand-rolled decodes; 12 `MaxBytesReader(1<<20)` calls dropped (all equal to `limitBody`'s cap). No handler imports `store` any more (tests included).
- 2026-11-12 14:35 — Handler validation tests ported to `service/validation_test.go`; new `service/owner_test.go` table-tests every owner-only op (non-owner, anonymous, missing calendar). Handler mock mirrors `requireOwner` so the existing 403/404 handler tests still run. `go test ./...` and `go test -p 1 -tags integration ./...` green.
- 2026-11-12 14:40 — Status → done
