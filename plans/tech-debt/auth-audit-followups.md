# Auth audit follow-ups (F12–F17)

Status: pending

## Goal

Close out the LOW-severity findings from the 2026-05-06 re-walk of `auth-audit.md` and add the email-transport section that was deferred from the original audit. None are individually urgent; bundled here so they ship as one coherent hardening pass and the audit can be marked `done`.

## Findings to close

### F12 — "Log out everywhere"

- **What:** add `POST /auth/logout-all` that deletes every session row for the authenticated user, then immediately re-issues a fresh session for the current device.
- **Why:** users with a suspected stolen token currently have no self-service revocation lever. Prune-to-10 + 90d absolute cap bound the worst case but can't be triggered by the user.
- **Where:** `handler/auth.go` (new handler), `store/session.go` (new `DeleteSessionsForUser(userID)`), `server/server.go` route registration, frontend hook in account/profile UI.

### F13 — Reject non-JSON Content-Type on mutating endpoints

- **What:** for non-GET/HEAD/OPTIONS requests, return 415 if `Content-Type` is not `application/json`.
- **Why:** defense-in-depth on top of SameSite=Lax. Forces a CORS preflight for any cross-origin attempt and makes the JSON-only contract explicit.
- **Where:** new helper in `server/middleware.go` (or fold into `requireAuth`/registration helpers). Applies to all `handle{Auth,Admin,Optional}` POST/PATCH/DELETE routes.

### F14 — Pin `isRelativePath` regression test

- **What:** table-driven test covering `""`, `/`, `/path`, `//evil.com`, `/\evil.com`, `/\\evil.com`, `/%2f%2fevil.com`, `\\evil.com`, plus an integration-shape assertion that the final `Location` header always starts with `frontendURL + "/"`.
- **Why:** F10 in the original audit recorded the host-fixed-by-prefix invariant as safe-by-construction. A test pin makes a future refactor that breaks the invariant fail loudly.
- **Where:** `handler/auth_test.go` (or new `auth_redirect_test.go`).

### F15 — Normalize email at the boundary

- **What:** `strings.ToLower(strings.TrimSpace(...))` on `req.Email` at the top of `SendMagicLink` (before validation, rate-limit, store calls). Same normalization for any future email-input handler. Add a migration to either lowercase existing `users.email` rows or convert the column to `citext` (or add a functional unique index on `lower(email)`).
- **Why:** `User@x.com` vs `user@x.com` currently:
  - bypasses the per-email rate limit (different bucket key)
  - bypasses the invalidate-prior-tokens cap in `CreateMagicLinkToken`
  - can create a second `users` row with a different casing on first login (account split)
- **Where:** `handler/auth.go:SendMagicLink`, `store/magic_link.go`, `store/user.go`, new migration.

### F16 — Security/transport headers

- **What:** new `secureHeaders` middleware setting:
  - `X-Content-Type-Options: nosniff` globally
  - `Referrer-Policy: no-referrer` globally (or at minimum on `/auth/*`)
  - `Strict-Transport-Security: max-age=31536000; includeSubDomains` when `SECURE_COOKIES=true`
  - `Cache-Control: no-store, private` on `/auth/*` responses
- Plus `<meta name="referrer" content="no-referrer">` on the magic-link callback page as a belt-and-braces measure for the token-in-URL.
- **Why:** the magic-link interstitial receives the token in `?token=...`. Currently the page loads no third-party assets, but if any is ever added (analytics, fonts, speed-insights) the token leaks via `Referer` before the user clicks Continue. The other headers are baseline hardening.
- **Where:** `server/middleware.go`, `server/server.go` middleware chain, `frontend/src/app/auth/magic-link/callback/page.tsx`.

### F17 — Verify Cloud Run XFF behavior

- **What:** investigation, not a code change up-front. Deploy a debug endpoint (or read existing access logs) that echoes `r.Header.Get("X-Forwarded-For")` and `r.RemoteAddr`. Curl the endpoint with a forged `X-Forwarded-For: 1.2.3.4` and inspect what the handler sees.
- **Decision matrix:**
  - If GCP overwrites/replaces client-supplied XFF → leftmost-trust is safe, document and close.
  - If GCP appends → client controls leftmost. Switch `remoteIP` to read `r.RemoteAddr` (Cloud Run sets this to the GFE-observed peer) or count from the right by trusted-proxy depth.
- **Why:** the magic-link per-IP rate limiter and the unlock-passphrase per-IP rate limiter both depend on `remoteIP` being attacker-uncontrollable. Rate limiting is excluded from the audit threat model so this is recorded as low priority, but the investigation cost is small and the answer informs both call sites.
- **Where:** `handler/passphrase.go:remoteIP`. Remove the debug endpoint after the test.

