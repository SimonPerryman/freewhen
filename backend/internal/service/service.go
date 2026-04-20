package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/freewhen/backend/internal/limits"
	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/realtime"
	"github.com/freewhen/backend/internal/store"
)

// MyCalendarsPageSize caps the number of calendars returned by the
// /users/me/calendars endpoint. See plans/my-calendars.md — we accept a hard
// cap for v1 instead of real pagination.
const MyCalendarsPageSize = 100

// ErrInvalidVisibility is returned when a caller passes an unrecognised
// visibility value to a mutation endpoint.
var ErrInvalidVisibility = errors.New("invalid visibility value")

// Publisher fans realtime events out to subscribers. Satisfied by
// realtime.Hub; a no-op implementation is provided for tests and for
// environments where realtime is disabled.
type Publisher interface {
	Publish(calendarID, eventType string)
}

type noopPublisher struct{}

func (noopPublisher) Publish(string, string) {}

// Store defines the data operations the service needs.
type Store interface {
	CreateCalendar(ctx context.Context, name, emoji, ownerID, visibility string) (model.Calendar, error)
	GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error)
	GetCalendarOnly(ctx context.Context, id string) (model.Calendar, error)
	AddParticipant(ctx context.Context, calendarID, name string, userID *string) (model.Participant, error)
	GetParticipant(ctx context.Context, id string) (model.Participant, error)
	SetAvailability(ctx context.Context, participantID string, dates []string, noteOverrides map[string]string) error
	SetFinalisedDate(ctx context.Context, calendarID, date string) (model.Calendar, error)
	ClearFinalisedDate(ctx context.Context, calendarID string) (model.Calendar, error)
	ClaimParticipant(ctx context.Context, calendarID, participantID, userID string) (model.Participant, error)
	UpdateCalendarName(ctx context.Context, calendarID, name string) (model.Calendar, error)
	UpdateCalendarVisibility(ctx context.Context, calendarID, visibility string) (model.Calendar, error)
	DeleteParticipant(ctx context.Context, calendarID, participantID string) error
	CountCalendarParticipants(ctx context.Context, calendarID string) (int, error)
	CountCalendarsForOwner(ctx context.Context, ownerID string) (int, error)
	ParticipantHasAvailability(ctx context.Context, participantID string) (bool, error)
	TouchCalendar(ctx context.Context, calendarID string) error
	ListCalendarsForUser(ctx context.Context, userID string, limit int) ([]store.CalendarListItem, error)
}

type Service struct {
	store Store
	pub   Publisher
}

func New(s Store, pub Publisher) *Service {
	if pub == nil {
		pub = noopPublisher{}
	}
	return &Service{store: s, pub: pub}
}

