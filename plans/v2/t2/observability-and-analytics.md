# Observability and Analytics

Status: done

## Goal

FreeWhen currently has no product instrumentation. We can't answer basic questions like "how many signups this week?", "how many calendars got created?", "how many invite links were shared and opened?", or "what's the average number of participants per calendar?". There's also no error tracking — failures in prod are invisible unless someone reports them.

Wire up a lean observability stack that answers those questions without adding operational burden or cost. Target: stay on free tiers, minimise vendor lock-in, keep the app fast.

## Approach

Two tracks, done in order so the cheap-and-SQL-derivable stuff ships first:

1. **SQL-derivable metrics** (signups, calendars created, participants, avg participants per calendar) — no vendor needed, just a small admin endpoint + SQL queries.
2. **Event-derivable metrics** (invite shares, calendar opens by invitees, share → join funnel) + error tracking — via PostHog, one SDK covering analytics + session replay + exceptions.

Why PostHog: free tier of 1M events/mo + 5K session recordings + error tracking in one SDK. EU hosting available. Replaces the need for separate Sentry + GA + Mixpanel.

## Plan

### Phase 1 — Admin role + SQL-derivable product metrics

**Admin gating (`is_admin` on users):**

- New migration in `internal/store/migrations/` adding `is_admin BOOLEAN NOT NULL DEFAULT FALSE` to `users`.
- Add `IsAdmin bool` to the `User` model and include it in user `SELECT`s in `internal/store/`.
- Add `requireAdmin` middleware in `internal/server/middleware.go` — wraps `requireAuth`, loads the session user, 403s if `!IsAdmin`.
- Include `is_admin` in the `/auth/me` response so the frontend can (later) conditionally render an admin link. No UI change in this phase.
- Bootstrap: run `UPDATE users SET is_admin = true WHERE email = '<your email>';` against Neon after the migration lands. Document the command in `docs/infra-notes.md`.

**Metrics endpoint:**

- Add a new backend handler `internal/handler/metrics.go` exposing `GET /admin/metrics`, mounted behind `requireAdmin`.
- Queries to implement in `internal/store/`:
  - `CountUsers()`, `CountUsersSince(t)` — total signups, signups in window
  - `CountCalendars()`, `CountCalendarsSince(t)` — total / recent calendars
  - `CountParticipants()`, `AvgParticipantsPerCalendar()` — engagement depth
  - `ActiveCalendars(days)` — calendars with any availability change in last N days
- Return JSON like `{ users: { total, last_7d, last_30d }, calendars: { total, last_7d, active_30d }, participants: { total, avg_per_calendar } }`.
- Add a backend integration test (under `-tags=integration`, run via `make test-integration`) covering both the admin-gate (403 for non-admin) and the metrics shape.
- Document the endpoint in `docs/api.md`.

### Phase 2 — Thread X-Request-ID end-to-end

Small ergonomic win that makes Phase 3 debugging much nicer.

