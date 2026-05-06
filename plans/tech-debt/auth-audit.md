# Auth subsystem security audit

Status: done (residual LOW findings tracked in `auth-audit-followups.md`)

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
- [ ] Magic-link tokens hashed at rest (same threat model as session tokens — DB read should not yield usable links)
- [ ] Session token entropy ≥ 128 bits, generated via `crypto/rand`
- [ ] Magic-link token entropy ≥ 128 bits, single-use (delete on verify), short-lived
- [ ] Magic-link verification uses `subtle.ConstantTimeCompare` against the hash (or relies on indexed DB lookup with no plaintext compare path)
- [ ] Session expiry enforced server-side on every read (not just at issue time), using server clock (`time.Now().UTC()`) — never a client-supplied or row-derived value an attacker influences
- [ ] `RefreshSession` cannot extend a session indefinitely without re-auth — or document this as intentional with a hard cap (absolute max-age separate from sliding expiry)
- [ ] Magic-link verify is atomic: single `DELETE ... RETURNING` (or equivalent), not SELECT-then-DELETE, so two concurrent verifies can't both succeed

### Cookie attributes
- [ ] `HttpOnly` set on session cookie
- [ ] `Secure` set when `SECURE_COOKIES=true` (config-driven, not env-detected — matches AGENTS.md principle)
- [ ] `SameSite=Lax` (or stricter) set — verify `Strict` isn't needed for the magic-link callback flow
- [ ] `Path=/`, `Domain` attribute scoped tightly (if `COOKIE_DOMAIN` is a parent domain, any compromised/owned subdomain can set/overwrite the session cookie — session-fixation vector)
- [ ] Decide on `__Host-` prefix for the session cookie (forces Path=/ + Secure + no Domain; cheap subdomain-injection defense). Pass/skip with reason recorded.
- [ ] Cookie cleared correctly on logout (Max-Age=0 / Expires in past, same `Path`/`Domain`/`Secure`/`SameSite` as set — mismatched attributes leave the cookie alive)

### Session fixation & rotation
- [ ] On successful login (magic-link callback / Google callback), any pre-existing session cookie is replaced, AND the prior server-side session row is deleted — overwriting the cookie alone leaves the previous row alive until expiry
- [ ] No path that lets an attacker pre-set a victim's cookie value and have the server later associate it with the victim's account (relevant if `COOKIE_DOMAIN` covers untrusted subdomains)
- [ ] Logout deletes the server-side session row (currently `_ = h.store.DeleteSession(...)` — error is dropped; confirm the delete actually runs, log on failure even if response stays 200)
- [ ] "Log out everywhere" path exists (or is explicitly out of scope) for the compromised-session case — deleting only the current row is not enough when a stolen token is suspected
- [ ] Rotate session ID on any privilege change post-login (e.g. linking a new identity, becoming admin), not just at login

