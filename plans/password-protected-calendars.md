# Password-Protected Calendars

Status: done

## Goal

Let calendar owners gate a calendar behind a shared password. Before the password is submitted, the calendar page reveals nothing beyond "this calendar is private — enter the password." Joiners submit the password once; a per-calendar access token is stored client-side and replayed on every API request for that calendar.

Implements the Tier 2 "Private links / password protection" item from `docs/v2-priorities.md`. Invite-code and approval flows are out of scope and deferred; the design leaves them as additive extensions (see "Future extensions").

## Scope

In:
- New nullable `passphrase` column on `calendars`, orthogonal to `visibility`. Server-generated, stored in plaintext (see Design Decisions).
- New `access_epoch INT NOT NULL DEFAULT 0` column on `calendars`, bumped on every passphrase change; included in issued tokens so rotation revokes existing access instantly.
- `POST /calendars/{id}/unlock` — body `{passphrase}`; returns `{token}` on success.
- `X-Calendar-Access: <token>` header on every calendar-scoped endpoint that leaks content (GET calendar, SSE stream, join, mark availability).
- Magic-link URL pattern: `?unlock=xyz` submitted on page mount and stripped from the URL via `history.replaceState` before any commit to history.
- Owner bypass — signed-in owner skips the gate.
- Owner UI: enable / regenerate / disable the passphrase from the calendar settings, alongside visibility. No free-form input — the server always generates.
- Brute-force protection: per-calendar + per-IP rate limit on the unlock endpoint.

Out:
- Invite codes (per-person or shared rotatable codes) — planned as an additive extension.
- Approval flow (owner clicks to let a joiner in) — different product shape.
- Forgotten-password reset — owner can clear + set a new password.
- Pushing password changes to already-connected SSE clients in real time.

## Design Decisions

### Orthogonal to `visibility`
`calendars.passphrase TEXT NULL`. Null means no passphrase set. Kept separate from `visibility` so "passphrase + submit_to_view" stacks naturally — the passphrase gates access, visibility gates what you see after joining. A single `access_mode` enum would collapse two orthogonal concepts.

### Server-generated passphrase (not user-chosen)
The server generates the passphrase. No free-form input, no complexity rules to design or validate. Four words from the EFF short wordlist (1296 words, 3–5 letters each), joined by hyphens: `word-word-word-word`, e.g. `frost-tulip-raven-stamp`. ~41 bits of entropy — easily enough against online guessing, especially with the rate limit in place. Reads fine in a URL, in a WhatsApp message, and aloud.

Owner actions: "Enable passphrase" (generates + stores), "Regenerate" (rotates + bumps `access_epoch`), "Disable" (clears). No "edit" — regenerate is the only path to a new value, which matches user intuition that changing the passphrase should revoke prior access.

Wordlist shipped with the backend binary (~20KB); single constant slice in an `internal/passphrase/` package.

### Storage: plaintext, no hashing
The passphrase is stored as plaintext. **This is a deliberate tradeoff, not an oversight.**

Why hashing isn't buying us much here:
- The standard reason to hash is to prevent password reuse harm — leaked hashes from Site A can't be tried against the user's bank. This passphrase is server-generated per-calendar and useless anywhere else, so that concern doesn't apply.
- A DB leak would already expose the gated content (participant names, availability, notes). Hashing the passphrase wouldn't protect anything the leak didn't already reveal — the passphrase only guards the API boundary.
- The passphrase is ~41 bits of entropy by construction, so offline brute-force of a hash is fast anyway; bcrypt's main benefit (slowing offline attacks on weak user-chosen passwords) doesn't apply.

Why plaintext is actually required here:
- Owner wants to share the link (`?unlock=<passphrase>`) to a second person next week without invalidating the first person's access. That needs the owner UI to display the current passphrase on any device — impossible with a one-way hash. The alternatives (show-once-on-generation, owner-side localStorage, a separate encrypted-at-rest column) all add complexity for no real security gain.

We'll leave a short schema comment (`COMMENT ON COLUMN calendars.passphrase IS ...`) pointing at this plan so future-you doesn't have to rederive the reasoning.

### Revocation on rotation: `access_epoch`
Stateless HMAC tokens are fast and DB-free but hard to revoke — without server state, they stay valid until their TTL expires (30 days here). That means a "regenerate passphrase" click wouldn't actually kick out anyone who had already unlocked; they'd keep accessing the calendar for up to a month.

