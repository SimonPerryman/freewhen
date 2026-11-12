package handler

import (
	"errors"
	"net/http"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/service"
)

// SetAvailability handles PUT /calendars/{id}/availability. Input rules
// (date format/range/count, note and preferred-date constraints) live in
// service.SetAvailability and come back as *service.ValidationError.
func (h *Handler) SetAvailability(w http.ResponseWriter, r *http.Request) {
	calendarID := r.PathValue("id")

	if ok, _ := h.requireCalendarAccess(w, r, calendarID); !ok {
		return
	}

	var req struct {
		ParticipantID string             `json:"participant_id"`
		Dates         []string           `json:"dates"`
		Notes         *map[string]string `json:"notes"`
		Preferred     *[]string          `json:"preferred"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	update := service.AvailabilityUpdate{Dates: req.Dates, Notes: req.Notes, Preferred: req.Preferred}
	if err := h.service.SetAvailability(r.Context(), calendarID, req.ParticipantID, participantCaller(r), update); err != nil {
		h.respondAvailabilityError(r, w, calendarID, err, "failed to set availability")
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ImportAvailability handles POST /calendars/{id}/availability/import.
// Non-destructive merge of imported dates for the caller's participant row.
// Used by the "Accept all suggestions" shortcut after a Google Calendar
// import — individual cell taps continue to go through SetAvailability.
func (h *Handler) ImportAvailability(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	calendarID := r.PathValue("id")
	if ok, _ := h.requireCalendarAccess(w, r, calendarID); !ok {
		return
	}

	var req struct {
		ParticipantID string   `json:"participant_id"`
		Days          []string `json:"days"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := h.service.ImportAvailability(r.Context(), calendarID, req.ParticipantID, participantCaller(r), req.Days); err != nil {
		h.respondAvailabilityError(r, w, calendarID, err, "failed to import availability")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"ok": true, "imported": len(req.Days)})
}

func (h *Handler) respondAvailabilityError(r *http.Request, w http.ResponseWriter, calendarID string, err error, internalMsg string) {
	if respondValidationError(w, err) {
		return
	}
	switch {
	case errors.Is(err, model.ErrNotFound):
		respondCalendarMissing(r, w, h.service, calendarID)
	case errors.Is(err, service.ErrForbidden):
		respondError(w, http.StatusForbidden, "forbidden", "not allowed to edit this participant")
	case errors.Is(err, model.ErrCalendarFinalised):
		respondError(w, http.StatusConflict, "already_finalised", "calendar is finalised")
	default:
		respondInternalError(r, w, err, "internal_error", internalMsg)
	}
}
