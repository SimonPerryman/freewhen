package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/freewhen/backend/internal/store"
)

func (h *Handler) AddParticipant(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	calendarID := r.PathValue("id")

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}

	p, err := h.service.AddParticipant(r.Context(), calendarID, req.Name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "calendar not found")
			return
		}
		if errors.Is(err, store.ErrDuplicateName) {
			respondError(w, http.StatusConflict, "participant name already taken")
			return
		}
		if errors.Is(err, store.ErrCalendarFinalised) {
			respondError(w, http.StatusConflict, "calendar is finalised")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to add participant")
		return
	}

	respondJSON(w, http.StatusCreated, p)
}
