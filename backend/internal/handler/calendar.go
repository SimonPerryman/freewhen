package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/freewhen/backend/internal/store"
)

func (h *Handler) CreateCalendar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	cal, err := h.service.CreateCalendar(r.Context(), req.Name, user.ID)
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

func (h *Handler) FinaliseCalendar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := r.PathValue("id")

	var req struct {
		Date string `json:"date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if _, err := time.Parse("2006-01-02", req.Date); err != nil {
		respondError(w, http.StatusBadRequest, "invalid date format")
		return
	}

	existing, err := h.service.GetCalendarOnly(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to load calendar")
		return
	}
	if existing.OwnerID != user.ID {
		respondError(w, http.StatusForbidden, "only the owner can finalise this calendar")
		return
	}

	cal, err := h.service.FinaliseCalendar(r.Context(), id, req.Date)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to finalise calendar")
		return
	}

	respondJSON(w, http.StatusOK, cal)
}

func (h *Handler) UnfinaliseCalendar(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := r.PathValue("id")

	existing, err := h.service.GetCalendarOnly(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to load calendar")
		return
	}
	if existing.OwnerID != user.ID {
		respondError(w, http.StatusForbidden, "only the owner can unfinalise this calendar")
		return
	}

	cal, err := h.service.UnfinaliseCalendar(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to unfinalise calendar")
		return
	}

	respondJSON(w, http.StatusOK, cal)
}
