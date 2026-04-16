# Store Layer

Status: done

## Goal

Build the Postgres query layer in `backend/internal/store/`. These are the data-access functions the API handlers will call. Each function is a method on the existing `Store` struct.

## Existing code

- `store/store.go` — `Store` struct wrapping `*sql.DB`, constructor `New(db)`
- `model/model.go` — `Calendar`, `Participant`, `Availability`, `CalendarWithDetails`
- `store/migrations/001_create_tables.sql` — schema already applied

## Functions to implement

### `CreateCalendar(ctx, name) (Calendar, error)`

- Generate a short random slug (8 chars, alphanumeric lowercase) for the ID
- Insert into `calendars` table
- Return the created row (including server-set `created_at`)

### `GetCalendar(ctx, id) (CalendarWithDetails, error)`

- Fetch calendar row by ID — return a clear error if not found
- Fetch all participants for that calendar
- Fetch all availability rows for that calendar
- Assemble into `CalendarWithDetails`
- Use separate queries, not a big join — simpler to map and avoids null-handling complexity

### `AddParticipant(ctx, calendarID, name) (Participant, error)`

- Generate a short random ID for the participant
- Auto-assign a colour from a preset palette based on participant count in the calendar
- Insert into `participants` table
- Handle unique constraint violation (duplicate name) with a specific error

### `SetAvailability(ctx, participantID, dates) error`

- Takes a slice of date strings representing the full set of dates this participant is free
- Look up the participant's `calendar_id` from the `participants` table (avoids callers passing a mismatched value)
- Run in a transaction:
  1. Delete all existing availability for this participant
  2. Bulk insert the new dates (using the looked-up `calendar_id`)
- This "replace all" approach is simpler than diffing adds/removes

## Helpers

### ID generation

- `generateID(n int) string` — crypto/rand, lowercase alphanumeric, unexported
- Used by `CreateCalendar` (8 chars) and `AddParticipant` (8 chars)
- Callers retry on primary key conflict (up to 3 attempts) to handle the astronomically unlikely collision

### Colour palette

- Fixed slice of 10-12 distinct colours (hex codes)
- `AddParticipant` picks `palette[count % len(palette)]` where count is current participant count
- Note: concurrent `AddParticipant` calls could read the same count and assign the same colour — acceptable for V1 since it's purely cosmetic

### Errors

- `ErrNotFound` — sentinel error for missing calendar
- `ErrDuplicateName` — sentinel error for duplicate participant name per calendar

## File layout

- `store/calendar.go` — `CreateCalendar`, `GetCalendar`
- `store/participant.go` — `AddParticipant`
- `store/availability.go` — `SetAvailability`
- `store/id.go` — `generateID`
- `store/errors.go` — sentinel errors
- `store/store_test.go` — shared test helper (`setupTestStore`)
- `store/calendar_test.go` — calendar integration tests
- `store/participant_test.go` — participant integration tests
- `store/availability_test.go` — availability integration tests
- `store/id_test.go` — ID generation unit tests

## Tests

Integration tests hitting a real Postgres database with build tag `integration`. One test file per store file, all in `store/`.

### Test setup

- `store/store_test.go` — shared test helper
- `setupTestStore(t)` connects to DB using `TEST_DATABASE_URL`, runs migrations, returns `*Store`
- Each test cleans up after itself (delete from tables) or uses unique IDs to avoid collisions

### `store/calendar_test.go`

**TestCreateCalendar**
```
store := setupTestStore(t)
cal, err := store.CreateCalendar(ctx, "Weekend Trip")
assert no error
assert cal.ID is 8 chars, alphanumeric
assert cal.Name == "Weekend Trip"
assert cal.CreatedAt is recent (within last 5 seconds)
```

**TestCreateCalendarDefaultName**
```
store := setupTestStore(t)
cal, err := store.CreateCalendar(ctx, "")
assert no error
assert cal.Name == ""  // DB DEFAULT '' kicks in for empty input
```

