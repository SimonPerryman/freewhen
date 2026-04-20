package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/freewhen/backend/internal/limits"
	"github.com/freewhen/backend/internal/store"
)

func (h *Handler) SetAvailability(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	calendarID := r.PathValue("id")

	var req struct {
		ParticipantID string   `json:"participant_id"`
		Dates         []string `json:"dates"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_json", "invalid JSON")
		return
	}

	if req.ParticipantID == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "participant_id is required")
		return
	}

	if len(req.Dates) > limits.MaxDatesPerSubmission {
		respondError(w, http.StatusBadRequest, "too_many_dates", "too many dates in a single submission")
		return
	}

	now := time.Now().UTC()
	for _, d := range req.Dates {
		parsed, err := time.Parse("2006-01-02", d)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid_date", "invalid date format: "+d)
			return
		}
		if !limits.DateWithinRange(parsed, now) {
			respondError(w, http.StatusBadRequest, "date_out_of_range", "date out of range: "+d)
			return
		}
	}

	err := h.service.SetAvailability(r.Context(), calendarID, req.ParticipantID, req.Dates)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "participant not found")
			return
		}
		if errors.Is(err, store.ErrCalendarFinalised) {
			respondError(w, http.StatusConflict, "already_finalised", "calendar is finalised")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to set availability")
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
