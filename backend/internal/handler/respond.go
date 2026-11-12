package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/service"
)

// CalendarLookup is the narrow read surface respondCalendarMissing needs to
// distinguish 404 (typo / never existed) from 410 (existed once, soft-deleted).
type CalendarLookup interface {
	IsCalendarSoftDeleted(ctx context.Context, calendarID string) (bool, error)
}

// respondCalendarMissing handles model.ErrNotFound on a `/calendars/{id}/...`
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

// decodeJSON decodes the request body into v. On failure it writes
// 400 invalid_json and returns false; callers should simply return. Body
// size is capped globally by the server's limitBody middleware, so handlers
// no longer wrap r.Body themselves.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	return decodeJSONMessage(w, r, v, "invalid JSON")
}

// decodeJSONMessage is decodeJSON with a custom human-readable message, for
// the auth endpoints whose historical message is "invalid request body".
func decodeJSONMessage(w http.ResponseWriter, r *http.Request, v any, message string) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_json", message)
		return false
	}
	return true
}

// respondValidationError maps a *service.ValidationError to 400 with its
// stable code. Returns false (writing nothing) for any other error.
func respondValidationError(w http.ResponseWriter, err error) bool {
	var ve *service.ValidationError
	if errors.As(err, &ve) {
		respondError(w, http.StatusBadRequest, ve.Code, ve.Message)
		return true
	}
	return false
}

// respondOwnerError maps the errors of the service's owner-only check:
// a missing calendar → 404/410 (via respondCalendarMissing) and
// service.ErrForbidden → 403 with forbiddenMsg. Returns false (writing
// nothing) for any other error.
func respondOwnerError(r *http.Request, w http.ResponseWriter, lookup CalendarLookup, calendarID string, err error, forbiddenMsg string) bool {
	switch {
	case errors.Is(err, model.ErrCalendarNotFound):
		respondCalendarMissing(r, w, lookup, calendarID)
	case errors.Is(err, service.ErrForbidden):
		respondError(w, http.StatusForbidden, "forbidden", forbiddenMsg)
	default:
		return false
	}
	return true
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
