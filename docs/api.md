# API Reference

Base URL: `http://localhost:8080` (development)

All endpoints return JSON with `Content-Type: application/json`. Every response includes an `X-Request-ID` header. CORS is enabled with credentials support.

Error responses use the shape: `{"error": "message"}`

Request bodies are capped at 1 MiB on every route. A request whose `Content-Length` exceeds the cap gets `413 {"error":"request_too_large"}` before reaching the handler; bodies of unknown length fail to read past the cap.

> The backend uses snake_case for JSON keys. The frontend API client converts to/from camelCase.

## Authentication

Session-based auth with HTTP-only cookies. Creator must sign in to create a calendar. Joiners stay anonymous.

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/auth/google` | — | Redirect to Google OAuth. Accepts `?return_url=` |
| GET | `/auth/google/callback` | — | Google OAuth callback, creates session, redirects |
| GET | `/auth/me` | Optional | Return current user, or `null` (200) when signed out |
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
  Joining is the only place a token is issued; an anonymous row with no stored
  token can never be written.
- **Calendar owner**: may rename / edit the email of any participant (and
  remove participants). The owner can **not** write another participant's
  availability.

A rejected write returns **403** `{"code": "forbidden"}`. The `participant_id`
body field on `PATCH /participants/{participantId}` is ignored.

A `PATCH /participants/{participantId}` carrying both `name` and `email` is a
single write: if either is rejected (e.g. `invalid_email`, `duplicate_name`)
nothing changes. Availability writes are rejected with `409 already_finalised`
even when the finalise commits while the write is in flight.

`POST /calendars/{id}/duplicate` returns `409 calendar_full` only if the
participants that would actually be copied (signed-in members other than you)
plus you would exceed the participant cap. Anonymous participants aren't copied,
so they don't count.

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
| GET | `/calendars/{id}/export.ics` | Optional + gate | Download the finalised event as .ics |
| GET | `/calendars/{id}/export/{provider}` | Optional + gate | 302 to the provider's "add event" page (e.g. `google`) |

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

Users are matched first by provider account ID. Matching an existing account by email, or creating a new account, requires the provider to report the email as verified (`email_verified`); otherwise the callback returns **403 `email_not_verified`** and no session is created.

---

### GET /calendars/{id}/export.ics and GET /calendars/{id}/export/{provider}

Export a finalised calendar's event (title, date, attendees and their notes for that day). `.ics` returns `text/calendar`; `{provider}` returns a 302 to that provider's add-event URL.

Passphrase-protected calendars apply the same gate as `GET /calendars/{id}`: the owner's session, or a valid access token from `POST /calendars/{id}/unlock`. The token can be sent as the `X-Calendar-Access` header or, for plain link navigations that can't set headers, as an `?access=<token>` query param.

**Errors:** 401 `passphrase_required` (gated, no valid token) | 404 `not_found` (calendar or provider not found) | 404 `not_finalised`

---

### GET /auth/me

Returns the current authenticated user, or `null` when signed out. Being signed out is a normal state, so a missing, expired, invalid or logged-out session cookie gets a `200` with a JSON `null` body, not a `401` (a 401 makes browsers log a console error on every signed-out page load). A valid session's expiry slides, as on any authenticated route. Every other authenticated endpoint still returns `401` without a session.

**Response 200 (signed in)**

```json
{"id": "string", "email": "string", "name": "string"}
```

**Response 200 (signed out)**

```json
null
```

**Errors:** none for auth state

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
