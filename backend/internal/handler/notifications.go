package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/freewhen/backend/internal/notifications"
)

// CalendarOwnerAuthorizer is the slice of the service the nudge handler needs
// to enforce owner-only access. Implemented by *service.Service.
type CalendarOwnerAuthorizer interface {
	AuthorizeCalendarOwner(ctx context.Context, calendarID, userID string) error
	IsCalendarSoftDeleted(ctx context.Context, calendarID string) (bool, error)
}

// NotificationsHandler serves the unsubscribe + nudge endpoints. Lives outside
// the main Handler so a nil notifications.Service (i.e. NOTIFICATIONS_HMAC_SECRET
// unset) can short-circuit registration in server.go.
type NotificationsHandler struct {
	svc    *notifications.Service
	owners CalendarOwnerAuthorizer
	logger *slog.Logger
}

// NewNotifications wires the notifications handler. svc may be nil to
// disable the routes; the server.go wiring will skip registration in that
// case so route-coverage doesn't see them.
func NewNotifications(svc *notifications.Service, owners CalendarOwnerAuthorizer, logger *slog.Logger) *NotificationsHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &NotificationsHandler{svc: svc, owners: owners, logger: logger}
}

// Unsubscribe handles GET /unsubscribe?t=<token>. Public — the token is
// the only credential. Renders an HTML page rather than returning JSON: this
// is what the user lands on after clicking the native Unsubscribe button in
// their email client.
func (h *NotificationsHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("t")
	if token == "" {
		writeUnsubscribeHTML(w, http.StatusBadRequest, false)
		return
	}
	if err := h.svc.ConsumeUnsubscribeToken(r.Context(), token); err != nil {
		writeUnsubscribeHTML(w, http.StatusBadRequest, false)
		return
	}
	writeUnsubscribeHTML(w, http.StatusOK, true)
}

func writeUnsubscribeHTML(w http.ResponseWriter, status int, success bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(notifications.UnsubscribePage(success)))
}

// Nudge handles POST /calendars/{id}/nudge. Owner-only. Returns
// {emailed, skipped} so the UI can show the "copy link for the rest"
// affordance.
func (h *NotificationsHandler) Nudge(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	calendarID := r.PathValue("id")

	if err := h.owners.AuthorizeCalendarOwner(r.Context(), calendarID, user.ID); err != nil {
		if respondOwnerError(r, w, h.owners, calendarID, err, "only the owner can send a reminder") {
			return
		}
		respondInternalError(r, w, err, "internal_error", "failed to load calendar")
		return
	}

	result, err := h.svc.Nudge(r.Context(), calendarID, user.Name)
	if err != nil {
		switch {
		case errors.Is(err, notifications.ErrNudgeRateLimited):
			respondError(w, http.StatusTooManyRequests, "rate_limited", "you can send another reminder in a few hours")
		case errors.Is(err, notifications.ErrNotificationsDisabled):
			respondError(w, http.StatusServiceUnavailable, "notifications_disabled", "notifications not configured")
		default:
			respondInternalError(r, w, err, "internal_error", "failed to send reminder")
		}
		return
	}

	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.logger.Warn("nudge encode response failed", "error", err)
	}
}
