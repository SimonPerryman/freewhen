package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/service"
	"github.com/freewhen/backend/internal/store"
)

// inviteMock extends mockService with invite-specific overrides used by the
// ListMissing/AddInvites/... handler tests. It embeds mockService so the
// unrelated CalendarService methods retain their zero-value stubs.
type inviteMock struct {
	mockService
	getMissingView func(ctx context.Context, calendarID string) (service.MissingView, error)
	addInvitesFn   func(ctx context.Context, calendarID string, names []string) (service.AddInvitesResult, error)
	renameInviteFn func(ctx context.Context, inviteID, name string) (model.Invite, error)
	deleteInviteFn func(ctx context.Context, inviteID string) error
	manualMatchFn  func(ctx context.Context, calendarID, inviteID string, participantID *string) (model.Invite, error)
	getInviteFn    func(ctx context.Context, inviteID string) (model.Invite, error)
}

func (m *inviteMock) GetMissingView(ctx context.Context, id string) (service.MissingView, error) {
	if m.getMissingView != nil {
		return m.getMissingView(ctx, id)
	}
	return service.MissingView{}, nil
}
func (m *inviteMock) AddInvites(ctx context.Context, id string, names []string) (service.AddInvitesResult, error) {
	if m.addInvitesFn != nil {
		return m.addInvitesFn(ctx, id, names)
	}
	return service.AddInvitesResult{}, nil
}
func (m *inviteMock) RenameInvite(ctx context.Context, id, name string) (model.Invite, error) {
	if m.renameInviteFn != nil {
		return m.renameInviteFn(ctx, id, name)
	}
	return model.Invite{}, nil
}
func (m *inviteMock) DeleteInvite(ctx context.Context, id string) error {
	if m.deleteInviteFn != nil {
		return m.deleteInviteFn(ctx, id)
	}
	return nil
}
func (m *inviteMock) ManualMatchInvite(ctx context.Context, calID, id string, pid *string) (model.Invite, error) {
	if m.manualMatchFn != nil {
		return m.manualMatchFn(ctx, calID, id, pid)
	}
	return model.Invite{}, nil
}
func (m *inviteMock) GetInvite(ctx context.Context, id string) (model.Invite, error) {
	if m.getInviteFn != nil {
		return m.getInviteFn(ctx, id)
	}
	return model.Invite{ID: id, CalendarID: "cal1"}, nil
}

func newInviteReq(t *testing.T, method, path, calendarID string, body any, user *model.User) *http.Request {
	t.Helper()
	var b *bytes.Reader
	if body != nil {
		b = jsonBody(t, body)
	} else {
		b = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, b)
	req.SetPathValue("id", calendarID)
	if user != nil {
		req = withUser(req, user)
	}
	return req
}

func TestListMissing_Unauthorized(t *testing.T) {
	h := New(&inviteMock{})
	req := newInviteReq(t, http.MethodGet, "/calendars/cal1/missing", "cal1", nil, nil)
	rec := httptest.NewRecorder()
	h.ListMissing(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestListMissing_Forbidden_NonOwner(t *testing.T) {
	m := &inviteMock{}
	m.getCalendarOnly = func(_ context.Context, id string) (model.Calendar, error) {
		return model.Calendar{ID: id, OwnerID: "other"}, nil
	}
	h := New(m)
	req := newInviteReq(t, http.MethodGet, "/calendars/cal1/missing", "cal1", nil, testUser)
	rec := httptest.NewRecorder()
	h.ListMissing(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestListMissing_OwnerHappyPath(t *testing.T) {
	m := &inviteMock{
		getMissingView: func(_ context.Context, _ string) (service.MissingView, error) {
			return service.MissingView{
				Invites:            []model.Invite{{ID: "i1", Name: "Alice"}},
				NotJoined:          []service.NotJoinedEntry{{InviteID: "i1", Name: "Alice"}},
				JoinedNotResponded: []service.JoinedEntry{},
			}, nil
		},
	}
	m.getCalendarOnly = func(_ context.Context, id string) (model.Calendar, error) {
		return model.Calendar{ID: id, OwnerID: testUser.ID}, nil
	}
	h := New(m)
	req := newInviteReq(t, http.MethodGet, "/calendars/cal1/missing", "cal1", nil, testUser)
	rec := httptest.NewRecorder()
	h.ListMissing(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestAddInvites_NotFoundCalendar(t *testing.T) {
	m := &inviteMock{}
	m.getCalendarOnly = func(_ context.Context, _ string) (model.Calendar, error) {
		return model.Calendar{}, store.ErrNotFound
	}
	h := New(m)
	req := newInviteReq(t, http.MethodPost, "/calendars/cal1/invites", "cal1",
		map[string]any{"names": []string{"Alice"}}, testUser)
	rec := httptest.NewRecorder()
	h.AddInvites(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestAddInvites_LimitReached(t *testing.T) {
	m := &inviteMock{
		addInvitesFn: func(_ context.Context, _ string, _ []string) (service.AddInvitesResult, error) {
			return service.AddInvitesResult{}, store.ErrInviteLimitReached
		},
	}
	m.getCalendarOnly = func(_ context.Context, id string) (model.Calendar, error) {
		return model.Calendar{ID: id, OwnerID: testUser.ID}, nil
	}
	h := New(m)
	req := newInviteReq(t, http.MethodPost, "/calendars/cal1/invites", "cal1",
		map[string]any{"names": []string{"Alice"}}, testUser)
	rec := httptest.NewRecorder()
	h.AddInvites(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestAddInvites_NamesRequired(t *testing.T) {
	m := &inviteMock{}
	m.getCalendarOnly = func(_ context.Context, id string) (model.Calendar, error) {
		return model.Calendar{ID: id, OwnerID: testUser.ID}, nil
	}
	h := New(m)
	req := newInviteReq(t, http.MethodPost, "/calendars/cal1/invites", "cal1",
		map[string]any{"names": []string{}}, testUser)
	rec := httptest.NewRecorder()
	h.AddInvites(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestManualMatchInvite_ParticipantNotOnCalendar(t *testing.T) {
	m := &inviteMock{
		manualMatchFn: func(_ context.Context, _, _ string, _ *string) (model.Invite, error) {
			return model.Invite{}, service.ErrParticipantNotOnCalendar
		},
	}
	m.getCalendarOnly = func(_ context.Context, id string) (model.Calendar, error) {
		return model.Calendar{ID: id, OwnerID: testUser.ID}, nil
	}
	h := New(m)
	pid := "p1"
	req := newInviteReq(t, http.MethodPost, "/calendars/cal1/invites/i1/match", "cal1",
		map[string]any{"participant_id": pid}, testUser)
	req.SetPathValue("inviteId", "i1")
	rec := httptest.NewRecorder()
	h.ManualMatchInvite(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
