package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/service"
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
		Name       string `json:"name"`
		Visibility string `json:"visibility"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	visibility := req.Visibility
	if visibility == "" {
		visibility = model.VisibilityOpen
	}
	if !model.IsValidVisibility(visibility) {
		respondError(w, http.StatusBadRequest, "invalid visibility value")
		return
	}

	cal, err := h.service.CreateCalendar(r.Context(), req.Name, user.ID, visibility)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create calendar")
		return
	}

	// Auto-join the owner as a participant
	participant, err := h.service.AddParticipant(r.Context(), cal.ID, user.Name, &user.ID)
	if err != nil {
		// Calendar was created but participant failed — still return the calendar.
		// The owner can join manually on the calendar page.
		respondJSON(w, http.StatusCreated, struct {
			model.Calendar
			Participant *model.Participant `json:"participant,omitempty"`
		}{Calendar: cal})
		return
	}

	respondJSON(w, http.StatusCreated, struct {
		model.Calendar
		Participant *model.Participant `json:"participant,omitempty"`
	}{Calendar: cal, Participant: &participant})
}

func (h *Handler) GetCalendar(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	viewer := service.Viewer{}
	if u := UserFromContext(r); u != nil {
		uid := u.ID
		viewer.UserID = &uid
	}
	if as := strings.TrimSpace(r.URL.Query().Get("as")); as != "" {
		viewer.ParticipantID = &as
	}

	view, err := h.service.GetCalendarView(r.Context(), id, viewer)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get calendar")
		return
	}

	respondJSON(w, http.StatusOK, view.Details)
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

// UpdateCalendar handles PATCH /calendars/{id}. Accepts any combination of
// optional name and visibility fields. Owner only.
func (h *Handler) UpdateCalendar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := r.PathValue("id")

	var req struct {
		Name       *string `json:"name"`
		Visibility *string `json:"visibility"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == nil && req.Visibility == nil {
		respondError(w, http.StatusBadRequest, "no updatable fields provided")
		return
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Visibility != nil && !model.IsValidVisibility(*req.Visibility) {
		respondError(w, http.StatusBadRequest, "invalid visibility value")
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
		respondError(w, http.StatusForbidden, "only the owner can update this calendar")
		return
	}

	cal := existing
	if req.Name != nil {
		updated, err := h.service.RenameCalendar(r.Context(), id, *req.Name)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				respondError(w, http.StatusNotFound, "calendar not found")
				return
			}
			respondError(w, http.StatusInternalServerError, "failed to rename calendar")
			return
		}
		cal = updated
	}
	if req.Visibility != nil {
		updated, err := h.service.UpdateCalendarVisibility(r.Context(), id, *req.Visibility)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				respondError(w, http.StatusNotFound, "calendar not found")
				return
			}
			respondError(w, http.StatusInternalServerError, "failed to update calendar visibility")
			return
		}
		cal = updated
	}

	respondJSON(w, http.StatusOK, cal)
}

func (h *Handler) RemoveParticipant(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	calendarID := r.PathValue("id")
	participantID := r.PathValue("participantId")

	existing, err := h.service.GetCalendarOnly(r.Context(), calendarID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to load calendar")
		return
	}
	if existing.OwnerID != user.ID {
		respondError(w, http.StatusForbidden, "only the owner can remove participants")
		return
	}

	if err := h.service.RemoveParticipant(r.Context(), calendarID, participantID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "participant not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to remove participant")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// myCalendarDTO is the per-row shape returned by GET /users/me/calendars.
// Kept separate from model.Calendar so we can add computed fields (my_role,
// participant_count) without polluting the shared calendar type.
type myCalendarDTO struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Emoji            string     `json:"emoji"`
	Visibility       string     `json:"visibility"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	FinalisedDate    *string    `json:"finalised_date"`
	FinalisedAt      *time.Time `json:"finalised_at"`
	ParticipantCount int        `json:"participant_count"`
	MyRole           string     `json:"my_role"`
}

// ListUserCalendars returns the authenticated user's calendars, sorted
// most-recently-active first. Capped server-side; see plans/my-calendars.md.
func (h *Handler) ListUserCalendars(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	items, hasMore, err := h.service.ListUserCalendars(r.Context(), user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list calendars")
		return
	}

	out := make([]myCalendarDTO, len(items))
	for i, item := range items {
		role := "joiner"
		if item.OwnerID == user.ID {
			role = "owner"
		}
		out[i] = myCalendarDTO{
			ID:               item.ID,
			Name:             item.Name,
			Emoji:            item.Emoji,
			Visibility:       item.Visibility,
			CreatedAt:        item.CreatedAt,
			UpdatedAt:        item.UpdatedAt,
			FinalisedDate:    item.FinalisedDate,
			FinalisedAt:      item.FinalisedAt,
			ParticipantCount: item.ParticipantCount,
			MyRole:           role,
		}
	}

	respondJSON(w, http.StatusOK, struct {
		Calendars []myCalendarDTO `json:"calendars"`
		HasMore   bool            `json:"has_more"`
	}{Calendars: out, HasMore: hasMore})
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
