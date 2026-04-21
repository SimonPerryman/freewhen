package handler

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/freewhen/backend/internal/service"
	"github.com/freewhen/backend/internal/store"
)

// Unlock handles POST /calendars/{id}/unlock. Verifies the submitted
// passphrase, enforces a per-(calendar, IP) rate limit, and returns an
// access token on success.
func (h *Handler) Unlock(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	id := r.PathValue("id")

	var req struct {
		Passphrase string `json:"passphrase"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_json", "invalid JSON")
		return
	}
	if strings.TrimSpace(req.Passphrase) == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "passphrase is required")
		return
	}

	token, err := h.service.UnlockCalendar(r.Context(), id, req.Passphrase, remoteIP(r))
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
		case errors.Is(err, service.ErrNotPassphraseProtected):
			respondError(w, http.StatusBadRequest, "no_passphrase", "calendar is not passphrase protected")
		case errors.Is(err, service.ErrPassphraseInvalid):
			respondError(w, http.StatusUnauthorized, "invalid_passphrase", "incorrect passphrase")
		case errors.Is(err, service.ErrRateLimited):
			w.Header().Set("Retry-After", "60")
			respondError(w, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again shortly")
		default:
			respondError(w, http.StatusInternalServerError, "internal_error", "failed to unlock calendar")
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"token": token})
}

// SetPassphrase handles POST /calendars/{id}/passphrase. Owner only.
// Generates a new passphrase, stores it, bumps access_epoch, and returns
// the plaintext.
func (h *Handler) SetPassphrase(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	id := r.PathValue("id")
	cal, err := h.service.GetCalendarOnly(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to load calendar")
		return
	}
	if cal.OwnerID != user.ID {
		respondError(w, http.StatusForbidden, "forbidden", "only the owner can manage the passphrase")
		return
	}

	p, err := h.service.RotateCalendarPassphrase(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to set passphrase")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"passphrase": p})
}

// ClearPassphrase handles DELETE /calendars/{id}/passphrase. Owner only.
// Clears the passphrase and bumps access_epoch so outstanding tokens are
// revoked.
func (h *Handler) ClearPassphrase(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	id := r.PathValue("id")
	cal, err := h.service.GetCalendarOnly(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to load calendar")
		return
	}
	if cal.OwnerID != user.ID {
		respondError(w, http.StatusForbidden, "forbidden", "only the owner can manage the passphrase")
		return
	}

	if err := h.service.ClearCalendarPassphrase(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to clear passphrase")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// requireCalendarAccess authorizes the caller for a gated calendar. On
// failure it writes a 401 and returns false; callers should simply return.
// Use on mutating endpoints and the SSE stream. For GET /calendars/{id},
// the dedicated handler returns a gate-aware JSON body instead.
func (h *Handler) requireCalendarAccess(w http.ResponseWriter, r *http.Request, calendarID string) (bool, error) {
	viewer := service.Viewer{}
	if u := UserFromContext(r); u != nil {
		uid := u.ID
		viewer.UserID = &uid
	}
	if as := strings.TrimSpace(r.URL.Query().Get("as")); as != "" {
		viewer.ParticipantID = &as
	}

	cal, err := h.service.GetCalendarOnly(r.Context(), calendarID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return false, err
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to load calendar")
		return false, err
	}

	if err := h.service.AuthorizeAccess(r.Context(), cal, viewer, r.Header.Get(AccessHeader)); err != nil {
		if errors.Is(err, service.ErrPassphraseRequired) {
			respondError(w, http.StatusUnauthorized, "passphrase_required", "passphrase required")
			return false, err
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to authorize access")
		return false, err
	}
	return true, nil
}

// remoteIP extracts the client IP from the request. Trusts X-Forwarded-For's
// left-most entry when present (Cloud Run / Vercel put the origin IP there),
// otherwise falls back to r.RemoteAddr's host portion.
func remoteIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
