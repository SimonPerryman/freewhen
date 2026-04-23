# Test coverage pass

Status: in-progress

## Goal

Raise confidence in the test suite without over-engineering it. Three things:

1. Fix a flaky backend test (`TestVerify_RejectsTampered`).
2. Get integration tests to cover every HTTP route, not just the core calendar flow.
3. Add unit tests for internal packages that currently have none, and make them testable where that requires a small design nudge.

Current state (2026-04-23):

- Frontend: **72% lines / 70% statements** (219 tests, 23 files). `@vitest/coverage-v8` added.
- Backend unit: strong in `availability` (95%), `ics` (96%), `limits` (100%), `ratelimit` (88%), `realtime` (98%), `passphrase` (88%). Weak in `handler` (35%), `service` (69%). Zero in `config`, `email`, `flags`, `google`, `model`, `cmd/server`, and `server` (unit; integration covers 86%).
- Backend integration: covers ~8 of ~39 routes.

## Plan

### 1. Fix `TestVerify_RejectsTampered` (flake — fails ~60% of runs)

Root cause: the test flips a single trailing base64 character. Because `RawURLEncoding` has unused tail bits for some input lengths, and the bit being flipped sometimes falls in those bits, the decoded bytes can round-trip unchanged. Fix: decode the token, replace the signature segment with a known-bad value, re-encode. Deterministic and still tests what it claims to test.

### 2. Integration test coverage — fill the route gaps

The `internal/server/server_integration_test.go` harness (`testdb` + `httptest.NewServer`) is already the right shape. Expand it.

**Already covered** (keep as-is): `POST /calendars`, `GET /calendars/{id}`, `POST /calendars/{id}/participants`, `POST /calendars/{id}/participants/{pid}/claim`, `PUT /calendars/{id}/availability`, `POST /calendars/{id}/finalise`, `GET /admin/metrics`, `GET /users/me/calendars`.

**Gaps to add, grouped by priority:**

- **Calendar CRUD round-trip:** `PATCH /calendars/{id}`, `POST /calendars/{id}/duplicate`, `DELETE /calendars/{id}/finalise`, `DELETE` + `PATCH /calendars/{id}/participants/{pid}`.
- **Passphrase / access tokens** (security-sensitive, zero integration coverage today): `POST /calendars/{id}/passphrase`, `DELETE /calendars/{id}/passphrase`, `POST /calendars/{id}/unlock`, plus re-verify that a passphrase-gated calendar rejects `GET /calendars/{id}` without a valid access token and accepts it with one. Epoch-rotation on passphrase change is the invariant worth pinning down.
- **Invites / missing:** `GET /calendars/{id}/missing`, `POST /calendars/{id}/invites`, `PUT`/`DELETE /calendars/{id}/invites/{inviteId}`, `POST /calendars/{id}/invites/{inviteId}/match`.
- **Exports:** `GET /calendars/{id}/export.ics` (assert Content-Type + a couple of lines of ICS), `GET /calendars/{id}/export/google` (assert redirect URL shape).
- **Health:** `GET /health` (200 with DB up — trivial, good smoke test).
- **Auth happy paths we can test without hitting Google:** `GET /auth/me` (with and without cookie), `POST /auth/logout`, `PATCH /auth/me`. Magic link send + callback flow can be tested by stubbing the email sender (see §3 on `email`).

**Deliberately out of scope for integration tests:**

- Google OAuth login (`/auth/google`, `/auth/google/callback`) — depends on Google identity server. Test the redirect shape and callback parsing at unit level instead.
- Google Calendar integration endpoints (`/auth/google/calendar/*`, `/import/suggestions`, `/availability/import`) — the handler surface can be integration-tested with a stub `freebusy` client (see §3 on `google`); the OAuth dance itself is a unit test on parser/state handling.
- SSE `GET /calendars/{id}/events` — keep the existing unit tests in `realtime` (97.6%); a minimal integration test that connects, asserts `Content-Type: text/event-stream`, receives one event after a write, and disconnects is nice-to-have but low value.

### 3. Unit test coverage — and the testability question

The packages sitting at 0% split into three groups. The user is right to ask whether the code shape is the blocker.

**Already testable — just nobody wrote tests:**

