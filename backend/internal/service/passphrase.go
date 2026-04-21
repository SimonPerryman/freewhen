package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"sync"
	"time"

	"github.com/freewhen/backend/internal/access"
	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/passphrase"
	"github.com/freewhen/backend/internal/realtime"
)

// Passphrase-related errors surfaced to handlers. Handlers map these to HTTP
// statuses — the service keeps status-unaware.
var (
	ErrNotPassphraseProtected = errors.New("calendar is not passphrase protected")
	ErrPassphraseInvalid      = errors.New("invalid passphrase")
	ErrPassphraseRequired     = errors.New("passphrase required")
	ErrRateLimited            = errors.New("rate limited")
)

// unlockRateLimitWindow and unlockRateLimitMax define the per-(calendar, IP)
// sliding-window budget on POST /unlock.
const (
	unlockRateLimitWindow = time.Minute
	unlockRateLimitMax    = 5
)

// unlockRateLimiter is a simple in-memory sliding-window limiter keyed by
// (calendarID, remoteAddr). Fine for a single-instance deployment; swap for
// Redis if we ever need horizontal scaling.
type unlockRateLimiter struct {
	mu      sync.Mutex
	attempt map[string][]time.Time
	now     func() time.Time
}

func newUnlockRateLimiter() *unlockRateLimiter {
	return &unlockRateLimiter{attempt: map[string][]time.Time{}, now: func() time.Time { return time.Now().UTC() }}
}

// allow records an attempt and reports whether it's under the budget.
func (r *unlockRateLimiter) allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	cutoff := now.Add(-unlockRateLimitWindow)

	filtered := r.attempt[key][:0]
	for _, t := range r.attempt[key] {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) >= unlockRateLimitMax {
		r.attempt[key] = filtered
		return false
	}
	r.attempt[key] = append(filtered, now)
	return true
}

// reset clears the attempt history for key. Called on successful unlock so a
// legitimate user isn't locked out by their own fumbled first attempts.
func (r *unlockRateLimiter) reset(key string) {
	r.mu.Lock()
	delete(r.attempt, key)
	r.mu.Unlock()
}

// AccessConfig carries the HMAC secret and token TTL. Wired once at boot.
type AccessConfig struct {
	Secret string
	TTL    time.Duration
}

// UnlockCalendar verifies the submitted passphrase and issues an access token
// on success. Rate-limited per (calendarID, remoteAddr). Returns
// ErrNotPassphraseProtected, ErrPassphraseInvalid, or ErrRateLimited.
func (s *Service) UnlockCalendar(ctx context.Context, calendarID, submitted, remoteAddr string) (string, error) {
	key := calendarID + "|" + remoteAddr
	if !s.unlockLimiter.allow(key) {
		return "", ErrRateLimited
	}

	cal, err := s.store.GetCalendarOnly(ctx, calendarID)
	if err != nil {
		return "", err
	}
	if cal.Passphrase == "" {
		return "", ErrNotPassphraseProtected
	}
	if subtle.ConstantTimeCompare([]byte(submitted), []byte(cal.Passphrase)) != 1 {
		return "", ErrPassphraseInvalid
	}

	s.unlockLimiter.reset(key)
	return access.IssueToken(cal.ID, cal.AccessEpoch, s.accessCfg.Secret, s.accessCfg.TTL), nil
}

// RotateCalendarPassphrase generates a new passphrase, stores it, bumps the
// access_epoch, and returns the plaintext. Callers (handlers) are
// responsible for enforcing the owner check.
func (s *Service) RotateCalendarPassphrase(ctx context.Context, calendarID string) (string, error) {
	p := passphrase.Generate()
	if _, err := s.store.SetCalendarPassphrase(ctx, calendarID, &p); err != nil {
		return "", err
	}
	s.pub.Publish(calendarID, realtime.EventCalendarUpdated)
	return p, nil
}

// ClearCalendarPassphrase nulls the passphrase column. Still bumps
// access_epoch so outstanding tokens die immediately even though the gate is
// now disabled.
func (s *Service) ClearCalendarPassphrase(ctx context.Context, calendarID string) error {
	if _, err := s.store.SetCalendarPassphrase(ctx, calendarID, nil); err != nil {
		return err
	}
	s.pub.Publish(calendarID, realtime.EventCalendarUpdated)
	return nil
}

// AuthorizeAccess checks whether the viewer may access a gated calendar.
// Owner short-circuits the gate; otherwise a valid access token with a
// matching epoch is required. Returns ErrPassphraseRequired when the gate
// is active and the caller lacks valid access. Unprotected calendars always
// authorize.
func (s *Service) AuthorizeAccess(ctx context.Context, cal model.Calendar, viewer Viewer, tokenHeader string) error {
	if cal.Passphrase == "" {
		return nil
	}
	if viewer.UserID != nil && *viewer.UserID == cal.OwnerID {
		return nil
	}
	if tokenHeader == "" {
		return ErrPassphraseRequired
	}
	tokenCal, tokenEpoch, err := access.VerifyToken(tokenHeader, s.accessCfg.Secret)
	if err != nil {
		return ErrPassphraseRequired
	}
	if tokenCal != cal.ID || tokenEpoch != cal.AccessEpoch {
		return ErrPassphraseRequired
	}
	return nil
}

// AuthorizeAccessByID loads the calendar and applies AuthorizeAccess.
// Convenience for handlers that need both.
func (s *Service) AuthorizeAccessByID(ctx context.Context, calendarID string, viewer Viewer, tokenHeader string) (model.Calendar, error) {
	cal, err := s.store.GetCalendarOnly(ctx, calendarID)
	if err != nil {
		return model.Calendar{}, err
	}
	return cal, s.AuthorizeAccess(ctx, cal, viewer, tokenHeader)
}