func (s *Service) CreateCalendar(ctx context.Context, name, ownerID, visibility string) (model.Calendar, error) {
	if !model.IsValidVisibility(visibility) {
		visibility = model.VisibilityOpen
	}
	count, err := s.store.CountCalendarsForOwner(ctx, ownerID)
	if err != nil {
		return model.Calendar{}, err
	}
	if count >= limits.MaxCalendarsPerUser {
		return model.Calendar{}, store.ErrCalendarLimitReached
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
	count, err := s.store.CountCalendarParticipants(ctx, calendarID)
	if err != nil {
		return model.Participant{}, err
	}
	if count >= limits.MaxParticipantsPerCalendar {
		return model.Participant{}, store.ErrCalendarFull
	}
	p, err := s.store.AddParticipant(ctx, calendarID, name, userID)
	if err != nil {
		return model.Participant{}, err
	}
	s.touch(ctx, calendarID)
	s.pub.Publish(calendarID, realtime.EventParticipantJoined)
	return p, nil
}

func (s *Service) ClaimParticipant(ctx context.Context, calendarID, participantID, userID string) (model.Participant, error) {
	p, err := s.store.ClaimParticipant(ctx, calendarID, participantID, userID)
	if err != nil {
		return model.Participant{}, err
	}
	s.touch(ctx, calendarID)
	s.pub.Publish(calendarID, realtime.EventParticipantClaimed)
	return p, nil
}

func (s *Service) SetAvailability(ctx context.Context, calendarID, participantID string, dates []string, noteOverrides map[string]string) error {
	p, err := s.store.GetParticipant(ctx, participantID)
	if err != nil {
		return err
	}
	if p.CalendarID != calendarID {
		return store.ErrNotFound
	}
	if err := s.store.SetAvailability(ctx, participantID, dates, noteOverrides); err != nil {
		return err
	}
	s.touch(ctx, calendarID)
	s.pub.Publish(calendarID, realtime.EventAvailabilityChanged)
	return nil
}

// touch bumps the parent calendar's updated_at. Failures are logged but not
// propagated — updated_at drift is not worth failing the user's write over.
func (s *Service) touch(ctx context.Context, calendarID string) {
	if err := s.store.TouchCalendar(ctx, calendarID); err != nil && !errors.Is(err, store.ErrNotFound) {
		slog.Default().Warn("touch calendar failed", "calendar_id", calendarID, "error", err)
	}
}

func (s *Service) GetCalendarOnly(ctx context.Context, id string) (model.Calendar, error) {
	return s.store.GetCalendarOnly(ctx, id)
}

func (s *Service) FinaliseCalendar(ctx context.Context, calendarID, date string) (model.Calendar, error) {
	cal, err := s.store.SetFinalisedDate(ctx, calendarID, date)
	if err != nil {
		return model.Calendar{}, err
	}
	s.pub.Publish(calendarID, realtime.EventCalendarFinalised)
	return cal, nil
}

func (s *Service) UnfinaliseCalendar(ctx context.Context, calendarID string) (model.Calendar, error) {
	cal, err := s.store.ClearFinalisedDate(ctx, calendarID)
	if err != nil {
		return model.Calendar{}, err
	}
	s.pub.Publish(calendarID, realtime.EventCalendarUnfinalised)
	return cal, nil
}

func (s *Service) RenameCalendar(ctx context.Context, calendarID, name string) (model.Calendar, error) {
	cal, err := s.store.UpdateCalendarName(ctx, calendarID, name)
	if err != nil {
		return model.Calendar{}, err
	}
	s.pub.Publish(calendarID, realtime.EventCalendarUpdated)
	return cal, nil
}

func (s *Service) UpdateCalendarVisibility(ctx context.Context, calendarID, visibility string) (model.Calendar, error) {
	if !model.IsValidVisibility(visibility) {
		return model.Calendar{}, ErrInvalidVisibility
	}
	cal, err := s.store.UpdateCalendarVisibility(ctx, calendarID, visibility)
	if err != nil {
		return model.Calendar{}, err
	}
	s.pub.Publish(calendarID, realtime.EventCalendarUpdated)
	return cal, nil
}

func (s *Service) RemoveParticipant(ctx context.Context, calendarID, participantID string) error {
	if err := s.store.DeleteParticipant(ctx, calendarID, participantID); err != nil {
		return err
	}
	s.touch(ctx, calendarID)
	s.pub.Publish(calendarID, realtime.EventParticipantRemoved)
	return nil
}

// ListUserCalendars returns the authenticated user's calendars, sorted
// most-recently-active first. Caps the result at MyCalendarsPageSize; hasMore
// reports whether more calendars exist beyond the cap.
func (s *Service) ListUserCalendars(ctx context.Context, userID string) ([]store.CalendarListItem, bool, error) {
	items, err := s.store.ListCalendarsForUser(ctx, userID, MyCalendarsPageSize+1)
	if err != nil {
		return nil, false, err
	}
	if len(items) > MyCalendarsPageSize {
		return items[:MyCalendarsPageSize], true, nil
	}
	return items, false, nil
}
