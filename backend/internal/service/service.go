package service

import (
	"context"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/store"
)

type Service struct {
	store *store.Store
}

func New(s *store.Store) *Service {
	return &Service{store: s}
}

func (s *Service) CreateCalendar(ctx context.Context, name string) (model.Calendar, error) {
	return s.store.CreateCalendar(ctx, name)
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