Fix: a single `access_epoch INT NOT NULL DEFAULT 0` column on `calendars`, bumped on every passphrase change. The epoch is baked into the signed token payload (`calendarID|epoch|expiry|sig`) and checked on every request: if the presented token's epoch is lower than the calendar's current epoch, reject. The client clears its stored token on the resulting 401 and re-renders the gate.

This is a standard pattern (Django's `get_session_auth_hash`, Devise's session versioning, JWT token-version claims) — the minimum server state needed to add revocation to an otherwise stateless token scheme.

### Client token storage: localStorage + custom header (chosen over cookies)

Two viable shapes. Writing both down for the next person who asks.

**Option A — one cookie per calendar** (`calendar_access_<id>=<token>`, `Path=/`, `SameSite=None; Secure` for cross-origin API). The browser sends the right cookie per request automatically; multi-tab is natural. Downsides:
- Cookie count per domain is capped (~50); power users hit the ceiling eventually.
- Cleanup on password rotation, calendar deletion, or logout requires explicit `Set-Cookie ... Max-Age=0`, which is awkward when the API is on a separate origin.
- `SameSite=None; Secure` + credentialed CORS is a non-trivial amount of plumbing given the frontend (Vercel) and backend (Cloud Run) are on different origins.
- HttpOnly would harden against XSS, but we'd still need a JS-readable token anyway (to strip from the URL and to send on `fetch`), so the HttpOnly benefit is lost in practice.

**Option B — localStorage + `X-Calendar-Access: <token>` header** (chosen). Client keeps `{ [calendarId]: token }` in localStorage and attaches the matching token as a custom header on every request to that calendar. Multi-tab works trivially because tabs share localStorage. No cookie-count ceiling; no `Path=` juggling. Tokens are cleared by the client on any 401 response. Mirrors the existing pattern — participant identity already lives in localStorage + `?as=` query param; the access token slots in next to it.

**Why B wins here:**
- Multi-tab is the dominant shape (explicitly called out).
- Frontend and backend are on different origins; cookies need extra credentialed-CORS plumbing that localStorage avoids.
- Cleanup is `delete calendarAccess[id]` — no server round-trip to clear a cookie.
- Pattern already established in this codebase for per-calendar identity.

The XSS tradeoff is neutral between A and B in this design — both require JS-readable tokens. Accepted for a low-stakes availability tool with no PII behind the gate.

### Token shape: stateless, HMAC-signed, epoch-aware
`base64(calendarID|epoch|expiry|sig)` where `sig = HMAC-SHA256(server_secret, calendarID|epoch|expiry)`. 30-day expiry. No DB table beyond the per-calendar `access_epoch` int already discussed. Rotating the server secret invalidates every outstanding access token at once — the right blast radius if the secret ever leaks. Store it in config (`CALENDAR_ACCESS_SECRET`, 32+ random bytes; refuse to boot if unset).

Alternative considered: per-access DB row with a random token. Rejected — adds a table and a write on every unlock for no feature gain over `access_epoch`, which gives us instant revocation with a single int per calendar.

### `?unlock=xyz` magic link
The share dialog offers "copy link with passphrase" → URL contains `?unlock=<passphrase>` (e.g. `?unlock=frost-tulip-raven-stamp`). On mount, `CalendarPageClient` detects the query param, POSTs it to `/unlock`, stores the returned token in localStorage, and calls `history.replaceState` to strip the query before any render commits the URL to history.

Caveats to flag in the owner-side UI:
- The URL containing `unlock` will land in browser history and in client-side HTTP access logs.
- Server MUST NOT log the `unlock` query param. Add it to the request-log redaction set.

One-click join is the whole point of the tool; worth the tradeoff.

### Rate limiting
Per-calendar + per-IP sliding window on `POST /unlock`: 5 attempts per minute per (calendarID, IP). Return 429 with `Retry-After`. In-memory store (map + mutex) for now; move to Redis if horizontal scaling becomes real. A successful unlock resets the counter for that pair.

### Owner bypass
Handler checks `userID == calendar.ownerID` before consulting the access token. Admin participants are NOT exempt — they're joiners with extra privileges and go through the same gate. Revisit if someone complains.

