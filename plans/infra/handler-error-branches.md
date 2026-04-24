# Handler error-branch coverage

Status: done

## Goal

Close the small set of handler error branches that are awkward or impossible to trigger from the current integration tests — without adopting a `ServiceInterface` + hand-rolled fake across the whole handler package.

The previous coverage pass (`plans/infra/test-coverage.md`) brought every 0%-coverage package up to 77–96% and raised `server` to 92.7% via new integration tests. The remaining gaps in `handler` aren't a shape problem — they're specific branches that need either contrived real state or an external-service stub.

## What's actually uncovered

From `go tool cover -func` on the merged handler+server+service+store profile, the branches that are hit by neither the handler's own unit tests nor the integration suite fall into three buckets:

1. **Limit-triggered 409s** — need contrived real state to reach:
   - `POST /calendars` → `ErrCalendarLimitReached` (user at calendar cap)
   - `POST /calendars/{id}/duplicate` → `ErrCalendarLimitReached` / `ErrCalendarFull`
   - `POST /calendars/{id}/participants` → `ErrCalendarFull`
   - `POST /calendars/{id}/invites` → `ErrInviteLimitReached`
2. **Rate-limited 429s** — need burst behaviour:
   - `POST /calendars/{id}/unlock` → 429 after N wrong attempts per (calendar, IP)
3. **Out-of-scope for now** (confirmed non-goals in the previous plan):
   - Google OAuth login / callback (`GoogleLogin`, `GoogleCallback`)
   - Magic link send / callback (`SendMagicLink`, `MagicLinkCallback`, `findOrCreateEmailUser`)
   - Google Calendar import handlers (`ConsentAck`, `Connect`, `Callback`, `GetSuggestions`, `ImportAvailability`)

The first two buckets are ~5–6 distinct error branches. The third bucket is a separate plan about stubbing Google/Resend for handler tests — deferred.

## Approach

Two paths considered; picking (A) because the tax of (B) doesn't pay off at our current service size.

**(A) Integration helpers that seed "extreme" state. Chosen.**
- Add small test helpers that fill a calendar to the participant cap, create a user at the calendar cap, drive the invite list to its limit, etc.
- Add a `TestUnlock_RateLimited` that issues N+1 wrong-passphrase attempts back-to-back and asserts the 429 + `Retry-After` header.
- No new abstractions. Lives alongside the existing `*_integration_test.go` files.

**(B) Extract `service.Interface` + hand-rolled fake.** Not doing this now.
- Closes the same branches trivially but adds a permanent maintenance tax: every new service method has to be added to the interface and the fake forever.
- The handler error-translation code is a simple `switch err` that reads obviously; unit-testing it mostly asserts the same shape three times (handler says `errors.Is(err, X) → Y`, fake returns `X`, test asserts `Y`).
- Worth revisiting only if we start shipping service-level behaviour fast and hitting 5xx leaks that the current tests miss.

## Plan

1. Pull the actual limit values from `internal/limits/` so the seeding helpers don't hardcode magic numbers. If any limit is so high that seeding is expensive, lower it via a test-only knob rather than inserting thousands of rows — but prefer seeding if it's cheap.
2. Add to `server_integration_test.go` (or a new `limits_integration_test.go`):
   - `fillCalendarToCap(t, ts, calendarID)` helper that adds participants up to `MaxParticipantsPerCalendar - 1` (leaving room for a final 409 probe).
   - `createCalendarsUpToCap(t, ts, userToken)` helper for the per-user calendar limit.
3. Write tests:
   - `TestAddParticipant_CalendarFull` — seed to cap, next add → 409 `calendar_full`.
   - `TestCreateCalendar_LimitReached` — seed to cap, next create → 409 `calendar_limit_reached`.
   - `TestDuplicateCalendar_LimitReached` — seed to cap, duplicate → 409.
   - `TestAddInvites_LimitReached` — seed invites to cap, next add → 409 `invite_limit_reached`.
   - `TestUnlock_RateLimited` — set passphrase, fire N+1 wrong attempts from the same IP, assert 429 + `Retry-After` on the last one, and that a correct passphrase from a *different* IP still unlocks.
4. Re-measure. Expected delta: handler `CreateCalendar`, `AddParticipant`, `DuplicateCalendar`, `AddInvites`, `Unlock` each pick up their 409/429 branches. `handler` own-unit stays ~35% (that's fine — it's just the surface area). Combined handler coverage via `-coverpkg` should move from ~60% average per function into the 80s on these handlers.

### What this plan deliberately does **not** do

- No `ServiceInterface` extraction. If we later decide (B) is worth it, it's a separate plan.
- No tests for scoped-out handlers (Google OAuth, magic link, Google Calendar import). Those need external-service stubs and are a different piece of work.
- No coverage threshold in CI. Same reason as last time — get the numbers up first; gate later if we want to.

## Log

- 2026-04-24 — Created plan after discussion: option (A) over option (B); scope is limit-triggered 409s + unlock 429.
- 2026-04-24 15:14 — Added `backend/internal/server/limits_integration_test.go` with seeding helpers (`fillCalendarParticipantsToCap`, `createCalendarsUpToCap`, `fillInvitesToCap`) and six tests: `TestAddParticipant_CalendarFull`, `TestCreateCalendar_LimitReached`, `TestDuplicateCalendar_LimitReached`, `TestDuplicateCalendar_CalendarFull`, `TestAddInvites_LimitReached`, `TestUnlock_RateLimited`. Helpers seed directly via the `store` package against the shared embedded-Postgres test DB so the HTTP layer only covers the final rejection. Unlock rate-limit test uses `X-Forwarded-For` to exercise the per-(calendar, IP) keying. `make lint` and `make test-integration` green.
