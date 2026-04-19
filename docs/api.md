# API Reference

Base URL: `http://localhost:8080` (development)

All endpoints return JSON with `Content-Type: application/json`. Every response includes an `X-Request-ID` header. CORS is enabled with credentials support.

Error responses use the shape: `{"error": "message"}`

> The backend uses snake_case for JSON keys. The frontend API client converts to/from camelCase.

## Authentication

Session-based auth with HTTP-only cookies. Creator must sign in to create a calendar. Joiners stay anonymous.

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/auth/google` | — | Redirect to Google OAuth. Accepts `?return_url=` |
| GET | `/auth/google/callback` | — | Google OAuth callback, creates session, redirects |
| GET | `/auth/me` | Session | Return current user or 401 |
| POST | `/auth/logout` | Session | Delete session, clear cookie |

## Calendars

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/calendars` | Required | Create a calendar (sets owner_id from session) |
| GET | `/calendars/{id}` | — | Get calendar with participants and availability |
| POST | `/calendars/{id}/participants` | — | Join a calendar |
| PUT | `/calendars/{id}/availability` | — | Update availability |

## Health

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/health` | — | Health check (200 ok / 503 unhealthy) |

---

### POST /calendars

Create a new calendar. Requires authentication.

**Request**

```json
{"name": "string"}
```

**Response 201**

```json
{"id": "string", "name": "string", "owner_id": "string", "created_at": "RFC3339"}
```

**Errors:** 400 (invalid JSON) | 401 (not authenticated)

---

### GET /calendars/{id}

Get a calendar with all participants and their availability.

**Response 200**

```json
{
  "calendar": {
    "id": "string",
    "name": "string",
    "owner_id": "string",
    "created_at": "RFC3339"
  },
  "participants": [
    {
      "id": "string",
      "calendar_id": "string",
      "name": "string",
      "colour": "string"
    }
  ],
  "availability": [
    {
      "participant_id": "string",
      "calendar_id": "string",
      "date": "YYYY-MM-DD"
    }
  ]
}
```

**Errors:** 404 (calendar not found)

---

### POST /calendars/{id}/participants

Add a participant to a calendar.

**Request**

```json
{"name": "string"}
```

**Response 201**

```json
{
  "id": "string",
  "calendar_id": "string",
  "name": "string",
  "colour": "string"
}
```

**Errors:** 400 (name required, invalid JSON) | 404 (calendar not found) | 409 (name taken)

---

### PUT /calendars/{id}/availability

Replace a participant's available dates. Sending an empty `dates` array clears availability.

**Request**

```json
{"participant_id": "string", "dates": ["YYYY-MM-DD"]}
```

**Response 200**

```json
{"ok": true}
```

**Errors:** 400 (participant_id required, invalid date) | 404 (participant not found)

---

### GET /auth/google

Redirects to Google OAuth consent screen. Pass `?return_url=/path` to return the user to a specific page after sign-in.

---

### GET /auth/google/callback

Handles the OAuth callback. Creates or finds the user, creates a session, sets an HTTP-only cookie, and redirects to the return URL encoded in the OAuth state.

---

### GET /auth/me

Returns the current authenticated user.

**Response 200**

```json
{"id": "string", "email": "string", "name": "string"}
```

**Errors:** 401 (no valid session)

---

### POST /auth/logout

Deletes the session and clears the session cookie.

**Response 200**

```json
{"ok": true}
```

---

### GET /health

Returns service health status.

**Response 200**

```json
{"status": "ok"}
```

**Response 503**

```json
{"status": "unhealthy"}
```