### What's behind the gate
Everything that leaks content:
- `GET /calendars/{id}` (name/emoji/finalised state are all potentially sensitive)
- `GET /calendars/{id}/events` (SSE)
- `POST /calendars/{id}/participants`
- `POST /calendars/{id}/availability`
- Any future per-participant endpoints

For an unauth'd viewer without a valid token, `GET /calendars/{id}` returns `200 {requires_passphrase: true}` with no other fields. That lets the client render the gate without leaking whether the calendar exists beyond the ID already in the URL. 404 was considered but rejected — the share link needs to render the gate cleanly, and the ID is already known to the attacker holding the URL.

## API

### Schema

Migration `012_add_calendar_passphrase.sql`:
```sql
ALTER TABLE calendars ADD COLUMN passphrase TEXT;
ALTER TABLE calendars ADD COLUMN access_epoch INT NOT NULL DEFAULT 0;
COMMENT ON COLUMN calendars.passphrase IS
  'Server-generated passphrase (word-word-word-word). Stored plaintext — deliberate tradeoff; see plans/password-protected-calendars.md.';
```

### Model

`model.Calendar` gains:
- `HasPassphrase bool` — derived, serialised to clients.
- `Passphrase string` — surfaced to the **owner only** in the API response (so the owner UI can render "copy link with passphrase" on any device). Never serialised to non-owners.
- `AccessEpoch int` — internal; baked into issued tokens, not serialised.

### Endpoints

- **`POST /calendars/{id}/unlock`** — body `{passphrase}`. Rate-limited. Constant-time compare against the stored passphrase; on success issues a token including the current `access_epoch`. On failure: 401. If no passphrase is set on the calendar: 400.
- **`POST /calendars/{id}/passphrase`** — owner only. Generates a new passphrase, bumps `access_epoch`, returns `{passphrase}`. This is the single mutator — used for both "enable" (first time) and "regenerate".
- **`DELETE /calendars/{id}/passphrase`** — owner only. Clears the passphrase and bumps `access_epoch` (so outstanding tokens die even though the gate is now disabled — clean symmetry).
- **`GET /calendars/{id}`** — if `passphrase IS NOT NULL` and caller is neither owner nor carrying a valid `X-Calendar-Access` token whose epoch matches the calendar's current `access_epoch`, respond `200 {requires_passphrase: true}` only. Otherwise return the normal view (with `passphrase` included iff the caller is the owner).
- **SSE and mutating endpoints** — return 401 without a valid token (or with a stale-epoch token); client clears local token and re-renders the gate.

## Backend Implementation

### Phase 1 — Schema, model, passphrase generator
- Migration 012 (`passphrase`, `access_epoch`, column comment).
- No new external dependency.
- New package `internal/passphrase/` with the EFF short wordlist as a constant slice and `Generate() string` → four random words joined by `-` (crypto/rand picked indices).
- Add `passphrase` (`sql.NullString`) and `access_epoch` (int) to the calendar store reads/writes; expose `HasPassphrase bool`, `Passphrase string`, and `AccessEpoch int` on the public model (with the serialiser gating `Passphrase` to owner-only responses at the handler layer).
- New package `internal/access/` with `IssueToken(calendarID string, epoch int, secret string, ttl time.Duration) string` and `VerifyToken(token, secret string) (calendarID string, epoch int, ok bool)`.

### Phase 2 — Store
- `SetCalendarPassphrase(ctx, calendarID string, passphrase *string)` — writes the value or NULL; bumps `access_epoch` by 1 in the same UPDATE.
- Include `passphrase` and `access_epoch` in existing SELECTs.

### Phase 3 — Service
- `UnlockCalendar(ctx, calendarID, passphrase, remoteAddr string) (token string, err error)`:
  - Rate-limit check.
  - Load calendar; if no passphrase set, `ErrNotPassphraseProtected` (400).
  - Constant-time compare (`subtle.ConstantTimeCompare`); on mismatch, register failure, return `ErrUnauthorized` (401).
  - Issue token embedding the current `access_epoch`; return.
