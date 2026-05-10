package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/freewhen/backend/internal/limits"
	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/notifications"
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

// ErrInvalidEmail is returned when a participant email submission is malformed
// (e.g. missing @). Empty strings clear the email rather than failing.
var ErrInvalidEmail = errors.New("invalid participant email")

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
	GetUser(ctx context.Context, id string) (model.User, error)
	AddParticipant(ctx context.Context, calendarID, name string, userID *string, email *string) (model.Participant, error)
	UpdateParticipantEmail(ctx context.Context, calendarID, participantID string, email *string) (model.Participant, error)
	ClearParticipantEmailByID(ctx context.Context, participantID string) error
	GetParticipant(ctx context.Context, id string) (model.Participant, error)
	SetAvailability(ctx context.Context, participantID string, dates []string, noteOverrides map[string]string, preferredOverrides map[string]bool) error
	MergeAvailabilityDates(ctx context.Context, participantID string, dates []string) error
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
	SoftDeleteCalendar(ctx context.Context, calendarID string) error
	SoftDeleteParticipantsForUser(ctx context.Context, calendarID, userID string) (int64, error)
	IsCalendarSoftDeleted(ctx context.Context, calendarID string) (bool, error)
}

type Service struct {
	store         Store
	pub           Publisher
	accessCfg     AccessConfig
	unlockLimiter *unlockRateLimiter
	notif         *notifications.Service
	baseURL       string
}

// WithNotifications wires the notifications.Service used to fan out
// transactional emails on finalise. Nil disables sends (the default).
// baseURL is read from the notifications.Service when present.
func (s *Service) WithNotifications(n *notifications.Service) *Service {
	s.notif = n
	return s
}

// WithBaseURL sets the public origin used to build calendar / export URLs in
// notification email bodies. Empty string disables URL substitution.
func (s *Service) WithBaseURL(u string) *Service {
	s.baseURL = strings.TrimRight(u, "/")
	return s
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

func (s *Service) AddParticipant(ctx context.Context, calendarID, name string, userID *string, email *string) (model.Participant, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Participant{}, ErrInvalidName
	}
	if len(name) > limits.MaxParticipantNameLength {
		return model.Participant{}, ErrInvalidName
	}
	if email != nil && *email != "" && !strings.Contains(*email, "@") {
		return model.Participant{}, ErrInvalidEmail
	}
	count, err := s.store.CountCalendarParticipants(ctx, calendarID)
	if err != nil {
		return model.Participant{}, err
	}
	if count >= limits.MaxParticipantsPerCalendar {
		return model.Participant{}, store.ErrCalendarFull
	}
	p, err := s.store.AddParticipant(ctx, calendarID, name, userID, email)
	if err != nil {
		return model.Participant{}, err
	}
	s.touch(ctx, calendarID)
	s.pub.Publish(calendarID, realtime.EventParticipantJoined)
	return p, nil
}

