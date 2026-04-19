package service

import (
	"context"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/store"
)

// Store defines the data operations the service needs.
type Store interface {
	CreateCalendar(ctx context.Context, name, emoji, ownerID string) (model.Calendar, error)
	GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error)
	GetCalendarOnly(ctx context.Context, id string) (model.Calendar, error)
	AddParticipant(ctx context.Context, calendarID, name string) (model.Participant, error)
	GetParticipant(ctx context.Context, id string) (model.Participant, error)
	SetAvailability(ctx context.Context, participantID string, dates []string) error
	SetFinalisedDate(ctx context.Context, calendarID, date string) (model.Calendar, error)
	ClearFinalisedDate(ctx context.Context, calendarID string) (model.Calendar, error)
}

type Service struct {
	store Store
}

func New(s Store) *Service {
	return &Service{store: s}
}

func (s *Service) CreateCalendar(ctx context.Context, name, ownerID string) (model.Calendar, error) {
	return s.store.CreateCalendar(ctx, name, pickRandomEmoji(), ownerID)
}

func (s *Service) GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error) {
	return s.store.GetCalendar(ctx, id)
}

func (s *Service) AddParticipant(ctx context.Context, calendarID, name string) (model.Participant, error) {
	return s.store.AddParticipant(ctx, calendarID, name)
}

func (s *Service) SetAvailability(ctx context.Context, calendarID, participantID string, dates []string) error {
	p, err := s.store.GetParticipant(ctx, participantID)
	if err != nil {
		return err
	}
	if p.CalendarID != calendarID {
		return store.ErrNotFound
	}
	return s.store.SetAvailability(ctx, participantID, dates)
}

func (s *Service) GetCalendarOnly(ctx context.Context, id string) (model.Calendar, error) {
	return s.store.GetCalendarOnly(ctx, id)
}

func (s *Service) FinaliseCalendar(ctx context.Context, calendarID, date string) (model.Calendar, error) {
	return s.store.SetFinalisedDate(ctx, calendarID, date)
}

func (s *Service) UnfinaliseCalendar(ctx context.Context, calendarID string) (model.Calendar, error) {
	return s.store.ClearFinalisedDate(ctx, calendarID)
}
