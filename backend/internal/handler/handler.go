package handler

import (
	"context"
	"net/http"

	"github.com/freewhen/backend/internal/model"
)

type contextKey struct{}

// UserContextKey is used by the auth middleware to attach the user to the request context.
var UserContextKey = contextKey{}

// CalendarService defines the operations the handler needs from the service layer.
type CalendarService interface {
	CreateCalendar(ctx context.Context, name, ownerID string) (model.Calendar, error)
	GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error)
	GetCalendarOnly(ctx context.Context, id string) (model.Calendar, error)
	AddParticipant(ctx context.Context, calendarID, name string) (model.Participant, error)
	SetAvailability(ctx context.Context, calendarID, participantID string, dates []string) error
	FinaliseCalendar(ctx context.Context, calendarID, date string) (model.Calendar, error)
	UnfinaliseCalendar(ctx context.Context, calendarID string) (model.Calendar, error)
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
