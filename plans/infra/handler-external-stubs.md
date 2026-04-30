# Handler coverage for external-service endpoints

Status: done

## Goal

Close the remaining uncovered handlers — magic link, Google OAuth login, and
Google Calendar import — that the previous two coverage passes deferred as
"needs external-service stubs". Adds the missing seams and the integration
tests that exercise them.

These handlers were out of scope in `plans/infra/test-coverage.md` and
`plans/infra/handler-error-branches.md` because hitting their error branches
requires faking Resend, Google OAuth (token + userinfo endpoints), and
Google Calendar (FreeBusy). That's a different shape of work: a small
refactor to make the external clients injectable plus one HTTP-stubbing
helper shared across the three tests.

## What's uncovered

From the last `go tool cover -func` pass, the 0%-ish handlers are:

1. **Magic link** — `SendMagicLink`, `MagicLinkCallback`, `findOrCreateEmailUser`.
2. **Google OAuth login** — `GoogleLogin`, `GoogleCallback`, `findOrCreateUser`.
3. **Google Calendar import** — `ConsentAck`, `Connect`, `Callback`,
   `GetSuggestions`, `ImportAvailability`.

Each has its own error branches (bad state, wrong nonce, exchange failure,
userinfo decode failure, FreeBusy failure, rate-limit on `Connect`, etc.)
that the current integration suite can't reach.

## Seams — what's already there, what's missing

- **Magic link**: `handler.EmailSender` is already an interface
  (`backend/internal/handler/auth.go:38`). No refactor — tests inject a fake
  sender that captures the link; then the test drives
  `GET /auth/magic-link?token=...` with the captured token.
- **Google OAuth login**: `AuthHandler.oauth` is built inside `NewAuth` from
  `cfg.GoogleClientID/Secret/RedirectURL` with `google.Endpoint` hardcoded.
  No seam. **Refactor needed**: accept an optional `oauth2.Endpoint` override
  (or a preconstructed `*oauth2.Config`) so tests can point token / auth URLs
  at an `httptest.Server`. Userinfo is fetched via
  `h.oauth.Client(ctx, tok).Get("https://www.googleapis.com/oauth2/v3/userinfo")`
  — add a `userInfoURL` field on `AuthHandler` defaulting to the Google URL
  so tests can override it.
- **Google Calendar import**: `CalendarIntegrationHandler.freeBusy` is
  already a `FreeBusyFetcher` seam (`calendar_integration.go:45`). Good.
  `CalendarIntegrationHandler.oauth` has the same problem as `AuthHandler` —
  needs the same endpoint override.

## Approach

One shared test helper (`fakeGoogle`) that spins up an `httptest.Server` with
the three endpoints Google exposes during OAuth: `/o/oauth2/v2/auth` (auth
page — the test harness doesn't actually hit this, but the URL needs to be
resolvable), `/token` (exchange), and `/oauth2/v3/userinfo`. Configurable
responses per test (happy, 500, malformed JSON). The fake also signs no
tokens — production uses bearer tokens, so we just return a canned access
token string and verify the `Authorization: Bearer <...>` header on the
userinfo call.

For the calendar integration, the same fake Google also serves
`/calendar/v3/freebusy`, consumed via the existing `FreeBusyFetcher` seam —
no need to wire the fake through the `oauth2.Config` twice.

Resend is a straight `http.Client.Post` inside `email.Sender`. The handler
test doesn't need to stub Resend at all — it swaps in a test
`EmailSender`. The `email.Sender` unit tests already cover the Resend wire
shape.

## Plan

### Phase 1 — Magic link (no refactor)

1. New test file `backend/internal/handler/auth_magic_link_test.go` (unit
   test — `AuthHandler` is wired with a fake store + fake `EmailSender`).
2. Cases:
   - Happy path: POST `/auth/magic-link` with a new email → 202, sender
     received one call with a link containing the raw token, follow-up GET
     `/auth/magic-link?token=<raw>` → 200 + session cookie set, user row
     created via `findOrCreateEmailUser`.
   - Returning user (email already exists from a prior OAuth signup) →
     session issued, no duplicate user.
   - Invalid email → 400 `invalid_email`.
   - Rate limiter (if present on `SendMagicLink`) → 429 after N sends.
   - Callback with unknown / expired token → 400 or 401 per current handler.
   - Callback with return_url that isn't a relative path → falls back to
     `/` and still redirects.
3. `EmailSender` failures: make the fake return an error → handler should
   **not** leak the existence of an email (per common anti-enumeration
   practice) — confirm current behaviour and lock it in.

### Phase 2 — Google OAuth (refactor + test)

