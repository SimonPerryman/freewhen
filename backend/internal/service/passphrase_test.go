package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/freewhen/backend/internal/access"
	"github.com/freewhen/backend/internal/model"
)

const testSecret = "test-secret-at-least-32-bytes-long-x"

type passphraseStore struct {
	mockStore
	cal        model.Calendar
	getCalOnly func(ctx context.Context, id string) (model.Calendar, error)
	setCalled  int
	setArg     *string
}

func (p *passphraseStore) GetCalendarOnly(ctx context.Context, id string) (model.Calendar, error) {
	if p.getCalOnly != nil {
		return p.getCalOnly(ctx, id)
	}
	return p.cal, nil
}

func (p *passphraseStore) SetCalendarPassphrase(_ context.Context, _ string, pp *string) (model.Calendar, error) {
	p.setCalled++
	p.setArg = pp
	p.cal.AccessEpoch++
	if pp == nil {
		p.cal.Passphrase = ""
	} else {
		p.cal.Passphrase = *pp
	}
	return p.cal, nil
}

func newSvc(t *testing.T, s Store) *Service {
	t.Helper()
	return New(s, nil, AccessConfig{Secret: testSecret, TTL: time.Hour})
}

func TestUnlockCalendar_HappyPath(t *testing.T) {
	ps := &passphraseStore{cal: model.Calendar{ID: "cal1", Passphrase: "frost-tulip-raven-stamp", AccessEpoch: 2}}
	s := newSvc(t, ps)
	tok, err := s.UnlockCalendar(context.Background(), "cal1", "frost-tulip-raven-stamp", "1.2.3.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	calID, epoch, err := access.VerifyToken(tok, testSecret)
	if err != nil {
		t.Fatalf("issued token invalid: %v", err)
	}
	if calID != "cal1" || epoch != 2 {
		t.Fatalf("token fields = %q,%d, want cal1,2", calID, epoch)
	}
}

func TestUnlockCalendar_WrongPassphrase(t *testing.T) {
	ps := &passphraseStore{cal: model.Calendar{ID: "cal1", Passphrase: "right"}}
	s := newSvc(t, ps)
	if _, err := s.UnlockCalendar(context.Background(), "cal1", "wrong", "1.2.3.4"); !errors.Is(err, ErrPassphraseInvalid) {
		t.Fatalf("err = %v, want ErrPassphraseInvalid", err)
	}
}

func TestUnlockCalendar_NotProtected(t *testing.T) {
	ps := &passphraseStore{cal: model.Calendar{ID: "cal1"}}
	s := newSvc(t, ps)
	if _, err := s.UnlockCalendar(context.Background(), "cal1", "x", "1.2.3.4"); !errors.Is(err, ErrNotPassphraseProtected) {
		t.Fatalf("err = %v, want ErrNotPassphraseProtected", err)
	}
}

func TestUnlockCalendar_RateLimits(t *testing.T) {
	ps := &passphraseStore{cal: model.Calendar{ID: "cal1", Passphrase: "right"}}
	s := newSvc(t, ps)
	for i := 0; i < unlockRateLimitMax; i++ {
		if _, err := s.UnlockCalendar(context.Background(), "cal1", "wrong", "1.2.3.4"); !errors.Is(err, ErrPassphraseInvalid) {
			t.Fatalf("attempt %d err = %v, want ErrPassphraseInvalid", i+1, err)
		}
	}
	if _, err := s.UnlockCalendar(context.Background(), "cal1", "wrong", "1.2.3.4"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
}

func TestUnlockCalendar_SuccessResetsCounter(t *testing.T) {
	ps := &passphraseStore{cal: model.Calendar{ID: "cal1", Passphrase: "right"}}
	s := newSvc(t, ps)
	for i := 0; i < unlockRateLimitMax-1; i++ {
		_, _ = s.UnlockCalendar(context.Background(), "cal1", "wrong", "1.2.3.4")
	}
	if _, err := s.UnlockCalendar(context.Background(), "cal1", "right", "1.2.3.4"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// A further failure should not immediately trip the limit.
	if _, err := s.UnlockCalendar(context.Background(), "cal1", "wrong", "1.2.3.4"); !errors.Is(err, ErrPassphraseInvalid) {
		t.Fatalf("err after reset = %v, want ErrPassphraseInvalid", err)
	}
}

func TestAuthorizeAccess_OwnerBypass(t *testing.T) {
	ps := &passphraseStore{}
	s := newSvc(t, ps)
	ownerID := "owner1"
	cal := model.Calendar{ID: "cal1", OwnerID: ownerID, Passphrase: "p", AccessEpoch: 1}
	if err := s.AuthorizeAccess(context.Background(), cal, Viewer{UserID: &ownerID}, ""); err != nil {
		t.Fatalf("expected owner bypass, got %v", err)
	}
}

func TestAuthorizeAccess_StaleEpochRejected(t *testing.T) {
	ps := &passphraseStore{}
	s := newSvc(t, ps)
	cal := model.Calendar{ID: "cal1", Passphrase: "p", AccessEpoch: 5}
	tok := access.IssueToken("cal1", 4, testSecret, time.Hour)
	if err := s.AuthorizeAccess(context.Background(), cal, Viewer{}, tok); !errors.Is(err, ErrPassphraseRequired) {
		t.Fatalf("err = %v, want ErrPassphraseRequired", err)
	}
}

func TestAuthorizeAccess_ValidTokenAllowed(t *testing.T) {
	ps := &passphraseStore{}
	s := newSvc(t, ps)
	cal := model.Calendar{ID: "cal1", Passphrase: "p", AccessEpoch: 5}
	tok := access.IssueToken("cal1", 5, testSecret, time.Hour)
	if err := s.AuthorizeAccess(context.Background(), cal, Viewer{}, tok); err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
}

func TestAuthorizeAccess_UnprotectedCalendar(t *testing.T) {
	ps := &passphraseStore{}
	s := newSvc(t, ps)
	cal := model.Calendar{ID: "cal1"}
	if err := s.AuthorizeAccess(context.Background(), cal, Viewer{}, ""); err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
}

func TestRotateCalendarPassphrase_BumpsEpoch(t *testing.T) {
	ps := &passphraseStore{cal: model.Calendar{ID: "cal1", OwnerID: "u1", AccessEpoch: 0}}
	s := newSvc(t, ps)
	p, err := s.RotateCalendarPassphrase(context.Background(), "cal1", "u1")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if p == "" {
		t.Fatal("expected non-empty passphrase")
	}
	if ps.setCalled != 1 || ps.setArg == nil || *ps.setArg != p {
		t.Fatalf("store not called with new passphrase: called=%d arg=%v", ps.setCalled, ps.setArg)
	}
}

func TestClearCalendarPassphrase_PassesNil(t *testing.T) {
	ps := &passphraseStore{cal: model.Calendar{ID: "cal1", OwnerID: "u1", Passphrase: "p", AccessEpoch: 1}}
	s := newSvc(t, ps)
	if err := s.ClearCalendarPassphrase(context.Background(), "cal1", "u1"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if ps.setArg != nil {
		t.Fatalf("expected nil passphrase arg, got %v", ps.setArg)
	}
}
