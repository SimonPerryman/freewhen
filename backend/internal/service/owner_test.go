package service

import (
	"context"
	"errors"
	"testing"

	"github.com/freewhen/backend/internal/model"
)

// ownerOps runs every owner-only service method as callerID against "cal1".
// The default mockStore panics on the write paths (SetFinalisedDate,
// UpdateCalendarName, DeleteParticipant, SetCalendarPassphrase, ...), so a
// rejected call that reached the store would fail the test loudly.
func ownerOps(s *Service, callerID string) map[string]func() error {
	ctx := context.Background()
	pid := "p1"
	return map[string]func() error{
		"DuplicateCalendar": func() error {
			_, _, err := s.DuplicateCalendar(ctx, "cal1", callerID, "Name", "", true)
			return err
		},
		"FinaliseCalendar": func() error {
			_, err := s.FinaliseCalendar(ctx, "cal1", callerID, testDate)
			return err
		},
		"UnfinaliseCalendar": func() error {
			_, err := s.UnfinaliseCalendar(ctx, "cal1", callerID)
			return err
		},
		"RenameCalendar": func() error {
			_, err := s.RenameCalendar(ctx, "cal1", callerID, "New")
			return err
		},
		"RemoveParticipant": func() error {
			return s.RemoveParticipant(ctx, "cal1", callerID, "p1")
		},
		"RotateCalendarPassphrase": func() error {
			_, err := s.RotateCalendarPassphrase(ctx, "cal1", callerID)
			return err
		},
		"ClearCalendarPassphrase": func() error {
			return s.ClearCalendarPassphrase(ctx, "cal1", callerID)
		},
		"GetMissingView": func() error {
			_, err := s.GetMissingView(ctx, "cal1", callerID)
			return err
		},
		"AddInvites": func() error {
			_, err := s.AddInvites(ctx, "cal1", callerID, []string{"Alice"})
			return err
		},
		"RenameInvite": func() error {
			_, err := s.RenameInvite(ctx, "cal1", callerID, "i1", "Bob")
			return err
		},
		"DeleteInvite": func() error {
			return s.DeleteInvite(ctx, "cal1", callerID, "i1")
		},
		"ManualMatchInvite": func() error {
			_, err := s.ManualMatchInvite(ctx, "cal1", callerID, "i1", &pid)
			return err
		},
		"DeleteCalendar": func() error {
			return s.DeleteCalendar(ctx, "cal1", callerID)
		},
		"AuthorizeCalendarOwner": func() error {
			return s.AuthorizeCalendarOwner(ctx, "cal1", callerID)
		},
	}
}

func TestOwnerOnly_NonOwnerForbidden(t *testing.T) {
	// mockStore's default calendar is owned by "owner1".
	for name, op := range ownerOps(newSvc(t, &mockStore{}), "intruder") {
		t.Run(name, func(t *testing.T) {
			if err := op(); !errors.Is(err, ErrForbidden) {
				t.Fatalf("err = %v, want ErrForbidden", err)
			}
		})
	}
}

func TestOwnerOnly_AnonymousForbidden(t *testing.T) {
	for name, op := range ownerOps(newSvc(t, &mockStore{}), "") {
		t.Run(name, func(t *testing.T) {
			if err := op(); !errors.Is(err, ErrForbidden) {
				t.Fatalf("err = %v, want ErrForbidden", err)
			}
		})
	}
}

func TestOwnerOnly_MissingCalendar(t *testing.T) {
	ms := &mockStore{
		getCalendarOnly: func(context.Context, string) (model.Calendar, error) {
			return model.Calendar{}, model.ErrNotFound
		},
	}
	for name, op := range ownerOps(newSvc(t, ms), "owner1") {
		t.Run(name, func(t *testing.T) {
			err := op()
			if !errors.Is(err, model.ErrCalendarNotFound) || !errors.Is(err, model.ErrNotFound) {
				t.Fatalf("err = %v, want ErrCalendarNotFound wrapping ErrNotFound", err)
			}
		})
	}
}

func TestRequireOwner_PropagatesStoreError(t *testing.T) {
	boom := errors.New("db down")
	ms := &mockStore{
		getCalendarOnly: func(context.Context, string) (model.Calendar, error) { return model.Calendar{}, boom },
	}
	err := newSvc(t, ms).AuthorizeCalendarOwner(context.Background(), "cal1", "owner1")
	if !errors.Is(err, boom) || errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want the store error untouched", err)
	}
}

func TestInviteOps_InviteOnOtherCalendarIsNotFound(t *testing.T) {
	ms := &mockStore{
		getInvite: func(id string) (model.Invite, error) {
			return model.Invite{ID: id, CalendarID: "other-cal"}, nil
		},
		updateInviteName: func(string, string) (model.Invite, error) {
			t.Fatal("rename must not run for a foreign invite")
			return model.Invite{}, nil
		},
		deleteInvite: func() error {
			t.Fatal("delete must not run for a foreign invite")
			return nil
		},
	}
	s := newSvc(t, ms)
	ctx := context.Background()
	if _, err := s.RenameInvite(ctx, "cal1", "owner1", "i1", "Bob"); !errors.Is(err, model.ErrInviteNotFound) {
		t.Fatalf("rename err = %v, want ErrInviteNotFound", err)
	}
	if err := s.DeleteInvite(ctx, "cal1", "owner1", "i1"); !errors.Is(err, model.ErrInviteNotFound) {
		t.Fatalf("delete err = %v, want ErrInviteNotFound", err)
	}
	if _, err := s.ManualMatchInvite(ctx, "cal1", "owner1", "i1", nil); !errors.Is(err, model.ErrInviteNotFound) {
		t.Fatalf("match err = %v, want ErrInviteNotFound", err)
	}
}

func TestDeleteInvite_ConcurrentDeleteIsSuccess(t *testing.T) {
	ms := &mockStore{
		getInvite: func(id string) (model.Invite, error) {
			return model.Invite{ID: id, CalendarID: "cal1"}, nil
		},
		deleteInvite: func() error { return model.ErrInviteNotFound },
	}
	if err := newSvc(t, ms).DeleteInvite(context.Background(), "cal1", "owner1", "i1"); err != nil {
		t.Fatalf("err = %v, want nil (idempotent delete)", err)
	}
}

func TestAddInvites_NamesRequiredAfterOwnerCheck(t *testing.T) {
	s := newSvc(t, &mockStore{})
	_, err := s.AddInvites(context.Background(), "cal1", "owner1", nil)
	if validationCode(t, err) != "names_required" {
		t.Fatalf("err = %v", err)
	}
	// A non-owner learns nothing about input rules.
	if _, err := s.AddInvites(context.Background(), "cal1", "intruder", nil); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-owner err = %v, want ErrForbidden", err)
	}
}
