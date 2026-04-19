package handler

import (
	"context"
	"net/http"

	"github.com/freewhen/backend/internal/model"
)

// UserContextKey is used by the auth middleware to attach the user to the request context.
var UserContextKey = struct{}{}

// CalendarService defines the operations the handler needs from the service layer.
type CalendarService interface {
	CreateCalendar(ctx context.Context, name, ownerID string) (model.Calendar, error)
	GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error)
	AddParticipant(ctx context.Context, calendarID, name string) (model.Participant, error)
	SetAvailability(ctx context.Context, calendarID, participantID string, dates []string) error
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
