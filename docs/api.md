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

## Participant authorization

Participant ids are public (they appear in `GET /calendars/{id}`), so they are
never proof of identity. Every write that targets a participant row —
`PUT /availability`, `POST /availability/import`,
`PATCH /participants/{participantId}` (name / email) and
`POST /participants/{participantId}/claim` — is authorized by the service
layer as follows:

- **Linked participant** (`user_id` set): the session user must be that user.
- **Anonymous participant**: the request must carry the participant's secret in
  the `X-Participant-Token` request header. The token is issued once, in the
  `X-Participant-Token` **response** header of the anonymous
  `POST /calendars/{id}/participants` call. Only its SHA-256 hash is stored.
- **Legacy anonymous participant** (joined before tokens existed): the first
  successful write is accepted by id alone (trust-on-first-use); the server
  mints a token, binds it to the row, and returns it in the
  `X-Participant-Token` response header. Every later write needs it.
- **Calendar owner**: may rename / edit the email of any participant (and
  remove participants). The owner can **not** write another participant's
  availability.

A rejected write returns **403** `{"code": "forbidden"}`. The `participant_id`
body field on `PATCH /participants/{participantId}` is ignored.

CORS allows the `X-Participant-Token` request header and exposes it (and
`X-Request-ID`) on responses.

## Calendars

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/calendars` | Required | Create a calendar (sets owner_id, auto-joins owner as participant) |
| GET | `/calendars/{id}` | — | Get calendar with participants and availability |
| POST | `/calendars/{id}/participants` | Optional | Join a calendar (auto-links user_id if signed in) |
| POST | `/calendars/{id}/participants/{participantId}/claim` | Required + participant token | Link an anonymous participant to the authenticated user |
| PATCH | `/calendars/{id}/participants/{participantId}` | Self or owner | Rename a participant / set or clear an anonymous participant's email |
| PUT | `/calendars/{id}/availability` | Self | Update availability (see [Participant authorization](#participant-authorization)) |
| POST | `/calendars/{id}/availability/import` | Required + self | Merge imported dates into the caller's own availability |
| POST | `/calendars/{id}/finalise` | Owner | Lock the calendar to a chosen date |
| DELETE | `/calendars/{id}/finalise` | Owner | Clear the locked date, reopen the calendar |
| GET | `/calendars/{id}/missing` | Owner | Returns invite list + "Not joined" / "Joined but not responded" buckets |
| POST | `/calendars/{id}/invites` | Owner | Add one or many invites (body: `{"names": ["Alice", "Bob"]}`) |
| PUT | `/calendars/{id}/invites/{inviteId}` | Owner | Rename an invite |
| DELETE | `/calendars/{id}/invites/{inviteId}` | Owner | Remove an invite |
| POST | `/calendars/{id}/invites/{inviteId}/match` | Owner | Manually link/unlink an invite to a participant |

## Admin

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/admin/metrics` | Admin | Product metrics (users, calendars, participants) |

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
{
  "id": "string",
  "name": "string",
  "owner_id": "string",
  "created_at": "RFC3339",
  "participant": {
    "id": "string",
    "calendar_id": "string",
    "name": "string",
    "colour": "string",
    "user_id": "string"
  }
}
```

The `participant` field contains the owner's auto-joined participant record.

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
    "created_at": "RFC3339",
    "finalised_date": "YYYY-MM-DD",
    "finalised_at": "RFC3339"
  },
  "participants": [
    {
      "id": "string",
      "calendar_id": "string",
      "name": "string",
      "colour": "string",
      "user_id": "string or null"
    }
  ],
  "availability": [
    {
      "participant_id": "string",
      "calendar_id": "string",
      "date": "YYYY-MM-DD",
      "note": "string (optional, up to 140 Unicode code points)",
      "preferred": true
    }
  ]
}
```

`finalised_date` and `finalised_at` are omitted when the calendar is open. When set, the calendar is read-only.

`note` is omitted from availability rows that have no note attached. `preferred` is omitted when `false` (clients should treat a missing field as `false`).

**Errors:** 404 (calendar not found)

---

### POST /calendars/{id}/participants