- In `frontend/src/lib/api.ts`, capture the `X-Request-ID` response header on every fetch.
- Attach `requestId` as a property on thrown API errors (not a module-level `lastRequestId()` — that's racy under concurrent fetches). The log wrapper and PostHog error capture can then read `err.requestId` to correlate with backend logs.

### Phase 3 — PostHog for events + errors

- Sign up for PostHog **EU cloud** (`https://eu.i.posthog.com`), grab project API key. Store in `NEXT_PUBLIC_POSTHOG_KEY` and `NEXT_PUBLIC_POSTHOG_HOST` (set host to the EU ingest URL).
- Install `posthog-js` in `frontend/`.
- Centralise init in a single module `src/lib/analytics.ts` — everything else imports from here. This is the single hook point for future consent gating.
  - **Gate on config, not `NODE_ENV`** (per AGENTS.md principle): if `NEXT_PUBLIC_POSTHOG_KEY` is unset, `analytics.init()` is a no-op and all event helpers become no-ops. Dev simply doesn't set the key; prod does. No `process.env.NODE_ENV` checks anywhere.
  - Before committing to option names (`autocapture`, `session_recording`, `capture_exceptions`, `person_profiles`), verify against current `posthog-js` docs via context7 — APIs drift.
  - Intended config: autocapture on, session recording on, exception capture on, `person_profiles: 'identified_only'` so anonymous visitors don't get profiled (reduces anonymous-tracking surface area for the friends-only stage).
  - Add a `// TODO: consent` comment next to the `posthog.init()` call as the hook for a future consent banner. No `ConsentProvider` yet — YAGNI until there's a real user base.
- Wire into `src/app/layout.tsx` via a small client-component wrapper that calls `analytics.init()` once on mount.
- Identify authenticated users via `analytics.identify(user.id, { email })` from `AuthProvider` when a user logs in; `analytics.reset()` on logout.
- Fire explicit events for the moments that matter (SQL can't see these):
  - `calendar_created` (from the create-calendar flow; include `calendar_id`)
  - `invite_shared` (in `ShareButton` — distinguish `method: 'native' | 'clipboard'`)
  - `calendar_opened` (on the calendar page load; include `is_owner: bool`, `calendar_id`)
  - `participant_added` (on successful participant submit; include `calendar_id`)
  - `availability_marked` (fire once per submit, not per cell, to avoid event spam)
- Verify events land in PostHog dashboard, build a starter dashboard with:
  - Daily signups, calendars created, participants added
  - Funnel: `calendar_opened` (non-owner) → `participant_added` → `availability_marked`
  - Share method split (native vs clipboard)

### Phase 4 — Dashboard link + docs

- Add a short section to `docs/infra-notes.md` covering the admin metrics endpoint, the PostHog project URL, and how to grant `is_admin` to a user.
- Call out in `docs/infra-notes.md` that **a consent banner is required before wider rollout** beyond friends-and-family — session replay + analytics cookies need explicit consent under UK/EU rules. The init module is the single hook point when that lands.
- Optional: add a reference memory so future Claude sessions know where to look for metrics (`reference` memory pointing at PostHog project + `/admin/metrics`).

## Non-goals

- Self-hosting PostHog. The cloud free tier is more than enough at current scale and avoids ops work.
- Backend tracing (OpenTelemetry, Tempo, etc.). Request-ID logs are sufficient until we have a real latency problem to chase.
- Invite-link click tracking via a redirect. We can derive opens from `calendar_opened { is_owner: false }` in PostHog without touching the URL structure.
- A separate errors vendor (Sentry). PostHog's exception capture covers us for now; if it proves too thin, revisit.
- Backfilling historical metrics — everything is forward-looking from deploy date.

## Log

- 2026-04-19 — Created plan
- 2026-04-19 — Resolved open questions: using `is_admin` flag on `users` (not env bearer token) and PostHog EU cloud region
- 2026-04-19 — Revised Phase 3 to gate on `NEXT_PUBLIC_POSTHOG_KEY` presence rather than `NODE_ENV` (per new AGENTS.md 12-factor principle); centralised init in `src/lib/analytics.ts` as the single hook point for future consent; added `person_profiles: 'identified_only'` for anonymous-tracking minimisation; deferred full consent UI as YAGNI but documented it as a rollout gate. Also fixed Phase 2 `requestId` handling (attach to errors, not a racy module-level getter) and Phase 1 test terminology (integration, not unit).
- 2026-04-19 16:44 — Implemented all four phases: migration 005 adds `is_admin` column, `requireAdmin` middleware, `GET /admin/metrics` endpoint with SQL-derived product metrics, `X-Request-ID` threaded through `ApiError`, PostHog analytics module with config-gated init, events wired (calendar_created, invite_shared, calendar_opened, participant_added, availability_marked), identify/reset in AuthProvider, docs updated. All tests pass (86 frontend, Go unit + vet clean).
