package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/freewhen/backend/internal/store"
)

func (h *Handler) CreateCalendar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	cal, err := h.service.CreateCalendar(r.Context(), req.Name)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create calendar")
		return
	}

	respondJSON(w, http.StatusCreated, cal)
}

func (h *Handler) GetCalendar(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	cal, err := h.service.GetCalendar(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get calendar")
		return
	}

	respondJSON(w, http.StatusOK, cal)
}
