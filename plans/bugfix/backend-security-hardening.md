# Backend security hardening (export gate, email_verified, body limits)

Status: done

## Goal

Close three findings from a read-only backend review:

- **M3** — `GET /calendars/{id}/export.ics` and `GET /calendars/{id}/export/{provider}`
  skip the passphrase gate. A leaked link to a passphrase-protected,
  finalised calendar exposes its title, date and attendee names + notes
  (via `calendarEventDescription`).
- **M4** — the Google identity provider ignores `email_verified`, and
  `findOrCreateUser` links accounts by email. An identity with an unverified
  email could be linked to (take over) an existing magic-link account.
- **L1** — several JSON handlers (`UpdateMe`, `SendMagicLink`,
  `MagicLinkVerify`, `Nudge`, `ConsentAck`) read bodies without
  `http.MaxBytesReader`.

## Plan

### M3 — gate exports with AuthorizeAccess

- Wrap both export routes in `handleOptional` so the owner's session bypasses
  the gate (same as `GET /calendars/{id}`).
- Call `service.AuthorizeAccess` before rendering. Token is read from the
  `X-Calendar-Access` header, falling back to an `?access=` query param.
  On `ErrPassphraseRequired` return `401 passphrase_required`.
- **Choice: token via query param (not omitting attendee data).** The export
  buttons are plain `<a href>` navigations (a download and a new-tab redirect
  to Google) that can't set headers. The SSE stream already accepts the same
  token via `?access=` for exactly this reason, so this reuses an established
  pattern rather than inventing a second one. Omitting names/notes would still
  leak the title and date to anyone holding the link, so it doesn't actually
  close the hole. The token is a scoped, epoch-bound artefact (rotating the
  passphrase invalidates it), not the passphrase itself, and the export
  responses are `Cache-Control: no-store` with `Referrer-Policy: no-referrer`.
- Frontend `ExportButtons` appends `?access=<token>` when a token is stored
  for the calendar (read after mount to avoid a hydration mismatch).

### M4 — require verified email for email-based linking/creation

- Add `EmailVerified bool` to the generic `idprovider.Identity`.
- Google provider decodes `email_verified` from userinfo.
- `findOrCreateUser`: provider-sub match still wins regardless (the
  provider account is already linked). Otherwise, if the email is not
  verified (or empty), return a sentinel `errEmailNotVerified` and the
  callback responds `403 email_not_verified` — no email-match linking and
  no new user row with that email.

### L1 — global body-size limit middleware

- New `limitBody` middleware in `internal/server` (1 MiB, matching existing
  `1<<20` usage) applied in `ServeHTTP` to every route.
- Requests with `Content-Length` over the limit get `413
  request_too_large` JSON before reaching the handler. Bodies without a
  length (chunked) are wrapped in `http.MaxBytesReader` so reads fail at the
  limit. Existing per-handler limits stay.

### Tests

- Handler tests: exports return 401 for gated calendars without a token,
  succeed with a header or `?access=` token, and for the owner.
- idprovider: `email_verified` decoded.
- Auth: unverified email doesn't link to an existing email user and doesn't
  create one; sub match still works when unverified.
- Middleware: oversize Content-Length → 413; oversize chunked body →
  read error; small body passes.
- Route coverage test updated (exports now `optional`).

### Docs

- `docs/api.md`: export endpoints, callback `email_not_verified`, 413 note.
- `docs/onboarding.html`: export rows now `opt`.

## Log

- 2026-11-12 10:00 — Created plan
- 2026-11-12 10:30 — M3: export handlers share `loadExportableCalendar` (AuthorizeAccess, header or `?access=`); routes moved to `handleOptional`; integration test `TestExport_PassphraseGate`
- 2026-11-12 10:40 — M4: `Identity.EmailVerified`, Google decodes `email_verified`; `findOrCreateUser` returns `errEmailNotVerified` → 403 `email_not_verified`; handler + idprovider tests
- 2026-11-12 10:45 — L1: `limitBody` middleware (1 MiB, 413 on oversize Content-Length, MaxBytesReader otherwise) wired into `ServeHTTP`; unit tests
- 2026-11-12 10:50 — Frontend `ExportButtons` appends stored access token; docs/api.md + onboarding.html updated
- 2026-11-12 10:55 — Go/make not available locally and CI not starting: backend lint/tests/integration NOT run; PR opened as draft. Frontend oxlint/oxfmt clean, ExportButtons tests pass (unrelated pre-existing `localStorage` failures in other suites in this environment)
- 2026-11-12 11:20 — Rebased onto main after #70–#72 (ExportButtons now uses `API_BASE_URL` from `@/lib/config` plus the `?access=` token); go build/vet/test + integration, vitest (316), oxlint, oxfmt all pass
- 2026-11-23 20:52 — Marked done: shipped in #73 (c56f179). Export gate (`handler/export.go`), `email_verified` check (`idprovider/google.go`) and `limitBody` (`server/middleware.go`) are all on main.