- `RotateCalendarPassphrase(ctx, calendarID string) (passphrase string, err error)` — generate, store via `SetCalendarPassphrase`, return the plaintext.
- `ClearCalendarPassphrase(ctx, calendarID string) error` — null the column via `SetCalendarPassphrase(nil)`; still bumps epoch.
- `AuthorizeAccess(calendarID, tokenHeader string, viewer Viewer) error` — owner first; then verify token signature, expiry, and epoch match. Returns `ErrPassphraseRequired` / `ErrPassphraseInvalid`.

### Phase 4 — Handler & middleware
- `POST /calendars/{id}/unlock` handler.
- `POST /calendars/{id}/passphrase` and `DELETE /calendars/{id}/passphrase` handlers (owner only; reuse the existing owner-check pattern).
- Middleware wrapping the gated endpoints: call `AuthorizeAccess`; on `ErrPassphraseRequired` respond `{requires_passphrase: true}` for GET, 401 for mutations / SSE.
- `GET /calendars/{id}` serialiser: include `passphrase` in the response body iff the caller is the owner.
- Add `unlock` to the request-log redaction set (query-param allowlist / denylist).
- Config: read `CALENDAR_ACCESS_SECRET`; refuse to boot if unset.

### Phase 5 — Routes
- `POST /calendars/{id}/unlock` — new route (rate-limited).
- `POST /calendars/{id}/passphrase`, `DELETE /calendars/{id}/passphrase` — new routes (owner only).
- Apply gate middleware to `/calendars/{id}`, `/calendars/{id}/events`, `/calendars/{id}/availability`, `/calendars/{id}/participants`.

## Frontend Implementation

### Phase 1 — API client & storage
- `src/lib/calendarAccess.ts` — load / save / delete per-calendar tokens in localStorage under a single `calendarAccess` key.
- `api.ts` — every calendar-scoped request attaches `X-Calendar-Access: <token>` when present. A 401 response clears the stored token and signals the caller to re-render the gate.
- `unlockCalendar(id, passphrase)` — POST `/unlock`, save token on success.
- `rotateCalendarPassphrase(id)` / `clearCalendarPassphrase(id)` — owner-only mutators against the new endpoints.

### Phase 2 — Gate component
- `<PassphraseGate calendarId=... onUnlocked=... />` — passphrase input, submit button, inline error, rate-limit notice.
- Rendered by `CalendarPageClient` when GET returns `{requires_passphrase: true}`.
- On unlock success, refetch the calendar.

### Phase 3 — Magic-link handling
- On mount, if `?unlock=xyz` present:
  - Call `unlockCalendar(id, unlock)`.
  - `history.replaceState` to strip the query param immediately (before any render commits it to history).
  - On success, continue to calendar load. On failure, show the gate with a "that link is no longer valid" banner.

### Phase 4 — Owner UI
- Calendar settings (next to the visibility toggle):
  - "Require a passphrase" toggle. Turning on calls `POST /passphrase`; turning off calls `DELETE /passphrase`.
  - When on: display the current passphrase (read from the owner-only field on the GET response), with a "Regenerate" button that calls `POST /passphrase` again and warns that existing share links will stop working.
  - "Copy link with passphrase" button in the share dialog — writes `?unlock=<passphrase>` to the clipboard.
  - Inline note: "Anyone with this link can view the calendar. Regenerating the passphrase invalidates the old link immediately."

### Phase 5 — Analytics
- `calendar_passphrase_set { calendar_id, action: "enabled" | "rotated" | "cleared" }`
- `calendar_unlock_attempted { calendar_id, via: "form" | "magic_link", success: bool }`

## Testing

### Backend
- `passphrase.Generate`: returns four hyphen-separated words from the wordlist; entropy smoke test (1000 generations → no collisions).
- Store: set / rotate / clear passphrase, `access_epoch` increments by exactly 1 each time, round-trip `HasPassphrase`.
- Service:
  - Unlock happy path — token includes current epoch.
  - Wrong passphrase → 401, counter increments.
  - 5 failures → 429 on the 6th.
  - Successful unlock resets the counter.
  - Token round-trip; verify rejects expired, tampered, wrong-calendar, and stale-epoch tokens.
  - Rotate passphrase → prior token fails on next request (stale epoch).
- Handler:
  - GET without token on passphrase-protected calendar → `requires_passphrase: true`.
  - GET with valid token → full view, passphrase NOT included.
  - GET as owner → full view, `passphrase` included in response body.
  - Owner-only enforcement on `POST` / `DELETE /passphrase`.
  - POST unlock rate limit returns 429.
