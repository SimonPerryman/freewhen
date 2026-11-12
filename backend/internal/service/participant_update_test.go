package service

import (
	"context"
	"errors"
	"testing"

	"github.com/freewhen/backend/internal/model"
)

func TestRenameParticipant_WithEmailIsOneStoreWrite(t *testing.T) {
	ms := authFixture()
	ms.updateParticipantName = func(context.Context, string, string, string) (model.Participant, error) {
		t.Fatal("name-only write used when an email was supplied")
		return model.Participant{}, nil
	}
	var gotName string
	var gotEmail *string
	ms.updateParticipantNameAndEmail = func(_ context.Context, calendarID, pid, name string, email *string) (model.Participant, error) {
		gotName, gotEmail = name, email
		return model.Participant{ID: pid, CalendarID: calendarID, Name: name, Email: email}, nil
	}
	s := newSvc(t, ms)
	p, err := s.RenameParticipant(context.Background(), "cal1", "anon", ParticipantCaller{Token: "tok-anon"}, " Ann ", strp("ann@example.com"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotName != "Ann" || gotEmail == nil || *gotEmail != "ann@example.com" {
		t.Fatalf("store got name=%q email=%v", gotName, gotEmail)
	}
	if p.Email == nil || *p.Email != "ann@example.com" {
		t.Fatalf("returned participant missing email: %+v", p)
	}
}

func TestRenameParticipant_InvalidEmailWritesNothing(t *testing.T) {
	ms := authFixture()
	ms.updateParticipantName = func(context.Context, string, string, string) (model.Participant, error) {
		t.Fatal("rename must not be written when the email is invalid")
		return model.Participant{}, nil
	}
	s := newSvc(t, ms)
	_, err := s.RenameParticipant(context.Background(), "cal1", "anon", ParticipantCaller{Token: "tok-anon"}, "Ann", strp("not-an-email"))
	if !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("err = %v, want ErrInvalidEmail", err)
	}
}

func TestRenameParticipant_SignedInCallerEmailIgnored(t *testing.T) {
	// Signed-in callers resolve email via their account: the email field is
	// ignored (not rejected) on a rename, as before.
	ms := authFixture()
	var renamed bool
	ms.updateParticipantName = func(_ context.Context, calendarID, pid, name string) (model.Participant, error) {
		renamed = true
		return model.Participant{ID: pid, CalendarID: calendarID, Name: name}, nil
	}
	s := newSvc(t, ms)
	_, err := s.RenameParticipant(context.Background(), "cal1", "linked", ParticipantCaller{UserID: strp("u1")}, "Lin", strp("whatever"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !renamed {
		t.Fatal("name-only write expected")
	}
}