## Email transport section (new audit scope)

Originally deferred; fold into this plan so it ships alongside the closures above. Walk these checks during implementation:

- [ ] **Resend API key handling** — confirmed in env (`RESEND_API_KEY`), not logged. Verify Terraform pulls from Secret Manager (not raw env in `cloud_run.tf`), confirm git history doesn't contain a leaked value, set up rotation playbook in `docs/infra-notes.md`.
- [ ] **DKIM/SPF/DMARC alignment** on the sender domain (`from` field in `email/resend.go`). Check live DNS:
  - SPF includes Resend's sending IPs
  - DKIM CNAME records present
  - DMARC policy at minimum `p=quarantine` with `rua=` reporting
- [ ] **No inbound webhook today.** If a Resend bounce/complaint webhook is ever added: signature-verify the request (Resend signs with a shared secret, see Resend docs). An unauthenticated bounce webhook is an account-lockout vector — an attacker POSTs a forged "bounced" event for a victim's email, the system marks the address undeliverable, magic-link login is silently broken for that user. Treat the webhook as a high-trust input and gate it.
- [ ] **From-address sanity** — `cfg.FromEmail` should be a single configured value, not derived from request input anywhere.

## Plan

1. Land F14 first (test-only, no risk, smallest diff). Builds confidence the redirect invariant is pinned before touching anything else.
2. F12 + F13 + F16 as one PR — three small middleware/handler additions, all defensive, all reviewable together.
3. F15 in its own PR because of the migration. Run on a copy of prod first to confirm no email collisions surface (two users with case-variant emails would conflict on the lowercase migration; need a merge plan if any exist).
4. F17 investigation ticket — half a day of work; either close as "GCP behavior is safe" or open a follow-up bugfix plan.
5. Email-transport audit alongside or after the above. DNS changes (SPF/DKIM/DMARC) are independent of code and can ship in parallel.
6. Once F12–F16 land + email-transport section walked, mark `auth-audit.md` status as `done` and reference this plan for the residual work.

## Log

- 2026-05-06 — Created plan from re-walk findings F12–F17 + email-transport section. F11 (concurrent session cap) was the last item from the original implementation order; PRs 1–5 closed F1–F11. This plan covers everything surfaced by the re-walk.
- 2026-05-06 22:45 — PR 6 (F12 + F13 + F14 + F16) implemented on a single branch.
  - F14: `auth_redirect_test.go` pins `isRelativePath` table + integration-shape assertion that `GoogleCallback` Location always starts with `frontendURL + "/"`.
  - F12: `store.DeleteSessionsForUser`, `AuthHandler.LogoutAll`, `POST /auth/logout-all` (handleAuth), frontend `logoutEverywhere` API + UserMenu "Sign out other devices" button with toast confirmation.
  - F13: `requireJSONContentType` middleware in the global chain returns 415 for non-GET/HEAD/OPTIONS without `Content-Type: application/json`. Frontend `request()` already always sets the header.
  - F16: `secureHeaders` middleware sets X-Content-Type-Options, Referrer-Policy, HSTS (gated on SECURE_COOKIES), Cache-Control no-store on `/auth/*`. Magic-link callback page also sets `referrer: "no-referrer"` via Next.js Metadata in a new `layout.tsx`.
  - Tests: handler unit tests for LogoutAll + redirect, server unit tests for both new middlewares, store integration test for DeleteSessionsForUser. `make lint`, `make test`, `make test-integration` all green.
- 2026-05-07 — PR 8 (F17) implemented. Skipped the debug-endpoint investigation in favour of acting on Google's documented behaviour: GFE appends to client-supplied `X-Forwarded-For` (`<spoofed>,<real-client>[,<lb>]`), so leftmost-trust was attacker-controlled. `remoteIP` now reads the rightmost XFF entry minus a configurable `TRUSTED_PROXY_DEPTH` (default 0 = Cloud Run direct, set 1 behind external HTTPS LB). Falls back to `r.RemoteAddr` when the proxy chain is shorter than the configured depth (safe failure mode). New `passphrase_test.go` table-tests forged-XFF rejection across direct + LB shapes. Both call sites (`auth.go:SendMagicLink`, `passphrase.go:Unlock`) plumbed through `cfg.TrustedProxyDepth`. Documented in `docs/infra-notes.md` and `backend/.env.example`. `make lint`, `make test`, `make test-integration` all green.
