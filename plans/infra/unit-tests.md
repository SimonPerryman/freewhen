# Unit Tests

Status: done

## Goal

Add unit tests to the backend handler/service layers and the frontend components that have untested logic. These complement the existing integration tests by running fast (no database), pinpointing failures to a specific layer, and making edge cases cheap to cover.

## Plan

### 1. Introduce a service interface for handler tests

The handlers currently depend on a concrete `*service.Service`. To unit test handlers without a database, introduce an interface in the handler package that `*service.Service` already satisfies:

```go
// handler/handler.go
type CalendarService interface {
    CreateCalendar(ctx context.Context, name string) (model.Calendar, error)
    GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error)
    AddParticipant(ctx context.Context, calendarID, name string) (model.Participant, error)
    SetAvailability(ctx context.Context, calendarID, participantID string, dates []string) error
}
```

Change `Handler` to hold this interface instead of `*service.Service`. No other code changes needed — `service.Service` already implements it.

### 2. Handler unit tests (`handler/*_test.go`)

Create `handler/handler_test.go` with a mock service and test helpers, then test each handler:

**calendar.go:**
- `CreateCalendar` — valid request returns 201 + JSON body
- `CreateCalendar` — malformed JSON returns 400
- `CreateCalendar` — service error returns 500

**participant.go:**
- `AddParticipant` — valid request returns 201
- `AddParticipant` — empty name returns 400
- `AddParticipant` — malformed JSON returns 400
- `AddParticipant` — calendar not found returns 404
- `AddParticipant` — duplicate name returns 409
- `AddParticipant` — service error returns 500

**availability.go:**
- `SetAvailability` — valid request returns 200
- `SetAvailability` — empty participant_id returns 400
- `SetAvailability` — malformed JSON returns 400
- `SetAvailability` — invalid date format returns 400
- `SetAvailability` — participant not found returns 404
- `SetAvailability` — service error returns 500

### 3. Service unit tests (`service/service_test.go`)

Introduce a store interface (or use the existing store methods) with a mock. Focus on `SetAvailability` since the other methods are direct passthrough:

- `SetAvailability` — participant belongs to calendar, calls store
- `SetAvailability` — participant not found, returns error
- `SetAvailability` — participant belongs to different calendar, returns `ErrNotFound`

### 4. Frontend: `BestDay.test.tsx`

Test the rendering logic:

- Renders nothing when fewer than 2 participants
- Renders nothing when multiple dates have everyone free (no single best)
- Renders "Everyone's free" when exactly one date has all participants
- Renders "X of Y free" when no universal date but one clear winner
- Renders nothing when top two dates are tied

### 5. Frontend: `CalendarPageClient.test.tsx`

Test the key logic paths (mock `fetch` and `localStorage`):

- Loads calendar and renders heading
- Shows join form when no stored participant
- Recognises stored participant from localStorage and skips join form
- Join flow: submitting name calls API, stores to localStorage, shows grid
- Join flow: duplicate name shows "already taken" error

## Log

- 2026-04-18 — Created plan
- 2026-04-18 23:28 — Implemented all 5 phases. Introduced CalendarService interface in handler package, Store interface in service package, added 18 handler tests, 4 service tests, 5 BestDay tests, 5 CalendarPageClient tests. All 81 tests pass, zero lint warnings.
