//go:build integration

package store

import (
	"context"
	"testing"
)

// Soft-deleted calendars are invisible to read paths: GetCalendarOnly +
// GetCalendar return ErrNotFound, ListCalendarsForUser drops the row, and
// CountCalendarsForOwner ignores it (so the owner quota frees up).
func TestSoftDeleteCalendar_HidesFromReads(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	cal, err := s.CreateCalendar(ctx, "Trip", "", owner.ID)
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}
	if _, err := s.AddParticipant(ctx, cal.ID, "Owner", &owner.ID, nil); err != nil {
		t.Fatalf("auto-join owner: %v", err)
	}

	if err := s.SoftDeleteCalendar(ctx, cal.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	if _, err := s.GetCalendarOnly(ctx, cal.ID); err != ErrNotFound {
		t.Fatalf("GetCalendarOnly = %v, want ErrNotFound", err)
	}
	if _, err := s.GetCalendar(ctx, cal.ID); err != ErrNotFound {
		t.Fatalf("GetCalendar = %v, want ErrNotFound", err)
	}
	items, err := s.ListCalendarsForUser(ctx, owner.ID, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 calendars after soft-delete, got %d", len(items))
	}
	count, err := s.CountCalendarsForOwner(ctx, owner.ID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 owned calendars after soft-delete, got %d", count)
	}
	soft, err := s.IsCalendarSoftDeleted(ctx, cal.ID)
	if err != nil {
		t.Fatalf("IsCalendarSoftDeleted: %v", err)
	}
	if !soft {
		t.Fatal("expected IsCalendarSoftDeleted = true")
	}
}

// SoftDeleteCalendar is idempotent — running it twice still returns nil and
// IsCalendarSoftDeleted stays true. Missing rows return nil too.
func TestSoftDeleteCalendar_Idempotent(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	cal, err := s.CreateCalendar(ctx, "Trip", "", owner.ID)
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}
	if err := s.SoftDeleteCalendar(ctx, cal.ID); err != nil {
		t.Fatalf("first delete: %v", err)
	}
	if err := s.SoftDeleteCalendar(ctx, cal.ID); err != nil {
		t.Fatalf("second delete: %v", err)
	}
	if err := s.SoftDeleteCalendar(ctx, "nope"); err != nil {
		t.Fatalf("delete missing: %v", err)
	}
}

// Mutations against a soft-deleted calendar surface as ErrNotFound at the
// store layer (handlers map this to 410 Gone).
func TestSoftDeletedCalendar_MutationsFail(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	cal, err := s.CreateCalendar(ctx, "Trip", "", owner.ID)
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}
	if err := s.SoftDeleteCalendar(ctx, cal.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if _, err := s.UpdateCalendarName(ctx, cal.ID, "Renamed"); err != ErrNotFound {
		t.Fatalf("UpdateCalendarName = %v, want ErrNotFound", err)
	}
	if err := s.TouchCalendar(ctx, cal.ID); err != ErrNotFound {
		t.Fatalf("TouchCalendar = %v, want ErrNotFound", err)
	}
	if _, err := s.SetFinalisedDate(ctx, cal.ID, "2026-06-01"); err != ErrNotFound {
		t.Fatalf("SetFinalisedDate = %v, want ErrNotFound", err)
	}
}

// Soft-deleting a participant hides them from reads and counts but their
// availability rows stay in storage (we just don't surface them).
func TestSoftDeleteParticipant_HidesFromReads(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	cal, err := s.CreateCalendar(ctx, "Trip", "", owner.ID)
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}
	p, err := s.AddParticipant(ctx, cal.ID, "Alice", nil, nil)
	if err != nil {
		t.Fatalf("add participant: %v", err)
	}
	if err := s.SetAvailability(ctx, p.ID, []string{"2026-06-01"}, nil, nil); err != nil {
		t.Fatalf("set availability: %v", err)
	}
	if err := s.DeleteParticipant(ctx, cal.ID, p.ID); err != nil {
		t.Fatalf("delete participant: %v", err)
	}

	got, err := s.GetCalendar(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}
	if len(got.Participants) != 0 {
		t.Fatalf("expected 0 live participants, got %d", len(got.Participants))
	}
	if len(got.Availability) != 0 {
		t.Fatalf("expected 0 live availability rows, got %d", len(got.Availability))
	}
	count, err := s.CountCalendarParticipants(ctx, cal.ID)
	if err != nil {
		t.Fatalf("count participants: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected count 0, got %d", count)
	}
}

// Leaving a calendar soft-deletes the user's participant rows. After leaving,
// the user can rejoin under the same name without colliding with the
// soft-deleted row, and their old availability does not resurface.
func TestSoftDeleteParticipantsForUser_LeaveAndRejoin(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	joiner := createTestUser(t, s, "Joiner")
	cal, err := s.CreateCalendar(ctx, "Trip", "", owner.ID)
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}
	p, err := s.AddParticipant(ctx, cal.ID, "Joiner", &joiner.ID, nil)
	if err != nil {
		t.Fatalf("add participant: %v", err)
	}
	if err := s.SetAvailability(ctx, p.ID, []string{"2026-06-01"}, nil, nil); err != nil {
		t.Fatalf("set availability: %v", err)
	}

	rows, err := s.SoftDeleteParticipantsForUser(ctx, cal.ID, joiner.ID)
	if err != nil {
		t.Fatalf("leave: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected 1 row affected, got %d", rows)
	}

	// Idempotent: second leave touches zero rows but returns nil.
	rows2, err := s.SoftDeleteParticipantsForUser(ctx, cal.ID, joiner.ID)
	if err != nil {
		t.Fatalf("second leave: %v", err)
	}
	if rows2 != 0 {
		t.Fatalf("expected 0 rows on idempotent leave, got %d", rows2)
	}

	// Rejoin under the same name; old soft-deleted row must not collide.
	rejoined, err := s.AddParticipant(ctx, cal.ID, "Joiner", &joiner.ID, nil)
	if err != nil {
		t.Fatalf("rejoin: %v", err)
	}
	if rejoined.ID == p.ID {
		t.Fatalf("rejoin should mint a fresh participant id, got %q == %q", rejoined.ID, p.ID)
	}
	got, err := s.GetCalendar(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}
	for _, a := range got.Availability {
		if a.ParticipantID == p.ID {
			t.Fatalf("old soft-deleted participant's availability resurfaced")
		}
	}
}