Add a participant to a calendar. If the user is signed in (session cookie present), the participant is automatically linked to their user account.

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
  "colour": "string",
  "user_id": "string (if signed in, otherwise omitted)"
}
```

**Response headers:** for an anonymous join, `X-Participant-Token: <secret>`. This is the only time the token is returned; the client must store it and send it back on writes for this participant. Signed-in joiners get no token (their session authorizes them).

**Errors:** 400 (name required, invalid JSON) | 404 (calendar not found) | 409 (name taken, calendar finalised, user already joined)

---

### POST /calendars/{id}/participants/{participantId}/claim

Link an anonymous participant to the authenticated user. Used when a user joins anonymously and later signs in. The participant must not already be linked to another user, and the caller must prove they hold it: send the participant's `X-Participant-Token` (legacy rows without a token follow the trust-on-first-use rule). Subject to the calendar passphrase gate (`X-Calendar-Access`). The response never includes the participant's stored `email`.

**Response 200**

```json
{
  "id": "string",
  "calendar_id": "string",
  "name": "string",
  "colour": "string",
  "user_id": "string"
}
```

**Errors:** 401 (not authenticated, passphrase required) | 403 (missing or wrong participant token) | 404 (participant not found) | 409 (already linked, user already joined)

---

### PUT /calendars/{id}/availability

Replace a participant's available dates. Sending an empty `dates` array clears availability. `participant_id` names the target row; the caller must be that participant (session for linked rows, `X-Participant-Token` for anonymous rows — see [Participant authorization](#participant-authorization)).

**Request**

```json
{
  "participant_id": "string",
  "dates": ["YYYY-MM-DD"],
  "notes": { "YYYY-MM-DD": "string (optional, ≤ 140 code points)" },
  "preferred": ["YYYY-MM-DD"]
}
```

`notes` is a partial-override map keyed by ISO date.

- `notes` **absent or `null`** → server preserves all existing notes on kept dates ("don't touch"). This is what protects a stale client bundle from silently wiping notes on the next availability edit.
- `notes` **present, partial** → only the listed dates are updated; other kept dates' notes are preserved.
- A key with value `""` → clears that date's note (stores NULL).
- A key that isn't in `dates` → 400.
- A non-empty value longer than 140 Unicode code points → 400.
- A value containing `\r` or `\n` → 400 (notes are single-line).
- An all-whitespace non-empty value → 400. (An empty string `""` is the explicit "clear" signal.)
- A date removed from `dates` takes its note with it (row deletion cascades).

`preferred` is the full set of dates the participant has voted for. It follows the same "absent = don't touch" rule as `notes`:

- `preferred` **absent or `null`** → server preserves existing votes on all kept dates.
- `preferred` **present (including `[]`)** → authoritative: every date in `dates` is set to `true` if listed in `preferred`, else `false`.
- A date in `preferred` that isn't in `dates` → 400.
- Duplicates in `preferred` are deduped server-side (not rejected).
- A date removed from `dates` takes its vote with it.

**Response 200**

```json
{"ok": true}
```

**Response headers:** `X-Participant-Token` when a legacy participant's first write minted a token.

**Errors:** 400 (participant_id required, invalid date, note validation) | 403 (caller is not this participant) | 404 (participant not found) | 409 (calendar finalised)

---

### PATCH /calendars/{id}

Update presentation fields on a calendar. Owner only.

**Request**

```json
{"name": "Tennis Fridays"}
```

- `name` — new display name.

**Response 200** — the updated calendar.

**Errors:** 400 (invalid JSON, no fields supplied, name too long) | 401 (not authenticated) | 403 (not owner) | 404 (calendar not found)

---

### POST /calendars/{id}/finalise

Lock the calendar to a single date. Only the owner may call this. Once set, availability writes and joins are rejected with 409 until the calendar is unfinalised. Re-calling this endpoint replaces the previously finalised date.

**Request**

```json
{"date": "YYYY-MM-DD"}
```

**Response 200** — the updated calendar (same shape as `GET /calendars/{id}`'s `calendar` field, with `finalised_date` and `finalised_at` set).

**Errors:** 400 (invalid date, invalid JSON) | 401 (not authenticated) | 403 (not owner) | 404 (calendar not found)

---

### DELETE /calendars/{id}/finalise

Clear the locked date. Only the owner may call this. Idempotent.

**Response 200** — the updated calendar with `finalised_date` and `finalised_at` absent.

**Errors:** 401 (not authenticated) | 403 (not owner) | 404 (calendar not found)

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

### GET /admin/metrics

Returns product metrics. Requires an authenticated admin user (`is_admin = true`).

**Response 200**

```json
{
  "users": {
    "total": 42,
    "last_7d": 5,
    "last_30d": 18
  },
  "calendars": {
    "total": 30,
    "last_7d": 3,
    "active_30d": 12
  },
  "participants": {
    "total": 120,
    "avg_per_calendar": 4.0
  }
}
```

**Errors:** 401 (not authenticated) | 403 (not admin)

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
