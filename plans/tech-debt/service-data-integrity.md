# Service data integrity

Status: done

## Goal

Stacked on `service-layer-authz-validation.md`. Fixes a set of race and
partial-write bugs found during the service-layer review, and moves the
remaining account logic out of the handlers:

1. Move findOrCreateUser into the service. Make first sign-in race-safe:
   concurrent sign-ins for a new email used to 500. Keep #73's
   `email_verified` rules.
2. Make participant rename + email update a single atomic write. Keep #74's
   authorization rules.
3. Move the `SetAvailability` finalised check inside the transaction, under a
   calendar row lock, and batch-insert the dates with `unnest`.
4. Give the middleware session refresh a timeout and a warning log. Log
   `MatchInviteByName` failures instead of discarding them.
5. Delete the unused `Handler.flags` / `WithFlags`.
6. The duplicate-calendar "full" check should count only the participants that
   will actually be copied (signed-in members other than the new owner).

## Plan

- Add `service.Accounts` (`FindOrCreateOAuthUser`, `FindOrCreateByEmail`).
  `store.CreateUser` uses `ON CONFLICT (email) DO NOTHING` and returns
  `model.ErrUserExists`, and the service then re-reads the existing row. Only an
  id collision is retried.
- Add `store.UpdateParticipantNameAndEmail`, one `UPDATE` that shares its SQL
  with `UpdateParticipantName`. `service.RenameParticipant` takes an optional
  email, validates it before writing anything, and uses the combined write for
  anonymous callers.
- Add a `lockOpenCalendarForParticipant` helper (`FOR SHARE OF c`) used by
  `SetAvailability` and `MergeAvailabilityDates`. Inserts become a single
  `INSERT ... SELECT FROM unnest(...)`.
- In the middleware, run `refreshSession` with `context.WithoutCancel` plus a
  5s timeout, and log failures at warn with no PII. Add a
  `matchInviteBestEffort` helper that logs at warn.
- Remove the flags field, `WithFlags` and the server wiring call.
- Add `store.CountDuplicableParticipants`. `DuplicateCalendar` checks
  `copied + 1 > cap`.
- Tests:
  - Unit tests: accounts (including the simulated race), rename+email, the
    duplicate count, and the handler contract.
  - Integration tests: concurrent sign-in, atomic rename+email over HTTP,
    availability writes blocking on an in-flight finalise, the anonymous
    duplicate cap, and a 366-date batch.

## Notes

- The signed-in "edit email under account settings" check stays in the handler's
  email-only path. #74's service semantics let the owner edit any participant's
  email, so moving it would have changed behaviour.
- The server-level flag evaluator / `flags` package / `POSTHOG_*` backend env
  vars are left alone: removing them touches Terraform-managed config.
- Behaviour changes (documented in `docs/api.md`):
  - A PATCH with name + invalid email no longer persists the name.
  - Duplicating a calendar full of anonymous participants now succeeds (201)
    instead of a spurious `calendar_full`.

## Log

- 2026-11-12 14:52 — Plan written up after implementation (should have come first; noted for honesty). Work done in this session: Accounts service + race-safe CreateUser; atomic rename+email (first try moved the signed-in email check into the service and broke #74's owner-allowed test, so it went back to the handler); availability finalised check under a FOR SHARE lock + unnest batch inserts; session-refresh timeout/log; invite auto-match logging; removed WithFlags; duplicate cap counts only copied participants; integration tests (TestDuplicateCalendar_CalendarFull rewritten since anonymous fills now correctly return 201).
- 2026-11-12 14:55 — Full verification passed (build, vet, vet integration, unit, -p 1 integration, golangci-lint on touched files). Updated docs/api.md + onboarding.html.
