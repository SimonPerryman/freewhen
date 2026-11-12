package handler

import (
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/service"
)

// Unlock handles POST /calendars/{id}/unlock. Verifies the submitted
// passphrase, enforces a per-(calendar, IP) rate limit, and returns an
// access token on success.
func (h *Handler) Unlock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Passphrase string `json:"passphrase"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Passphrase) == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "passphrase is required")
		return
	}

	token, err := h.service.UnlockCalendar(r.Context(), id, req.Passphrase, remoteIP(r, h.trustedProxyDepth))
	if err != nil {
		switch {
		case errors.Is(err, model.ErrNotFound):
			respondCalendarMissing(r, w, h.service, id)
		case errors.Is(err, service.ErrNotPassphraseProtected):
			respondError(w, http.StatusBadRequest, "no_passphrase", "calendar is not passphrase protected")
		case errors.Is(err, service.ErrPassphraseInvalid):
			respondError(w, http.StatusUnauthorized, "invalid_passphrase", "incorrect passphrase")
		case errors.Is(err, service.ErrRateLimited):
			w.Header().Set("Retry-After", "60")
			respondError(w, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again shortly")
		default:
			respondInternalError(r, w, err, "internal_error", "failed to unlock calendar")
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
	p, err := h.service.RotateCalendarPassphrase(r.Context(), id, user.ID)
	if err != nil {
		if respondOwnerError(r, w, h.service, id, err, "only the owner can manage the passphrase") {
			return
		}
		respondInternalError(r, w, err, "internal_error", "failed to set passphrase")
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
	if err := h.service.ClearCalendarPassphrase(r.Context(), id, user.ID); err != nil {
		if respondOwnerError(r, w, h.service, id, err, "only the owner can manage the passphrase") {
			return
		}
		respondInternalError(r, w, err, "internal_error", "failed to clear passphrase")
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
	cal, err := h.service.GetCalendarOnly(r.Context(), calendarID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			respondCalendarMissing(r, w, h.service, calendarID)
			return false, err
		}
		respondInternalError(r, w, err, "internal_error", "failed to load calendar")
		return false, err
	}

	if err := h.service.AuthorizeAccess(r.Context(), cal, viewer, r.Header.Get(AccessHeader)); err != nil {
		if errors.Is(err, service.ErrPassphraseRequired) {
			respondError(w, http.StatusUnauthorized, "passphrase_required", "passphrase required")
			return false, err
		}
		respondInternalError(r, w, err, "internal_error", "failed to authorize access")
		return false, err
	}
	return true, nil
}

// remoteIP extracts the client IP from the request, treating the rightmost
// trustedProxyDepth entries of X-Forwarded-For as infrastructure hops we
// control. The client IP is the entry immediately to the left of those
// trusted hops; anything further left is attacker-supplied input and ignored.
//
// Cloud Run direct ingress appends only the actual client IP, so depth=0
// (the default) reads the rightmost entry — the real client. With an external
// HTTPS load balancer in front, set TRUSTED_PROXY_DEPTH=1 to skip the LB hop.
//
// If XFF is absent or has fewer entries than depth+1 (a misconfigured proxy
// chain — fail safe rather than silently trusting attacker input), we fall
// back to r.RemoteAddr's host portion.
func remoteIP(r *http.Request, trustedProxyDepth int) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		idx := len(parts) - 1 - trustedProxyDepth
		if idx >= 0 {
			return strings.TrimSpace(parts[idx])
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
