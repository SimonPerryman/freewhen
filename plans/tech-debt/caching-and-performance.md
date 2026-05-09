# Caching and Performance

Status: parked

## Goal

Capture performance/caching ideas considered but deliberately deferred until there's a real signal (cost pressure, latency complaint, or measurable payload pain). This is a parking lot, not a backlog — none of these should be picked up without a concrete trigger.

## Context

Discussed 2026-05-04 while reviewing architecture. App is currently low-traffic, frontend is on Vercel (already CDN-cached), backend is on Cloud Run + Neon. Nothing here is justified yet by AGENTS.md principles ("don't design for hypothetical future requirements"), but worth recording so we don't re-litigate the same options later.

## Ideas (not prioritised — triggers listed)

### 1. ETag + 304 on `GET /calendars/{id}`

- **What:** Weak ETag derived from `calendars.updated_at` (e.g. `W/"<updated_at_unix>"`). Handle `If-None-Match` and return 304 when unchanged.
- **Trigger to revisit:** Mobile users report slow calendar loads on flaky networks, or egress costs become noticeable, or a typical calendar response grows past ~50KB.
- **Cost:** ~20 lines in `handler/calendar.go`, no new deps, no invalidation logic. Saves bandwidth, not DB load.
- **Why deferred:** SSE already tells clients when to refetch, so blind polling isn't happening. No observed payload or latency problem.

### 2. `Cache-Control: public, max-age=3600` on `GET /calendars/{id}/export.ics` (finalised only)

- **What:** Once a calendar has `finalised_at` set, the ICS export is effectively immutable for the chosen date. External calendar apps (Google, Apple) poll these on their own schedule.
- **Trigger:** Cloud Run egress or invocation count climbs and ICS exports are a meaningful slice.
- **Why deferred:** Volume is too low to matter, and reasoning about "is this really immutable?" (e.g. participant renames) needs care.

### 3. In-process LRU cache for `GET /calendars/{id}` on the backend

- **What:** Keyed by calendar ID, invalidated by the same write paths that publish SSE events.
- **Trigger:** Neon query latency or connection pressure becomes visible in metrics; Cloud Run instance count starts scaling on read traffic.
- **Why deferred:** Invalidation bugs are easy to introduce; at current QPS Neon handles reads trivially.

### 4. Shared cache (Upstash Redis / Vercel Runtime Cache via a proxy)

- **What:** Centralised read cache for calendar GETs, tag-based invalidation on writes.
- **Trigger:** Multi-instance Cloud Run with measurable cache-miss rate from #3, OR sustained Neon CPU pressure that read replicas don't solve.
- **Why deferred:** Two orders of magnitude more infra than the problem warrants today.

### 5. Frontend `Cache-Control` review

- **What:** Audit Next.js route segment config (`revalidate`, `dynamic`) and confirm Vercel is caching what it should. Mostly a sanity check.
- **Trigger:** Lighthouse score regression, or a PR that adds `dynamic = 'force-dynamic'` somewhere it shouldn't.
- **Why deferred:** Vercel defaults are sensible and we haven't fought them.

## Plan

When a trigger fires for one of the above:

1. Confirm the trigger with a metric (Cloud Run logs, Neon dashboard, user report with reproduction).
2. Pick the smallest item that addresses it (usually #1 or #5 first).
3. Move that item out of this file into its own `plans/tech-debt/<name>.md` with a real implementation plan.
4. Delete the entry here.

## Log

- 2026-05-04 21:23 — Created plan after architecture review. No work scheduled; this is a deferred-ideas register.
- 2026-05-09 — Status changed to `parked`. No trigger has fired and none is expected at current scale; this is a register, not a pending task.