// UpdateParticipantEmail sets or clears the participant's contact email.
// Authorization mirrors RenameParticipant: owner, self (via callerParticipantID
// matching), or signed-in user matching the participant's user_id.
func (s *Service) UpdateParticipantEmail(ctx context.Context, calendarID, targetParticipantID string, callerUserID *string, callerParticipantID string, email *string) (model.Participant, error) {
	cal, err := s.store.GetCalendarOnly(ctx, calendarID)
	if err != nil {
		return model.Participant{}, err
	}

	allowed := callerUserID != nil && *callerUserID == cal.OwnerID
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

	if email != nil && *email != "" && !strings.Contains(*email, "@") {
		return model.Participant{}, ErrInvalidEmail
	}

	p, err := s.store.UpdateParticipantEmail(ctx, calendarID, targetParticipantID, email)
	if err != nil {
		return model.Participant{}, err
	}
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

// ImportAvailability merges calendar-imported dates into the participant's
// existing availability without deleting any hand-entered rows. Used by the
// "Accept all suggestions" shortcut.
func (s *Service) ImportAvailability(ctx context.Context, calendarID, participantID string, dates []string) error {
	p, err := s.store.GetParticipant(ctx, participantID)
	if err != nil {
		return err
	}
	if p.CalendarID != calendarID {
		return store.ErrNotFound
	}
	if err := s.store.MergeAvailabilityDates(ctx, participantID, dates); err != nil {
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
	s.fanOutFinalised(ctx, cal, date)
	return cal, nil
}

// fanOutFinalised builds the FinaliseEvent payload (participants, owner,
// formatted date, URLs) and dispatches finalised emails in a goroutine. No-ops
// if notifications are disabled or required dependencies are missing.
func (s *Service) fanOutFinalised(ctx context.Context, cal model.Calendar, date string) {
	if s.notif == nil || s.baseURL == "" {
		return
	}
	details, err := s.store.GetCalendar(context.WithoutCancel(ctx), cal.ID)
	if err != nil {
		slog.Default().Warn("finalise fanout: load calendar failed", "calendar_id", cal.ID, "error", err)
		return
	}
	owner, err := s.store.GetUser(context.WithoutCancel(ctx), cal.OwnerID)
	if err != nil {
		slog.Default().Warn("finalise fanout: load owner failed", "calendar_id", cal.ID, "error", err)
		return
	}

	formatted := formatDate(date)
	calURL := s.baseURL + "/calendars/" + cal.ID
	icsURL := s.baseURL + "/api/calendars/" + cal.ID + "/export.ics"
	gcalURL := s.baseURL + "/api/calendars/" + cal.ID + "/export/google"
	ev := notifications.FinaliseEvent{
		Calendar:      cal,
		Participants:  append([]model.Participant(nil), details.Participants...),
		OwnerEmail:    owner.Email,
		OwnerName:     owner.Name,
		Date:          date,
		FormattedDate: formatted,
		CalendarURL:   calURL,
		ICSURL:        icsURL,
		GCalURL:       gcalURL,
	}
	go s.notif.SendFinalised(context.Background(), ev)
}

// formatDate renders YYYY-MM-DD as e.g. "Saturday 12 July 2026".
func formatDate(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.Format("Monday 2 January 2006")
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

	allowed := callerUserID != nil && *callerUserID == cal.OwnerID
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

// IsCalendarSoftDeleted reports whether a calendar id refers to a row that
// has been soft-deleted. Used by mutating handlers to map ErrNotFound to
// 410 Gone (vs the default 404).
func (s *Service) IsCalendarSoftDeleted(ctx context.Context, calendarID string) (bool, error) {
	return s.store.IsCalendarSoftDeleted(ctx, calendarID)
}

// DeleteCalendar soft-deletes a calendar after verifying that the caller
// is the current owner. Idempotent — deleting an already-deleted calendar
// returns nil. Returns ErrForbidden if the caller is not the owner, or
// ErrNotFound if the calendar never existed.
func (s *Service) DeleteCalendar(ctx context.Context, calendarID, userID string) error {
	cal, err := s.store.GetCalendarOnly(ctx, calendarID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Distinguish 404 (never existed) from 410 (was deleted).
			soft, sErr := s.store.IsCalendarSoftDeleted(ctx, calendarID)
			if sErr == nil && soft {
				// Already deleted — idempotent success.
				return nil
			}
		}
		return err
	}
	if cal.OwnerID != userID {
		return ErrForbidden
	}
	if err := s.store.SoftDeleteCalendar(ctx, calendarID); err != nil {
		return err
	}
	s.pub.Publish(calendarID, realtime.EventCalendarUpdated)
	return nil
}

// LeaveCalendar soft-deletes every live participant row for (calendarID,
// userID). Idempotent. Owners cannot leave their own calendar — they must
// Delete instead — and get back ErrCannotLeaveOwnedCalendar in that case.
func (s *Service) LeaveCalendar(ctx context.Context, calendarID, userID string) error {
	cal, err := s.store.GetCalendarOnly(ctx, calendarID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			soft, sErr := s.store.IsCalendarSoftDeleted(ctx, calendarID)
			if sErr == nil && soft {
				// Calendar is gone — leaving is trivially satisfied.
				return nil
			}
		}
		return err
	}
	if cal.OwnerID == userID {
		return store.ErrCannotLeaveOwnedCalendar
	}
	rows, err := s.store.SoftDeleteParticipantsForUser(ctx, calendarID, userID)
	if err != nil {
		return err
	}
	if rows > 0 {
		s.touch(ctx, calendarID)
		s.pub.Publish(calendarID, realtime.EventParticipantRemoved)
	}
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
