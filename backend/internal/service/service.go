package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/freewhen/backend/internal/limits"
	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/realtime"
	"github.com/freewhen/backend/internal/store"
)

// MyCalendarsPageSize caps the number of calendars returned by the
// /users/me/calendars endpoint. See plans/my-calendars.md — we accept a hard
// cap for v1 instead of real pagination.
const MyCalendarsPageSize = 100

// ErrForbidden is returned when the caller is not permitted to perform the
// requested mutation (e.g. trying to rename a participant that isn't theirs
// on a calendar they don't own).
var ErrForbidden = errors.New("forbidden")

// ErrInvalidName is returned when a rename submission is empty or too long.
var ErrInvalidName = errors.New("invalid participant name")

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
	CreateCalendar(ctx context.Context, name, emoji, ownerID string) (model.Calendar, error)
	DuplicateCalendar(ctx context.Context, sourceID, newOwnerID, finalName, ownerParticipantName string, includeParticipants bool) (model.Calendar, model.Participant, error)
	GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error)
	GetCalendarOnly(ctx context.Context, id string) (model.Calendar, error)
	AddParticipant(ctx context.Context, calendarID, name string, userID *string) (model.Participant, error)
	GetParticipant(ctx context.Context, id string) (model.Participant, error)
	SetAvailability(ctx context.Context, participantID string, dates []string, noteOverrides map[string]string, preferredOverrides map[string]bool) error
	SetFinalisedDate(ctx context.Context, calendarID, date string) (model.Calendar, error)
	ClearFinalisedDate(ctx context.Context, calendarID string) (model.Calendar, error)
	ClaimParticipant(ctx context.Context, calendarID, participantID, userID string) (model.Participant, error)
	UpdateCalendarName(ctx context.Context, calendarID, name string) (model.Calendar, error)
	SetCalendarPassphrase(ctx context.Context, calendarID string, passphrase *string) (model.Calendar, error)
	DeleteParticipant(ctx context.Context, calendarID, participantID string) error
	UpdateParticipantName(ctx context.Context, calendarID, participantID, name string) (model.Participant, error)
	CountCalendarParticipants(ctx context.Context, calendarID string) (int, error)
	CountCalendarsForOwner(ctx context.Context, ownerID string) (int, error)
	TouchCalendar(ctx context.Context, calendarID string) error
	ListCalendarsForUser(ctx context.Context, userID string, limit int) ([]store.CalendarListItem, error)
	ListInvites(ctx context.Context, calendarID string) ([]model.Invite, error)
	GetInvite(ctx context.Context, inviteID string) (model.Invite, error)
	AddInvites(ctx context.Context, calendarID string, names []string) (store.AddInvitesResult, error)
	UpdateInviteName(ctx context.Context, inviteID, name string) (model.Invite, error)
	DeleteInvite(ctx context.Context, inviteID string) error
	SetInviteMatch(ctx context.Context, inviteID string, participantID *string) (model.Invite, error)
	ParticipantsWithNoAvailability(ctx context.Context, calendarID string) ([]model.Participant, error)
}

type Service struct {
	store         Store
	pub           Publisher
	accessCfg     AccessConfig
	unlockLimiter *unlockRateLimiter
}

func New(s Store, pub Publisher, cfg AccessConfig) *Service {
	if pub == nil {
		pub = noopPublisher{}
	}
	return &Service{
		store:         s,
		pub:           pub,
		accessCfg:     cfg,
		unlockLimiter: newUnlockRateLimiter(),
	}
}

func (s *Service) CreateCalendar(ctx context.Context, name, ownerID string) (model.Calendar, error) {
	count, err := s.store.CountCalendarsForOwner(ctx, ownerID)
	if err != nil {
		return model.Calendar{}, err
	}
	if count >= limits.MaxCalendarsPerUser {
		return model.Calendar{}, store.ErrCalendarLimitReached
	}
	return s.store.CreateCalendar(ctx, name, pickRandomEmoji(), ownerID)
}

