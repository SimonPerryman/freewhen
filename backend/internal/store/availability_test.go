//go:build integration

package store

import (
	"context"
	"testing"
)

func TestSetAvailability(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	cal, err := s.CreateCalendar(ctx, "Test", "", createTestUser(t, s, "Owner").ID, "open")
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}
	p, err := s.AddParticipant(ctx, cal.ID, "Alice", nil)
	if err != nil {
		t.Fatalf("add participant: %v", err)
	}

	if err := s.SetAvailability(ctx, p.ID, []string{"2026-05-01", "2026-05-02"}); err != nil {
		t.Fatalf("set availability: %v", err)
	}

	result, err := s.GetCalendar(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}
	if len(result.Availability) != 2 {
		t.Fatalf("expected 2 availability rows, got %d", len(result.Availability))
	}

	dates := map[string]bool{}
	for _, a := range result.Availability {
		dates[a.Date] = true
	}
	if !dates["2026-05-01"] || !dates["2026-05-02"] {
		t.Errorf("expected dates 2026-05-01 and 2026-05-02, got %v", dates)
	}
}

func TestSetAvailabilityReplacesExisting(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	cal, err := s.CreateCalendar(ctx, "Test", "", createTestUser(t, s, "Owner").ID, "open")
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}
	p, err := s.AddParticipant(ctx, cal.ID, "Alice", nil)
	if err != nil {
		t.Fatalf("add participant: %v", err)
	}

	if err := s.SetAvailability(ctx, p.ID, []string{"2026-05-01", "2026-05-02"}); err != nil {
		t.Fatalf("set first: %v", err)
	}
	if err := s.SetAvailability(ctx, p.ID, []string{"2026-05-03"}); err != nil {
		t.Fatalf("set second: %v", err)
	}

	result, err := s.GetCalendar(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}
	if len(result.Availability) != 1 {
		t.Fatalf("expected 1 availability row, got %d", len(result.Availability))
	}
	if result.Availability[0].Date != "2026-05-03" {
		t.Errorf("expected date 2026-05-03, got %s", result.Availability[0].Date)
	}
}

func TestSetAvailabilityClearAll(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	cal, err := s.CreateCalendar(ctx, "Test", "", createTestUser(t, s, "Owner").ID, "open")
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}
	p, err := s.AddParticipant(ctx, cal.ID, "Alice", nil)
	if err != nil {
		t.Fatalf("add participant: %v", err)
	}

	if err := s.SetAvailability(ctx, p.ID, []string{"2026-05-01", "2026-05-02"}); err != nil {
		t.Fatalf("set first: %v", err)
	}
	if err := s.SetAvailability(ctx, p.ID, []string{}); err != nil {
		t.Fatalf("set empty: %v", err)
	}

	result, err := s.GetCalendar(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}
	if len(result.Availability) != 0 {
		t.Errorf("expected 0 availability rows, got %d", len(result.Availability))
	}
}

func TestSetAvailabilityMultipleParticipants(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	cal, err := s.CreateCalendar(ctx, "Test", "", createTestUser(t, s, "Owner").ID, "open")
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}
	alice, err := s.AddParticipant(ctx, cal.ID, "Alice", nil)
	if err != nil {
		t.Fatalf("add alice: %v", err)
	}
	bob, err := s.AddParticipant(ctx, cal.ID, "Bob", nil)
	if err != nil {
		t.Fatalf("add bob: %v", err)
	}

	if err := s.SetAvailability(ctx, alice.ID, []string{"2026-05-01", "2026-05-02"}); err != nil {
		t.Fatalf("set alice: %v", err)
	}
	if err := s.SetAvailability(ctx, bob.ID, []string{"2026-05-02", "2026-05-03"}); err != nil {
		t.Fatalf("set bob: %v", err)
	}

	result, err := s.GetCalendar(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}
	if len(result.Availability) != 4 {
		t.Fatalf("expected 4 availability rows, got %d", len(result.Availability))
	}

	aliceDates := 0
	for _, a := range result.Availability {
		if a.ParticipantID == alice.ID {
			aliceDates++
		}
	}
	if aliceDates != 2 {
		t.Errorf("expected 2 dates for alice, got %d", aliceDates)
	}
}
