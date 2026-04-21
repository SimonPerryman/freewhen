package handler

import (
	"context"
	"net/http"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/service"
	"github.com/freewhen/backend/internal/store"
)

type contextKey struct{}

// UserContextKey is used by the auth middleware to attach the user to the request context.
var UserContextKey = contextKey{}

// CalendarService defines the operations the handler needs from the service layer.
type CalendarService interface {
	CreateCalendar(ctx context.Context, name, ownerID, visibility string) (model.Calendar, error)
	DuplicateCalendar(ctx context.Context, sourceID, newOwnerID, ownerParticipantName, overrideName string, includeParticipants bool) (model.Calendar, model.Participant, error)
	GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error)
	GetCalendarView(ctx context.Context, calendarID string, viewer service.Viewer) (service.CalendarView, error)
	GetCalendarOnly(ctx context.Context, id string) (model.Calendar, error)
	AddParticipant(ctx context.Context, calendarID, name string, userID *string) (model.Participant, error)
	SetAvailability(ctx context.Context, calendarID, participantID string, dates []string, noteOverrides map[string]string, preferredOverrides map[string]bool) error
	FinaliseCalendar(ctx context.Context, calendarID, date string) (model.Calendar, error)
	UnfinaliseCalendar(ctx context.Context, calendarID string) (model.Calendar, error)
	ClaimParticipant(ctx context.Context, calendarID, participantID, userID string) (model.Participant, error)
	RenameCalendar(ctx context.Context, calendarID, name string) (model.Calendar, error)
	UpdateCalendarVisibility(ctx context.Context, calendarID, visibility string) (model.Calendar, error)
	RemoveParticipant(ctx context.Context, calendarID, participantID string) error
	RenameParticipant(ctx context.Context, calendarID, targetParticipantID string, callerUserID *string, callerParticipantID, name string) (model.Participant, error)
	ListUserCalendars(ctx context.Context, userID string) ([]store.CalendarListItem, bool, error)
	UnlockCalendar(ctx context.Context, calendarID, submitted, remoteAddr string) (string, error)
	RotateCalendarPassphrase(ctx context.Context, calendarID string) (string, error)
	ClearCalendarPassphrase(ctx context.Context, calendarID string) error
	AuthorizeAccess(ctx context.Context, cal model.Calendar, viewer service.Viewer, tokenHeader string) error
}

type Handler struct {
	service CalendarService
}

func New(s CalendarService) *Handler {
	return &Handler{service: s}
}

// UserFromContext returns the authenticated user from the request context, or nil.
func UserFromContext(r *http.Request) *model.User {
	u, _ := r.Context().Value(UserContextKey).(*model.User)
	return u
}
