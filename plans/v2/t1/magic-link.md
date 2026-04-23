# Magic Link Authentication

Status: done

## Goal

Add email magic link sign-in as a second auth method alongside Google OAuth. Users without Google accounts can sign in by entering their email and clicking a link. Uses Resend for transactional email delivery.

## Design Decisions

**Resend as email provider.** 3,000 emails/month free tier, simple HTTP API (single POST), no SDK needed. Fits the "minimal dependencies" principle.

**Token format:** Crypto-random 32-byte hex string, stored hashed (SHA-256) in a `magic_link_tokens` table. Token expires after 15 minutes. One-time use — deleted after verification.

**No rate limiting in V1.** We could add per-email rate limiting later, but for now the 15-minute expiry and one-time use are sufficient. The endpoint is `POST` so `SameSite=Lax` cookies block CSRF.

**Reuse existing user/session infrastructure.** Magic link callback creates/finds user (provider = 'email'), creates a session, sets the same cookie as Google OAuth. No new session logic.

**Return URL.** Encoded in the magic link token row so the user lands back where they started after clicking the link.

## Database

Migration `004_add_magic_link_tokens.sql`:

```sql
CREATE TABLE magic_link_tokens (
    token_hash  TEXT PRIMARY KEY,
    email       TEXT NOT NULL,
    return_url  TEXT NOT NULL DEFAULT '/',
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_magic_link_tokens_expires_at ON magic_link_tokens(expires_at);
```

## API

| Method | Path | Description |
|--------|------|-------------|
| POST | `/auth/magic-link` | Accept `{"email": "...", "return_url": "/..."}`. Generate token, store hash, send email via Resend. Always return 200 (don't leak whether email exists). |
| GET | `/auth/magic-link/callback?token=...` | Look up token hash, verify not expired, create/find user, create session, set cookie, delete token, redirect to return URL. |

## Backend Implementation

### Phase 1: Token store

1. **Migration** — `004_add_magic_link_tokens.sql`
2. **Store** — `internal/store/magic_link.go`
   - `CreateMagicLinkToken(email, returnURL) → plainToken` (generates random token, stores SHA-256 hash, 15-min expiry)
   - `VerifyMagicLinkToken(plainToken) → (email, returnURL, error)` (looks up hash, checks expiry, deletes token)
   - `DeleteExpiredMagicLinkTokens()` (cleanup)

### Phase 2: Resend integration

1. **Config** — add `RESEND_API_KEY` and `MAGIC_LINK_FROM_EMAIL` env vars
2. **Email sender** — `internal/email/resend.go`
   - Simple HTTP POST to `https://api.resend.com/emails`
   - `SendMagicLink(to, link string) error`
   - Minimal HTML email: "Click here to sign in to FreeWhen" with the link

### Phase 3: Handler

1. **Handler** — add to `internal/handler/auth.go`
   - `POST /auth/magic-link` — validate email format, create token, build link (`FRONTEND_URL` + `/auth/magic-link/callback?token=...`), send via Resend, return `{"ok": true}`
   - `GET /auth/magic-link/callback` — verify token, create/find user (provider = 'email'), create session, set cookie, redirect to return URL
2. **Wire up** — register routes in server setup

### Phase 4: Frontend

1. **Login page** — add email input + "Send magic link" button below the Google button
2. **Check-your-email page** — `src/app/login/check-email/page.tsx`. "We sent a link to your@email.com. Click it to sign in."
3. **Callback page** — `src/app/auth/magic-link/callback/page.tsx`. Hits the backend callback endpoint (which redirects after setting the cookie). May not need a dedicated page if the backend redirect handles everything.

## Infrastructure

- **Resend:** Create account, verify sending domain (DNS records), generate API key
- **Cloud Run:** Add `RESEND_API_KEY` and `MAGIC_LINK_FROM_EMAIL` to Secret Manager and mount as env vars

## Build Order

1. Resend account setup + domain verification
2. Backend Phase 1 (migration, token store)
3. Backend Phase 2 (Resend integration)
4. Backend Phase 3 (handler, routes)
5. Frontend Phase 4 (email input, check-email page)
6. Test end-to-end

## Log

- 2026-04-19 — Created plan. Chose Resend for email delivery (3k/month free, simple API).
- 2026-04-19 — Implemented all phases. Migration 004_add_magic_link_tokens.sql. Store: magic_link.go (create with SHA-256 hash, verify + delete atomically, cleanup). Email: internal/email/resend.go (simple HTTP POST to Resend API). Config: added RESEND_API_KEY and MAGIC_LINK_FROM_EMAIL env vars. Handler: POST /auth/magic-link (always returns 200 to avoid leaking email existence), GET /auth/magic-link/callback (verify token, create/find user, create session, redirect). Frontend: login page updated with email input + "Send magic link" button, check-email confirmation page, magic-link callback page (redirects to backend). All tests pass.
