//go:build integration

package store

import (
	"context"
	"errors"
	"testing"
)

func TestAddInvites_BasicAndDedupe(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	cal, err := s.CreateCalendar(ctx, "Trip", "", owner.ID, "open")
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	res, err := s.AddInvites(ctx, cal.ID, []string{"Alice", "Bob", "Alice"})
	if err != nil {
		t.Fatalf("add invites: %v", err)
	}
	if len(res.Added) != 2 {
		t.Fatalf("expected 2 added, got %d", len(res.Added))
	}
	if len(res.Skipped) != 1 || res.Skipped[0] != "Alice" {
		t.Fatalf("expected 1 skipped (Alice), got %v", res.Skipped)
	}

	// Case-insensitive uniqueness against existing row.
	res, err = s.AddInvites(ctx, cal.ID, []string{"alice", "Charlie"})
	if err != nil {
		t.Fatalf("add invites 2: %v", err)
	}
	if len(res.Added) != 1 || res.Added[0].Name != "Charlie" {
		t.Fatalf("expected only Charlie added, got %+v", res.Added)
	}
	if len(res.Skipped) != 1 || res.Skipped[0] != "alice" {
		t.Fatalf("expected alice skipped, got %v", res.Skipped)
	}

	invites, err := s.ListInvites(ctx, cal.ID)
	if err != nil {
		t.Fatalf("list invites: %v", err)
	}
	if len(invites) != 3 {
		t.Fatalf("expected 3 invites, got %d", len(invites))
	}
}

func TestMatchInviteByName_AutoMatchOnAddParticipant(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	cal, err := s.CreateCalendar(ctx, "Trip", "", owner.ID, "open")
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	if _, err := s.AddInvites(ctx, cal.ID, []string{"Alice", "Bob"}); err != nil {
		t.Fatalf("add invites: %v", err)
	}

	p, err := s.AddParticipant(ctx, cal.ID, " alice ", nil)
	if err != nil {
		t.Fatalf("add participant: %v", err)
	}

	invites, err := s.ListInvites(ctx, cal.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var aliceInvite *string
	for _, inv := range invites {
		if inv.Name == "Alice" {
			aliceInvite = inv.MatchedParticipantID
		}
	}
	if aliceInvite == nil {
		t.Fatalf("expected Alice invite to be matched")
	}
	if *aliceInvite != p.ID {
		t.Fatalf("expected match to %q, got %q", p.ID, *aliceInvite)
	}

	// Deleting the participant flips invite back to unmatched (ON DELETE SET NULL).
	if err := s.DeleteParticipant(ctx, cal.ID, p.ID); err != nil {
		t.Fatalf("delete participant: %v", err)
	}
	invites, _ = s.ListInvites(ctx, cal.ID)
	for _, inv := range invites {
		if inv.Name == "Alice" && inv.MatchedParticipantID != nil {
			t.Fatalf("expected Alice invite to be unmatched after participant delete")
		}
	}
}

func TestMatchInviteByName_ReMatchOnRename(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	cal, err := s.CreateCalendar(ctx, "Trip", "", owner.ID, "open")
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	if _, err := s.AddInvites(ctx, cal.ID, []string{"Alice", "Bobby"}); err != nil {
		t.Fatalf("add invites: %v", err)
	}
	p, err := s.AddParticipant(ctx, cal.ID, "Alice", nil)
	if err != nil {
		t.Fatalf("add participant: %v", err)
	}

	if _, err := s.UpdateParticipantName(ctx, cal.ID, p.ID, "Bobby"); err != nil {
		t.Fatalf("rename: %v", err)
	}

	invites, _ := s.ListInvites(ctx, cal.ID)
	for _, inv := range invites {
		if inv.Name == "Bobby" {
			if inv.MatchedParticipantID == nil || *inv.MatchedParticipantID != p.ID {
				t.Fatalf("expected Bobby invite matched to %q, got %+v", p.ID, inv.MatchedParticipantID)
			}
		}
	}
}

func TestUpdateInviteName_Collision(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	cal, _ := s.CreateCalendar(ctx, "Trip", "", owner.ID, "open")
	res, err := s.AddInvites(ctx, cal.ID, []string{"Alice", "Bob"})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	// Attempt to rename Alice -> "bob" (same normalised key as existing invite).
	_, err = s.UpdateInviteName(ctx, res.Added[0].ID, "bob")
	if !errors.Is(err, ErrInviteNameTaken) {
		t.Fatalf("expected ErrInviteNameTaken, got %v", err)
	}
}

func TestDeleteInvite(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	cal, _ := s.CreateCalendar(ctx, "Trip", "", owner.ID, "open")
	res, _ := s.AddInvites(ctx, cal.ID, []string{"Alice"})
	if err := s.DeleteInvite(ctx, res.Added[0].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := s.DeleteInvite(ctx, res.Added[0].ID); !errors.Is(err, ErrInviteNotFound) {
		t.Fatalf("expected ErrInviteNotFound on second delete, got %v", err)
	}
}

func TestParticipantsWithNoAvailability(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	cal, _ := s.CreateCalendar(ctx, "Trip", "", owner.ID, "open")
	alice, _ := s.AddParticipant(ctx, cal.ID, "Alice", nil)
	bob, _ := s.AddParticipant(ctx, cal.ID, "Bob", nil)
	// Alice submits, Bob doesn't.
	if err := s.SetAvailability(ctx, alice.ID, []string{"2030-01-01"}, nil, nil); err != nil {
		t.Fatalf("set availability: %v", err)
	}
	quiet, err := s.ParticipantsWithNoAvailability(ctx, cal.ID)
	if err != nil {
		t.Fatalf("list quiet: %v", err)
	}
	if len(quiet) != 1 || quiet[0].ID != bob.ID {
		t.Fatalf("expected only Bob in quiet list, got %+v", quiet)
	}
}
