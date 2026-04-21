package service

import (
	"context"
	"testing"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/store"
)

func TestParseBulkNames(t *testing.T) {
	got := ParseBulkNames([]string{"Alice, Bob\nCharlie", "  ", "Dave,,Eve"})
	want := []string{"Alice", "Bob", "Charlie", "Dave", "Eve"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("at %d expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestAddInvites_Validation(t *testing.T) {
	tooLong := ""
	for i := 0; i < 60; i++ {
		tooLong += "x"
	}
	ms := &mockStore{
		addInvites: func(names []string) (store.AddInvitesResult, error) {
			added := []model.Invite{}
			for _, n := range names {
				added = append(added, model.Invite{ID: "inv_" + n, Name: n, CalendarID: "c1"})
			}
			return store.AddInvitesResult{Added: added}, nil
		},
	}
	s := newSvc(t, ms)

	res, err := s.AddInvites(context.Background(), "c1", []string{"Alice", "Alice", tooLong, "  ", "Bob"})
	if err != nil {
		t.Fatalf("add invites: %v", err)
	}
	if len(res.Added) != 2 {
		t.Fatalf("expected 2 added, got %d (%+v)", len(res.Added), res.Added)
	}
	if len(res.Skipped) != 1 {
		t.Fatalf("expected 1 skipped (duplicate Alice), got %+v", res.Skipped)
	}
	if len(res.Invalid) != 1 || res.Invalid[0].Reason != "name_too_long" {
		t.Fatalf("expected 1 invalid (name_too_long), got %+v", res.Invalid)
	}
}

func TestGetMissingView(t *testing.T) {
	matchedPID := "p1"
	ms := &mockStore{
		listInvites: func() ([]model.Invite, error) {
			return []model.Invite{
				{ID: "i1", Name: "Alice", MatchedParticipantID: &matchedPID},
				{ID: "i2", Name: "Bob"},
			}, nil
		},
		participantsWithNoAvailability: func() ([]model.Participant, error) {
			return []model.Participant{{ID: "p2", Name: "Charlie", CalendarID: "c1"}}, nil
		},
	}
	s := newSvc(t, ms)
	view, err := s.GetMissingView(context.Background(), "c1")
	if err != nil {
		t.Fatalf("get missing view: %v", err)
	}
	if len(view.Invites) != 2 {
		t.Fatalf("expected 2 invites, got %d", len(view.Invites))
	}
	if len(view.NotJoined) != 1 || view.NotJoined[0].Name != "Bob" {
		t.Fatalf("expected Bob in not_joined, got %+v", view.NotJoined)
	}
	if len(view.JoinedNotResponded) != 1 || view.JoinedNotResponded[0].Name != "Charlie" {
		t.Fatalf("expected Charlie in joined_not_responded, got %+v", view.JoinedNotResponded)
	}
}

func TestManualMatchInvite_ParticipantOnDifferentCalendar(t *testing.T) {
	ms := &mockStore{
		getInvite: func(id string) (model.Invite, error) {
			return model.Invite{ID: id, CalendarID: "c1", Name: "Alice"}, nil
		},
	}
	// Override GetParticipant via service_test's mockStore getParticipant field.
	ms.getParticipant = func(_ context.Context, id string) (model.Participant, error) {
		return model.Participant{ID: id, CalendarID: "other"}, nil
	}
	s := newSvc(t, ms)
	pid := "px"
	_, err := s.ManualMatchInvite(context.Background(), "c1", "i1", &pid)
	if err != ErrParticipantNotOnCalendar {
		t.Fatalf("expected ErrParticipantNotOnCalendar, got %v", err)
	}
}
