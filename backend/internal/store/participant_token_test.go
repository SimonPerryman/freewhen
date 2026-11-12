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

	noTok, err := s.AddParticipant(ctx, cal.ID, "NoToken", nil, nil)
	if err != nil {
		t.Fatalf("add without token: %v", err)
	}
	_, got, err = s.GetParticipantForAuth(ctx, noTok.ID)
	if err != nil || got != nil {
		t.Fatalf("no-token hash = %v err=%v, want nil", got, err)
	}

	if _, _, err := s.GetParticipantForAuth(ctx, "missing1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing participant: err = %v, want ErrNotFound", err)
	}
}