// DuplicateCalendar creates a new calendar derived from sourceID. The caller
// must have already verified that newOwnerID is the source's owner — the
// service does not repeat the ownership check because handlers already load
// the source to produce 404 vs 403 distinctions.
//
// overrideName, if non-empty, replaces the default "<source> (copy)" name.
// Names longer than the cap return ErrInvalidName. includeParticipants
// defaults to true at the handler layer; when false, only the owner ends up
// on the new calendar.
func (s *Service) DuplicateCalendar(
	ctx context.Context,
	sourceID, newOwnerID, ownerParticipantName, overrideName string,
	includeParticipants bool,
) (model.Calendar, model.Participant, error) {
	if overrideName != "" {
		if len(overrideName) > limits.MaxCalendarNameLength {
			return model.Calendar{}, model.Participant{}, ErrInvalidName
		}
	}

	count, err := s.store.CountCalendarsForOwner(ctx, newOwnerID)
	if err != nil {
		return model.Calendar{}, model.Participant{}, err
	}
	if count >= limits.MaxCalendarsPerUser {
		return model.Calendar{}, model.Participant{}, store.ErrCalendarLimitReached
	}

	source, err := s.store.GetCalendarOnly(ctx, sourceID)
	if err != nil {
		return model.Calendar{}, model.Participant{}, err
	}

	finalName := strings.TrimSpace(overrideName)
	if finalName == "" {
		finalName = source.Name + " (copy)"
		if len([]rune(finalName)) > limits.MaxCalendarNameLength {
			runes := []rune(finalName)
			finalName = string(runes[:limits.MaxCalendarNameLength])
		}
	}

	if includeParticipants {
		srcCount, err := s.store.CountCalendarParticipants(ctx, sourceID)
		if err != nil {
			return model.Calendar{}, model.Participant{}, err
		}
		// Owner auto-join on the new calendar adds 1; exclude the source owner
		// row from the copy (it folds into the auto-join), so total upper
		// bound is srcCount when the source owner participates, srcCount+1
		// otherwise. Use srcCount+1 as the defensive ceiling.
		if srcCount+1 > limits.MaxParticipantsPerCalendar {
			return model.Calendar{}, model.Participant{}, store.ErrCalendarFull
		}
	}

	cal, ownerP, err := s.store.DuplicateCalendar(ctx, sourceID, newOwnerID, finalName, ownerParticipantName, includeParticipants)
	if err != nil {
		return model.Calendar{}, model.Participant{}, err
	}
	return cal, ownerP, nil
}

func (s *Service) GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error) {
	return s.store.GetCalendar(ctx, id)
}

// Viewer identifies a caller of AuthorizeAccess. UserID is set for
// authenticated users.
type Viewer struct {
	UserID *string
}

func (s *Service) AddParticipant(ctx context.Context, calendarID, name string, userID *string) (model.Participant, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Participant{}, ErrInvalidName
	}
	if len(name) > limits.MaxParticipantNameLength {
		return model.Participant{}, ErrInvalidName
	}
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

func (s *Service) SetAvailability(ctx context.Context, calendarID, participantID string, dates []string, noteOverrides map[string]string, preferredOverrides map[string]bool) error {
	p, err := s.store.GetParticipant(ctx, participantID)
	if err != nil {
		return err
	}
	if p.CalendarID != calendarID {
		return store.ErrNotFound
	}
	if err := s.store.SetAvailability(ctx, participantID, dates, noteOverrides, preferredOverrides); err != nil {
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

// RenameParticipant updates the name of a participant on a calendar.
// Authorization: the caller is allowed if they are the calendar owner, if the
// supplied callerParticipantID matches the target (the honor-system trust
// boundary used by the anon-join availability write path), or if the caller
// is signed in and the target participant is linked to the same user.
func (s *Service) RenameParticipant(ctx context.Context, calendarID, targetParticipantID string, callerUserID *string, callerParticipantID, name string) (model.Participant, error) {
	cal, err := s.store.GetCalendarOnly(ctx, calendarID)
	if err != nil {
		return model.Participant{}, err
	}
	if cal.FinalisedDate != nil {
		return model.Participant{}, store.ErrCalendarFinalised
	}

	allowed := false
	if callerUserID != nil && *callerUserID == cal.OwnerID {
		allowed = true
	}
	if !allowed && callerParticipantID != "" && callerParticipantID == targetParticipantID {
		allowed = true
	}
	if !allowed && callerUserID != nil {
		target, err := s.store.GetParticipant(ctx, targetParticipantID)
		if err != nil {
			return model.Participant{}, err
		}
		if target.CalendarID != calendarID {
			return model.Participant{}, store.ErrNotFound
		}
		if target.UserID != nil && *target.UserID == *callerUserID {
			allowed = true
		}
	}
	if !allowed {
		return model.Participant{}, ErrForbidden
	}

	name = strings.TrimSpace(name)
	if name == "" || len(name) > limits.MaxParticipantNameLength {
		return model.Participant{}, ErrInvalidName
	}

	p, err := s.store.UpdateParticipantName(ctx, calendarID, targetParticipantID, name)
	if err != nil {
		return model.Participant{}, err
	}
	s.touch(ctx, calendarID)
	s.pub.Publish(calendarID, realtime.EventParticipantRenamed)
	return p, nil
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