1. Refactor `NewAuth` to accept an optional `oauthOverrides` struct (or
   functional options) exposing `Endpoint` and `UserInfoURL`. Production
   call site in `server.go` unchanged (passes nil / defaults). Keep
   `google.Endpoint` and the Google userinfo URL as defaults.
2. New `backend/internal/handler/auth_google_test.go`. Spin up
   `httptest.Server` serving `/token` and `/userinfo`. Wire the handler
   with `Endpoint{TokenURL: fake+"/token", AuthURL: fake+"/auth"}` and
   `UserInfoURL: fake+"/userinfo"`.
3. Cases:
   - `GoogleLogin`: redirect to `AuthURL` with encoded state; nonce cookie
     set; `return_url` sanitised (absolute URL rejected).
   - `GoogleCallback` happy path: fake `/token` returns canned access
     token, `/userinfo` returns `{sub,email,name}`, handler creates user,
     issues session, redirects to `frontendURL+return_url`.
   - Missing / mismatched nonce → 400.
   - Missing code → 400.
   - `/token` returns 500 → handler 500 `authentication failed` (no token
     details leaked).
   - `/userinfo` returns 500 or malformed JSON → handler 500.
   - Returning user (same provider_id) → `findUserByProviderID` path, no
     duplicate insert.
   - Email collision with an existing magic-link user on the same email →
     verify current behaviour (link-by-email upgrade vs 409 vs insert).
     Whatever it does, lock it in.

### Phase 3 — Google Calendar import (refactor + test)

1. Same endpoint-override refactor on `NewCalendarIntegration`. Keep the
   existing `FreeBusyFetcher` seam.
2. New `backend/internal/handler/calendar_integration_test.go`. Wire the
   handler with the fake OAuth `Endpoint` and inject a `FreeBusyFetcher`
   that the test controls directly (bypass the HTTP fake for FreeBusy —
   it's already a pure function seam).
3. Cases:
   - `ConsentAck`: unauth → 401; auth → 204 and store method invoked once.
   - `Connect`: no calendar_id → 400; bad `return_url` → coerced to
     `/calendars/{id}`; OAuth not configured (`h.oauth == nil`) → 503;
     rate-limited on 11th call in an hour → 429.
   - `Callback`: invalid base64 state → 400; bad JSON state → 400; nonce
     mismatch → 400; token exchange 500 → redirect back with error query
     param (verify current UX); `FreeBusyFetcher` returns error → same.
   - `GetSuggestions` / `ImportAvailability`: drive via a preseeded
     single-use cookie (the one Callback sets). Cover: missing cookie → 400;
     expired / bad signature → 400; happy path → expected JSON shape and
     the availability merge call.

### Phase 4 — Verify and trim

1. Re-measure with `-coverpkg=./internal/handler,./internal/service,./internal/server`.
2. Delete any now-redundant unit tests in `handler_test.go` that the
   integration-style suite subsumes (don't proactively delete — only if
   there's obvious duplication).
3. Add one-line comment on each seam field explaining it exists for tests.

### What this plan deliberately does **not** do

- No `ServiceInterface` extraction (still deferred — option B from the
  previous plan).
- No real HTTP calls to Google / Resend in CI.
- No coverage threshold gate.

## Log

- 2026-04-24 — Created plan after the `handler-error-branches` work
  landed. Scope is the three buckets the previous plan explicitly
  deferred. Shape: small seam refactors (`oauth2.Endpoint` override on
  both auth handlers) plus a shared `fakeGoogle` `httptest.Server` helper.
- 2026-04-30 — Implemented all three phases. Phase 1: `auth_magic_link_test.go`
  (10 cases, no refactor needed). Phase 2: added functional options
  `WithOAuthEndpoint`/`WithUserInfoURL` on `NewAuth`, replaced the hardcoded
  userinfo URL with `h.userInfoURL`; new `auth_google_test.go` with shared
  `fakeGoogle` helper covers login redirect, callback happy/returning/email-
  collision paths, and exchange/userinfo/JSON failure modes (13 cases).
  Phase 3: matching options `WithCalendarOAuthEndpoint`/`WithFreeBusyFetcher`
  on `NewCalendarIntegration`; `calendar_integration_test.go` covers
  ConsentAck (3), Connect (5 inc. rate limit), Callback (6), GetSuggestions
  (6). Coverage of the previously 0% handlers: SendMagicLink 95%,
  MagicLinkCallback 74%, findOrCreateEmailUser 83%, GoogleLogin 86%,
  GoogleCallback 80%, findOrCreateUser 82%, ConsentAck 100%, Connect 86%,
  Callback (calendar) 85%, GetSuggestions 92%. `make lint`, `make test`,
  and `make test-integration` all pass.
