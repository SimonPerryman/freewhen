package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/freewhen/backend/internal/service"
	"github.com/freewhen/backend/internal/store"
)

// requireCalendarOwner gates the invite endpoints. Returns the authenticated
// owner on success. On failure it writes the HTTP response and returns nil —
// callers should simply return.
func (h *Handler) requireCalendarOwner(w http.ResponseWriter, r *http.Request, calendarID string) bool {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return false
	}
	cal, err := h.service.GetCalendarOnly(r.Context(), calendarID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return false
		}
		respondInternalError(r, w, err, "internal_error", "failed to load calendar")
		return false
	}
	if cal.OwnerID != user.ID {
		respondError(w, http.StatusForbidden, "forbidden", "only the owner can manage invites")
		return false
	}
	return true
}

// ListMissing handles GET /calendars/{id}/missing. Owner-only.
func (h *Handler) ListMissing(w http.ResponseWriter, r *http.Request) {
	calendarID := r.PathValue("id")
	if !h.requireCalendarOwner(w, r, calendarID) {
		return
	}
	view, err := h.service.GetMissingView(r.Context(), calendarID)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "failed to load missing view")
		return
	}
	respondJSON(w, http.StatusOK, view)
}

// AddInvites handles POST /calendars/{id}/invites.
func (h *Handler) AddInvites(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	calendarID := r.PathValue("id")
	if !h.requireCalendarOwner(w, r, calendarID) {
		return
	}

	var req struct {
		Names []string `json:"names"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_json", "invalid JSON")
		return
	}
	if len(req.Names) == 0 {
		respondError(w, http.StatusBadRequest, "names_required", "names is required")
		return
	}

	res, err := h.service.AddInvites(r.Context(), calendarID, req.Names)
	if err != nil {
		h.respondInviteError(r, w, err)
		return
	}
	respondJSON(w, http.StatusCreated, res)
}

// RenameInvite handles PUT /calendars/{id}/invites/{inviteId}.
func (h *Handler) RenameInvite(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	calendarID := r.PathValue("id")
	inviteID := r.PathValue("inviteId")
	if !h.requireCalendarOwner(w, r, calendarID) {
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_json", "invalid JSON")
		return
	}

	// Verify invite belongs to this calendar before mutation.
	if ok := h.checkInviteOnCalendar(w, r, calendarID, inviteID); !ok {
		return
	}

	inv, err := h.service.RenameInvite(r.Context(), inviteID, req.Name)
	if err != nil {
		h.respondInviteError(r, w, err)
		return
	}
	respondJSON(w, http.StatusOK, inv)
}

// DeleteInvite handles DELETE /calendars/{id}/invites/{inviteId}. Idempotent:
// 404 is only returned if the invite was never created on this calendar.
func (h *Handler) DeleteInvite(w http.ResponseWriter, r *http.Request) {
	calendarID := r.PathValue("id")
	inviteID := r.PathValue("inviteId")
	if !h.requireCalendarOwner(w, r, calendarID) {
		return
	}
	if ok := h.checkInviteOnCalendar(w, r, calendarID, inviteID); !ok {
		return
	}
	if err := h.service.DeleteInvite(r.Context(), inviteID); err != nil {
		if errors.Is(err, store.ErrInviteNotFound) {
			w.WriteHeader(http.StatusNoContent)
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
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	calendarID := r.PathValue("id")
	inviteID := r.PathValue("inviteId")
	if !h.requireCalendarOwner(w, r, calendarID) {
		return
	}

	var req struct {
		ParticipantID *string `json:"participant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_json", "invalid JSON")
		return
	}

	inv, err := h.service.ManualMatchInvite(r.Context(), calendarID, inviteID, req.ParticipantID)
	if err != nil {
		h.respondInviteError(r, w, err)
		return
	}
	respondJSON(w, http.StatusOK, inv)
}

func (h *Handler) checkInviteOnCalendar(w http.ResponseWriter, r *http.Request, calendarID, inviteID string) bool {
	inv, err := h.service.GetInvite(r.Context(), inviteID)
	if err != nil {
		if errors.Is(err, store.ErrInviteNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "invite not found")
			return false
		}
		respondInternalError(r, w, err, "internal_error", "failed to load invite")
		return false
	}
	if inv.CalendarID != calendarID {
		respondError(w, http.StatusNotFound, "not_found", "invite not found")
		return false
	}
	return true
}

func (h *Handler) respondInviteError(r *http.Request, w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrInviteLimitReached):
		respondError(w, http.StatusConflict, "invite_limit_reached", "invite list is full")
	case errors.Is(err, store.ErrInviteNameTaken):
		respondError(w, http.StatusConflict, "invite_name_taken", "an invite with this name already exists")
	case errors.Is(err, store.ErrInviteNotFound):
		respondError(w, http.StatusNotFound, "not_found", "invite not found")
	case errors.Is(err, store.ErrNotFound):
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
