# Frontend API Client

Status: done

## Goal

Create a typed API client in `src/lib/` that the frontend uses to talk to the backend. Shared TypeScript types that match the backend's JSON contract exactly.

## Context

Backend serves four endpoints on `:8080` with JSON request/response bodies. Frontend is a clean Next.js project with no existing types, HTTP client, or utilities beyond the defaults.

## Plan

### 1. Types — `src/lib/types.ts`

Define interfaces using camelCase (standard TypeScript convention):

- `Calendar` — `id`, `name`, `createdAt`
- `Participant` — `id`, `calendarId`, `name`, `colour`
- `Availability` — `participantId`, `calendarId`, `date`
- `CalendarWithDetails` — `calendar`, `participants`, `availability`
- Request types: `CreateCalendarRequest`, `AddParticipantRequest`, `SetAvailabilityRequest`
- Error shape: `ApiError` — `error: string`

### 2. API client — `src/lib/api.ts`

Thin wrapper around `fetch`. No external dependencies.

- Base URL from `NEXT_PUBLIC_API_URL` env var (defaults to `http://localhost:8080`)
- Helper that handles JSON serialisation, error responses, content-type headers, and snake_case ↔ camelCase conversion at the boundary
- One function per endpoint:
  - `createCalendar(req: CreateCalendarRequest): Promise<Calendar>`
  - `getCalendar(id: string): Promise<CalendarWithDetails>`
  - `addParticipant(calendarId: string, req: AddParticipantRequest): Promise<Participant>`
  - `setAvailability(calendarId: string, req: SetAvailabilityRequest): Promise<void>`
- Custom `ApiError` class thrown on non-2xx responses, carrying the status code and error message from the backend

### 3. Tests — `src/lib/api.test.ts`

Unit tests using Vitest. Mock `fetch` with `vi.fn()` to verify:

- Correct URL, method, headers, and body sent for each function
- Successful responses deserialised into correct types
- Non-2xx responses throw `ApiError` with status and message
- Base URL configuration works

### 4. Env config

- Add `.env.local.example` with `NEXT_PUBLIC_API_URL=http://localhost:8080`

## Decisions

- **camelCase types**: Frontend types use camelCase (TypeScript convention). The API client converts snake_case ↔ camelCase at the boundary so the rest of the frontend never sees snake_case.
- **No axios**: `fetch` is built in and sufficient for four endpoints.
- **No React hooks yet**: The client is a plain async module. Hooks for data fetching come later when building the UI pages.

## Log

- 2026-04-16 — Created plan
- 2026-04-16 23:56 — Implemented all four parts: types.ts, api.ts, api.test.ts, .env.local.example. 10 tests passing, 0 lint warnings.
