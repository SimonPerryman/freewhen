# Import-suggestions 404 noise + availability lost on unload

Status: done

## Goal

Two issues surfaced by a full-site e2e run:

1. Every calendar page load for a signed-in user logs a console 404 from
   `GET /calendars/{id}/import/suggestions` when there is no pending Google
   import. "Nothing to suggest" is the normal state, not a missing resource.
2. Availability edits made just before a reload/close can be lost: the save
   queue in `useAvailabilitySave` sends one PUT at a time and nothing flushes
   queued edits when the page goes away.

## Plan

### Suggestions

- Backend `GetSuggestions`: no cookie → `200 {"pending": false, "free": [], "busy": []}`.
  A real pending import returns `pending: true` plus the arrays. Unknown
  calendar stays `404 not_found`; malformed cookie stays `400`.
- Frontend `fetchImportSuggestions`: `pending === false` → `null`; still
  treats a 404 as `null` so a new frontend against an old backend (staggered
  deploy) stays quiet.
- Tests: handler unit test, integration test, frontend api test.
- Update `docs/api.md` and `docs/onboarding.html`.

### Unload flush

- On `pagehide` and `visibilitychange` → `hidden`, if there is queued
  (not-yet-sent) state, send it immediately with `fetch(..., { keepalive: true })`
  through the normal `request()` path (so `X-Participant-Token`,
  `X-Calendar-Access`, `credentials: "include"` and `Content-Type: application/json`
  are all present). Not `sendBeacon`: it can't set custom headers or a JSON
  content type for a credentialed cross-origin request.
- Nothing queued → nothing sent (an in-flight save already carries the
  latest state, so no double-send).
- Queued refs are consumed by the flush so the normal loop doesn't resend it.
  If the keepalive request fails while the page is still alive, re-queue it.
- CORS: preflight already allows `X-Participant-Token`, `Content-Type`,
  `PUT` and credentials (`server/middleware.go`). Body is a few KB at most,
  well under the 64 KiB keepalive budget.
- Tests: unit tests for the flush; e2e "toggle then reload immediately".

## Log

- 2026-11-12 10:00 — Created plan
- 2026-11-12 10:30 — Backend: no-cookie path returns 200 `{"pending":false,"free":[],"busy":[]}`; pending path adds `pending:true`. Handler unit + integration tests. Frontend `fetchImportSuggestions` maps `pending:false` and 404 to null; old responses without `pending` still work.
- 2026-11-12 11:00 — Unload flush: `flushOnHide` on `pagehide` / `visibilitychange`→hidden sends the queued-but-unsent request with fetch keepalive. Unit tests for in-flight/no-pending/visibility/failure-requeue/unmount.
- 2026-11-12 11:15 — First e2e run showed two more holes: (a) a non-keepalive save in flight at unload is cancelled, so every save now goes out with keepalive (in-flight saves survive; body >48 KiB falls back to a normal request); (b) the reloaded page's SSR snapshot can predate the flushed save, and live refreshes never touched the own selection, so the next toggle re-sent the stale set. `rehydrate` now syncs the selection until the first local edit after a seed.
- 2026-11-12 11:40 — e2e: "edits made right before a reload are not lost" (2 toggles + immediate reload) and "no 404 on suggestions" added. Full suite 15/15; reload test 24/24 with --repeat-each 8. Docs updated (api.md, onboarding.html).
