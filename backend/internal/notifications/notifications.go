package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/freewhen/backend/internal/model"
)

// Sender is the minimum surface notifications.Service needs from the
// transactional email provider. Implemented by *email.Sender in production
// and a capturing fake in tests.
type Sender interface {
	SendTransactional(to, subject, html string, listUnsubscribe string) error
}

// Store is the data surface notifications needs. Kept narrow so tests can
// stub without dragging in the full *store.Store.
type Store interface {
	GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error)
	GetUserByID(ctx context.Context, userID string) (model.User, error)
	ClearParticipantEmail(ctx context.Context, participantID string) error
	GetCalendarLastNudgeAt(ctx context.Context, calendarID string) (*time.Time, error)
	SetCalendarLastNudgeAt(ctx context.Context, calendarID string, t time.Time) error
}

// Counters reports the per-kind send counts. Lifted via atomics so the
// metrics handler can read snapshots without locking.
type Counters struct {
	sentFinalised        atomic.Uint64
	sentFinalisedOwner   atomic.Uint64
	sentNudge            atomic.Uint64
	failedFinalised      atomic.Uint64
	failedFinalisedOwner atomic.Uint64
	failedNudge          atomic.Uint64
	nudges               atomic.Uint64
	unsubscribes         atomic.Uint64
}

// Snapshot returns the current counter values.
func (c *Counters) Snapshot() map[string]uint64 {
	return map[string]uint64{
		"sent_finalised":         c.sentFinalised.Load(),
		"sent_finalised_owner":   c.sentFinalisedOwner.Load(),
		"sent_nudge":             c.sentNudge.Load(),
		"failed_finalised":       c.failedFinalised.Load(),
		"failed_finalised_owner": c.failedFinalisedOwner.Load(),
		"failed_nudge":           c.failedNudge.Load(),
		"nudges_total":           c.nudges.Load(),
		"unsubscribes_total":     c.unsubscribes.Load(),
	}
}

// NudgeCooldown is the minimum interval between owner-triggered nudges on the
// same calendar. Bounds the spam an owner can produce by sheer button-mashing.
const NudgeCooldown = 6 * time.Hour

// Service wires email send paths for the three transactional kinds and the
// unsubscribe + nudge endpoints. A nil instance means notifications are
// disabled (no secret configured).
type Service struct {
	sender   Sender
	signer   *TokenSigner
	store    Store
	logger   *slog.Logger
	counters *Counters
	baseURL  string
}

// NewService constructs a Service. A nil sender is permitted — sends become
// no-ops (useful in dev / tests). baseURL is the public origin used to build
// the calendar / unsubscribe URLs in email bodies; trailing slashes are
// trimmed.
func NewService(sender Sender, signer *TokenSigner, st Store, logger *slog.Logger, baseURL string) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		sender:   sender,
		signer:   signer,
		store:    st,
		logger:   logger,
		counters: &Counters{},
		baseURL:  strings.TrimRight(baseURL, "/"),
	}
}

// Counters returns the per-kind counter snapshotter for the metrics endpoint.
func (s *Service) Counters() *Counters {
	if s == nil {
		return nil
	}
	return s.counters
}

// Signer returns the token signer (used to verify unsubscribe links).
func (s *Service) Signer() *TokenSigner {
	if s == nil {
		return nil
	}
	return s.signer
}

// ConsumeUnsubscribeToken verifies the supplied token and clears the matching
// participant's email. Idempotent: a second call after the row is already
// cleared still returns nil.
func (s *Service) ConsumeUnsubscribeToken(ctx context.Context, token string) error {
	if s == nil || s.signer == nil {
		return ErrInvalidToken
	}
	pid, err := s.signer.Verify(token)
	if err != nil {
		return err
	}
	if err := s.store.ClearParticipantEmail(ctx, pid); err != nil {
		// "not found" is fine — already unsubscribed (or participant deleted).
		// Don't leak the difference to the caller.
		s.logger.Info("notification_unsubscribe_clear_noop", "participant_id", pid, "error", err)
	}
	s.counters.unsubscribes.Add(1)
	return nil
}

// FinaliseEvent is the payload the calendar service hands to SendFinalised.
// All URL fields are pre-built by the caller (the notifications package
// doesn't know the export route shape).
type FinaliseEvent struct {
	Calendar      model.Calendar
	Participants  []model.Participant
	OwnerEmail    string
	OwnerName     string
	Date          string // YYYY-MM-DD
	FormattedDate string // human-readable, e.g. "Saturday 12 July"
	CalendarURL   string
	GCalURL       string
	ICSURL        string
}

