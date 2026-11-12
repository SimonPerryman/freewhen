# Calendar ID entropy + by-id lookup rate limit

Status: done

## Goal

Security review finding L6. For a calendar without a passphrase, the calendar
ID is the only secret. IDs were 8 chars of `[a-z0-9]` (~41 bits) produced by
`store.generateID` with a slight modulo bias (`byte % 36`), and the routes that
resolve a calendar by ID without a session had no rate limit, so the ID space
could be probed for free.

1. New calendar IDs: 12 chars of `[a-z0-9]` (~62 bits), sampled without bias.
2. Per-client-IP rate limit on every route that resolves a calendar by ID
   without requiring a session, so enumeration is impractical while real
   users never notice.

## Plan

### IDs

- `generateID` switches to rejection sampling: draw random bytes, discard any
  byte >= 252 (the largest multiple of 36 that fits in a byte), map the rest
  with `% 36`. Every character is uniform.
- `store.CalendarIDLength = 12`, used by `CreateCalendar` and
  `DuplicateCalendar`. No migration: `calendars.id` is `TEXT`, there is no ID
  regex/length check in routes, handlers or the frontend, so existing 8-char
  IDs keep resolving.
- Participant, user and invite IDs share the generator and get the unbiased
  sampling, but keep their lengths: none of them is an access secret
  (participant IDs are returned in every calendar read and authorised by a
  separate participant token; user IDs sit behind sessions; invite IDs are
  owner-only routes).

### Rate limit

- New `handler.CalendarLookupLimiter`, reusing `internal/ratelimit` buckets and
  the existing `remoteIP` (trusted `X-Forwarded-For` depth, same as magic-link
  and unlock limiting). IPv6 clients are keyed by their /64 so a single host
  can't rotate through its prefix.
- Applies to every `handleOptional` `/calendars/{id}...` route (GET calendar,
  join, rename participant, set availability, unlock, exports, SSE). Each of
  these returns 404 for an unknown ID before any credential check, so each is
  an enumeration oracle; limiting only the GETs would leave the others open.
  Applied regardless of session so a throwaway account doesn't bypass it.
- Two budgets, constants in `internal/limits` (no existing env-config pattern
  for rate limits; magic-link limits are constants too):
  - `CalendarLookupsPerIPPerMinute = 120` — burst 120, refills 2/s. A real
    page view is ~2 reads; polling fallback is 4/min; vote toggles are one PUT
    each.
  - `CalendarStreamsPerIPPerMinute = 60` — SSE connects only, a separate
    bucket so a burst of refetches can never starve the stream. The browser
    reconnects ~every 3s at worst (20/min) and the hook falls back to polling
    after repeated errors, so 60/min with burst 60 never trips for a real
    client, including several tabs.
- 429 with `{"error": ..., "code": "rate_limited"}` and `Retry-After` (time for
  one token to refill, rounded up to whole seconds).
- `ratelimit.Bucket` gains a periodic prune of fully-refilled keys (dropping a
  key is equivalent to keeping a full bucket), since a bucket keyed on every
  client IP of every calendar read would otherwise grow without bound.

### Known trade-off

The Next.js server render (`getCalendarForServer`) calls `GET /calendars/{id}`
from Vercel's egress IPs, so SSR traffic shares those IPs' budget. On a 429
the SSR fetch returns null and the client does its own fetch from the user's
IP, so the failure mode is a slower first paint, not a broken page. Revisit
(e.g. a signed SSR header) if traffic ever gets near 120 SSR renders/min per
egress IP.

### Docs

- `docs/api.md`: rate-limit section for calendar lookups.
- `docs/onboarding.html`: mention the limiter and 12-char IDs.

### Tests

- ID length/charset, uniqueness, per-character distribution (chi-squared-ish
  bound), store integration asserting 12-char calendar IDs.
- Old 8-char IDs still resolve (integration: insert an 8-char calendar row,
  GET it).
- Limiter: 429 + Retry-After + JSON shape after the burst, separate IPs
  independent, IPv6 /64 grouping, SSE bucket independent of the read bucket
  and a simulated reconnect loop never tripping.
- Bucket prune keeps semantics.

## Log

- 2026-11-12 10:00 — Created plan
- 2026-11-12 10:20 — `generateID` switched to rejection sampling; `store.CalendarIDLength = 12` used by create + duplicate. Participant/user/invite IDs keep their lengths (not access secrets) but get the unbiased sampler. No ID-length validation existed anywhere, so no frontend/route changes were needed.
- 2026-11-12 10:40 — `ratelimit.Bucket` gained `RefillInterval`, `WithClock` (test clock) and an amortised prune of fully-refilled keys.
- 2026-11-12 10:55 — `handler.CalendarLookupLimiter` (lookups 120/min, SSE 60/min, constants in `internal/limits`) wired into `handleOptional`, which now requires a limiter; the limit runs before `optionalAuth`. Route-coverage test asserts every optional route is limited.
- 2026-11-12 11:15 — Tests: ID charset/length/chi-squared distribution, legacy 8-char ID resolves (store + HTTP), 429 + Retry-After + JSON shape, per-IP independence, trusted XFF hop, IPv6 /64 grouping, SSE bucket independence, a simulated 1h 3-tab reconnect loop never throttled. Docs: api.md, onboarding.html. Unit + integration suites green.
