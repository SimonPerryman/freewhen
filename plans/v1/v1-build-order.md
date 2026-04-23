# V1 Build Order

Status: done

## Goal

Define the sequence for building all V1 features, from database to polished frontend.

## Build Order

### 1. Database migrations ✓
Create the `calendars`, `participants`, and `availability` tables. This is the foundation everything else sits on.

### 2. Store layer
Postgres queries for all operations: create calendar, get calendar with details, add participant, set/clear availability. Unit-testable against a real DB.

### 3. REST API handlers + routing
Wire up the four endpoints:
- `POST /calendars`
- `GET /calendars/:id`
- `POST /calendars/:id/participants`
- `PUT /calendars/:id/availability`

Add CORS middleware for frontend dev.

### 4. Integration tests
End-to-end tests hitting the API with a real database. Cover the core flows: create calendar, join, mark availability, read back overlap.

### 5. Frontend API client
Typed client in `src/lib/` that talks to the backend. Shared types for Calendar, Participant, Availability.

### 6. Calendar creation page
Simple form: optional event name, submit, get redirected to the new calendar URL. Copy-link button.

### 7. Calendar view — join + mark availability
Open a calendar link, enter your name (stored in localStorage), click days to toggle free/busy. Saves to backend on change.

### 8. Visual overlap
Colour assignment per participant. Each day split into segments showing who's free. Days where everyone is free highlighted.

### 9. Summary view
Sidebar or section listing "days everyone is free" and "days most people are free" with counts.

### 10. Polish
- Returning user recognition (localStorage identity)
- Mobile-friendly layout
- Loading/error states
- Copy-link sharing button

## Log

- 2026-04-16 — Created build order plan
- 2026-04-19 — All V1 steps complete. Marked done.
