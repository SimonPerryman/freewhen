package service

import (
	"context"
	"errors"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/store"
)

// ErrInvalidVisibility is returned when a caller passes an unrecognised
// visibility value to a mutation endpoint.
var ErrInvalidVisibility = errors.New("invalid visibility value")

// Store defines the data operations the service needs.
type Store interface {
	CreateCalendar(ctx context.Context, name, emoji, ownerID, visibility string) (model.Calendar, error)
	GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error)
	GetCalendarOnly(ctx context.Context, id string) (model.Calendar, error)
	AddParticipant(ctx context.Context, calendarID, name string, userID *string) (model.Participant, error)
	GetParticipant(ctx context.Context, id string) (model.Participant, error)
	SetAvailability(ctx context.Context, participantID string, dates []string) error
	SetFinalisedDate(ctx context.Context, calendarID, date string) (model.Calendar, error)
	ClearFinalisedDate(ctx context.Context, calendarID string) (model.Calendar, error)
	ClaimParticipant(ctx context.Context, calendarID, participantID, userID string) (model.Participant, error)
	UpdateCalendarName(ctx context.Context, calendarID, name string) (model.Calendar, error)
	UpdateCalendarVisibility(ctx context.Context, calendarID, visibility string) (model.Calendar, error)
	DeleteParticipant(ctx context.Context, calendarID, participantID string) error
	CountCalendarParticipants(ctx context.Context, calendarID string) (int, error)
	ParticipantHasAvailability(ctx context.Context, participantID string) (bool, error)
}

type Service struct {
	store Store
}

func New(s Store) *Service {
	return &Service{store: s}
}

func (s *Service) CreateCalendar(ctx context.Context, name, ownerID, visibility string) (model.Calendar, error) {
	if !model.IsValidVisibility(visibility) {
		visibility = model.VisibilityOpen
	}
	return s.store.CreateCalendar(ctx, name, pickRandomEmoji(), ownerID, visibility)
}

func (s *Service) GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error) {
	return s.store.GetCalendar(ctx, id)
}

// Viewer identifies a caller of GetCalendarView. UserID is set for authenticated
// users; ParticipantID is the anonymous-participant identifier from ?as=.
type Viewer struct {
	UserID        *string
	ParticipantID *string
}

// CalendarView is the hide-aware response shape. When Revealed is true, the
// Participants/Availability slices are populated and the full view is visible.
// When Revealed is false, only ParticipantCount is meaningful on the details.
type CalendarView struct {
	Details  model.CalendarWithDetails
	Revealed bool
}

// GetCalendarView returns the calendar with visibility rules applied. Owners
// and participants who have submitted at least one availability row always see
// the full view. Open-visibility calendars always reveal.
func (s *Service) GetCalendarView(ctx context.Context, calendarID string, viewer Viewer) (CalendarView, error) {
	details, err := s.store.GetCalendar(ctx, calendarID)
	if err != nil {
		return CalendarView{}, err
	}

	reveal, err := s.shouldReveal(ctx, details, viewer)
	if err != nil {
		return CalendarView{}, err
	}

	if reveal {
		return CalendarView{Details: details, Revealed: true}, nil
	}

	hidden := model.CalendarWithDetails{
		Calendar:         details.Calendar,
		Participants:     []model.Participant{},
		Availability:     []model.Availability{},
		ParticipantCount: details.ParticipantCount,
	}
	return CalendarView{Details: hidden, Revealed: false}, nil
}

func (s *Service) shouldReveal(ctx context.Context, details model.CalendarWithDetails, viewer Viewer) (bool, error) {
	if details.Calendar.Visibility == model.VisibilityOpen {
		return true, nil
	}
	if viewer.UserID != nil && *viewer.UserID == details.Calendar.OwnerID {
		return true, nil
	}

	if viewer.UserID != nil {
		for _, p := range details.Participants {
			if p.UserID != nil && *p.UserID == *viewer.UserID {
				has, err := s.store.ParticipantHasAvailability(ctx, p.ID)
				if err != nil {
					return false, err
				}
				if has {
					return true, nil
				}
			}
		}
	}

	if viewer.ParticipantID != nil {
		for _, p := range details.Participants {
			if p.ID == *viewer.ParticipantID {
				has, err := s.store.ParticipantHasAvailability(ctx, p.ID)
				if err != nil {
					return false, err
				}
				if has {
					return true, nil
				}
				break
			}
		}
	}

	return false, nil
}

func (s *Service) AddParticipant(ctx context.Context, calendarID, name string, userID *string) (model.Participant, error) {
	return s.store.AddParticipant(ctx, calendarID, name, userID)
}

func (s *Service) ClaimParticipant(ctx context.Context, calendarID, participantID, userID string) (model.Participant, error) {
	return s.store.ClaimParticipant(ctx, calendarID, participantID, userID)
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

func (s *Service) RenameCalendar(ctx context.Context, calendarID, name string) (model.Calendar, error) {
	return s.store.UpdateCalendarName(ctx, calendarID, name)
}

func (s *Service) UpdateCalendarVisibility(ctx context.Context, calendarID, visibility string) (model.Calendar, error) {
	if !model.IsValidVisibility(visibility) {
		return model.Calendar{}, ErrInvalidVisibility
	}
	return s.store.UpdateCalendarVisibility(ctx, calendarID, visibility)
}

func (s *Service) RemoveParticipant(ctx context.Context, calendarID, participantID string) error {
	return s.store.DeleteParticipant(ctx, calendarID, participantID)
}
