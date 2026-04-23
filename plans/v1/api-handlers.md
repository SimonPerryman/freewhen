# REST API Handlers + Routing

Status: done

## Goal

Wire up the four REST endpoints that the frontend will call. Introduce a service layer between handler and store — pass-through for now, but establishes the right place for business logic from day one. Add CORS middleware so the frontend dev server can talk to the backend.

## Existing code

- `server/server.go` — `Server` struct with empty `routes()` method, `ServeHTTP` delegates to `mux`
- `handler/handler.go` — `Handler` struct wrapping `*store.Store`, constructor only (will change to wrap service)
- `model/model.go` — all types have JSON tags already
- Store methods: `CreateCalendar`, `GetCalendar`, `AddParticipant`, `SetAvailability`
- Store errors: `ErrNotFound`, `ErrDuplicateName`

## Service layer

New package `internal/service/` sits between handler and store. The request flow becomes:

```
Handler → Service → Store
```

- **Handler** owns HTTP concerns: decode request, validate shape, call service, encode response, map errors to status codes
- **Service** owns business logic: for now just pass-through calls to the store, but this is where rules like "max 20 participants" or "dates must be in the future" will live
- **Store** owns data access: SQL queries, transactions, row mapping

### `service/service.go`

```go
type Service struct {
    store *store.Store
}

func New(s *store.Store) *Service {
    return &Service{store: s}
}
```

### New store method

Add `GetParticipant(ctx, id) (Participant, error)` to `internal/store/participant.go`. Simple single-row query on the `participants` table, returns `ErrNotFound` when no row matches.

### Service methods (all pass-through for now, except SetAvailability)

- `CreateCalendar(ctx, name) (Calendar, error)` → calls `store.CreateCalendar`
- `GetCalendar(ctx, id) (CalendarWithDetails, error)` → calls `store.GetCalendar`
- `AddParticipant(ctx, calendarID, name) (Participant, error)` → calls `store.AddParticipant`
- `SetAvailability(ctx, calendarID, participantID, dates) error` → calls `store.GetParticipant(ctx, participantID)`, returns `ErrNotFound` if the participant doesn't exist or its `CalendarID` doesn't match the given `calendarID`, then calls `store.SetAvailability(ctx, participantID, dates)`

### Wiring change

- `server.New(db)` creates store, then service, then handler
- `handler.New` takes `*service.Service` instead of `*store.Store`
- Handler methods call `h.service.X()` instead of `h.store.X()`

## Endpoints

### `POST /calendars`

**Request body:**
```json
{ "name": "Weekend Trip" }
```
Name is optional — omit or empty string for unnamed calendars.

**Handler logic:**
1. Decode JSON body into `struct { Name string }`
2. Call `service.CreateCalendar(ctx, name)`
3. Return 201 with the created calendar as JSON

**Errors:**
- 400 if body isn't valid JSON
- 500 on store error

---

### `GET /calendars/{id}`

**Handler logic:**
1. Extract calendar ID from URL path
2. Call `service.GetCalendar(ctx, id)`
3. Return 200 with `CalendarWithDetails` as JSON

**Errors:**
- 404 if `ErrNotFound`
- 500 on other store error

---

### `POST /calendars/{id}/participants`

**Request body:**
```json
{ "name": "Alice" }
```

**Handler logic:**
1. Extract calendar ID from URL path
2. Decode JSON body into `struct { Name string }`
3. Validate name is non-empty
4. Call `service.AddParticipant(ctx, calendarID, name)`
5. Return 201 with the created participant as JSON

**Errors:**
- 400 if name is empty or body isn't valid JSON
- 404 if calendar doesn't exist (`ErrNotFound`)
- 409 if `ErrDuplicateName`
- 500 on other store error

---

### `PUT /calendars/{id}/availability`

**Request body:**
```json
{ "participant_id": "abc12345", "dates": ["2026-05-01", "2026-05-02"] }
```

**Handler logic:**
1. Extract calendar ID from URL path
2. Decode JSON body into `struct { ParticipantID string; Dates []string }`
3. Validate participant_id is non-empty
4. Parse each date string as `YYYY-MM-DD` — reject if any are invalid
5. Call `service.SetAvailability(ctx, calendarID, participantID, dates)`
6. Return 200 with `{ "ok": true }`

**Errors:**
- 400 if participant_id is empty, dates contain invalid format, or body isn't valid JSON
- 404 if participant doesn't exist or doesn't belong to the calendar in the URL
- 500 on other store error

---

## Request body size limit

All endpoints that read a request body must wrap `r.Body` with `http.MaxBytesReader(w, r.Body, 1<<20)` (1 MB). This prevents abuse from oversized payloads. Implement as the first line in each handler that decodes JSON, or as middleware — handler-level is simpler for now since only three endpoints read a body.

## Date validation

The availability endpoint receives dates as `[]string`. Before passing to the service layer, the handler must parse each string with `time.Parse("2006-01-02", date)`. Return 400 with a clear message if any date fails to parse. This keeps the store layer from inserting garbage into the database.

## CORS Middleware

Wrap the mux in `Server.ServeHTTP` with CORS headers:
- `Access-Control-Allow-Origin: *` (good enough for V1)
- `Access-Control-Allow-Methods: GET, POST, PUT, OPTIONS`
- `Access-Control-Allow-Headers: Content-Type`
- Handle `OPTIONS` preflight requests by returning 204 immediately

Implement as a private method `cors(next http.Handler) http.Handler` in a new file `server/middleware.go`.

## Routing

Register in `Server.routes()` using Go 1.22+ pattern syntax:
```go
s.mux.HandleFunc("POST /calendars", s.handler.CreateCalendar)
s.mux.HandleFunc("GET /calendars/{id}", s.handler.GetCalendar)
s.mux.HandleFunc("POST /calendars/{id}/participants", s.handler.AddParticipant)
s.mux.HandleFunc("PUT /calendars/{id}/availability", s.handler.SetAvailability)
```

Extract path params with `r.PathValue("id")`.

## Shared helpers

Add a small `handler/respond.go` file with:
- `respondJSON(w, status, data)` — sets Content-Type, writes status code, encodes JSON
- `respondError(w, status, message)` — calls respondJSON with `{ "error": message }`

Keep these unexported — they're internal to the handler package.

## File layout

- `internal/store/participant.go` — add `GetParticipant` method
- `internal/service/service.go` — `Service` struct, constructor, all four methods
- `internal/handler/handler.go` — update to wrap `*service.Service` instead of `*store.Store`
- `internal/handler/calendar.go` — `CreateCalendar`, `GetCalendar`
- `internal/handler/participant.go` — `AddParticipant`
- `internal/handler/availability.go` — `SetAvailability`
- `internal/handler/respond.go` — `respondJSON`, `respondError`
- `internal/server/middleware.go` — `cors`
- `internal/server/server.go` — update `routes()`, `ServeHTTP`, and wiring (store → service → handler)

## Log

- 2026-04-16 — Created plan
- 2026-04-16 — Added service layer between handler and store (pass-through for now, establishes where business logic lives)
- 2026-04-16 — Added: participant-calendar cross-check (service rejects with 404 if participant doesn't belong to URL calendar), date format validation, request body size limit, 404 for non-existent calendar on add-participant
- 2026-04-16 — Clarified cross-check mechanism: add GetParticipant store method, service calls it to verify participant belongs to URL calendar before calling SetAvailability
- 2026-04-16 — Implemented all files: GetParticipant store method, service layer, handler respond helpers, calendar/participant/availability handlers, CORS middleware, routing and server wiring. Build, vet, and tests pass.
