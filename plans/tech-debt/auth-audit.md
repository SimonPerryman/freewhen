# Auth subsystem security audit

Status: pending

## Goal

Per-PR security review keeps missing system-level weaknesses in code that isn't currently changing. Run a focused audit of the auth subsystem (magic-link issuance, session lifecycle, cookies, CSRF, related endpoints) to surface latent issues, then triage findings into concrete follow-up plans.

The plaintext-session-token finding (covered by `hash-session-tokens.md`) is the seed for this audit but not the scope — the goal is to enumerate the rest of the same shape of bug before they bite.

## Scope (what to look at)

- `backend/internal/handler/auth.go` — `SendMagicLink`, `MagicLinkCallback`, `Logout`, `Me`, session-cookie helpers
- `backend/internal/store/session.go`, `magic_link.go`
- `backend/internal/store/migrations/003_add_auth.sql`, `004_add_magic_link_tokens.sql`
- Middleware that authenticates requests (whatever wraps protected routes)
- Frontend cookie handling in `frontend/src/lib/` and any login/logout flows

## Audit checklist

For each item: read the code, decide pass / weak / broken, and capture a one-paragraph finding with file:line references.

### Token storage & lifecycle
- [ ] Session tokens hashed at rest (tracked in `hash-session-tokens.md` — confirm scope, not duplicate work)
- [ ] Session token entropy ≥ 128 bits, generated via `crypto/rand`
- [ ] Magic-link token entropy ≥ 128 bits, single-use (delete on verify), short-lived
- [ ] Magic-link verification is constant-time against the hash (sha256 hash compare via `==` on bytes is fine; flag if there's any plaintext compare)
- [ ] Session expiry enforced server-side on every read (not just at issue time)
- [ ] `RefreshSession` cannot extend a session indefinitely without re-auth — or document this as intentional with a hard cap

### Cookie attributes
- [ ] `HttpOnly` set on session cookie
- [ ] `Secure` set when `SECURE_COOKIES=true` (config-driven, not env-detected — matches AGENTS.md principle)
- [ ] `SameSite=Lax` (or stricter) set — verify `Strict` isn't needed for the magic-link callback flow
- [ ] `Path=/`, no overly permissive `Domain` attribute
- [ ] Cookie cleared correctly on logout (Max-Age=0 / Expires in past, same attributes as set)

### Session fixation & rotation
- [ ] On successful login (magic-link callback), any pre-existing session cookie is replaced, not reused
- [ ] No path that lets an attacker pre-set a victim's cookie value and have the server later associate it with the victim's account
- [ ] Logout deletes the server-side session row, not just the cookie

### CSRF
- [ ] Identify all mutating endpoints (anything that's not GET/HEAD/OPTIONS) that rely on the session cookie for auth
- [ ] Confirm `SameSite=Lax` is sufficient given the endpoint shapes (no top-level navigation POSTs that mutate state without confirmation)
- [ ] If any endpoint is `SameSite=None` or accepts cross-site cookies, require a CSRF token

### Magic-link flow
- [ ] Email enumeration: `SendMagicLink` response is identical for known vs unknown email
- [ ] Return-URL validation: open-redirect via the `return` param after callback (must be same-origin / allowlisted path)
- [ ] Token replay: token is deleted on first verify (atomic delete-and-return)
- [ ] Magic-link links over HTTPS only in prod (config-driven base URL)
- [ ] Email content does not include the user's session, only the one-time token

### Authorization (lighter pass — main scope is auth, not authz)
- [ ] Spot-check that the auth middleware actually runs on every protected route (no handler accidentally wired without it)
- [ ] Admin-only routes (`is_admin` flag from migration `005`) gate on the flag server-side, not just UI

### Logging & data exposure
- [ ] No PII (email, IP) in structured log fields on auth paths — re-confirm `make lint-pii` covers these files
- [ ] Magic-link tokens never logged, even in error paths
- [ ] `X-Request-ID` is propagated; no auth-bypass timing differential between known/unknown email paths

## Plan

1. Walk the checklist top-to-bottom in one sitting; capture findings inline in this file under a `## Findings` section, each with severity (high/med/low), file:line, and proposed remediation.
2. For each high/med finding, spawn a follow-up plan in `plans/tech-debt/` (or `plans/bugfix/` if a concrete bug). Cross-link from the Findings section.
3. Low / informational findings stay in this file as a record.
4. Re-run the audit checklist after the highest-severity follow-up plans land, to confirm closure.

## Log

- 2026-05-05 10:30 — Created audit plan. Seed finding: plaintext session tokens (covered by `hash-session-tokens.md`).
