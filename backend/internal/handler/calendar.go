package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/freewhen/backend/internal/limits"
	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/service"
)

func (h *Handler) CreateCalendar(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if len(req.Name) > limits.MaxCalendarNameLength {
		respondError(w, http.StatusBadRequest, "name_too_long", "calendar name is too long")
		return
	}

	cal, err := h.service.CreateCalendar(r.Context(), req.Name, user.ID)
	if err != nil {
		if errors.Is(err, model.ErrCalendarLimitReached) {
			respondError(w, http.StatusConflict, "calendar_limit_reached", "you have reached the calendar limit")
			return
		}
		respondInternalError(r, w, err, "internal_error", "failed to create calendar")
		return
	}

	// Auto-join the owner as a participant
	// Owner auto-join is user-linked, so no participant token is issued.
	participant, _, err := h.service.AddParticipant(r.Context(), cal.ID, user.Name, &user.ID, nil)
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
// Creates a new calendar derived from the source: same emoji and (by default)
// participants. Availability, finalised state, and passphrase are
// intentionally dropped.
func (h *Handler) DuplicateCalendar(w http.ResponseWriter, r *http.Request) {
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
	if r.ContentLength != 0 && !decodeJSON(w, r, &req) {
		return
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

	cal, participant, err := h.service.DuplicateCalendar(r.Context(), id, user.ID, user.Name, overrideName, includeParticipants)
	if err != nil {
		if respondOwnerError(r, w, h.service, id, err, "only the owner can duplicate this calendar") {
			return
		}
		switch {
		case errors.Is(err, model.ErrCalendarLimitReached):
			respondError(w, http.StatusConflict, "calendar_limit_reached", "you have reached the calendar limit")
		case errors.Is(err, model.ErrCalendarFull):
			respondError(w, http.StatusConflict, "calendar_full", "calendar is full")
		case errors.Is(err, service.ErrInvalidName):
			respondError(w, http.StatusBadRequest, "name_too_long", "calendar name is too long")
		case errors.Is(err, model.ErrNotFound):
			respondCalendarMissing(r, w, h.service, id)
		default:
			respondInternalError(r, w, err, "internal_error", "failed to duplicate calendar")
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

// ParticipantTokenHeader carries the per-participant secret. Clients send it
// on participant-scoped writes; the server sets it only on the anonymous join
// response, the one place a token is issued.
const ParticipantTokenHeader = "X-Participant-Token"

// participantCaller builds the caller identity for a participant-scoped
// write from the session user and the participant-token header.
func participantCaller(r *http.Request) service.ParticipantCaller {
	c := service.ParticipantCaller{Token: r.Header.Get(ParticipantTokenHeader)}
	if u := UserFromContext(r); u != nil {
		uid := u.ID
		c.UserID = &uid
	}
	return c
}

// setJoinParticipantToken hands an anonymous joiner their freshly issued
// participant token. No-op for "" (signed-in joiners get no token).
func setJoinParticipantToken(w http.ResponseWriter, token string) {
	if token != "" {
		w.Header().Set(ParticipantTokenHeader, token)
	}
}

// calendarResponseDTO wraps CalendarWithDetails so we can inject
// passphrase-specific fields that don't belong on the shared model. The
// embedded Calendar is replaced with a DTO that adds has_passphrase /
// passphrase (owner only).
type calendarResponseDTO struct {
	Calendar     calendarDTO          `json:"calendar"`
	Participants []model.Participant  `json:"participants"`
	Availability []model.Availability `json:"availability"`
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

	cal, err := h.service.GetCalendarOnly(r.Context(), id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return
		}
		respondInternalError(r, w, err, "internal_error", "failed to get calendar")
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
		respondInternalError(r, w, err, "internal_error", "failed to authorize access")
		return
	}

	details, err := h.service.GetCalendar(r.Context(), id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return
		}
		respondInternalError(r, w, err, "internal_error", "failed to get calendar")
		return
	}

	isOwner := user != nil && user.ID == cal.OwnerID
	// Strip participant emails before serialising — this endpoint is read by
	// every viewer of the calendar, and the email is private metadata that
	// should never leak to other participants. Participants edit their own
	// email by re-submitting it through PATCH /participants/{id}.
	scrubbed := make([]model.Participant, len(details.Participants))
	for i, p := range details.Participants {
		p.Email = nil
		scrubbed[i] = p
	}
	resp := calendarResponseDTO{
		Calendar:     toCalendarDTO(details.Calendar, isOwner),
		Participants: scrubbed,
		Availability: details.Availability,
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *Handler) FinaliseCalendar(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	id := r.PathValue("id")

	var req struct {
		Date string `json:"date"`
	}
	if !decodeJSON(w, r, &req) {
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

	cal, err := h.service.FinaliseCalendar(r.Context(), id, user.ID, req.Date)
	if err != nil {
		if respondOwnerError(r, w, h.service, id, err, "only the owner can finalise this calendar") {
			return
		}
		if errors.Is(err, model.ErrNotFound) {
			respondCalendarMissing(r, w, h.service, id)
			return
		}
		respondInternalError(r, w, err, "internal_error", "failed to finalise calendar")
		return
	}

	respondJSON(w, http.StatusOK, cal)
}

// UpdateCalendar handles PATCH /calendars/{id}. Owner only.
func (h *Handler) UpdateCalendar(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	id := r.PathValue("id")

	var req struct {
		Name *string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "no updatable fields provided")
		return
	}
	if strings.TrimSpace(*req.Name) == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "name is required")
		return
	}
	if len(*req.Name) > limits.MaxCalendarNameLength {
		respondError(w, http.StatusBadRequest, "name_too_long", "calendar name is too long")
		return
	}

	cal, err := h.service.RenameCalendar(r.Context(), id, user.ID, *req.Name)
	if err != nil {
		if respondOwnerError(r, w, h.service, id, err, "only the owner can update this calendar") {
			return
		}
		if errors.Is(err, model.ErrNotFound) {
			respondCalendarMissing(r, w, h.service, id)
			return
		}
		respondInternalError(r, w, err, "internal_error", "failed to rename calendar")
		return
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

	if err := h.service.RemoveParticipant(r.Context(), calendarID, user.ID, participantID); err != nil {
		if respondOwnerError(r, w, h.service, calendarID, err, "only the owner can remove participants") {
			return
		}
		if errors.Is(err, model.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "participant not found")
			return
		}
		respondInternalError(r, w, err, "internal_error", "failed to remove participant")
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
		respondInternalError(r, w, err, "internal_error", "failed to list calendars")
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

// DeleteCalendar handles DELETE /calendars/{id}. Owner only. Soft-deletes
// the calendar and is idempotent: deleting an already-deleted calendar
// returns 204 (and the helper-mapped 410 only fires for non-owners hitting
// a soft-deleted calendar).
func (h *Handler) DeleteCalendar(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	id := r.PathValue("id")

	if err := h.service.DeleteCalendar(r.Context(), id, user.ID); err != nil {
		switch {
		case errors.Is(err, service.ErrForbidden):
			respondError(w, http.StatusForbidden, "forbidden", "only the owner can delete this calendar")
		case errors.Is(err, model.ErrNotFound):
			respondCalendarMissing(r, w, h.service, id)
		default:
			respondInternalError(r, w, err, "internal_error", "failed to delete calendar")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// LeaveCalendar handles DELETE /calendars/{id}/participants/me. Soft-
// deletes every live participant row for (calendarID, currentUserID).
// Idempotent. Owners cannot leave (must Delete) — returns 409.
func (h *Handler) LeaveCalendar(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	id := r.PathValue("id")

	if err := h.service.LeaveCalendar(r.Context(), id, user.ID); err != nil {
		switch {
		case errors.Is(err, model.ErrCannotLeaveOwnedCalendar):
			respondError(w, http.StatusConflict, "cannot_leave_owned_calendar", "owners cannot leave their own calendar; delete it instead")
		case errors.Is(err, model.ErrNotFound):
			respondCalendarMissing(r, w, h.service, id)
		default:
			respondInternalError(r, w, err, "internal_error", "failed to leave calendar")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UnfinaliseCalendar(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	id := r.PathValue("id")

	cal, err := h.service.UnfinaliseCalendar(r.Context(), id, user.ID)
	if err != nil {
		if respondOwnerError(r, w, h.service, id, err, "only the owner can unfinalise this calendar") {
			return
		}
		if errors.Is(err, model.ErrNotFound) {
			respondCalendarMissing(r, w, h.service, id)
			return
		}
		respondInternalError(r, w, err, "internal_error", "failed to unfinalise calendar")
		return
	}

	respondJSON(w, http.StatusOK, cal)
}
