# Config: fail fast on missing prod vars

Status: done

## Goal

Today `config.Load()` only returns errors for `DATABASE_URL`, `CALENDAR_ACCESS_SECRET`, `LOG_LEVEL`, and `SECURE_COOKIES`. Everything else — Resend, magic-link from-address, `FRONTEND_URL`, `COOKIE_DOMAIN`, Google OAuth — is read bare with `os.Getenv`. The server boots happily with any subset set, and features degrade silently at request time.

The biggest prod risks:

- **Magic-link auth**: if `RESEND_API_KEY` is unset, `server.go:60` leaves `emailSender = nil` and the auth endpoint silently fails to send.
- **Email links**: if `FRONTEND_URL` is unset, magic-link emails go out with broken URLs.
- **Auth cookies**: in prod (`SECURE_COOKIES=true`) a missing `COOKIE_DOMAIN` scopes cookies to the API host instead of the shared apex, quietly breaking login on the frontend.
- **Google OAuth**: half-set credentials initialise the client and break at redirect time.

The fix: validate these at startup and refuse to boot when inconsistent, with explicit opt-out flags for the dev case. Matches the 12-factor "config over environment detection" rule in AGENTS.md — prod sets the keys, dev sets the opt-out, same binary.

## Plan

1. Add pair/group validation to `config.Load()`:
   - **Auth email** (`RESEND_API_KEY` + `MAGIC_LINK_FROM_EMAIL` + `FRONTEND_URL`): required together, opt-out via `AUTH_EMAIL_DISABLED=true`. Reject the mixed state (some set, some not, and not disabled).
   - **Google OAuth credentials** (`GOOGLE_CLIENT_ID` + `GOOGLE_CLIENT_SECRET`): required together or neither.
   - **Google redirect URLs**: `GOOGLE_REDIRECT_URL` and `GOOGLE_CALENDAR_REDIRECT_URL` each require `GOOGLE_CLIENT_ID` to be set. At least one must be set if credentials are provided (otherwise creds are dead weight).
   - **Cookie domain**: required when `SECURE_COOKIES=true`.
2. Surface a new `AuthEmailDisabled bool` on `Config` so `server.go` can branch on intent instead of inferring from `ResendAPIKey != ""`.
3. Update `backend/.env.example` to document the new opt-out flags.
4. `make lint` + `make test` + `make test-integration`.

## Log

- 2026-04-23 — Created plan.
- 2026-04-23 — Added `validateAuthEmail` and `validateGoogleOAuth` to `config.Load()`. Introduced `AuthEmailDisabled` on `Config`; `server.go` now branches on it instead of inferring from `ResendAPIKey`. Required `COOKIE_DOMAIN` when `SECURE_COOKIES=true`. Refreshed `backend/.env.example`. `make lint`, `make test`, `make test-integration` all pass.