// SendFinalised fans out the finalised emails: one owner-confirmation, plus
// one participant email for every non-owner participant with a non-empty
// email. The function blocks; callers run it in a goroutine if they want
// fire-and-forget semantics.
func (s *Service) SendFinalised(ctx context.Context, ev FinaliseEvent) {
	if s == nil || s.sender == nil {
		return
	}

	ownerEmail := strings.ToLower(strings.TrimSpace(ev.OwnerEmail))

	// Owner confirmation. No List-Unsubscribe header — they pressed Finalise,
	// they didn't opt in to a list.
	if ownerEmail != "" {
		subject := fmt.Sprintf("✅ %s — you picked %s", ev.Calendar.Name, ev.FormattedDate)
		body := renderFinalisedOwner(ev)
		if err := s.sender.SendTransactional(ownerEmail, subject, body, ""); err != nil {
			s.counters.failedFinalisedOwner.Add(1)
			s.logger.Warn("notification_failed", "kind", "finalised_owner", "calendar_id", ev.Calendar.ID, "error", err)
		} else {
			s.counters.sentFinalisedOwner.Add(1)
			s.logger.Info("notification_sent", "kind", "finalised_owner", "calendar_id", ev.Calendar.ID)
		}
	}

	for _, p := range ev.Participants {
		if p.Email == nil || *p.Email == "" {
			continue
		}
		// Treat a participant whose email matches the owner's account email
		// as the owner — avoid double-sending. The owner-confirmation above
		// covers them.
		if ownerEmail != "" && strings.EqualFold(*p.Email, ownerEmail) {
			continue
		}
		token := s.signer.Sign(p.ID)
		unsubscribeURL := s.baseURL + "/unsubscribe?t=" + url.QueryEscape(token)
		listUnsub := "<" + unsubscribeURL + ">"

		subject := fmt.Sprintf("📅 %s — date picked: %s", ev.Calendar.Name, ev.FormattedDate)
		body := renderFinalised(ev, p, unsubscribeURL)
		if err := s.sender.SendTransactional(*p.Email, subject, body, listUnsub); err != nil {
			s.counters.failedFinalised.Add(1)
			s.logger.Warn("notification_failed", "kind", "finalised", "calendar_id", ev.Calendar.ID, "participant_id", p.ID, "error", err)
			continue
		}
		s.counters.sentFinalised.Add(1)
		s.logger.Info("notification_sent", "kind", "finalised", "calendar_id", ev.Calendar.ID, "participant_id", p.ID)
	}
}

// NudgeResult reports how many recipients received the nudge and how many
// were skipped (no email on file). The handler uses skipped to drive the
// "copy a link for the rest" affordance.
type NudgeResult struct {
	Emailed int `json:"emailed"`
	Skipped int `json:"skipped"`
}

// ErrNudgeRateLimited is returned by Nudge when the calendar is within the
// cooldown window. The handler maps it to 429.
var ErrNudgeRateLimited = errors.New("nudge rate limited")

// ErrNotificationsDisabled is returned by Nudge / SendFinalised callers when
// no signer is configured (i.e. NOTIFICATIONS_HMAC_SECRET is unset).
var ErrNotificationsDisabled = errors.New("notifications disabled")

// Nudge sends the nudge email to every participant on the calendar with an
// email on file (other than the owner). Owner-only — caller must have already
// verified ownership.
func (s *Service) Nudge(ctx context.Context, calendarID string, ownerName string) (NudgeResult, error) {
	if s == nil || s.sender == nil || s.signer == nil {
		return NudgeResult{}, ErrNotificationsDisabled
	}

	last, err := s.store.GetCalendarLastNudgeAt(ctx, calendarID)
	if err != nil {
		return NudgeResult{}, err
	}
	if last != nil && time.Since(*last) < NudgeCooldown {
		return NudgeResult{}, ErrNudgeRateLimited
	}

	details, err := s.store.GetCalendar(ctx, calendarID)
	if err != nil {
		return NudgeResult{}, err
	}

	ownerEmail := ""
	if owner, err := s.store.GetUserByID(ctx, details.Calendar.OwnerID); err == nil {
		ownerEmail = strings.ToLower(strings.TrimSpace(owner.Email))
	}

	calURL := s.baseURL + "/calendars/" + calendarID

	result := NudgeResult{}
	for _, p := range details.Participants {
		if p.UserID != nil && *p.UserID == details.Calendar.OwnerID {
			continue
		}
		if p.Email == nil || *p.Email == "" {
			result.Skipped++
			continue
		}
		if ownerEmail != "" && strings.EqualFold(*p.Email, ownerEmail) {
			continue
		}

		token := s.signer.Sign(p.ID)
		unsubscribeURL := s.baseURL + "/unsubscribe?t=" + url.QueryEscape(token)
		listUnsub := "<" + unsubscribeURL + ">"

		subject := fmt.Sprintf("📅 Reminder: %s is waiting on you", details.Calendar.Name)
		body := renderNudge(details.Calendar, p, ownerName, calURL, unsubscribeURL)
		if err := s.sender.SendTransactional(*p.Email, subject, body, listUnsub); err != nil {
			s.counters.failedNudge.Add(1)
			s.logger.Warn("notification_failed", "kind", "nudge", "calendar_id", calendarID, "participant_id", p.ID, "error", err)
			continue
		}
		result.Emailed++
		s.counters.sentNudge.Add(1)
		s.logger.Info("notification_sent", "kind", "nudge", "calendar_id", calendarID, "participant_id", p.ID)
	}

	if err := s.store.SetCalendarLastNudgeAt(ctx, calendarID, time.Now().UTC()); err != nil {
		s.logger.Warn("notification_set_last_nudge_failed", "calendar_id", calendarID, "error", err)
	}
	s.counters.nudges.Add(1)

	return result, nil
}