**TestGetCalendar**
```
store := setupTestStore(t)
cal := create a calendar
add participant "Alice"
add participant "Bob"
set availability for Alice on ["2026-05-01", "2026-05-02"]
set availability for Bob on ["2026-05-02", "2026-05-03"]

result, err := store.GetCalendar(ctx, cal.ID)
assert no error
assert result.Calendar.ID == cal.ID
assert len(result.Participants) == 2
assert len(result.Availability) == 4
```

**TestGetCalendarNotFound**
```
store := setupTestStore(t)
_, err := store.GetCalendar(ctx, "nonexistent")
assert err == ErrNotFound
```

**TestGetCalendarEmpty**
```
store := setupTestStore(t)
cal := create a calendar
result, err := store.GetCalendar(ctx, cal.ID)
assert no error
assert len(result.Participants) == 0
assert len(result.Availability) == 0
```

### `store/participant_test.go`

**TestAddParticipant**
```
store := setupTestStore(t)
cal := create a calendar
p, err := store.AddParticipant(ctx, cal.ID, "Alice")
assert no error
assert p.ID is 8 chars
assert p.CalendarID == cal.ID
assert p.Name == "Alice"
assert p.Colour is a valid hex colour
```

**TestAddParticipantColourAssignment**
```
store := setupTestStore(t)
cal := create a calendar
p1 := add participant "Alice"
p2 := add participant "Bob"
p3 := add participant "Charlie"
assert p1.Colour != p2.Colour
assert p2.Colour != p3.Colour
```

**TestAddParticipantDuplicateName**
```
store := setupTestStore(t)
cal := create a calendar
add participant "Alice"
_, err := store.AddParticipant(ctx, cal.ID, "Alice")
assert err == ErrDuplicateName
```

**TestAddParticipantSameNameDifferentCalendar**
```
store := setupTestStore(t)
cal1 := create a calendar
cal2 := create a calendar
_, err1 := store.AddParticipant(ctx, cal1.ID, "Alice")
_, err2 := store.AddParticipant(ctx, cal2.ID, "Alice")
assert no error on both — same name is fine across different calendars
```

### `store/availability_test.go`

**TestSetAvailability**
```
store := setupTestStore(t)
cal := create a calendar
p := add participant "Alice"
err := store.SetAvailability(ctx, p.ID, ["2026-05-01", "2026-05-02"])
assert no error

result := get calendar
assert len(result.Availability) == 2
assert dates match ["2026-05-01", "2026-05-02"]
```

**TestSetAvailabilityReplacesExisting**
```
store := setupTestStore(t)
cal := create a calendar
p := add participant "Alice"
set availability ["2026-05-01", "2026-05-02"]
set availability ["2026-05-03"]  // replaces, not appends

result := get calendar
assert len(result.Availability) == 1
assert date == "2026-05-03"
```

**TestSetAvailabilityClearAll**
```
store := setupTestStore(t)
cal := create a calendar
p := add participant "Alice"
set availability ["2026-05-01", "2026-05-02"]
set availability []  // empty slice clears all

result := get calendar
assert len(result.Availability) == 0
```

**TestSetAvailabilityMultipleParticipants**
```
store := setupTestStore(t)
cal := create a calendar
alice := add participant "Alice"
bob := add participant "Bob"
set availability for alice ["2026-05-01", "2026-05-02"]
set availability for bob ["2026-05-02", "2026-05-03"]

result := get calendar
assert len(result.Availability) == 4
// setting bob's availability didn't affect alice's
alice_dates := filter availability by alice's ID
assert len(alice_dates) == 2
```

### `store/id_test.go`

**TestGenerateID**
```
id := generateID(8)
assert len(id) == 8
assert all chars are in [a-z0-9]
```

**TestGenerateIDUniqueness**
```
ids := set{}
for i := 0; i < 1000; i++ {
    ids.add(generateID(8))
}
assert len(ids) == 1000  // no collisions
```

## Log

- 2026-04-16 — Created plan
- 2026-04-16 — Plan review: fixed migration path, removed redundant calendarID param from SetAvailability (look up from participant instead), added ID collision retry, noted colour race condition as acceptable for V1, renamed empty name test
- 2026-04-16 — Implemented all store functions, helpers, and tests. Build passes, unit tests (id_test.go) pass. Integration tests require TEST_DATABASE_URL to run.
