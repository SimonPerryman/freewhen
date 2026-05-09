package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/freewhen/backend/internal/limits"
	"github.com/freewhen/backend/internal/service"
	"github.com/freewhen/backend/internal/store"
)

func (h *Handler) AddParticipant(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	calendarID := r.PathValue("id")

	if ok, _ := h.requireCalendarAccess(w, r, calendarID); !ok {
		return
	}

	var req struct {
		Name  string  `json:"name"`
		Email *string `json:"email"`
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

	// Auto-link to user if signed in (optionalAuth middleware). Signed-in
	// joiners reuse their account email; ignore any client-supplied address
	// rather than splitting state across users.email and participants.email.
	var userID *string
	email := req.Email
	if user := UserFromContext(r); user != nil {
		userID = &user.ID
		email = nil
	}

	p, err := h.service.AddParticipant(r.Context(), calendarID, req.Name, userID, email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return
		}
		if errors.Is(err, service.ErrInvalidName) {
			respondError(w, http.StatusBadRequest, "invalid_request", "name is required")
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
		if errors.Is(err, service.ErrInvalidEmail) {
			respondError(w, http.StatusBadRequest, "invalid_email", "email is invalid")
			return
		}
		respondInternalError(r, w, err, "internal_error", "failed to add participant")
		return
	}

	respondJSON(w, http.StatusCreated, p)
}

// RenameParticipant handles PATCH /calendars/{id}/participants/{participantId}.
// Accepts a name and (optionally) the caller's own participant_id; see
// service.RenameParticipant for the authorization rules.
func (h *Handler) RenameParticipant(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	calendarID := r.PathValue("id")
	participantID := r.PathValue("participantId")

	if ok, _ := h.requireCalendarAccess(w, r, calendarID); !ok {
		return
	}

	var req struct {
		Name          *string `json:"name"`
		Email         *string `json:"email"`
		ParticipantID string  `json:"participant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_json", "invalid JSON")
		return
	}

	var userID *string
	if user := UserFromContext(r); user != nil {
		userID = &user.ID
	}

	// Email-only updates skip the rename / finalise guards — clearing or
	// setting your contact address is allowed even after the calendar locks.
	emailOnly := req.Name == nil && req.Email != nil
	if emailOnly {
		// Signed-in participants resolve email via users.email at send time;
		// disallow editing the per-participant override they don't have.
		if userID != nil {
			respondError(w, http.StatusBadRequest, "invalid_request", "signed-in users edit their email under account settings")
			return
		}
		p, err := h.service.UpdateParticipantEmail(r.Context(), calendarID, participantID, userID, req.ParticipantID, req.Email)
		if err != nil {
			switch {
			case errors.Is(err, store.ErrNotFound):
				respondError(w, http.StatusNotFound, "not_found", "participant not found")
			case errors.Is(err, service.ErrForbidden):
				respondError(w, http.StatusForbidden, "forbidden", "not allowed to edit this participant")
			case errors.Is(err, service.ErrInvalidEmail):
				respondError(w, http.StatusBadRequest, "invalid_email", "email is invalid")
			default:
				respondInternalError(r, w, err, "internal_error", "failed to update participant")
			}
			return
		}
		respondJSON(w, http.StatusOK, p)
		return
	}

	if req.Name == nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "name is required")
		return
	}
	name := strings.TrimSpace(*req.Name)
	if name == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "name is required")
		return
	}
	if len(name) > limits.MaxParticipantNameLength {
		respondError(w, http.StatusBadRequest, "name_too_long", "participant name is too long")
		return
	}

	p, err := h.service.RenameParticipant(r.Context(), calendarID, participantID, userID, req.ParticipantID, name)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			respondError(w, http.StatusNotFound, "not_found", "participant not found")
		case errors.Is(err, service.ErrForbidden):
			respondError(w, http.StatusForbidden, "forbidden", "not allowed to rename this participant")
		case errors.Is(err, service.ErrInvalidName):
			respondError(w, http.StatusBadRequest, "invalid_request", "name is required")
		case errors.Is(err, store.ErrDuplicateName):
			respondError(w, http.StatusConflict, "duplicate_name", "participant name already taken")
		case errors.Is(err, store.ErrCalendarFinalised):
			respondError(w, http.StatusConflict, "already_finalised", "calendar is finalised")
		default:
			respondInternalError(r, w, err, "internal_error", "failed to rename participant")
		}
		return
	}

	// If the caller also supplied an email, apply it as a follow-up update.
	if req.Email != nil && userID == nil {
		updated, err := h.service.UpdateParticipantEmail(r.Context(), calendarID, participantID, userID, req.ParticipantID, req.Email)
		if err != nil {
			if errors.Is(err, service.ErrInvalidEmail) {
				respondError(w, http.StatusBadRequest, "invalid_email", "email is invalid")
				return
			}
			respondInternalError(r, w, err, "internal_error", "failed to update participant email")
			return
		}
		p = updated
	}

	respondJSON(w, http.StatusOK, p)
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
		respondInternalError(r, w, err, "internal_error", "failed to claim participant")
		return
	}

	respondJSON(w, http.StatusOK, p)
}
