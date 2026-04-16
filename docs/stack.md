# Tech Stack

## Architecture

Go backend + React (Next.js) frontend + Postgres. Two deployable pieces with clean separation.

## Backend - Go

API server handling all business logic and data access.

- Standard library `net/http` where possible, minimal dependencies
- Single binary, cheap to run, easy to deploy
- Connects to Postgres directly

## Frontend - Next.js (React)

The calendar UI. Chosen because we know React and the calendar interactions (colour segments, toggling days, visual overlap) will need fast iteration.

## Database - Postgres

Hosted anywhere - Supabase, Neon, Railway, or a VPS. Decision deferred until deployment. All are just Postgres underneath, so portable.

### Schema (rough)

- `calendars` - id, name, created_at
- `participants` - id, calendar_id, name, colour
- `availability` - participant_id, calendar_id, date

## API - REST

Simple RESTful API between frontend and backend:

- `POST /calendars` - create a calendar
- `GET /calendars/:id` - get calendar with participants and availability
- `POST /calendars/:id/participants` - join a calendar
- `PUT /calendars/:id/availability` - update your free days

## Hosting

TBD. Decide at deploy time. Options include Vercel (frontend) + Fly.io/Railway (backend), or similar.

## Future Considerations

- WebSockets for real-time updates (V2) - added alongside REST, not replacing it
- Auth can be added in Go or via Supabase's built-in auth if we're on Supabase by then
