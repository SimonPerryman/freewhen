package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

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
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.ParticipantID == "" {
		respondError(w, http.StatusBadRequest, "participant_id is required")
		return
	}

	for _, d := range req.Dates {
		if _, err := time.Parse("2006-01-02", d); err != nil {
			respondError(w, http.StatusBadRequest, "invalid date format: "+d)
			return
		}
	}

	err := h.service.SetAvailability(r.Context(), calendarID, req.ParticipantID, req.Dates)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "participant not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to set availability")
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
