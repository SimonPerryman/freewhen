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
	owner := createTestUser(t, s, "Owner")

	cal, err := s.CreateCalendar(ctx, "Test", "", owner.ID)
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	p, err := s.AddParticipant(ctx, cal.ID, "Alice", nil)
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
	if p.UserID != nil {
		t.Errorf("expected nil user_id, got %q", *p.UserID)
	}
}

func TestAddParticipantWithUserID(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	alice := createTestUser(t, s, "Alice")

	cal, err := s.CreateCalendar(ctx, "Test", "", owner.ID)
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	p, err := s.AddParticipant(ctx, cal.ID, "Alice", &alice.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.UserID == nil {
		t.Fatal("expected user_id to be set")
	}
	if *p.UserID != alice.ID {
		t.Errorf("expected user_id %q, got %q", alice.ID, *p.UserID)
	}
}

func TestAddParticipantUserAlreadyJoined(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	alice := createTestUser(t, s, "Alice")

	cal, err := s.CreateCalendar(ctx, "Test", "", owner.ID)
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	if _, err := s.AddParticipant(ctx, cal.ID, "Alice", &alice.ID); err != nil {
		t.Fatalf("first add: %v", err)
	}

	_, err = s.AddParticipant(ctx, cal.ID, "Alice Again", &alice.ID)
	if err != ErrUserAlreadyJoined {
		t.Errorf("expected ErrUserAlreadyJoined, got %v", err)
	}
}

func TestAddParticipantSameUserDifferentCalendar(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	alice := createTestUser(t, s, "Alice")

	cal1, err := s.CreateCalendar(ctx, "Calendar 1", "", owner.ID)
	if err != nil {
		t.Fatalf("create cal1: %v", err)
	}
	cal2, err := s.CreateCalendar(ctx, "Calendar 2", "", owner.ID)
	if err != nil {
		t.Fatalf("create cal2: %v", err)
	}

	if _, err := s.AddParticipant(ctx, cal1.ID, "Alice", &alice.ID); err != nil {
		t.Errorf("expected no error for cal1, got %v", err)
	}
	if _, err := s.AddParticipant(ctx, cal2.ID, "Alice", &alice.ID); err != nil {
		t.Errorf("expected no error for cal2, got %v", err)
	}
}

func TestAddParticipantColourAssignment(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")

	cal, err := s.CreateCalendar(ctx, "Test", "", owner.ID)
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	p1, err := s.AddParticipant(ctx, cal.ID, "Alice", nil)
	if err != nil {
		t.Fatalf("add alice: %v", err)
	}
	p2, err := s.AddParticipant(ctx, cal.ID, "Bob", nil)
	if err != nil {
		t.Fatalf("add bob: %v", err)
	}
	p3, err := s.AddParticipant(ctx, cal.ID, "Charlie", nil)
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
	owner := createTestUser(t, s, "Owner")

	cal, err := s.CreateCalendar(ctx, "Test", "", owner.ID)
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	if _, err := s.AddParticipant(ctx, cal.ID, "Alice", nil); err != nil {
		t.Fatalf("first add: %v", err)
	}

	_, err = s.AddParticipant(ctx, cal.ID, "Alice", nil)
	if err != ErrDuplicateName {
		t.Errorf("expected ErrDuplicateName, got %v", err)
	}
}

func TestAddParticipantSameNameDifferentCalendar(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")

	cal1, err := s.CreateCalendar(ctx, "Calendar 1", "", owner.ID)
	if err != nil {
		t.Fatalf("create cal1: %v", err)
	}
	cal2, err := s.CreateCalendar(ctx, "Calendar 2", "", owner.ID)
	if err != nil {
		t.Fatalf("create cal2: %v", err)
	}

	if _, err := s.AddParticipant(ctx, cal1.ID, "Alice", nil); err != nil {
		t.Errorf("expected no error for cal1, got %v", err)
	}
	if _, err := s.AddParticipant(ctx, cal2.ID, "Alice", nil); err != nil {
		t.Errorf("expected no error for cal2, got %v", err)
	}
}

func TestClaimParticipant(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	alice := createTestUser(t, s, "Alice")

	cal, err := s.CreateCalendar(ctx, "Test", "", owner.ID)
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	// Add anonymous participant
	p, err := s.AddParticipant(ctx, cal.ID, "Alice", nil)
	if err != nil {
		t.Fatalf("add participant: %v", err)
	}
	if p.UserID != nil {
		t.Fatal("expected nil user_id before claim")
	}

	// Claim it
	claimed, err := s.ClaimParticipant(ctx, cal.ID, p.ID, alice.ID)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claimed.UserID == nil {
		t.Fatal("expected user_id to be set after claim")
	}
	if *claimed.UserID != alice.ID {
		t.Errorf("expected user_id %q, got %q", alice.ID, *claimed.UserID)
	}

	// Verify via GetParticipant
	got, err := s.GetParticipant(ctx, p.ID)
	if err != nil {
		t.Fatalf("get participant: %v", err)
	}
	if got.UserID == nil || *got.UserID != alice.ID {
		t.Errorf("expected user_id %q on re-read, got %v", alice.ID, got.UserID)
	}
}

func TestClaimParticipantAlreadyLinked(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	alice := createTestUser(t, s, "Alice")
	bob := createTestUser(t, s, "Bob")

	cal, err := s.CreateCalendar(ctx, "Test", "", owner.ID)
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	p, err := s.AddParticipant(ctx, cal.ID, "Alice", &alice.ID)
	if err != nil {
		t.Fatalf("add participant: %v", err)
	}

	_, err = s.ClaimParticipant(ctx, cal.ID, p.ID, bob.ID)
	if err != ErrAlreadyLinked {
		t.Errorf("expected ErrAlreadyLinked, got %v", err)
	}
}

func TestClaimParticipantUserAlreadyInCalendar(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	alice := createTestUser(t, s, "Alice")

	cal, err := s.CreateCalendar(ctx, "Test", "", owner.ID)
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	// Alice joins with her account
	if _, err := s.AddParticipant(ctx, cal.ID, "Alice", &alice.ID); err != nil {
		t.Fatalf("add alice: %v", err)
	}

	// Another anonymous participant
	anon, err := s.AddParticipant(ctx, cal.ID, "Anonymous", nil)
	if err != nil {
		t.Fatalf("add anon: %v", err)
	}

	// Alice tries to claim the anonymous participant — already in calendar
	_, err = s.ClaimParticipant(ctx, cal.ID, anon.ID, alice.ID)
	if err != ErrUserAlreadyJoined {
		t.Errorf("expected ErrUserAlreadyJoined, got %v", err)
	}
}

func TestClaimParticipantNotFound(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	alice := createTestUser(t, s, "Alice")

	cal, err := s.CreateCalendar(ctx, "Test", "", owner.ID)
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	_, err = s.ClaimParticipant(ctx, cal.ID, "nonexistent", alice.ID)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetCalendarParticipantUserID(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	alice := createTestUser(t, s, "Alice")

	cal, err := s.CreateCalendar(ctx, "Test", "", owner.ID)
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	// One linked, one anonymous
	if _, err := s.AddParticipant(ctx, cal.ID, "Alice", &alice.ID); err != nil {
		t.Fatalf("add alice: %v", err)
	}
	if _, err := s.AddParticipant(ctx, cal.ID, "Bob", nil); err != nil {
		t.Fatalf("add bob: %v", err)
	}

	result, err := s.GetCalendar(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}

	var linkedCount, anonCount int
	for _, p := range result.Participants {
		if p.UserID != nil {
			linkedCount++
			if *p.UserID != alice.ID {
				t.Errorf("expected linked user_id %q, got %q", alice.ID, *p.UserID)
			}
		} else {
			anonCount++
		}
	}
	if linkedCount != 1 {
		t.Errorf("expected 1 linked participant, got %d", linkedCount)
	}
	if anonCount != 1 {
		t.Errorf("expected 1 anonymous participant, got %d", anonCount)
	}
}
