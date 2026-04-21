package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/freewhen/backend/internal/limits"
	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/service"
	"github.com/freewhen/backend/internal/store"
)

func (h *Handler) CreateCalendar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	var req struct {
		Name       string `json:"name"`
		Visibility string `json:"visibility"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_json", "invalid JSON")
		return
	}

	if len(req.Name) > limits.MaxCalendarNameLength {
		respondError(w, http.StatusBadRequest, "name_too_long", "calendar name is too long")
		return
	}

	visibility := req.Visibility
	if visibility == "" {
		visibility = model.VisibilityOpen
	}
	if !model.IsValidVisibility(visibility) {
		respondError(w, http.StatusBadRequest, "invalid_visibility", "invalid visibility value")
		return
	}

	cal, err := h.service.CreateCalendar(r.Context(), req.Name, user.ID, visibility)
	if err != nil {
		if errors.Is(err, store.ErrCalendarLimitReached) {
			respondError(w, http.StatusConflict, "calendar_limit_reached", "you have reached the calendar limit")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to create calendar")
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

// DuplicateCalendar handles POST /calendars/{id}/duplicate. Owner-only.
// Creates a new calendar derived from the source: same emoji, visibility,
// and (by default) participants. Availability, finalised state, and
// passphrase are intentionally dropped.
func (h *Handler) DuplicateCalendar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	id := r.PathValue("id")

	// Missing body is OK — duplicate-with-defaults is the happy path.
	req := struct {
		Name                *string `json:"name"`
		IncludeParticipants *bool   `json:"include_participants"`
	}{}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid_json", "invalid JSON")
			return
		}
	}

	overrideName := ""
	if req.Name != nil {
		overrideName = *req.Name
	}
	if len(overrideName) > limits.MaxCalendarNameLength {
		respondError(w, http.StatusBadRequest, "name_too_long", "calendar name is too long")
		return
	}

	includeParticipants := true
	if req.IncludeParticipants != nil {
		includeParticipants = *req.IncludeParticipants
	}

	source, err := h.service.GetCalendarOnly(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to load calendar")
		return
	}
	if source.OwnerID != user.ID {
		respondError(w, http.StatusForbidden, "forbidden", "only the owner can duplicate this calendar")
		return
	}

	cal, participant, err := h.service.DuplicateCalendar(r.Context(), id, user.ID, user.Name, overrideName, includeParticipants)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrCalendarLimitReached):
			respondError(w, http.StatusConflict, "calendar_limit_reached", "you have reached the calendar limit")
		case errors.Is(err, store.ErrCalendarFull):
			respondError(w, http.StatusConflict, "calendar_full", "calendar is full")
		case errors.Is(err, service.ErrInvalidName):
			respondError(w, http.StatusBadRequest, "name_too_long", "calendar name is too long")
		case errors.Is(err, store.ErrNotFound):
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
		default:
			respondError(w, http.StatusInternalServerError, "internal_error", "failed to duplicate calendar")
		}
		return
	}

	respondJSON(w, http.StatusCreated, struct {
		model.Calendar
		Participant *model.Participant `json:"participant,omitempty"`
	}{Calendar: cal, Participant: &participant})
}

// AccessHeader is the HTTP header carrying a calendar access token issued by
// POST /calendars/{id}/unlock.
const AccessHeader = "X-Calendar-Access"

// calendarResponseDTO wraps CalendarWithDetails so we can inject
// passphrase-specific fields that don't belong on the shared model. The
// embedded Calendar is replaced with a DTO that adds has_passphrase /
// passphrase (owner only).
type calendarResponseDTO struct {
	Calendar         calendarDTO          `json:"calendar"`
	Participants     []model.Participant  `json:"participants"`
	Availability     []model.Availability `json:"availability"`
	ParticipantCount int                  `json:"participantCount"`
}

type calendarDTO struct {
	model.Calendar
	HasPassphrase bool   `json:"has_passphrase"`
	Passphrase    string `json:"passphrase,omitempty"`
}

func toCalendarDTO(cal model.Calendar, includePassphrase bool) calendarDTO {
	out := calendarDTO{Calendar: cal, HasPassphrase: cal.HasPassphrase()}
	if includePassphrase {
		out.Passphrase = cal.Passphrase
	}
	return out
}

func (h *Handler) GetCalendar(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	viewer := service.Viewer{}
	user := UserFromContext(r)
	if user != nil {
		uid := user.ID
		viewer.UserID = &uid
	}
	if as := strings.TrimSpace(r.URL.Query().Get("as")); as != "" {
		viewer.ParticipantID = &as
	}

	cal, err := h.service.GetCalendarOnly(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to get calendar")
		return
	}

	if err := h.service.AuthorizeAccess(r.Context(), cal, viewer, r.Header.Get(AccessHeader)); err != nil {
		if errors.Is(err, service.ErrPassphraseRequired) {
			respondJSON(w, http.StatusOK, map[string]any{
				"calendar":            map[string]any{"id": cal.ID, "has_passphrase": true},
				"requires_passphrase": true,
			})
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to authorize access")
		return
	}

	view, err := h.service.GetCalendarView(r.Context(), id, viewer)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to get calendar")
		return
	}

	isOwner := user != nil && user.ID == cal.OwnerID
	resp := calendarResponseDTO{
		Calendar:         toCalendarDTO(view.Details.Calendar, isOwner),
		Participants:     view.Details.Participants,
		Availability:     view.Details.Availability,
		ParticipantCount: view.Details.ParticipantCount,
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *Handler) FinaliseCalendar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	id := r.PathValue("id")

	var req struct {
		Date string `json:"date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_json", "invalid JSON")
		return
	}
	parsedDate, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_date", "invalid date format")
		return
	}
	if !limits.DateWithinRange(parsedDate, time.Now().UTC()) {
		respondError(w, http.StatusBadRequest, "date_out_of_range", "date out of range: "+req.Date)
		return
	}

	existing, err := h.service.GetCalendarOnly(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to load calendar")
		return
	}
	if existing.OwnerID != user.ID {
		respondError(w, http.StatusForbidden, "forbidden", "only the owner can finalise this calendar")
		return
	}

	cal, err := h.service.FinaliseCalendar(r.Context(), id, req.Date)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to finalise calendar")
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
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	id := r.PathValue("id")

	var req struct {
		Name       *string `json:"name"`
		Visibility *string `json:"visibility"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_json", "invalid JSON")
		return
	}
	if req.Name == nil && req.Visibility == nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "no updatable fields provided")
		return
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "name is required")
		return
	}
	if req.Name != nil && len(*req.Name) > limits.MaxCalendarNameLength {
		respondError(w, http.StatusBadRequest, "name_too_long", "calendar name is too long")
		return
	}
	if req.Visibility != nil && !model.IsValidVisibility(*req.Visibility) {
		respondError(w, http.StatusBadRequest, "invalid_visibility", "invalid visibility value")
		return
	}

	existing, err := h.service.GetCalendarOnly(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to load calendar")
		return
	}
	if existing.OwnerID != user.ID {
		respondError(w, http.StatusForbidden, "forbidden", "only the owner can update this calendar")
		return
	}

	cal := existing
	if req.Name != nil {
		updated, err := h.service.RenameCalendar(r.Context(), id, *req.Name)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				respondError(w, http.StatusNotFound, "not_found", "calendar not found")
				return
			}
			respondError(w, http.StatusInternalServerError, "internal_error", "failed to rename calendar")
			return
		}
		cal = updated
	}
	if req.Visibility != nil {
		updated, err := h.service.UpdateCalendarVisibility(r.Context(), id, *req.Visibility)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				respondError(w, http.StatusNotFound, "not_found", "calendar not found")
				return
			}
			respondError(w, http.StatusInternalServerError, "internal_error", "failed to update calendar visibility")
			return
		}
		cal = updated
	}

	respondJSON(w, http.StatusOK, cal)
}

func (h *Handler) RemoveParticipant(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	calendarID := r.PathValue("id")
	participantID := r.PathValue("participantId")

	existing, err := h.service.GetCalendarOnly(r.Context(), calendarID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to load calendar")
		return
	}
	if existing.OwnerID != user.ID {
		respondError(w, http.StatusForbidden, "forbidden", "only the owner can remove participants")
		return
	}

	if err := h.service.RemoveParticipant(r.Context(), calendarID, participantID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "participant not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to remove participant")
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
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	items, hasMore, err := h.service.ListUserCalendars(r.Context(), user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to list calendars")
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
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	id := r.PathValue("id")

	existing, err := h.service.GetCalendarOnly(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to load calendar")
		return
	}
	if existing.OwnerID != user.ID {
		respondError(w, http.StatusForbidden, "forbidden", "only the owner can unfinalise this calendar")
		return
	}

	cal, err := h.service.UnfinaliseCalendar(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to unfinalise calendar")
		return
	}

	respondJSON(w, http.StatusOK, cal)
}