- `internal/server/middleware.go` — `cors`, `requireAuth`, `requireAdmin`, `optionalAuth`, `requestLog`, `generateRequestID`. Standard Go: wrap a tiny `http.HandlerFunc` that records what it sees, fire `httptest.NewRecorder()`. No refactor needed. High value: auth middleware is security-critical.
- `internal/config/config.go` — env var parsing. Use `t.Setenv` + table tests. Trivial.
- `internal/google/freebusy.go` — **already accepts `*http.Client`**. Only blocker is the hardcoded `https://www.googleapis.com/...` URL. Lift that to a package-level `var freebusyURL = "..."` so tests can point it at an `httptest.Server`. This is the standard Go pattern (see `net/http/httptest` docs examples).

**Need a small design nudge to become testable:**

- `internal/email/resend.go` — same story as `google`. The URL is hardcoded and `client` is a private field. Either (a) lift URL to a package var, or (b) accept an `http.RoundTripper` in `NewSender`. Once done, test with `httptest.Server`. The handler-level code that *uses* `Sender` should depend on an interface (`type MagicLinkSender interface { SendMagicLink(to, link string) error }`) so magic-link handler tests can pass a fake. This is the normal Go approach — the stdlib does it for `http.Transport`, `database/sql/driver`, etc.
- `internal/flags/posthog.go` — wrap the PostHog client behind an interface we own (`type Client interface { IsFlagEnabled(...) bool; ... }`). Test code uses an in-memory fake; prod code wires the real PostHog SDK. Again standard.

**Not worth testing directly:**

- `internal/model` — just struct definitions and small helpers. Coverage of 0% here is fine; they're exercised transitively by every other test.
- `cmd/server/main.go` — wiring. Integration tests cover the wired behaviour.
- `internal/store` at 0.7% unit — deliberate; it's exercised via integration tests against real Postgres (54% there). Leave.

**Is this the normal way?** Yes. Go's standard approach to IO-heavy code:

1. Accept dependencies (HTTP client, DB, clock) as parameters or struct fields.
2. Define interfaces at the *consumer* side, not the producer — small, focused, one method where possible.
3. For external HTTP, test against `httptest.Server` rather than mocking the client.
4. Extract pure functions (parsing, transformation) from IO so they can be tested without any setup.

None of the refactors above are invasive — mostly one-line changes (package-level URL var, extract an interface). Worth doing because the current shape makes these packages untestable without fakes reaching into private fields.

### 4. Handler and service packages

`handler` at 35% and `service` at 69% are not 0% — they have tests, just incomplete. Lower priority than §1–§3 because integration tests hit the handler layer anyway. Audit after §2 lands: anywhere integration coverage + unit coverage together still leaves a handler branch uncovered, add a focused handler unit test.

### Order of work

1. Fix the flaky token test. (minutes)
2. Add a small handful of integration tests for the highest-value gaps — passphrase flow, invites, exports, health. (larger)
3. Unit tests for packages currently at 0%:
   - `server/middleware` — no refactor, just write tests. Auth middleware is security-critical; do this first.
   - `config` — no refactor, `t.Setenv` table tests.
   - `google/freebusy` — lift hardcoded URL to a package var, then test against `httptest.Server`.
   - `email/resend` — lift URL to a package var, extract a `MagicLinkSender` interface for handler-side tests, then test the Resend client against `httptest.Server`.
   - `flags/posthog` — wrap PostHog SDK behind a `Client` interface we own, then test the wrapper and use the interface in consumer tests.
4. Stand back and re-measure. Decide whether `handler` and `service` need more targeted work.

### Non-goals

- No coverage thresholds in CI yet. Get the numbers up first; ratchet later if we want to.
- No e2e tests. Integration + unit is enough for the current scope.
- No mocking frameworks. Plain `httptest` and hand-rolled fakes are the right fit.

## Log

- 2026-04-23 22:10 — Created plan. Current numbers: frontend 72% lines, backend unit coverage varies 0–100% by package with integration bringing `server` to 86% and `store` to 54%. Integration covers 8/39 routes. Flaky test `TestVerify_RejectsTampered` reproduces ~60% of runs.
- 2026-04-23 22:18 — Fixed `TestVerify_RejectsTampered`: decode token, replace signature segment with a known-bad value, re-encode. Confirmed deterministic over 20 runs.
