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

	// Auto-link to user if signed in (optionalAuth middleware)
	var userID *string
	if user := UserFromContext(r); user != nil {
		userID = &user.ID
	}

	p, err := h.service.AddParticipant(r.Context(), calendarID, req.Name, userID)
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
		if errors.Is(err, store.ErrUserAlreadyJoined) {
			respondError(w, http.StatusConflict, "you have already joined this calendar")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to add participant")
		return
	}

	respondJSON(w, http.StatusCreated, p)
}

func (h *Handler) ClaimParticipant(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	calendarID := r.PathValue("id")
	participantID := r.PathValue("participantId")

	p, err := h.service.ClaimParticipant(r.Context(), calendarID, participantID, user.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "participant not found")
			return
		}
		if errors.Is(err, store.ErrAlreadyLinked) {
			respondError(w, http.StatusConflict, "participant already linked to a user")
			return
		}
		if errors.Is(err, store.ErrUserAlreadyJoined) {
			respondError(w, http.StatusConflict, "you have already joined this calendar")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to claim participant")
		return
	}

	respondJSON(w, http.StatusOK, p)
}
