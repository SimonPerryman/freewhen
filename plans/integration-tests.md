# Integration Tests

Status: done

## Goal

End-to-end tests that exercise the full request path — HTTP handler → service → store → Postgres — against a real database. Covers the core flows a user will drive from the frontend and locks in behaviour before we start building UI on top of it.

These tests are the safety net for the remaining V1 work: once the API is green end-to-end, we can refactor internals or change SQL with confidence.

## Scope

- Hit the API via `httptest` using the real `server.Server` (routing, CORS middleware, handlers, service, store all in play).
- Use the same Postgres instance and migration path as the store tests (`TEST_DATABASE_URL` + `Migrate`).
- Gated behind the `integration` build tag — same pattern as `store/*_test.go`.
- Out of scope: frontend, load/performance, auth (none exists).

## File layout

- `backend/internal/server/server_integration_test.go` — all integration tests live here for V1. One file is fine; split later if it grows.

Use `package server_test` — everything we need (`New`, `Server`, `ServeHTTP`) is already exported, so no reason to sit inside the package.

## Request shapes (reference)

Since this isn't obvious from the routes alone:

- `POST /calendars` — body `{"name": "..."}`
- `POST /calendars/{id}/participants` — body `{"name": "..."}`
- `PUT /calendars/{id}/availability` — body `{"participant_id": "...", "dates": ["YYYY-MM-DD", ...]}`. Participant is identified in the body, not the path.
- `GET /calendars/{id}` — returns `{calendar, participants, availability}` with availability as a flat list of `{participant_id, calendar_id, date}` rows.

## Test harness

Private helpers in the same file:

- `setupTestServer(t *testing.T) *httptest.Server` — opens the DB from `TEST_DATABASE_URL` (`t.Fatal` if unset, matching the store tests), runs `store.Migrate`, clears `availability`, `participants`, `calendars` (in that order) via `DELETE FROM` — matching `store/store_test.go`, constructs `server.New(db)`, wraps in `httptest.NewServer`, registers `t.Cleanup` for both server and DB. A fresh connection is opened per test — same pattern as the store tests. Revisit if the suite gets slow. DB isn't returned — every assertion goes through the API.
- `doJSON(t *testing.T, ts *httptest.Server, method, path string, body any) (*http.Response, []byte)` — marshals body, builds request, reads full response body, returns both. Keeps each test terse.
- `decodeJSON(t *testing.T, data []byte, v any)` — small wrapper that fails the test on unmarshal error.

## Flows to cover

### Golden path (one test, walks the full lifecycle)

`TestFullFlow`:
1. `POST /calendars` with `{"name":"Weekend Trip"}` → expect 201, capture calendar ID.
2. `POST /calendars/{id}/participants` with `{"name":"Alice"}` → expect 201, capture Alice's participant ID.
3. `POST /calendars/{id}/participants` with `{"name":"Bob"}` → expect 201, capture Bob's participant ID.
4. `PUT /calendars/{id}/availability` with `{"participant_id": "<alice>", "dates": ["2026-05-01","2026-05-02","2026-05-03"]}` → expect 200.
5. `PUT /calendars/{id}/availability` with `{"participant_id": "<bob>", "dates": ["2026-05-02","2026-05-03","2026-05-04"]}` → expect 200.
6. `GET /calendars/{id}` → expect 200. Assert both participants are present by name, and assert each participant's set of dates in the flat `availability` list matches exactly what was PUT (Alice → {05-01, 05-02, 05-03}, Bob → {05-02, 05-03, 05-04}). No overlap computation — that's a frontend concern; testing it here just asserts the test's own math.

### Per-endpoint edge cases

Kept small — most validation is already covered by unit tests. We only re-check the cases the handler layer owns.

`TestCreateCalendar_BadJSON` — `POST /calendars` with `"not json"` → 400.

`TestGetCalendar_NotFound` — `GET /calendars/doesnotexist` → 404.

`TestAddParticipant_CalendarNotFound` — `POST /calendars/doesnotexist/participants` with valid body → 404.

`TestAddParticipant_EmptyName` — name `""` → 400.

`TestAddParticipant_DuplicateName` — add "Alice" twice in same calendar → second call returns 409.

`TestSetAvailability_ParticipantWrongCalendar` — create calendars A and B, create participant on A, PUT availability on B with A's participant ID → 404 (this is the cross-check the service layer added).

`TestSetAvailability_InvalidDate` — dates `["not-a-date"]` → 400.

`TestSetAvailability_EmptyParticipantID` → 400.

`TestCORS_Preflight` — `OPTIONS /calendars` → 204, `Access-Control-Allow-*` headers present. The middleware short-circuits OPTIONS and sets headers unconditionally (no Origin check), so don't condition the test on an Origin header being sent — that would imply behavior the middleware doesn't have.

Clearing availability (empty `dates` array) is worth one assertion: set some dates, then PUT with `[]`, then GET and confirm the participant has no availability rows. Add as a sub-test of `TestFullFlow` or a standalone test — either works, will decide during implementation.

## Isolation between tests

Each test calls `setupTestServer`, which clears the three tables (`DELETE FROM`, children first) before returning. Simple, slow-ish, but fine for V1 (< 20 tests). No transactions-per-test, no parallel runs — just the same pattern the store tests already use.

## Makefile

`make test-integration` already runs `go test -tags=integration ./...`, so the new file will be picked up automatically. No Makefile change needed.

## Log

- 2026-04-16 — Created plan
- 2026-04-16 — Resolved setup questions: `t.Fatal` when `TEST_DATABASE_URL` unset (matches store tests); fresh DB connection per test (revisit if slow)
- 2026-04-16 — Plan review fixes: spelled out request shapes (availability needs `participant_id` in body); switched to `package server_test`; dropped `*sql.DB` from `setupTestServer` return; corrected CORS preflight expectation (no Origin-header check in middleware); replaced client-side overlap assertion with direct per-participant date assertions; aligned wording with `DELETE FROM` (not TRUNCATE).
- 2026-04-17 — Implemented `backend/internal/server/server_integration_test.go`: helpers (`setupTestServer`, `doJSON`, `decodeJSON`), `TestFullFlow` with `ClearAvailability` sub-test, and all listed edge-case tests. `go build -tags=integration ./...` and `go vet -tags=integration ./...` clean. Tests skip locally because `TEST_DATABASE_URL` is unset (same behaviour as store tests) — actual run requires a live test DB.
