package handler

import (
	"errors"
	"net/http"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/service"
)

// The invite endpoints are owner-only. Ownership (and "invite belongs to this
// calendar") is enforced by the service; handlers map the result.

// ListMissing handles GET /calendars/{id}/missing. Owner-only.
func (h *Handler) ListMissing(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	calendarID := r.PathValue("id")
	view, err := h.service.GetMissingView(r.Context(), calendarID, user.ID)
	if err != nil {
		if respondOwnerError(r, w, h.service, calendarID, err, "only the owner can manage invites") {
			return
		}
		respondInternalError(r, w, err, "internal_error", "failed to load missing view")
		return
	}
	respondJSON(w, http.StatusOK, view)
}

// AddInvites handles POST /calendars/{id}/invites.
func (h *Handler) AddInvites(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	calendarID := r.PathValue("id")

	var req struct {
		Names []string `json:"names"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	res, err := h.service.AddInvites(r.Context(), calendarID, user.ID, req.Names)
	if err != nil {
		h.respondInviteError(r, w, calendarID, err)
		return
	}
	respondJSON(w, http.StatusCreated, res)
}

// RenameInvite handles PUT /calendars/{id}/invites/{inviteId}.
func (h *Handler) RenameInvite(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	calendarID := r.PathValue("id")
	inviteID := r.PathValue("inviteId")

	var req struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	inv, err := h.service.RenameInvite(r.Context(), calendarID, user.ID, inviteID, req.Name)
	if err != nil {
		h.respondInviteError(r, w, calendarID, err)
		return
	}
	respondJSON(w, http.StatusOK, inv)
}

// DeleteInvite handles DELETE /calendars/{id}/invites/{inviteId}. Idempotent:
// 404 is only returned if the invite was never created on this calendar.
func (h *Handler) DeleteInvite(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	calendarID := r.PathValue("id")
	inviteID := r.PathValue("inviteId")
	if err := h.service.DeleteInvite(r.Context(), calendarID, user.ID, inviteID); err != nil {
		if respondOwnerError(r, w, h.service, calendarID, err, "only the owner can manage invites") {
			return
		}
		if errors.Is(err, model.ErrInviteNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "invite not found")
			return
		}
		respondInternalError(r, w, err, "internal_error", "failed to delete invite")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ManualMatchInvite handles POST /calendars/{id}/invites/{inviteId}/match.
// Body: `{"participant_id": "..."} | {"participant_id": null}` — the second
// form unlinks.
func (h *Handler) ManualMatchInvite(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	calendarID := r.PathValue("id")
	inviteID := r.PathValue("inviteId")

	var req struct {
		ParticipantID *string `json:"participant_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	inv, err := h.service.ManualMatchInvite(r.Context(), calendarID, user.ID, inviteID, req.ParticipantID)
	if err != nil {
		h.respondInviteError(r, w, calendarID, err)
		return
	}
	respondJSON(w, http.StatusOK, inv)
}

func (h *Handler) respondInviteError(r *http.Request, w http.ResponseWriter, calendarID string, err error) {
	if respondOwnerError(r, w, h.service, calendarID, err, "only the owner can manage invites") {
		return
	}
	if respondValidationError(w, err) {
		return
	}
	switch {
	case errors.Is(err, model.ErrInviteLimitReached):
		respondError(w, http.StatusConflict, "invite_limit_reached", "invite list is full")
	case errors.Is(err, model.ErrInviteNameTaken):
		respondError(w, http.StatusConflict, "invite_name_taken", "an invite with this name already exists")
	case errors.Is(err, model.ErrInviteNotFound):
		respondError(w, http.StatusNotFound, "not_found", "invite not found")
	case errors.Is(err, model.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", "calendar not found")
	case errors.Is(err, service.ErrInviteNameRequired):
		respondError(w, http.StatusBadRequest, "name_required", "name is required")
	case errors.Is(err, service.ErrInviteNameTooLong):
		respondError(w, http.StatusBadRequest, "name_too_long", "invite name is too long")
	case errors.Is(err, service.ErrParticipantNotOnCalendar):
		respondError(w, http.StatusBadRequest, "participant_not_on_calendar", "participant does not belong to this calendar")
	default:
		respondInternalError(r, w, err, "internal_error", "failed to process invite")
	}
}
