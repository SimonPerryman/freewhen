package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/freewhen/backend/internal/limits"
	"github.com/freewhen/backend/internal/store"
)

func (h *Handler) AddParticipant(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	calendarID := r.PathValue("id")

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_json", "invalid JSON")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "name is required")
		return
	}
	if len(req.Name) > limits.MaxParticipantNameLength {
		respondError(w, http.StatusBadRequest, "name_too_long", "participant name is too long")
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
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return
		}
		if errors.Is(err, store.ErrDuplicateName) {
			respondError(w, http.StatusConflict, "duplicate_name", "participant name already taken")
			return
		}
		if errors.Is(err, store.ErrCalendarFinalised) {
			respondError(w, http.StatusConflict, "already_finalised", "calendar is finalised")
			return
		}
		if errors.Is(err, store.ErrUserAlreadyJoined) {
			respondError(w, http.StatusConflict, "already_joined", "you have already joined this calendar")
			return
		}
		if errors.Is(err, store.ErrCalendarFull) {
			respondError(w, http.StatusConflict, "calendar_full", "this calendar is full")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to add participant")
		return
	}

	respondJSON(w, http.StatusCreated, p)
}

func (h *Handler) ClaimParticipant(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	calendarID := r.PathValue("id")
	participantID := r.PathValue("participantId")

	p, err := h.service.ClaimParticipant(r.Context(), calendarID, participantID, user.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "participant not found")
			return
		}
		if errors.Is(err, store.ErrAlreadyLinked) {
			respondError(w, http.StatusConflict, "already_linked", "participant already linked to a user")
			return
		}
		if errors.Is(err, store.ErrUserAlreadyJoined) {
			respondError(w, http.StatusConflict, "already_joined", "you have already joined this calendar")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to claim participant")
		return
	}

	respondJSON(w, http.StatusOK, p)
}
