package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

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
