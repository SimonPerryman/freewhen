# API Reference

Base URL: `http://localhost:8080` (development)

All endpoints return JSON with `Content-Type: application/json`. Every response includes an `X-Request-ID` header. CORS is enabled.

Error responses use the shape: `{"error": "message"}`

> The backend uses snake_case for JSON keys. The frontend API client converts to/from camelCase.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/calendars` | Create a calendar |
| GET | `/calendars/{id}` | Get calendar with participants and availability |
| POST | `/calendars/{id}/participants` | Join a calendar |
| PUT | `/calendars/{id}/availability` | Update availability |
| GET | `/health` | Health check |

---

### POST /calendars

Create a new calendar.

**Request**

```json
{"name": "string"}
```

**Response 201**

```json
{"id": "string", "name": "string", "created_at": "RFC3339"}
```

**Errors:** 400 (invalid JSON)

---

### GET /calendars/{id}

Get a calendar with all participants and their availability.

**Response 200**

```json
{
  "calendar": {
    "id": "string",
    "name": "string",
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
