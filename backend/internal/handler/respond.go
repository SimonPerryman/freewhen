package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// CalendarLookup is the narrow read surface respondCalendarMissing needs to
// distinguish 404 (typo / never existed) from 410 (existed once, soft-deleted).
type CalendarLookup interface {
	IsCalendarSoftDeleted(ctx context.Context, calendarID string) (bool, error)
}

// respondCalendarMissing handles store.ErrNotFound on a `/calendars/{id}/...`
// mutating path. If the row exists but is soft-deleted, returns 410 Gone
// with `calendar_unavailable`; otherwise the standard 404. Frontend maps
// 410+code to a "calendar gone" toast and routes back to /calendars.
func respondCalendarMissing(r *http.Request, w http.ResponseWriter, lookup CalendarLookup, calendarID string) {
	if soft, err := lookup.IsCalendarSoftDeleted(r.Context(), calendarID); err == nil && soft {
		respondError(w, http.StatusGone, "calendar_unavailable", "this calendar is no longer available")
		return
	}
	respondError(w, http.StatusNotFound, "not_found", "calendar not found")
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// respondError writes a JSON error body with a stable machine-readable code.
// Clients branch on code rather than the human-readable message.
func respondError(w http.ResponseWriter, status int, code, message string) {
	respondJSON(w, status, map[string]string{"error": message, "code": code})
}

// respondInternalError logs the underlying error with request context, then
// writes a 500 response. Use at every 5xx site so the wrapped error is not
// swallowed — the client only sees the generic message, but logs capture the
// real cause keyed by request_id.
func respondInternalError(r *http.Request, w http.ResponseWriter, err error, code, message string) {
	slog.ErrorContext(r.Context(), "internal error",
		"error", err,
		"code", code,
		"message", message,
		"method", r.Method,
		"path", r.URL.Path,
		"request_id", RequestIDFromContext(r.Context()),
	)
	respondError(w, http.StatusInternalServerError, code, message)
}
