package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/service"
)

// Input rules live in the service (see service/validation_test.go). These
// tests pin the handler's half of the contract: request fields reach the
// service untouched, and a *service.ValidationError becomes a 400 carrying
// its stable code.

func TestSetAvailability_ForwardsUpdateToService(t *testing.T) {
	var got service.AvailabilityUpdate
	h := New(&mockService{
		setAvailability: func(_ context.Context, _, _ string, u service.AvailabilityUpdate) error {
			got = u
			return nil
		},
	})
	body := map[string]any{
		"participant_id": "p1",
		"dates":          []string{"2026-05-01"},
		"notes":          map[string]string{"2026-05-01": " x "},
		"preferred":      []string{"2026-05-01"},
	}
	req := httptest.NewRequest(http.MethodPut, "/calendars/cal1/availability", jsonBody(t, body))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.SetAvailability(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got.Notes == nil || (*got.Notes)["2026-05-01"] != " x " {
		t.Fatalf("notes not forwarded verbatim: %+v", got.Notes)
	}
	if got.Preferred == nil || len(*got.Preferred) != 1 {
		t.Fatalf("preferred not forwarded: %+v", got.Preferred)
	}
}

func TestSetAvailability_AbsentNotesAndPreferredForwardNil(t *testing.T) {
	var got service.AvailabilityUpdate
	h := New(&mockService{
		setAvailability: func(_ context.Context, _, _ string, u service.AvailabilityUpdate) error {
			got = u
			return nil
		},
	})
	body := map[string]any{"participant_id": "p1", "dates": []string{"2026-05-01"}}
	req := httptest.NewRequest(http.MethodPut, "/calendars/cal1/availability", jsonBody(t, body))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.SetAvailability(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got.Notes != nil || got.Preferred != nil {
		t.Fatalf("absent fields must stay nil (don't-touch): notes=%v preferred=%v", got.Notes, got.Preferred)
	}
}

func TestSetAvailability_ValidationErrorIs400WithCode(t *testing.T) {
	h := New(&mockService{
		setAvailability: func(_ context.Context, _, _ string, _ service.AvailabilityUpdate) error {
			return &service.ValidationError{Code: "note_too_long", Message: "note exceeds maximum length"}
		},
	})
	body := map[string]any{"participant_id": "p1", "dates": []string{"2026-05-01"}}
	req := httptest.NewRequest(http.MethodPut, "/calendars/cal1/availability", jsonBody(t, body))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.SetAvailability(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	assertErrorCode(t, rec, "note_too_long")
}

func TestImportAvailability_ValidationErrorIs400WithCode(t *testing.T) {
	h := New(&mockService{importErr: &service.ValidationError{Code: "too_many_dates", Message: "too many dates in a single submission"}})
	req := withUser(httptest.NewRequest(http.MethodPost, "/calendars/cal1/availability/import",
		jsonBody(t, map[string]any{"participant_id": "p1", "days": []string{"2026-05-01"}})), testUser)
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.ImportAvailability(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	assertErrorCode(t, rec, "too_many_dates")
}

func TestRenameParticipant_ValidationErrorIs400WithCode(t *testing.T) {
	var gotName string
	h := New(&mockService{
		renameParticipant: func(_ context.Context, _, _ string, _ service.ParticipantCaller, name string) (model.Participant, error) {
			gotName = name
			return model.Participant{}, &service.ValidationError{Code: "name_too_long", Message: "participant name is too long", Err: service.ErrInvalidName}
		},
	})
	req := httptest.NewRequest(http.MethodPatch, "/calendars/cal1/participants/p1", jsonBody(t, map[string]any{"name": "  Bob  "}))
	req.SetPathValue("id", "cal1")
	req.SetPathValue("participantId", "p1")
	rec := httptest.NewRecorder()
	h.RenameParticipant(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	assertErrorCode(t, rec, "name_too_long")
	if gotName != "  Bob  " {
		t.Fatalf("name should reach the service untrimmed (service owns trimming), got %q", gotName)
	}
}

func TestRemoveParticipant_SoftDeletedCalendarIs410(t *testing.T) {
	h := New(&softDeletedMock{mockService: mockService{
		getCalendarOnly: func(_ context.Context, _ string) (model.Calendar, error) {
			return model.Calendar{}, model.ErrNotFound
		},
	}})
	req := withUser(httptest.NewRequest(http.MethodDelete, "/calendars/gone/participants/p1", nil), testUser)
	req.SetPathValue("id", "gone")
	req.SetPathValue("participantId", "p1")
	rec := httptest.NewRecorder()
	h.RemoveParticipant(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want 410", rec.Code)
	}
	assertErrorCode(t, rec, "calendar_unavailable")
}

func TestRenameParticipant_NameAndEmailIsOneServiceCall(t *testing.T) {
	m := &mockService{}
	h := New(m)
	req := httptest.NewRequest(http.MethodPatch, "/calendars/cal1/participants/p1",
		jsonBody(t, map[string]any{"name": "Bob", "email": "bob@example.com"}))
	req.SetPathValue("id", "cal1")
	req.SetPathValue("participantId", "p1")
	rec := httptest.NewRecorder()
	h.RenameParticipant(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if m.lastRenameEmail == nil || *m.lastRenameEmail != "bob@example.com" {
		t.Fatalf("email not passed to RenameParticipant: %v", m.lastRenameEmail)
	}
	if m.updateEmailCalls != 0 {
		t.Fatalf("UpdateParticipantEmail called %d times; rename+email must be a single write", m.updateEmailCalls)
	}
}

func TestRenameParticipant_InvalidEmailIs400(t *testing.T) {
	h := New(&mockService{
		renameParticipant: func(context.Context, string, string, service.ParticipantCaller, string) (model.Participant, error) {
			return model.Participant{}, service.ErrInvalidEmail
		},
	})
	req := httptest.NewRequest(http.MethodPatch, "/calendars/cal1/participants/p1",
		jsonBody(t, map[string]any{"name": "Bob", "email": "nope"}))
	req.SetPathValue("id", "cal1")
	req.SetPathValue("participantId", "p1")
	rec := httptest.NewRecorder()
	h.RenameParticipant(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	assertErrorCode(t, rec, "invalid_email")
}

func TestUpdateParticipantEmail_SignedInIs400WithoutServiceCall(t *testing.T) {
	m := &mockService{}
	h := New(m)
	req := withUser(httptest.NewRequest(http.MethodPatch, "/calendars/cal1/participants/p1",
		jsonBody(t, map[string]any{"email": "me@example.com"})), testUser)
	req.SetPathValue("id", "cal1")
	req.SetPathValue("participantId", "p1")
	rec := httptest.NewRecorder()
	h.RenameParticipant(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	assertErrorCode(t, rec, "invalid_request")
	if m.updateEmailCalls != 0 {
		t.Fatal("service must not be called")
	}
}

// softDeletedMock reports every calendar as soft-deleted.
type softDeletedMock struct{ mockService }

func (m *softDeletedMock) IsCalendarSoftDeleted(context.Context, string) (bool, error) {
	return true, nil
}
