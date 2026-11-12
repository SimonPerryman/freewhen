//go:build integration

package store

import (
	"context"
	"errors"
	"testing"
)

func TestParticipantTokenHash(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")

	cal, err := s.CreateCalendar(ctx, "Tokens", "", owner.ID)
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	hash := "abc123"
	withTok, err := s.AddParticipantWithTokenHash(ctx, cal.ID, "Alice", nil, nil, &hash)
	if err != nil {
		t.Fatalf("add with token: %v", err)
	}
	_, got, err := s.GetParticipantForAuth(ctx, withTok.ID)
	if err != nil {
		t.Fatalf("get for auth: %v", err)
	}
	if got == nil || *got != hash {
		t.Fatalf("token hash = %v, want %q", got, hash)
	}
	// Already set → TOFU update is refused.
	ok, err := s.SetParticipantTokenHashIfUnset(ctx, withTok.ID, "other")
	if err != nil || ok {
		t.Fatalf("set on hashed row: ok=%v err=%v, want false/nil", ok, err)
	}

	legacy, err := s.AddParticipant(ctx, cal.ID, "Legacy", nil, nil)
	if err != nil {
		t.Fatalf("add legacy: %v", err)
	}
	_, got, err = s.GetParticipantForAuth(ctx, legacy.ID)
	if err != nil || got != nil {
		t.Fatalf("legacy hash = %v err=%v, want nil", got, err)
	}
	ok, err = s.SetParticipantTokenHashIfUnset(ctx, legacy.ID, "first")
	if err != nil || !ok {
		t.Fatalf("first TOFU set: ok=%v err=%v", ok, err)
	}
	ok, err = s.SetParticipantTokenHashIfUnset(ctx, legacy.ID, "second")
	if err != nil || ok {
		t.Fatalf("second TOFU set must lose: ok=%v err=%v", ok, err)
	}
	_, got, _ = s.GetParticipantForAuth(ctx, legacy.ID)
	if got == nil || *got != "first" {
		t.Fatalf("hash after race = %v, want first", got)
	}

	if _, _, err := s.GetParticipantForAuth(ctx, "missing1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing participant: err = %v, want ErrNotFound", err)
	}
}
