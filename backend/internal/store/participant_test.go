//go:build integration

package store

import (
	"context"
	"regexp"
	"testing"
)

func TestAddParticipant(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	cal, err := s.CreateCalendar(ctx, "Test")
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	p, err := s.AddParticipant(ctx, cal.ID, "Alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.ID) != 8 {
		t.Errorf("expected 8-char ID, got %q", p.ID)
	}
	if !regexp.MustCompile(`^[a-z0-9]+$`).MatchString(p.ID) {
		t.Errorf("expected alphanumeric ID, got %q", p.ID)
	}
	if p.CalendarID != cal.ID {
		t.Errorf("expected calendar ID %q, got %q", cal.ID, p.CalendarID)
	}
	if p.Name != "Alice" {
		t.Errorf("expected name %q, got %q", "Alice", p.Name)
	}
	if !regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`).MatchString(p.Colour) {
		t.Errorf("expected hex colour, got %q", p.Colour)
	}
}

func TestAddParticipantColourAssignment(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	cal, err := s.CreateCalendar(ctx, "Test")
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	p1, err := s.AddParticipant(ctx, cal.ID, "Alice")
	if err != nil {
		t.Fatalf("add alice: %v", err)
	}
	p2, err := s.AddParticipant(ctx, cal.ID, "Bob")
	if err != nil {
		t.Fatalf("add bob: %v", err)
	}
	p3, err := s.AddParticipant(ctx, cal.ID, "Charlie")
	if err != nil {
		t.Fatalf("add charlie: %v", err)
	}

	if p1.Colour == p2.Colour {
		t.Errorf("p1 and p2 should have different colours: %s", p1.Colour)
	}
	if p2.Colour == p3.Colour {
		t.Errorf("p2 and p3 should have different colours: %s", p2.Colour)
	}
}

func TestAddParticipantDuplicateName(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	cal, err := s.CreateCalendar(ctx, "Test")
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	if _, err := s.AddParticipant(ctx, cal.ID, "Alice"); err != nil {
		t.Fatalf("first add: %v", err)
	}

	_, err = s.AddParticipant(ctx, cal.ID, "Alice")
	if err != ErrDuplicateName {
		t.Errorf("expected ErrDuplicateName, got %v", err)
	}
}

func TestAddParticipantSameNameDifferentCalendar(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	cal1, err := s.CreateCalendar(ctx, "Calendar 1")
	if err != nil {
		t.Fatalf("create cal1: %v", err)
	}
	cal2, err := s.CreateCalendar(ctx, "Calendar 2")
	if err != nil {
		t.Fatalf("create cal2: %v", err)
	}

	if _, err := s.AddParticipant(ctx, cal1.ID, "Alice"); err != nil {
		t.Errorf("expected no error for cal1, got %v", err)
	}
	if _, err := s.AddParticipant(ctx, cal2.ID, "Alice"); err != nil {
		t.Errorf("expected no error for cal2, got %v", err)
	}
}