### CSRF
- [ ] Identify all mutating endpoints (anything that's not GET/HEAD/OPTIONS) that rely on the session cookie for auth
- [ ] Confirm `SameSite=Lax` is sufficient given the endpoint shapes (no top-level navigation POSTs that mutate state without confirmation)
- [ ] No GET endpoints mutate state — except magic-link/OAuth callbacks, which are the documented exception. Any other state-changing GET is a CSRF bug under Lax.
- [ ] Mutating endpoints reject `application/x-www-form-urlencoded` / `multipart/form-data` and require JSON (forces a CORS preflight, layered defense beyond Lax)
- [ ] If any endpoint is `SameSite=None` or accepts cross-site cookies, require a CSRF token
- [ ] CORS allow-list (`Access-Control-Allow-Origin`) is a fixed config value, not reflected from `Origin`, and `Allow-Credentials: true` is only ever sent with an exact-match origin

### Magic-link flow
- [ ] Email enumeration: `SendMagicLink` response shape AND timing are identical for known vs unknown email (always enqueue work, or add constant-time floor — not-sending is faster than sending)
- [ ] Token-to-email binding: callback logs the user in as the email the token was *issued for*, never an email read from query/body on the callback request
- [ ] Return-URL validation: `isRelativePath` rejects empty, `//evil.com`, backslash tricks (`/\evil.com`), and protocol-relative URLs. Verify against this exact list, plus any other URL-parsing quirks (`\\`, `\/`, NULs).
- [ ] Token replay: token is deleted on first verify via atomic `DELETE ... RETURNING` (already the pattern in `magic_link.go` — confirm no SELECT-then-DELETE elsewhere)
- [ ] Magic-link links over HTTPS only in prod (config-driven base URL — `FRONTEND_URL` must be `https://` in prod)
- [ ] Email content does not include the user's session, only the one-time token
- [ ] Link-prefetcher resilience: GET callback consumes the token, so Outlook SafeLinks / Slack unfurl / corporate AV can burn the token before the user clicks. Decide: (a) interstitial GET + POST exchange, (b) short grace window allowing re-use within N seconds from the same IP/UA, or (c) accept and document the failure mode.
- [ ] No magic-link rate limit per email/IP — even though rate-limiting is excluded from this audit's threat model, unbounded `SendMagicLink` lets an attacker mailbomb a victim and burn sender reputation. Record as a finding for follow-up regardless.
- [ ] Magic-link tokens never appear in logs (current `logger.Error` calls pass `error` only — verify `respondInternalError` doesn't render the request URL or token into structured fields)

### Authorization (lighter pass — main scope is auth, not authz)
- [ ] Spot-check that the auth middleware actually runs on every protected route — current router uses opt-in `s.requireAuth(...)` per route in `server.go`. That's a footgun shape: a forgotten wrap silently exposes a handler. Decide whether to leave as-is with a lint/test, or restructure to default-deny (sub-router that requires auth, handlers opt *out*).
- [ ] `optionalAuth` routes audited individually — handlers must enforce their own access rules (passphrase, calendar membership) when `UserFromContext` returns nil
- [ ] IDOR pass on at least the calendar/participant/invite handlers: every handler that takes an ID from the URL verifies the session user can access that resource (don't trust the ID alone)
- [ ] Admin-only routes (`is_admin` flag from migration `005`) gate on the flag server-side via `requireAdmin`, not just UI
- [ ] Google Calendar OAuth flow (`/auth/google/calendar/callback`) has its own state/nonce validation — the existing `oauth_nonce` cookie is scoped to `Path=/auth/google/callback` and won't be sent here

### Logging & data exposure
- [ ] No PII (email, IP) in structured log fields on auth paths — re-confirm `make lint-pii` covers these files
- [ ] Magic-link tokens never logged, even in error paths
- [ ] `X-Request-ID` is propagated on auth responses

### Transport & response headers
- [ ] `Strict-Transport-Security` set on auth responses (or globally) when `SECURE_COOKIES=true`
- [ ] `Referrer-Policy: no-referrer` (or `same-origin`) on the magic-link callback page so the token in the URL doesn't leak via `Referer` to third-party assets loaded by the post-login page
- [ ] `X-Content-Type-Options: nosniff` on JSON responses
- [ ] `Cache-Control: no-store` on auth responses that include session cookies / user data

### Concurrent-session policy
- [ ] Document whether multiple concurrent sessions per user are allowed (current code: yes, unbounded — every login creates a new row, prior rows are not touched). Decide: cap, prune, or accept. A stolen session coexisting invisibly with the legitimate one is the failure mode to weigh.

## Severity rubric

- **High** — directly exploitable: account takeover, auth bypass, session theft, persistent privilege escalation. Spawn a bugfix plan immediately.
- **Medium** — exploitable under specific but realistic conditions (subdomain compromise, link prefetcher, race), or significantly weakens defense-in-depth. Spawn a follow-up tech-debt plan.
- **Low / informational** — hardening, consistency, or documentation. Record here, fix opportunistically.

For passes: record *why* it passes (file:line that satisfies the check), not just a tick. Makes the audit re-runnable and gives future-you something to diff against.

## Plan

1. Walk the checklist top-to-bottom in one sitting; capture findings inline under `## Findings` below, each with severity, file:line, and proposed remediation. Pre-seeded findings below already came out of writing this plan — verify and triage them as part of the walk.
2. For each high/med finding, spawn a follow-up plan in `plans/tech-debt/` (or `plans/bugfix/` if a concrete bug). Cross-link from the Findings section.
3. Low / informational findings stay in this file as a record.
4. Re-run the audit checklist after the highest-severity follow-up plans land, to confirm closure.

## Implementation order

Findings are split into separate PRs so each is independently reviewable and revertable. Auth regressions are easier to bisect when migrations, handler logic, and flow changes don't ship together.

- **PR 1 — defense-in-depth basics (F3 + F7 + F9):** log `DeleteSession` errors on logout; route-coverage test asserting every non-allow-listed route is wrapped in `requireAuth`/`requireAdmin`; boot-time validation that rejects parent-domain `COOKIE_DOMAIN` unless `ALLOW_PARENT_DOMAIN_COOKIE=true`. Small, mechanical, no migrations, no behavior change for end users.
- **PR 2 — absolute session cap (F2):** add `absolute_expires_at` column, migration `006`, clamp in `RefreshSession`, store + middleware tests. Contained; one new column, one query change.
- **PR 3 — magic-link abuse controls (F5 + F6):** per-email + per-IP rate limit on `SendMagicLink`, cap concurrent unexpired tokens per email, invalidate prior outstanding tokens on new issue. Pin the constant-time-floor invariant on the response (F6) in the same PR since it lives next door.
- **PR 4 — magic-link interstitial (F4):** GET callback renders an interstitial page; new POST endpoint runs the existing exchange logic. Touches both stacks. Largest piece; ship last so earlier hardening is in place first.
- **PR 5 — concurrent session cap + close F1 (F11 + F1):** add `pruneSessionsForUser(userID, keep=10)` in `store/session.go`; call from both login paths (`MagicLinkVerify`, Google callback) after `CreateSession`. Index on `sessions(user_id, created_at DESC)` if not already covered. Close F1 in the same PR — the original "invalidate all sessions on login" framing is dropped; cookie overwrite + F2's 90d absolute cap + the new prune-to-10 together bound the residual-row threat without bumping users off legitimate devices on every login. Document the decision in F1.

Gaps to fold in alongside PR 5:

- **F11 — concurrent session policy: DECIDED — cap at 10, prune oldest on login.** Single-session ("invalidate all on login") is too strict for a coordination tool people open infrequently from multiple devices (phone + laptop + work laptop + tablet); they'd get bumped on every fresh login. Unbounded is the current state and lets a token-theft attacker hoard parallel sessions invisibly across the 90d absolute window. 10 is loose enough to never bite a real user, tight enough to bound attacker accumulation. Implementation in PR 5 above.
- **Email transport** checklist section to add before the re-walk: SMTP creds handling, DKIM/SPF alignment on the sender domain, bounce/complaint webhook auth (verify any inbound webhook isn't an unauthenticated POST that could be spoofed to mark accounts as bounced — account-lockout vector).
- **Session cookie value format: DECIDED — raw token, no signed envelope.** Considered adding HMAC-signed envelope for tamper detection without a DB hit. Declined: the DB lookup already returns 401 for unknown tokens (tokens are now hashed at rest per #30), so signing adds complexity without a proportionate gain. Record in checklist as "considered, declined."
- **Re-walk trigger:** re-run the audit checklist after PR 5 ships (covers the post-PR-2/PR-4 trigger too — they've already landed).
- Renumber F8–F10 to be sequential before the formal walk.

## Findings (pre-seeded from plan-authoring pass — verify during walk)

### F1 — Old session row not deleted on login (session fixation residual)
- **Severity:** medium
- **Location:** `backend/internal/handler/auth.go:218-224` (Google), `:390-396` (magic link)
- **Detail:** On successful auth, `CreateSession` inserts a new row and `setSessionCookie` overwrites the cookie. The previously-set session row (if any) is never deleted — it lives until its 30-day expiry. If an attacker successfully pre-set a victim's session cookie (e.g. via subdomain cookie injection), then the victim logs in fresh, the attacker's session row remains valid and usable.
- **Fix:** before/after issuing the new session, delete any prior session row keyed off the cookie value the request arrived with (if any) AND consider whether to invalidate all sessions for the user on login.

### F2 — `RefreshSession` extends sessions indefinitely with no absolute cap
- **Severity:** medium
- **Location:** `backend/internal/server/middleware.go:104-107`, `backend/internal/store/session.go:61-72`
- **Detail:** Every authenticated request fires `RefreshSession` in a goroutine, pushing `expires_at` forward 30 days. An active user's session never expires; a stolen session refreshes itself for as long as the attacker uses it. There is no absolute max-age column.
- **What's normal:**
  - **High-security (banks, healthcare):** short idle (15-30 min) + short absolute (hours to a day).
  - **Consumer SaaS w/ sliding sessions (Stripe, Vercel, Linear class):** 14-30d sliding + 90-180d absolute. After absolute hits, user re-auths via password / magic link / SSO.
  - **GitHub / Google:** effectively no hard absolute; rely on risk signals (new device, new IP, suspicious activity) and explicit revocation. Requires infra FreeWhen doesn't have.
  - **Long-lived "remember me":** 1y absolute is the high end before it starts being a real liability.
  - For FreeWhen (free coordination tool, low session value but no risk-engine), **30d sliding + 90d absolute** matches the consumer-SaaS norm and is the safe default. Bump to 180d if the friction is a real complaint.
- **Fix:** add `absolute_expires_at` at session creation set to `now + 90d`. `RefreshSession` updates `expires_at = min(now+30d, absolute_expires_at)`. When the clamp pins `expires_at` to the absolute, the user re-authenticates next time the sliding window would have advanced.

### F3 — `Logout` drops the `DeleteSession` error
- **Severity:** low
- **Location:** `backend/internal/handler/auth.go:301-305`
- **Detail:** `_ = h.store.DeleteSession(...)` swallows DB errors. Cookie is cleared client-side, but the server-side row may still be valid. If the same token is later replayed (e.g. from a backup, a logged URL, an exfiltrated cookie), the session still authenticates.
- **Fix:** log the error at warn/error level (no token in fields). Optionally retry once. Response can stay 200.

### F4 — Magic-link GET callback is vulnerable to link prefetchers
- **Severity:** medium
- **Location:** `backend/internal/handler/auth.go:369-402`
- **Detail:** `MagicLinkCallback` is a GET that consumes the token and creates a session. Outlook SafeLinks, Slack unfurl, AV scanners, and link previews can fetch the URL before the user clicks, burning the one-time token. The user then sees "invalid or expired link" on click. Worse: under specific conditions a prefetcher could end up with a Set-Cookie session — though typically prefetchers strip cookies, this depends on the client.
- **What's normal:**
  - **Interstitial pattern (Slack, Notion, Auth0, Vercel, most modern magic-link auth):** GET renders a "Sign in" / "Continue" page with a button that POSTs the token. The POST is the actual exchange. Prefetchers fetch the GET page (no token consumed), real users click through. This is the dominant pattern.
  - **Grace-window pattern (some smaller SaaS):** GET still consumes the token, but the row stays valid for re-use within ~30-60s from the same IP/UA. Simpler retrofit, weaker — fails if the prefetcher and the user are on different egress IPs (corporate VPN, mobile carrier NAT change between scan and click).
  - **Hash-fragment pattern:** token in URL fragment (`#token=...`) so it never hits the server until JS reads it and POSTs. Used by some SPA-first auth flows. Doesn't fit FreeWhen's current SSR redirect shape.
  - **Accept and document:** the smallest-cost option. Reasonable while user volume is low and link-scanner reports are rare.
  - The interstitial is the right end-state. Grace window is the cheap retrofit if interstitial is a bigger lift than time allows.
- **Fix:** prefer the interstitial. The GET handler renders a page with the token in a hidden form, the user clicks "Sign in", the POST runs the existing exchange logic. Magic-link emails keep their existing GET URL; only the server response changes.

### F5 — `SendMagicLink` enables unbounded mailbomb
- **Severity:** medium (out of scope for rate-limit audit but in scope for auth abuse)
- **Location:** `backend/internal/handler/auth.go:328-365`
- **Detail:** No per-email or per-IP cap. An attacker can submit a victim's email repeatedly, generating tokens and sending mail. Burns sender reputation, harasses the victim, and (if any token leaks) creates many concurrent valid tokens.
- **Fix:** per-email and per-IP rate limit on `SendMagicLink`; cap concurrent unexpired tokens per email (e.g. 3); when issuing a new token, invalidate prior outstanding tokens for the same email.

### F6 — `SendMagicLink` timing leak (email enumeration)
- **Severity:** low
- **Location:** `backend/internal/handler/auth.go:328-365`
- **Detail:** Response shape is identical for known/unknown email (good), but the email-send path takes meaningfully longer than a no-op path. The code currently *always* creates a token and sends, regardless of whether the user exists (user is created at callback time via `findOrCreateEmailUser`), so the leak is muted — but if that ever changes to "only send for known users," the timing channel reopens. Pin this invariant in a test.
- **Fix:** add a comment + test asserting the no-existence-check property, or add a constant-time floor on the response.

### F7 — `optionalAuth` route surface is large; default-deny would be safer
- **Severity:** low (architectural)
- **Location:** `backend/internal/server/server.go:104-165`, `backend/internal/server/middleware.go:77-86`
- **Detail:** Routing is opt-in: every protected route must remember `s.requireAuth(...)`. A new route added without the wrapper is silently public. `optionalAuth` is used heavily, which means each of those handlers must also enforce its own access rules — easy to get wrong as the route count grows.
- **What's normal:**
  - **Go stdlib `net/http` (the small/medium app default):** opt-in middleware per route, exactly the current pattern. Common, and not unsafe at small scale.
  - **Sub-mux split (Go medium/large):** `publicMux` + `protectedMux` (the latter wrapped end-to-end with `requireAuth`). New routes default to whichever mux they're added to. Common in larger Go services and is the typical "graduate from opt-in" step. Cheap with `http.ServeMux` since 1.22.
  - **Chi/Echo/Gin sub-router groups:** same idea, `r.Group(func(r chi.Router) { r.Use(authMiddleware); ... })` — public routes register outside the group.
  - **Rails / Django:** default-deny via `before_action`/decorator inheritance, with explicit `skip_before_action` / `@public` opt-out. Strong default.
  - **Route-coverage test (defense-in-depth, used alongside any of the above):** a test walks the registered routes at startup and asserts every non-allow-listed path has either `requireAuth` or `requireAdmin` in the chain. Cheap. Linear, Vercel-style services often run this.
  - For FreeWhen now, the route count is borderline. The sub-mux split is the proportionate next step; the route-coverage test is the smallest defense-in-depth that makes the current opt-in pattern safer without a refactor.
- **Fix:** start with the route-coverage test (a single test in `server_test.go` that iterates the registered patterns and asserts auth wrapping against an explicit public allow-list). If the route count keeps growing, do the sub-mux split.

`optionalAuth` is a separate concern and should stay — it's used for routes that are reachable via passphrase / participant token without a session, not by accident. The handlers it wraps must each be re-audited to confirm they enforce their own gate (covered by the IDOR pass checklist item).

### F8 — Calendar OAuth callback state/nonce protection — RESOLVED
- **Severity:** none (resolved)
- **Location:** `backend/internal/handler/calendar_integration.go:121-226`
- **Detail:** Calendar flow has its own `calendar_oauth_nonce` cookie + `calendarOAuthState` struct, set in `Connect` and verified in `Callback`. Path-scoped separately from the login-flow nonce. No issue.

### F9 — Cookie domain governance
- **Severity:** low (confirmed; design choice, not a bug)
- **Location:** `backend/internal/handler/auth.go:439-450`, `infra/cloud_run.tf:40-46`
- **Detail:** Prod sets `COOKIE_DOMAIN=.freewhen.simonperryman.co.uk` (leading-dot parent domain). The session cookie is therefore sent to *every* subdomain of `freewhen.simonperryman.co.uk` — currently `freewhen.*` (frontend, Vercel) and `api.freewhen.*` (backend, Cloud Run). This is intentional: the frontend and backend live on different subdomains and need to share the cookie, so a host-only cookie or `__Host-` prefix is incompatible without an architectural change (e.g. proxy the API under the frontend origin via Vercel rewrites and drop cross-subdomain entirely).
- **Trust boundary:** anyone who can stand up another subdomain under `freewhen.simonperryman.co.uk` can read/set the session cookie. Today that's only the project owner (DNS is under personal control), so the practical risk is low. It would matter if a third party ever got a subdomain there.
- **Fix:**
  1. **Validate the shape, not just presence.** `COOKIE_DOMAIN` is already required (`config.go:100-102` errors when empty + `SECURE_COOKIES=true`), but any value is accepted. Add a check: if `COOKIE_DOMAIN` starts with `.` (parent-domain shape), require an explicit opt-in env var like `ALLOW_PARENT_DOMAIN_COOKIE=true`, otherwise refuse to boot. Makes the dangerous shape a deliberate, typed-out decision rather than a quiet default.
  2. Document the trust boundary in `docs/infra-notes.md`.
  3. Longer-term, consider proxying `api.*` under the frontend origin via Vercel rewrites so the cookie can be host-only / `__Host-`-prefixed and the parent-domain opt-in can be removed.

### F10 — Return-URL validation: backslash/protocol-relative paths — assessed safe
- **Severity:** none (recorded so future checklist walks don't re-flag)
- **Location:** `backend/internal/handler/auth.go:462-467` (`isRelativePath`), `:231` and `:401` (redirect targets)
- **Detail:** `isRelativePath` rejects empty, non-`/`-prefixed, and `//`-prefixed strings. It does NOT reject `/\evil.com` or `/\\evil.com`. Initially looked like an open-redirect bypass via browser backslash normalization. On closer reading the redirect target is built as `h.frontendURL + returnURL`, so the host is always fixed by `FRONTEND_URL`; backslash-in-path does not let a browser switch host. Safe as written. Add a unit test pinning these inputs (`/\evil.com`, `/\\evil.com`, `/%2f%2fevil.com`) to assert "host stays on frontend" so a future refactor can't regress this without a failing test.

### F11 — Concurrent session policy
- **Severity:** medium
- **Location:** `backend/internal/store/session.go` (`CreateSession`), `backend/internal/handler/auth.go` (Google + magic-link login paths)
- **Detail:** Every successful login inserts a new row in `sessions`; prior rows are untouched and live until their sliding/absolute expiry. A user accumulates one row per device per login event. A token-theft attacker can hold a parallel session indefinitely (within the 90d absolute cap from F2) without colliding with the legitimate user.
- **Decision:** cap at 10 active sessions per user; prune oldest on login. Single-session would bump real users on every cross-device login (phone + laptop + work laptop + tablet) on a tool people open infrequently. Unbounded leaves attacker accumulation invisible across the F2 window. 10 is loose enough to never bite a real user, tight enough to bound the attacker case.
- **Fix:** `pruneSessionsForUser(userID, keep=10)` deletes rows for the user beyond the most-recent 10 by `created_at`. Call after `CreateSession` in both login paths. Add index on `sessions(user_id, created_at DESC)` if not already covered.

## Log

- 2026-05-05 10:30 — Created audit plan. Seed finding: plaintext session tokens (covered by `hash-session-tokens.md`).
- 2026-05-05 11:15 — Expanded checklist (cookie `__Host-`/domain governance, link-prefetcher resilience, transport headers, IDOR/default-deny pass, severity rubric). Pre-seeded `## Findings` with F1–F9 surfaced while reading `auth.go`, `session.go`, `magic_link.go`, and `middleware.go` — to be verified during the formal walk.
- 2026-05-05 11:40 — Resolved F8 (calendar OAuth has its own nonce cookie + state struct). Refined F9 (config enforces `COOKIE_DOMAIN` non-empty when `SECURE_COOKIES=true`; remaining risk is the deployed value). Added F10 documenting the `isRelativePath` backslash analysis as safe-by-construction (host fixed by `FRONTEND_URL` prefix), with a test pin recommended.
- 2026-05-05 12:05 — Confirmed prod `COOKIE_DOMAIN=.freewhen.simonperryman.co.uk` from `infra/cloud_run.tf:40-41` (parent-domain by design, frontend/backend share the cookie). Added "what's normal" guidance to F2 (recommend 30d sliding + 90d absolute), F4 (interstitial is the dominant pattern; grace window is the cheap retrofit), and F7 (sub-mux split + route-coverage test as the proportionate next step from opt-in middleware).
- 2026-05-05 12:20 — F9 fix sharpened: presence is already enforced; add value-shape validation that requires an explicit `ALLOW_PARENT_DOMAIN_COOKIE=true` opt-in when `COOKIE_DOMAIN` starts with a leading dot.
- 2026-05-05 21:00 — Added "Implementation order" section splitting findings into 5 PRs. Starting PR 1 (F3 + F7 + F9).
- 2026-05-05 21:30 — PR 2 implemented (F2 — absolute session cap):
  - Migration `020_session_absolute_expiry.sql` adds `absolute_expires_at TIMESTAMPTZ NOT NULL` to `sessions`; existing rows backfilled with `created_at + 90 days`.
  - `store.CreateSession` sets `absolute_expires_at = now + 90d` alongside the 30d sliding `expires_at`.
  - `store.RefreshSession` clamps with `expires_at = LEAST(now+30d, absolute_expires_at)`. Once the clamp pins, the next read sees a past `expires_at` and returns nil.
  - `store.GetSession` also rejects on `now > absolute_expires_at` directly (defense in depth — covers the case where a row's sliding expiry hasn't refreshed since the absolute went past).
  - `model.Session` gains `AbsoluteExpiresAt`.
  - Tests: `TestSessionAbsoluteExpiryCap`, `TestRefreshSessionClampsToAbsolute`, `TestGetSessionRejectsPastAbsolute`. Backend unit + integration tests pass; `make lint` clean.
- 2026-05-05 22:00 — PR 3 implemented (F5 + F6 — magic-link abuse controls):
  - Migration `021_magic_link_email_index.sql` adds `idx_magic_link_tokens_email` so per-email invalidate-on-issue is an index lookup, not a sequential scan.
  - `store.CreateMagicLinkToken` now wraps DELETE-by-email + INSERT in a transaction. Issuing a new token for an email invalidates any prior outstanding token for that email — only the most recent link is ever valid. Caps the concurrent-token surface and shortens the window for any link inadvertently shared (forwarding, Slack unfurl, AV scan).
  - `handler/magic_link_limiter.go` pairs two `internal/ratelimit` token buckets: per-email (5 / 15 min) and per-IP (30 / 15 min). Both must pass; both are debited on every attempt so neither acts as a side channel for the other's state.
  - `auth.SendMagicLink` checks the limiter before issuing and returns `429 rate_limited` with `Retry-After: 900` on rejection. Docstring on `SendMagicLink` pins F6: handler must never branch on user existence — `findOrCreateEmailUser` runs at callback time so known/unknown timing stays uniform.
  - Tests: `TestSendMagicLink_RateLimitedPerEmail`, `TestSendMagicLink_RateLimitedPerIP` (asserts `Retry-After`), `TestSendMagicLink_AlwaysIssuesEvenForUnknownUser` (F6 invariant pin); store integration tests `TestCreateMagicLinkToken_InvalidatesPriorTokensForEmail`, `TestCreateMagicLinkToken_DifferentEmailsIndependent`. Fake `AuthStore` updated to mirror the new invalidate-prior semantics. Backend unit + integration + frontend tests pass; `make lint` clean.
- 2026-05-05 22:30 — PR 4 implemented (F4 — magic-link interstitial):
  - Backend: replaced `GET /auth/magic-link/callback` (which set the cookie + 307'd) with `POST /auth/magic-link/verify`. Same exchange logic (`VerifyMagicLinkToken` → `findOrCreateEmailUser` → `CreateSession` → `setSessionCookie`), but returns `{return_url}` JSON instead of redirecting. Docstring on `MagicLinkVerify` pins F4: do not add a GET variant — that re-opens the prefetcher hole this PR closed.
  - Frontend: `app/auth/magic-link/callback/page.tsx` no longer auto-fires `window.location.href`. Renders an interstitial UI ("Sign in to FreeWhen" + Continue button). Token consumption only happens on user-initiated click → `verifyMagicLink(token)` POST → `window.location.href = returnUrl`. New `verifyMagicLink` helper in `lib/api.ts` replaces the deleted `getMagicLinkCallbackUrl`.
  - Email link unchanged — still `${FRONTEND_URL}/auth/magic-link/callback?token=...`. Prefetchers (Outlook SafeLinks, Slack unfurl, AV scanners) hitting that URL load the interstitial without running JS or following user-initiated POSTs, so the one-time token survives until the real click.
  - Tests: handler tests rewired through new `postVerify` helper; added `TestMagicLinkVerify_InvalidJSON`. Route-coverage allow-list updated. Backend unit + integration + frontend tests pass; `make lint` clean.
- 2026-05-05 21:10 — PR 1 implemented:
  - F3: `auth.go` Logout now logs `DeleteSession` errors at error level (no token in fields).
  - F9: `config.go` rejects leading-dot `COOKIE_DOMAIN` unless `ALLOW_PARENT_DOMAIN_COOKIE=true`. Prod Terraform (`infra/cloud_run.tf`) sets the opt-in. Tests added (`config_test.go`).
  - F7: `server.go` `routes()` refactored to use `handlePublic`/`handleAuth`/`handleAdmin`/`handleOptional` registration helpers that record each route's auth class on the Server. New `route_coverage_test.go` asserts every registered route matches an explicit allow-list (kind change or new route fails the test) and that every required/admin route returns 401 without a session cookie.
  - All backend unit + integration + frontend tests pass; `make lint` clean.
- 2026-05-06 — F11 decided (cap concurrent sessions at 10, prune oldest on login). PR 5 reframed: was "invalidate all on login" for F1 fixation residual; now "prune-to-10 + close F1" since cookie overwrite + F2's 90d absolute cap + the new prune already bound the residual-row threat. Cookie value format decision recorded: raw token, no signed envelope (DB lookup with hashed-at-rest tokens already covers tamper). Findings reordered F1–F11 sequentially; F11 added with severity medium and the cap-at-10 rationale.
- 2026-05-06 — Re-walk completed against current main (PRs #30–#35 merged). No HIGH/MEDIUM findings. Six LOW findings surfaced (F12 logout-everywhere, F13 reject non-JSON Content-Type, F14 isRelativePath regression test, F15 email normalization, F16 security/transport headers + Referrer-Policy, F17 verify Cloud Run XFF behavior) plus the deferred email-transport section (Resend key handling, DKIM/SPF/DMARC, future bounce-webhook auth). All tracked in `plans/tech-debt/auth-audit-followups.md`. Audit status flipped to `done` — residual hardening work lives in the follow-up plan.
- 2026-05-06 — PR 5 implemented (F11 + F1 close):
  - Migration `022_session_user_created_index.sql` drops the user_id-only `idx_sessions_user_id` and replaces it with composite `idx_sessions_user_id_created_at` on `(user_id, created_at DESC)` so the prune query is an index scan. Composite covers user_id equality lookups too, so the simple index is redundant.
  - `store.PruneSessionsForUser(userID, keep)` deletes rows for the user beyond the most-recent `keep` by `created_at DESC`. Single statement (`DELETE … WHERE user_id = $1 AND token_hash NOT IN (SELECT … ORDER BY created_at DESC LIMIT $2)`) so concurrent logins can't race with each other to widen the cap.
  - `maxConcurrentSessionsPerUser = 10` constant in `handler/auth.go`. Called after `CreateSession` in `GoogleCallback`, `MagicLinkVerify`, and `TestLoginHandler.Login`. Errors logged at error level and ignored — login still succeeds even if prune fails (the user has a working session; the worst case is one extra row that the next prune cleans up).
  - F1 closed in this PR. Original framing was "invalidate all sessions on login" to kill any pre-set session fixation row. Reframed: cookie overwrite at login + F2's 90d absolute cap + the new prune-to-10 together bound the residual-row threat without bumping users off legitimate devices on every login. A pre-set row gets pruned alongside legitimate old rows once enough fresh logins accumulate, and at worst lives until the absolute cap.
  - Tests: store integration `TestPruneSessionsForUser` (12 → 10, oldest 2 deleted, other user's session untouched), `TestPruneSessionsForUserBelowKeep` (no-op when below cap); handler unit `TestMagicLinkVerify_PrunesOldSessions` (12 logins for the same user → 10 sessions remain). Fake `AuthStore` updated with `sessionOrder` slice + `PruneSessionsForUser` mirror. Backend unit + integration + frontend tests pass; `make lint` clean.
