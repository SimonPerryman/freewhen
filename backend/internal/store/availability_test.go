//go:build integration

package store

import (
	"context"
	"testing"

	"github.com/freewhen/backend/internal/model"
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

	if err := s.SetAvailability(ctx, p.ID, []string{"2026-05-01", "2026-05-02"}, nil); err != nil {
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

	if err := s.SetAvailability(ctx, p.ID, []string{"2026-05-01", "2026-05-02"}, nil); err != nil {
		t.Fatalf("set first: %v", err)
	}
	if err := s.SetAvailability(ctx, p.ID, []string{"2026-05-03"}, nil); err != nil {
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

	if err := s.SetAvailability(ctx, p.ID, []string{"2026-05-01", "2026-05-02"}, nil); err != nil {
		t.Fatalf("set first: %v", err)
	}
	if err := s.SetAvailability(ctx, p.ID, []string{}, nil); err != nil {
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

	if err := s.SetAvailability(ctx, alice.ID, []string{"2026-05-01", "2026-05-02"}, nil); err != nil {
		t.Fatalf("set alice: %v", err)
	}
	if err := s.SetAvailability(ctx, bob.ID, []string{"2026-05-02", "2026-05-03"}, nil); err != nil {
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

func TestSetAvailabilityNoteRoundTrip(t *testing.T) {
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

	notes := map[string]string{"2026-05-01": "only after 6pm"}
	if err := s.SetAvailability(ctx, p.ID, []string{"2026-05-01", "2026-05-02"}, notes); err != nil {
		t.Fatalf("set availability: %v", err)
	}

	result, err := s.GetCalendar(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}
	byDate := map[string]*model.Availability{}
	for i := range result.Availability {
		a := &result.Availability[i]
		byDate[a.Date] = a
	}
	if byDate["2026-05-01"].Note == nil || *byDate["2026-05-01"].Note != "only after 6pm" {
		t.Errorf("2026-05-01 note = %v, want 'only after 6pm'", byDate["2026-05-01"].Note)
	}
	if byDate["2026-05-02"].Note != nil {
		t.Errorf("2026-05-02 note = %v, want nil", byDate["2026-05-02"].Note)
	}
}

func TestSetAvailabilityPreservesNotesWhenNilOverrides(t *testing.T) {
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

	if err := s.SetAvailability(ctx, p.ID, []string{"2026-05-01"}, map[string]string{"2026-05-01": "evening only"}); err != nil {
		t.Fatalf("set first: %v", err)
	}
	// Re-submit same date with nil overrides — note should survive.
	if err := s.SetAvailability(ctx, p.ID, []string{"2026-05-01", "2026-05-02"}, nil); err != nil {
		t.Fatalf("set second: %v", err)
	}

	result, err := s.GetCalendar(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}
	for _, a := range result.Availability {
		if a.Date == "2026-05-01" {
			if a.Note == nil || *a.Note != "evening only" {
				t.Errorf("note not preserved across nil-overrides save: got %v", a.Note)
			}
		}
	}
}

func TestSetAvailabilityEmptyStringClearsNote(t *testing.T) {
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

	if err := s.SetAvailability(ctx, p.ID, []string{"2026-05-01"}, map[string]string{"2026-05-01": "x"}); err != nil {
		t.Fatalf("set first: %v", err)
	}
	if err := s.SetAvailability(ctx, p.ID, []string{"2026-05-01"}, map[string]string{"2026-05-01": ""}); err != nil {
		t.Fatalf("set second: %v", err)
	}

	result, err := s.GetCalendar(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}
	if len(result.Availability) != 1 || result.Availability[0].Note != nil {
		t.Errorf("expected note cleared, got %+v", result.Availability)
	}
}

func TestSetAvailabilityRemovingDateDropsNote(t *testing.T) {
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

	if err := s.SetAvailability(ctx, p.ID, []string{"2026-05-01"}, map[string]string{"2026-05-01": "x"}); err != nil {
		t.Fatalf("set first: %v", err)
	}
	// Deselect the date entirely — row goes away with its note.
	if err := s.SetAvailability(ctx, p.ID, []string{}, nil); err != nil {
		t.Fatalf("clear: %v", err)
	}
	// Reselect same date with nil overrides — note must not resurrect.
	if err := s.SetAvailability(ctx, p.ID, []string{"2026-05-01"}, nil); err != nil {
		t.Fatalf("re-add: %v", err)
	}

	result, err := s.GetCalendar(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}
	if len(result.Availability) != 1 || result.Availability[0].Note != nil {
		t.Errorf("expected note not to resurrect, got %+v", result.Availability)
	}
}

func TestSetAvailabilityParticipantIsolation(t *testing.T) {
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

	if err := s.SetAvailability(ctx, alice.ID, []string{"2026-05-01"}, map[string]string{"2026-05-01": "alice note"}); err != nil {
		t.Fatalf("set alice: %v", err)
	}
	// Bob submits availability with no notes — must not touch alice's note.
	if err := s.SetAvailability(ctx, bob.ID, []string{"2026-05-01"}, nil); err != nil {
		t.Fatalf("set bob: %v", err)
	}

	result, err := s.GetCalendar(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}
	for _, a := range result.Availability {
		if a.ParticipantID == alice.ID && a.Date == "2026-05-01" {
			if a.Note == nil || *a.Note != "alice note" {
				t.Errorf("alice's note got touched by bob's edit: %v", a.Note)
			}
		}
	}
}