- Integration: create passphrase-protected calendar, unlock as second user, load calendar, owner rotates passphrase, verify stale token is rejected on next request, new unlock with new passphrase works.

### Frontend
- `<PassphraseGate>` submits, calls `unlockCalendar`, stores token, invokes `onUnlocked`.
- Mount with `?unlock=xyz` triggers unlock + `replaceState`.
- 401 on any gated call clears the stored token and re-renders the gate.
- Owner toggle: on generates + displays passphrase; regenerate bumps it; off clears.

## Future extensions (invite codes)

The unlock plumbing is deliberately generic so invite codes slot in cleanly later:
- New table `calendar_invite_codes(id, calendar_id, code_hash, created_by, expires_at, revoked_at)`.
- `POST /calendars/{id}/unlock` gains a branch: try the shared passphrase first, fall back to the invite-code table.
- Same token issuance, same `X-Calendar-Access` header, same `?unlock=` URL pattern, same frontend gate. No migration needed for existing passphrase users.

Approval flows are a bigger design and not implied by this plan.

## Build Order
1. Backend Phase 1–2 (migration, store, access package).
2. Backend Phase 3 (service, rate limiter, unlock).
3. Backend Phase 4–5 (handler, middleware, routes).
4. Backend tests (unit + integration).
5. Frontend Phase 1 (storage + API client).
6. Frontend Phase 2 (gate component).
7. Frontend Phase 3 (magic-link URL handling).
8. Frontend Phase 4 (owner UI).
9. Frontend Phase 5 (analytics).
10. Manual QA: create a passphrase-protected calendar, open in a second browser, verify gate, unlock, verify access. Open in two tabs at once, verify both unlocked. Clear localStorage, verify gate returns. Rotate passphrase on owner side, verify stale tabs are re-gated on their very next request (not after a 30-day expiry).

## Resolved questions
- **Passphrase complexity rules** — moot. Server generates a 4-word passphrase from the EFF short list; no user input path, no validation UI, no rules.
- **Revocation on passphrase change** — resolved via `access_epoch`. Rotate or clear bumps the epoch; stale tokens are rejected on the next request.

## Log

- 2026-04-20 16:00 — Created plan. Confirmed scope with user: shared passphrase only (invite codes + approval deferred); localStorage + `X-Calendar-Access` header chosen over cookies (both options documented, multi-tab + cross-origin drove the choice); stateless HMAC token; `?unlock=` magic link with `history.replaceState` strip on mount.
- 2026-04-20 16:30 — Resolved remaining design questions: (1) server generates a 4-word passphrase from the EFF short list — no user-chosen passwords, no complexity rules; (2) passphrase stored plaintext, not hashed — deliberate tradeoff (no credential-reuse risk for server-generated values; DB leak already exposes the gated content; owner needs readback to share the `?unlock=` link from any device); (3) `access_epoch` column bumped on rotate / clear and embedded in signed tokens → instant revocation, same pattern Django / Devise / JWT recipes use; (4) dropped the bcrypt dependency from the plan accordingly.
- 2026-04-21 08:28 — Implemented. Backend: migration 012 adds `passphrase` + `access_epoch`; new `internal/passphrase` (curated ~700-word list + `Generate`), `internal/access` (HMAC-signed stateless token), service-level unlock/rotate/clear + per-(calendar,IP) sliding-window rate limiter, handler + middleware for `POST /calendars/{id}/unlock`, `POST|DELETE /calendars/{id}/passphrase`; `GET /calendars/{id}` now emits `{requires_passphrase: true}` for unauthorized viewers and includes plaintext passphrase only for the owner. `AddParticipant` / `SetAvailability` / SSE stream gate on the token. `CALENDAR_ACCESS_SECRET` required at boot. Frontend: per-calendar tokens in `calendarAccess.ts`; `X-Calendar-Access` header threaded through `api.ts`; SSE carries the token via `?access=`; new `PassphraseGate` component; `CalendarPageClient` handles `?unlock=` magic link with `history.replaceState`, re-gates on 401, owner UI for enable/regenerate/disable/copy-link. Tests added for passphrase generator, access token round-trip, service unlock/rotate/clear/auth.
