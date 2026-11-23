# Live Updates

Status: done

## Goal

Push server-side changes to connected clients so that when one participant updates their availability (or the owner finalises, renames, or toggles visibility), everyone else viewing the calendar sees it within ~1s without a manual refresh. This makes the "everyone fill this in now, on the same Zoom call" flow seamless and removes the stale-view bug that exists today whenever two people are on the page at once.

It also closes a loose end from the Submit-to-View plan: a visibility mode change by the owner currently only takes effect on the next refetch. With live updates, viewers flip to the new mode in real time.

## Scope

**In**
- Server → client push for:
  - availability changes (any participant's dates edited)
  - participant added / removed / renamed (rename is not yet a feature — availability-only)
  - calendar renamed / emoji changed
  - calendar visibility changed
  - calendar finalised / unfinalised
- Browser subscribes while the calendar page is open and unsubscribes on unmount / page hide.
- Graceful fallback to polling if the stream fails or is unavailable.
- Enforce the submit-to-view reveal rules on the stream (hidden viewers must not receive participant data they're not entitled to).

**Out**
- Presence ("N people viewing now"). Interesting but separate product question.
- Typing indicators / draft selections before save.
- Push notifications to offline users — separate Tier 2 item.
- Client → server push (all writes stay on REST).

## Design Decisions

**Transport: Server-Sent Events, not WebSockets.** All push traffic in this app is one-way (server → client); client writes are already REST. SSE is plain HTTP, works through Cloud Run and any reasonable proxy without special config, has browser-native automatic reconnect with `Last-Event-ID`, and needs no extra Go dependency (`http.Flusher` is stdlib). WebSockets would give us bidirectional traffic we don't need, plus a library dependency and extra Cloud Run wiring (session affinity, upgrade-header handling) for no benefit. If a future feature needs bidirectional (e.g. collaborative drag-select), revisit then.

**Refresh-pointer events, not deltas.** The server emits a tiny typed event like `{"type":"availability.changed","calendar_id":"..."}`. The client reacts by calling the existing `getCalendar(id, as?)` to re-sync. Benefits:

- One code path for visibility/reveal logic — the authoritative hide happens in `GetCalendarView`, not duplicated into an event filter.
- No risk of an event leaking data a hidden viewer shouldn't see: the payload carries no participant info.
- Client state stays derived from a single source (the GET response), so no merge/ordering bugs.
- The cost is one extra HTTP GET per event (~tens of ms on same-region). Fine for this app's scale and event rate.

**In-process hub for v1, document scale-out.** A `realtime.Hub` with one channel per calendar, held in the `Server` struct. Publishers (service layer, after a successful write) call `hub.Publish(calendarID, event)`. Subscribers (the SSE handler) register a channel and drain it. This works as long as all SSE connections for a given calendar land on the same process instance. On Cloud Run today with low traffic this is effectively always true (one instance), but it breaks silently once we scale horizontally.

Plan explicitly:
1. Ship in-process hub.
2. Set Cloud Run `min_instances=1`, `max_instances=1` for the backend while this ships, to avoid the fan-out problem.
3. File a follow-up for Postgres `LISTEN`/`NOTIFY` (Neon supports it on direct connections) as the scale-out hop. One `NOTIFY freewhen_events, '<calendar_id>:<event_type>'` per publish; each instance holds a dedicated `LISTEN` connection and re-fans events into the in-process hub.

Not doing LISTEN/NOTIFY now because (a) it adds a second DB connection pattern and (b) single-instance is fine at current scale. But the hub interface is designed so swapping the transport is one file change.

**Authenticating the stream.** The SSE endpoint lives at `GET /calendars/{id}/events` and uses `optionalAuth` — same as `GET /calendars/{id}`. Cookie auth works out of the box with `EventSource(url, { withCredentials: true })` given our existing CORS setup. Anonymous viewers identify themselves with `?as={participantId}` in the query string, consistent with the GET.

Why no token-in-URL concern: the calendar ID is the primary access token in this product already (no-auth invite links). The SSE endpoint exposes strictly less information than the GET — it pushes event *pointers*, not content. A viewer without reveal access who subscribes will receive `availability.changed` pings but their subsequent GET will return the hidden shape. No data leak; at worst they learn "activity happened."

**Heartbeat + reconnect.** Server writes an SSE comment line (`: ping\n\n`) every 25s per connection to keep proxies from killing idle streams. Client relies on browser-native `EventSource` reconnect (exponential backoff, automatic). On reconnect, client refetches immediately as if a generic event had fired, so any events missed during the gap are caught by the resync.

**Fallback: polling.** If `EventSource` fails to connect or errors three times in a row, the hook falls back to a 15s `setInterval` refetch. Covers broken corporate proxies and anything that blocks streaming. Logged once to analytics so we can measure how often this happens.

**Which writes publish.** Every service method that changes calendar-visible state publishes exactly one event after the DB commit:

| Service method | Event type |
|---|---|
| `SetAvailability` | `availability.changed` |
| `AddParticipant` | `participant.joined` |
| `DeleteParticipant` | `participant.removed` |
| `ClaimParticipant` | `participant.claimed` |
| `UpdateCalendarName` | `calendar.updated` |
| `UpdateCalendarVisibility` | `calendar.updated` |
| `SetFinalisedDate` | `calendar.finalised` |
| `ClearFinalisedDate` | `calendar.unfinalised` |

The event type is informational only — the client's reaction is the same ("refetch") in every case. Typing exists so we can (a) log useful analytics, (b) show optional toasts like "Jamie joined" later without a schema change.

Publish happens in the service layer (not the handler) so every code path — including future internal callers, cron jobs, etc. — emits correctly. Publish failures never fail the request; we log and move on.

**Debounce on the client.** If several events arrive in quick succession (owner flips visibility, then two people mark availability), coalesce to a single refetch with a 150ms trailing debounce. Keeps the page from hammering `getCalendar` during a burst.

## API

### New endpoint

`GET /calendars/{id}/events` — SSE stream.

Query params:
- `as` (optional) — participant ID. Accepted for symmetry with the GET, though the stream itself carries no participant-scoped payload, so `as` is currently unused server-side. Keeping the param reserved so we don't break clients when it becomes meaningful (e.g. future "your own changes are echoed" suppression).

Response headers:
- `Content-Type: text/event-stream`
- `Cache-Control: no-cache, no-transform`
- `Connection: keep-alive`
- `X-Accel-Buffering: no` — harmless if ignored, but tells reverse proxies not to buffer.

Event format:
```
event: availability.changed
data: {"calendar_id":"abc123"}

```

Heartbeat:
```
: ping

```

Close semantics: handler returns when `r.Context().Done()` fires (client disconnect) or when the hub closes the subscriber channel (server shutdown).

### No other API changes

All existing endpoints are unchanged. Publishers are wired into the service layer.

## Backend Implementation

### Phase 1 — Hub

New package `internal/realtime/`:

```go
type Event struct {
    Type       string
    CalendarID string
}

type Hub struct { /* mu, subs map[calendarID]map[*sub]struct{} */ }
func NewHub() *Hub
func (h *Hub) Publish(calendarID, eventType string)
func (h *Hub) Subscribe(calendarID string) (<-chan Event, func())
```

- Bounded per-subscriber channel (capacity 16). If full, drop the event and log at debug — a slow client isn't allowed to back-pressure the hub.
- `Subscribe` returns an unsubscribe closure that removes the channel and closes it.
- All state guarded by a `sync.RWMutex`. No goroutine owned by the hub itself.
- Unit tests: publish with no subs is a no-op; publish fans out to all subs for that calendar; subs for other calendars don't see it; unsubscribe removes cleanly; full channel drops without blocking.

### Phase 2 — Service wiring

- Add `Publisher` interface (single method `Publish(calendarID, eventType string)`) to `service.Service`. `Hub` satisfies it.
- Inject via `service.New(store Store, pub Publisher)`. Tests can pass a no-op publisher.
- After each successful mutation in `service/service.go`, call `s.pub.Publish(calendarID, "<type>")`. One-liner per method, defer-free so we only publish on success.

### Phase 3 — Handler

`internal/handler/events.go`:
- `func (h *Handler) StreamEvents(w, r)`.
- Verify calendar exists (404 if not) — one cheap `GetCalendarOnly`.
- Subscribe, then loop on:
  - `<-ctx.Done()` → return
  - `<-ch` → write `event:` + `data:` frames, then `w.(http.Flusher).Flush()`
  - `<-time.After(25s)` → write heartbeat comment + flush
- Always unsubscribe in defer.
- Set the response headers listed above before the first flush.

### Phase 4 — Routing & server wiring

- `server.New` constructs `realtime.NewHub()` and passes it to `service.New`.
- Route: `s.mux.HandleFunc("GET /calendars/{id}/events", s.optionalAuth(s.handler.StreamEvents))`.
- No auth requirement change — optional, like the GET.

### Phase 5 — Observability

- `hub` exposes `ActiveSubscribers() int` and `ActiveCalendars() int`.
- Add these to the existing `/admin/metrics` handler output.
- Structured log on stream open/close at `info`: `event=sse_open calendar_id=... duration_ms=...` on close. No PII (calendar ID is not PII).

## Frontend Implementation

### Phase 1 — Hook

New file `src/lib/useCalendarEvents.ts`:

```ts
useCalendarEvents(calendarId: string, participantId: string | null, onRefresh: () => void): {
  status: "connecting" | "open" | "polling" | "closed";
}
```

Behaviour:
- Opens `new EventSource(\`${API_BASE}/calendars/${id}/events?as=${pid ?? ""}\`, { withCredentials: true })`.
- On any message, calls `onRefresh()` through a 150ms trailing debouncer.
- On open, calls `onRefresh()` once to catch anything missed before the stream was live.
- Tracks consecutive `error` events; after 3, closes the stream and falls back to `setInterval(onRefresh, 15000)`. Status flips to `"polling"`.
- Cleanup on unmount: `es.close()`, clear interval, cancel debounce.

### Phase 2 — Wire into calendar page

In `CalendarPageClient`:
- Extract the existing reload logic into a stable `refresh = useCallback(async () => { ... }, [deps])`.
- `useCalendarEvents(calendarId, currentParticipant?.id ?? null, refresh)`.
- Suppress the refetch for ~500ms after a local mutation we just issued (availability save, rename, etc.) — prevents the user seeing their own change flicker away and come back.

### Phase 3 — UI feedback (minimal)

- Tiny dot indicator somewhere unobtrusive (next to the share button?) showing stream status. Live: no dot. Polling: small amber dot with tooltip "Updates every 15s". Closed: red dot with tooltip "Updates paused — refresh to retry". Keep it quiet — only surface the degraded states.
- No toast spam on events. The visible change on the page is the feedback.

### Phase 4 — Analytics

- `sse_connected` `{ calendar_id }` — on first open.
- `sse_fallback_polling` `{ calendar_id }` — once per mount if we fall back.
- `sse_event_received` `{ calendar_id, type }` — sampled at 10% to avoid flooding.

## Testing

### Backend
- `realtime` unit tests per above.
- Service tests: assert `Publish` is called with expected type after each mutation (pass a recording publisher).
- Handler test: open the stream against an `httptest.Server`, trigger a publish, assert the frame is received and well-formed. Assert heartbeat fires by advancing a fake clock / short interval in tests.
- Integration test: two SSE subscribers on the same calendar, publish once, both receive.

### Frontend
- Hook unit test with a mocked `EventSource`: open → message → debounced `onRefresh` call count = 1 even for 3 rapid messages.
- Hook test: 3 errors flip status to `"polling"` and start the interval.
- Component test for `CalendarPageClient`: mounting subscribes; unmounting unsubscribes; an event triggers a refetch.

### Manual
- Two browsers, one mobile one desktop, same calendar: mark a date on one, assert it appears on the other within ~1s.
- Owner flips visibility from Open → Submit to view while a second non-owner is on the page: second browser flips to hidden view.
- Kill backend mid-session: observe polling fallback kick in.
- Deploy with `min_instances=max_instances=1`; verify no dropped events under multi-client load.

## Build Order

1. Backend Phase 1 (hub + unit tests).
2. Backend Phase 2 (service wiring).
3. Backend Phase 3–4 (handler + route).
4. Backend Phase 5 (metrics).
5. Backend integration test.
6. Frontend Phase 1 (hook + unit tests).
7. Frontend Phase 2 (wire into page).
8. Frontend Phase 3 (status indicator).
9. Frontend Phase 4 (analytics).
10. Manual QA matrix above.
11. Cloud Run config: pin `min_instances=max_instances=1` as part of the rollout. Open follow-up ticket for LISTEN/NOTIFY-based fan-out.

## Forward Compatibility (WebSocket Swap)

SSE is the right choice now (see Design Decisions), but we should write the code so that swapping in WebSockets later costs about a day, not a rewrite. The rule is: **separate the transport from the event model and the orchestration.**

### Backend — keep transport-agnostic

These stay identical under a WS swap:

- `realtime.Hub` — pub/sub, doesn't know how events reach clients.
- `realtime.Event{Type, CalendarID}` — plain struct, JSON-serializable. Wire format is the transport's problem.
- Service-layer publish calls — `s.pub.Publish(calendarID, "availability.changed")`.

### Backend — isolate only the framing

Introduce an interface the handler loop writes to:

```go
type EventTransport interface {
    WriteEvent(e realtime.Event) error
    WriteHeartbeat() error
}
```

The SSE implementation wraps an `http.ResponseWriter` + `Flusher` and emits `event:` / `data:` frames. A future `wsTransport` wraps a WebSocket conn and emits JSON text frames. The handler body — `Subscribe`, loop on `ctx.Done()` / `<-ch` / heartbeat ticker, `Unsubscribe` in defer — is byte-identical between transports. Swap = one new file implementing `EventTransport`, plus a route change.

### Frontend — thin transport wrapper

The hook depends only on a minimal interface, not `EventSource` directly:

```ts
interface RealtimeStream {
  onEvent(fn: (e: Event) => void): void;
  onStatus(fn: (s: Status) => void): void;
  close(): void;
}

function createSseStream(url: string): RealtimeStream { /* EventSource inside */ }
// Later: createWsStream(url) — identical signature
```

`useCalendarEvents` keeps the debounce, status tracking, and polling fallback; it never touches `EventSource` APIs directly. Swap = one new ~40-line file plus a factory switch.

### What we deliberately don't abstract

- **No `send()` upstream method on the stream interface.** That's the one capability WS adds over SSE. Adding it speculatively is YAGNI — all client→server traffic is REST today, and will stay that way until a specific feature (live drag-select, typing indicators) actually needs push. When that day comes, add `send()` and `wsTransport` together.
- **No generic message-bus protocol.** The event payload is `{type, calendar_id}` — a human can hold it in their head. No versioning envelopes, ack semantics, or sequence numbers. If the schema needs to grow, grow it then.
- **No runtime transport selector.** One transport at a time. Picking via config doubles the test matrix for a swap that will happen once, if ever.

### The migration checklist, for the record

If we ever do swap:

1. Add `wsTransport` (backend) — implements `EventTransport` over a WebSocket upgrade handler.
2. Add `createWsStream` (frontend) — implements `RealtimeStream` over `WebSocket` with reconnect/backoff (browser doesn't give this free for WS).
3. Flip the route handler + frontend factory. Delete the SSE files.
4. Revisit Cloud Run config: WS needs session affinity once we're multi-instance.

Everything else — hub, event types, service publish calls, the hook's public API, analytics event names, tests against the event model — is untouched.

## Open Questions

- **Resume semantics on reconnect.** Browser `EventSource` reconnects with `Last-Event-ID` if we set `id:` on frames. We're not using it in v1 (refresh-on-reconnect is enough). Revisit if we ever move to delta payloads.
- **Rate-limiting connection opens.** Not a concern at current scale. Revisit if we see clients reconnect-looping against a broken path.

## Log

- 2026-04-20 17:00 — Created plan.
- 2026-04-20 17:30 — Added Forward Compatibility section: `EventTransport` interface on backend, `RealtimeStream` interface on frontend, explicit list of what not to abstract, migration checklist.
- 2026-04-20 18:45 — Implemented all phases end-to-end. Backend: `internal/realtime` hub with bounded per-subscriber channels and active counts; `service.Publisher` injected via `service.New(store, pub)` with publish calls after every mutation; `handler.EventsHandler` with SSE transport behind the `EventTransport` interface; `GET /calendars/{id}/events` routed under `optionalAuth`; realtime subscriber/calendar counts surfaced in `/admin/metrics`; structured `sse_open`/`sse_close` logs. Frontend: `RealtimeStream` transport interface in `src/lib/realtimeStream.ts`; `useCalendarEvents` hook with 150ms trailing debounce, 3-error → 15s polling fallback, analytics (`sse_connected`, `sse_fallback_polling`, `sse_event_received` sampled at 10%); `LiveStatusDot` component (amber when polling, red when closed, hidden otherwise) placed in the calendar header; `CalendarPageClient` wired with a lightweight `refresh` callback plus 500ms `markLocalMutation` window to suppress echo flicker. Unit tests: `realtime_test.go` (hub behaviour), service tests (recording publisher), `events_test.go` (SSE 404, frame delivery, multi-subscriber fan-out), `useCalendarEvents.test.ts` (debounce, status transitions, polling fallback, participantId propagation). All backend tests pass; frontend tests pass except for 4 pre-existing failures unrelated to this work. Cloud Run `min/max_instances=1` and LISTEN/NOTIFY follow-up remain deployment-time tasks per the plan.
- 2026-11-23 20:52 — Superseded: stream connection opens are now rate-limited per IP (`CalendarLookupLimiter.Streams` in `backend/internal/handler/calendar_lookup_limiter.go`).
