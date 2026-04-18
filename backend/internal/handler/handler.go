package handler

import (
	"context"

	"github.com/freewhen/backend/internal/model"
)

// CalendarService defines the operations the handler needs from the service layer.
type CalendarService interface {
	CreateCalendar(ctx context.Context, name string) (model.Calendar, error)
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
