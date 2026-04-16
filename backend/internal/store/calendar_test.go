//go:build integration

package store

import (
	"context"
	"regexp"
	"testing"
	"time"
)

func TestCreateCalendar(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	cal, err := s.CreateCalendar(ctx, "Weekend Trip")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cal.ID) != 8 {
		t.Errorf("expected 8-char ID, got %q", cal.ID)
	}
	if !regexp.MustCompile(`^[a-z0-9]+$`).MatchString(cal.ID) {
		t.Errorf("expected alphanumeric ID, got %q", cal.ID)
	}
	if cal.Name != "Weekend Trip" {
		t.Errorf("expected name %q, got %q", "Weekend Trip", cal.Name)
	}
	if time.Since(cal.CreatedAt) > 5*time.Second {
		t.Errorf("expected recent created_at, got %v", cal.CreatedAt)
	}
}

func TestCreateCalendarDefaultName(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	cal, err := s.CreateCalendar(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cal.Name != "" {
		t.Errorf("expected empty name, got %q", cal.Name)
	}
}

func TestGetCalendar(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	cal, err := s.CreateCalendar(ctx, "Test")
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	alice, err := s.AddParticipant(ctx, cal.ID, "Alice")
	if err != nil {
		t.Fatalf("add alice: %v", err)
	}
	bob, err := s.AddParticipant(ctx, cal.ID, "Bob")
	if err != nil {
		t.Fatalf("add bob: %v", err)
	}

	if err := s.SetAvailability(ctx, alice.ID, []string{"2026-05-01", "2026-05-02"}); err != nil {
		t.Fatalf("set alice availability: %v", err)
	}
	if err := s.SetAvailability(ctx, bob.ID, []string{"2026-05-02", "2026-05-03"}); err != nil {
		t.Fatalf("set bob availability: %v", err)
	}

	result, err := s.GetCalendar(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}
	if result.Calendar.ID != cal.ID {
		t.Errorf("expected calendar ID %q, got %q", cal.ID, result.Calendar.ID)
	}
	if len(result.Participants) != 2 {
		t.Errorf("expected 2 participants, got %d", len(result.Participants))
	}
	if len(result.Availability) != 4 {
		t.Errorf("expected 4 availability rows, got %d", len(result.Availability))
	}
}

func TestGetCalendarNotFound(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	_, err := s.GetCalendar(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetCalendarEmpty(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	cal, err := s.CreateCalendar(ctx, "Empty")
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	result, err := s.GetCalendar(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}
	if len(result.Participants) != 0 {
		t.Errorf("expected 0 participants, got %d", len(result.Participants))
	}
	if len(result.Availability) != 0 {
		t.Errorf("expected 0 availability, got %d", len(result.Availability))
	}
}
