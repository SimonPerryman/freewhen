package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/service"
)

func TestAddParticipant_ReturnsParticipantTokenHeader(t *testing.T) {
	h := New(&mockService{
		addParticipant: func(_ context.Context, calendarID, name string, _ *string) (model.Participant, error) {
			return model.Participant{ID: "p1", CalendarID: calendarID, Name: name}, nil
		},
		addParticipantToken: "raw-token",
	})
	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/participants", jsonBody(t, map[string]string{"name": "Alice"}))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.AddParticipant(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get(ParticipantTokenHeader); got != "raw-token" {
		t.Fatalf("%s = %q, want raw-token", ParticipantTokenHeader, got)
	}
	if strings.Contains(rec.Body.String(), "raw-token") {
		t.Fatal("token must not be echoed in the JSON body")
	}
}

func TestAddParticipant_NoTokenHeaderWhenNoneIssued(t *testing.T) {
	h := New(&mockService{
		addParticipant: func(_ context.Context, calendarID, name string, _ *string) (model.Participant, error) {
			return model.Participant{ID: "p1", CalendarID: calendarID, Name: name}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/participants", jsonBody(t, map[string]string{"name": "Alice"}))
	req.SetPathValue("id", "cal1")
	req = withUser(req, testUser)
	rec := httptest.NewRecorder()
	h.AddParticipant(rec, req)
	if _, ok := rec.Header()[ParticipantTokenHeader]; ok {
		t.Fatal("no token header expected for signed-in joiners")
	}
}

func TestSetAvailability_ForwardsCallerCredentials(t *testing.T) {
	ms := &mockService{
		setAvailability: func(_ context.Context, _, _ string, _ []string, _ map[string]string, _ map[string]bool) error {
			return nil
		},
	}
	h := New(ms)
	req := httptest.NewRequest(http.MethodPut, "/calendars/cal1/availability",
		jsonBody(t, map[string]any{"participant_id": "p1", "dates": []string{}}))
	req.SetPathValue("id", "cal1")
	req.Header.Set(ParticipantTokenHeader, "tok")
	req = withUser(req, testUser)
	rec := httptest.NewRecorder()
	h.SetAvailability(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ms.lastCaller == nil || ms.lastCaller.Token != "tok" || ms.lastCaller.UserID == nil || *ms.lastCaller.UserID != testUser.ID {
		t.Fatalf("caller = %+v, want token+user forwarded", ms.lastCaller)
	}
}

func TestSetAvailability_ForbiddenIs403(t *testing.T) {
	h := New(&mockService{
		setAvailability: func(_ context.Context, _, _ string, _ []string, _ map[string]string, _ map[string]bool) error {
			return service.ErrForbidden
		},
	})
	req := httptest.NewRequest(http.MethodPut, "/calendars/cal1/availability",
		jsonBody(t, map[string]any{"participant_id": "someone-else", "dates": []string{}}))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.SetAvailability(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestImportAvailability_ForbiddenIs403(t *testing.T) {
	h := New(&mockService{importErr: service.ErrForbidden})
	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/availability/import",
		jsonBody(t, map[string]any{"participant_id": "someone-else", "days": []string{}}))
	req.SetPathValue("id", "cal1")
	req = withUser(req, testUser)
	rec := httptest.NewRecorder()
	h.ImportAvailability(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestRenameParticipant_ForbiddenIs403AndIgnoresBodyParticipantID(t *testing.T) {
	ms := &mockService{
		renameParticipant: func(_ context.Context, _, _ string, caller service.ParticipantCaller, _ string) (model.Participant, error) {
			if caller.Token != "" || caller.UserID != nil {
				t.Fatalf("unexpected caller %+v", caller)
			}
			return model.Participant{}, service.ErrForbidden
		},
	}
	h := New(ms)
	// Spoofed body participant_id must not grant anything.
	req := httptest.NewRequest(http.MethodPatch, "/calendars/cal1/participants/p1",
		jsonBody(t, map[string]any{"name": "Mallory", "participant_id": "p1"}))
	req.SetPathValue("id", "cal1")
	req.SetPathValue("participantId", "p1")
	rec := httptest.NewRecorder()
	h.RenameParticipant(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestClaimParticipant_ScrubsEmailAndForwardsToken(t *testing.T) {
	ms := &mockService{
		claimParticipant: func(_ context.Context, calendarID, pid, userID string) (model.Participant, error) {
			e := "anon@example.com"
			return model.Participant{ID: pid, CalendarID: calendarID, UserID: &userID, Email: &e}, nil
		},
	}
	h := New(ms)
	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/participants/p1/claim", nil)
	req.SetPathValue("id", "cal1")
	req.SetPathValue("participantId", "p1")
	req.Header.Set(ParticipantTokenHeader, "tok")
	req = withUser(req, testUser)
	rec := httptest.NewRecorder()
	h.ClaimParticipant(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "anon@example.com") {
		t.Fatalf("claim response leaked email: %s", rec.Body.String())
	}
	if ms.lastCaller == nil || ms.lastCaller.Token != "tok" {
		t.Fatalf("token not forwarded: %+v", ms.lastCaller)
	}
}

func TestClaimParticipant_RequiresCalendarAccess(t *testing.T) {
	h := New(&mockService{
		authorizeAccess: func(_ context.Context, _ model.Calendar, _ service.Viewer, _ string) error {
			return service.ErrPassphraseRequired
		},
		claimParticipant: func(_ context.Context, _, _, _ string) (model.Participant, error) {
			t.Fatal("claim must not run when the passphrase gate rejects")
			return model.Participant{}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/participants/p1/claim", nil)
	req.SetPathValue("id", "cal1")
	req.SetPathValue("participantId", "p1")
	req = withUser(req, testUser)
	rec := httptest.NewRecorder()
	h.ClaimParticipant(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestClaimParticipant_ForbiddenIs403(t *testing.T) {
	h := New(&mockService{
		claimParticipant: func(_ context.Context, _, _, _ string) (model.Participant, error) {
			return model.Participant{}, service.ErrForbidden
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/participants/p1/claim", nil)
	req.SetPathValue("id", "cal1")
	req.SetPathValue("participantId", "p1")
	req = withUser(req, testUser)
	rec := httptest.NewRecorder()
	h.ClaimParticipant(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}
