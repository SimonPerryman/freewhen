package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/service"
	"github.com/freewhen/backend/internal/store"
)

// mockService implements CalendarService for testing.
type mockService struct {
	createCalendar           func(ctx context.Context, name, ownerID, visibility string) (model.Calendar, error)
	getCalendar              func(ctx context.Context, id string) (model.CalendarWithDetails, error)
	getCalendarView          func(ctx context.Context, id string, viewer service.Viewer) (service.CalendarView, error)
	getCalendarOnly          func(ctx context.Context, id string) (model.Calendar, error)
	addParticipant           func(ctx context.Context, calendarID, name string, userID *string) (model.Participant, error)
	setAvailability          func(ctx context.Context, calendarID, participantID string, dates []string) error
	finaliseCalendar         func(ctx context.Context, calendarID, date string) (model.Calendar, error)
	unfinaliseCalendar       func(ctx context.Context, calendarID string) (model.Calendar, error)
	claimParticipant         func(ctx context.Context, calendarID, participantID, userID string) (model.Participant, error)
	renameCalendar           func(ctx context.Context, calendarID, name string) (model.Calendar, error)
	updateCalendarVisibility func(ctx context.Context, calendarID, visibility string) (model.Calendar, error)
	removeParticipant        func(ctx context.Context, calendarID, participantID string) error
}

func (m *mockService) CreateCalendar(ctx context.Context, name, ownerID, visibility string) (model.Calendar, error) {
	return m.createCalendar(ctx, name, ownerID, visibility)
}

func (m *mockService) GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error) {
	return m.getCalendar(ctx, id)
}

func (m *mockService) GetCalendarView(ctx context.Context, id string, viewer service.Viewer) (service.CalendarView, error) {
	return m.getCalendarView(ctx, id, viewer)
}

func (m *mockService) GetCalendarOnly(ctx context.Context, id string) (model.Calendar, error) {
	return m.getCalendarOnly(ctx, id)
}

func (m *mockService) AddParticipant(ctx context.Context, calendarID, name string, userID *string) (model.Participant, error) {
	return m.addParticipant(ctx, calendarID, name, userID)
}

func (m *mockService) ClaimParticipant(ctx context.Context, calendarID, participantID, userID string) (model.Participant, error) {
	return m.claimParticipant(ctx, calendarID, participantID, userID)
}

func (m *mockService) SetAvailability(ctx context.Context, calendarID, participantID string, dates []string) error {
	return m.setAvailability(ctx, calendarID, participantID, dates)
}

func (m *mockService) FinaliseCalendar(ctx context.Context, calendarID, date string) (model.Calendar, error) {
	return m.finaliseCalendar(ctx, calendarID, date)
}

func (m *mockService) UnfinaliseCalendar(ctx context.Context, calendarID string) (model.Calendar, error) {
	return m.unfinaliseCalendar(ctx, calendarID)
}

func (m *mockService) RenameCalendar(ctx context.Context, calendarID, name string) (model.Calendar, error) {
	return m.renameCalendar(ctx, calendarID, name)
}

func (m *mockService) UpdateCalendarVisibility(ctx context.Context, calendarID, visibility string) (model.Calendar, error) {
	return m.updateCalendarVisibility(ctx, calendarID, visibility)
}

func (m *mockService) RemoveParticipant(ctx context.Context, calendarID, participantID string) error {
	return m.removeParticipant(ctx, calendarID, participantID)
}

func jsonBody(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(b)
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// --- CreateCalendar tests ---

func withUser(r *http.Request, u *model.User) *http.Request {
	ctx := context.WithValue(r.Context(), UserContextKey, u)
	return r.WithContext(ctx)
}

var testUser = &model.User{ID: "u1", Email: "test@example.com", Name: "Test"}

func TestCreateCalendar_Valid(t *testing.T) {
	cal := model.Calendar{ID: "abc12345", Name: "Trip", OwnerID: "u1", CreatedAt: time.Now()}
	ownerParticipant := model.Participant{ID: "p1", CalendarID: "abc12345", Name: "Test", Colour: "#E63946", UserID: strPtr("u1")}
	h := New(&mockService{
		createCalendar: func(_ context.Context, name, ownerID, visibility string) (model.Calendar, error) {
			if name != "Trip" {
				t.Fatalf("expected name %q, got %q", "Trip", name)
			}
			if ownerID != "u1" {
				t.Fatalf("expected ownerID %q, got %q", "u1", ownerID)
			}
			if visibility != model.VisibilityOpen {
				t.Fatalf("expected visibility %q, got %q", model.VisibilityOpen, visibility)
			}
			return cal, nil
		},
		addParticipant: func(_ context.Context, calendarID, name string, userID *string) (model.Participant, error) {
			if calendarID != "abc12345" {
				t.Fatalf("expected calendarID %q, got %q", "abc12345", calendarID)
			}
			if userID == nil || *userID != "u1" {
				t.Fatalf("expected userID u1, got %v", userID)
			}
			return ownerParticipant, nil
		},
	})

	req := withUser(httptest.NewRequest(http.MethodPost, "/calendars", jsonBody(t, map[string]string{"name": "Trip"})), testUser)
	rec := httptest.NewRecorder()
	h.CreateCalendar(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var got struct {
		ID          string            `json:"id"`
		Name        string            `json:"name"`
		Participant *model.Participant `json:"participant"`
	}
	decodeJSON(t, rec, &got)
	if got.ID != cal.ID || got.Name != cal.Name {
		t.Fatalf("body id=%s name=%s, want id=%s name=%s", got.ID, got.Name, cal.ID, cal.Name)
	}
	if got.Participant == nil {
		t.Fatal("expected participant in response")
	}
	if got.Participant.UserID == nil || *got.Participant.UserID != "u1" {
		t.Fatalf("expected participant user_id u1, got %v", got.Participant.UserID)
	}
}

func strPtr(s string) *string { return &s }

func TestCreateCalendar_Unauthorized(t *testing.T) {
	h := New(&mockService{})
	req := httptest.NewRequest(http.MethodPost, "/calendars", jsonBody(t, map[string]string{"name": "Trip"}))
	rec := httptest.NewRecorder()
	h.CreateCalendar(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCreateCalendar_MalformedJSON(t *testing.T) {
	h := New(&mockService{})
	req := withUser(httptest.NewRequest(http.MethodPost, "/calendars", bytes.NewReader([]byte("{bad"))), testUser)
	rec := httptest.NewRecorder()
	h.CreateCalendar(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateCalendar_ServiceError(t *testing.T) {
	h := New(&mockService{
		createCalendar: func(_ context.Context, _, _, _ string) (model.Calendar, error) {
			return model.Calendar{}, fmt.Errorf("db down")
		},
	})
	req := withUser(httptest.NewRequest(http.MethodPost, "/calendars", jsonBody(t, map[string]string{"name": "X"})), testUser)
	rec := httptest.NewRecorder()
	h.CreateCalendar(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- GetCalendar tests ---

func TestGetCalendar_Valid(t *testing.T) {
	details := model.CalendarWithDetails{
		Calendar:     model.Calendar{ID: "abc12345", Name: "Trip"},
		Participants: []model.Participant{},
		Availability: []model.Availability{},
	}
	h := New(&mockService{
		getCalendarView: func(_ context.Context, id string, _ service.Viewer) (service.CalendarView, error) {
			if id != "abc12345" {
				t.Fatalf("expected id %q, got %q", "abc12345", id)
			}
			return service.CalendarView{Details: details, Revealed: true}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/calendars/abc12345", nil)
	req.SetPathValue("id", "abc12345")
	rec := httptest.NewRecorder()
	h.GetCalendar(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestGetCalendar_NotFound(t *testing.T) {
	h := New(&mockService{
		getCalendarView: func(_ context.Context, _ string, _ service.Viewer) (service.CalendarView, error) {
			return service.CalendarView{}, store.ErrNotFound
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/calendars/missing", nil)
	req.SetPathValue("id", "missing")
	rec := httptest.NewRecorder()
	h.GetCalendar(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetCalendar_ServiceError(t *testing.T) {
	h := New(&mockService{
		getCalendarView: func(_ context.Context, _ string, _ service.Viewer) (service.CalendarView, error) {
			return service.CalendarView{}, fmt.Errorf("db down")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/calendars/x", nil)
	req.SetPathValue("id", "x")
	rec := httptest.NewRecorder()
	h.GetCalendar(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestGetCalendar_HiddenForUnsubmittedViewer(t *testing.T) {
	hidden := model.CalendarWithDetails{
		Calendar:         model.Calendar{ID: "c", Visibility: model.VisibilitySubmitToView},
		Participants:     []model.Participant{},
		Availability:     []model.Availability{},
		ParticipantCount: 3,
	}
	h := New(&mockService{
		getCalendarView: func(_ context.Context, _ string, v service.Viewer) (service.CalendarView, error) {
			if v.ParticipantID == nil || *v.ParticipantID != "p1" {
				t.Fatalf("expected participant id p1, got %v", v.ParticipantID)
			}
			return service.CalendarView{Details: hidden, Revealed: false}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/calendars/c?as=p1", nil)
	req.SetPathValue("id", "c")
	rec := httptest.NewRecorder()
	h.GetCalendar(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got model.CalendarWithDetails
	decodeJSON(t, rec, &got)
	if got.ParticipantCount != 3 {
		t.Fatalf("participantCount = %d, want 3", got.ParticipantCount)
	}
	if len(got.Participants) != 0 || len(got.Availability) != 0 {
		t.Fatal("expected empty participants/availability in hidden view")
	}
}

// --- AddParticipant tests ---

func TestAddParticipant_Valid(t *testing.T) {
	p := model.Participant{ID: "p1", CalendarID: "cal1", Name: "Alice", Colour: "#E63946"}
	h := New(&mockService{
		addParticipant: func(_ context.Context, calendarID, name string, _ *string) (model.Participant, error) {
			if calendarID != "cal1" || name != "Alice" {
				t.Fatalf("unexpected args: calendarID=%q name=%q", calendarID, name)
			}
			return p, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/participants", jsonBody(t, map[string]string{"name": "Alice"}))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.AddParticipant(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var got model.Participant
	decodeJSON(t, rec, &got)
	if got.Name != "Alice" {
		t.Fatalf("name = %q, want Alice", got.Name)
	}
}

func TestAddParticipant_EmptyName(t *testing.T) {
	h := New(&mockService{})
	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/participants", jsonBody(t, map[string]string{"name": ""}))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.AddParticipant(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAddParticipant_MalformedJSON(t *testing.T) {
	h := New(&mockService{})
	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/participants", bytes.NewReader([]byte("not json")))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.AddParticipant(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAddParticipant_NotFound(t *testing.T) {
	h := New(&mockService{
		addParticipant: func(_ context.Context, _, _ string, _ *string) (model.Participant, error) {
			return model.Participant{}, store.ErrNotFound
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/calendars/missing/participants", jsonBody(t, map[string]string{"name": "Bob"}))
	req.SetPathValue("id", "missing")
	rec := httptest.NewRecorder()
	h.AddParticipant(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAddParticipant_DuplicateName(t *testing.T) {
	h := New(&mockService{
		addParticipant: func(_ context.Context, _, _ string, _ *string) (model.Participant, error) {
			return model.Participant{}, store.ErrDuplicateName
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/participants", jsonBody(t, map[string]string{"name": "Alice"}))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.AddParticipant(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestAddParticipant_ServiceError(t *testing.T) {
	h := New(&mockService{
		addParticipant: func(_ context.Context, _, _ string, _ *string) (model.Participant, error) {
			return model.Participant{}, fmt.Errorf("db down")
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/participants", jsonBody(t, map[string]string{"name": "Bob"}))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.AddParticipant(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- SetAvailability tests ---

func TestSetAvailability_Valid(t *testing.T) {
	h := New(&mockService{
		setAvailability: func(_ context.Context, calendarID, participantID string, dates []string) error {
			if calendarID != "cal1" || participantID != "p1" {
				t.Fatalf("unexpected args: calendarID=%q participantID=%q", calendarID, participantID)
			}
			if len(dates) != 2 || dates[0] != "2026-05-01" || dates[1] != "2026-05-02" {
				t.Fatalf("unexpected dates: %v", dates)
			}
			return nil
		},
	})

	body := map[string]any{
		"participant_id": "p1",
		"dates":          []string{"2026-05-01", "2026-05-02"},
	}
	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/availability", jsonBody(t, body))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.SetAvailability(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestSetAvailability_EmptyParticipantID(t *testing.T) {
	h := New(&mockService{})
	body := map[string]any{"participant_id": "", "dates": []string{"2026-05-01"}}
	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/availability", jsonBody(t, body))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.SetAvailability(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestSetAvailability_MalformedJSON(t *testing.T) {
	h := New(&mockService{})
	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/availability", bytes.NewReader([]byte("{nope")))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.SetAvailability(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestSetAvailability_InvalidDateFormat(t *testing.T) {
	h := New(&mockService{})
	body := map[string]any{"participant_id": "p1", "dates": []string{"05/01/2026"}}
	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/availability", jsonBody(t, body))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.SetAvailability(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestSetAvailability_NotFound(t *testing.T) {
	h := New(&mockService{
		setAvailability: func(_ context.Context, _, _ string, _ []string) error {
			return store.ErrNotFound
		},
	})

	body := map[string]any{"participant_id": "p1", "dates": []string{"2026-05-01"}}
	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/availability", jsonBody(t, body))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.SetAvailability(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestSetAvailability_ServiceError(t *testing.T) {
	h := New(&mockService{
		setAvailability: func(_ context.Context, _, _ string, _ []string) error {
			return fmt.Errorf("db down")
		},
	})

	body := map[string]any{"participant_id": "p1", "dates": []string{"2026-05-01"}}
	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/availability", jsonBody(t, body))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.SetAvailability(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestSetAvailability_Finalised(t *testing.T) {
	h := New(&mockService{
		setAvailability: func(_ context.Context, _, _ string, _ []string) error {
			return store.ErrCalendarFinalised
		},
	})
	body := map[string]any{"participant_id": "p1", "dates": []string{"2026-05-01"}}
	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/availability", jsonBody(t, body))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.SetAvailability(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestAddParticipant_Finalised(t *testing.T) {
	h := New(&mockService{
		addParticipant: func(_ context.Context, _, _ string, _ *string) (model.Participant, error) {
			return model.Participant{}, store.ErrCalendarFinalised
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/participants", jsonBody(t, map[string]string{"name": "Bob"}))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.AddParticipant(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

// --- FinaliseCalendar tests ---

func TestFinaliseCalendar_Valid(t *testing.T) {
	date := "2026-05-01"
	updated := model.Calendar{ID: "cal1", Name: "Trip", OwnerID: "u1", FinalisedDate: &date}
	h := New(&mockService{
		getCalendarOnly: func(_ context.Context, id string) (model.Calendar, error) {
			return model.Calendar{ID: id, OwnerID: "u1"}, nil
		},
		finaliseCalendar: func(_ context.Context, calendarID, d string) (model.Calendar, error) {
			if calendarID != "cal1" || d != "2026-05-01" {
				t.Fatalf("unexpected args: calendarID=%q date=%q", calendarID, d)
			}
			return updated, nil
		},
	})

	req := withUser(httptest.NewRequest(http.MethodPost, "/calendars/cal1/finalise", jsonBody(t, map[string]string{"date": "2026-05-01"})), testUser)
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.FinaliseCalendar(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got model.Calendar
	decodeJSON(t, rec, &got)
	if got.FinalisedDate == nil || *got.FinalisedDate != "2026-05-01" {
		t.Fatalf("finalised_date = %v, want 2026-05-01", got.FinalisedDate)
	}
}

func TestFinaliseCalendar_Unauthorized(t *testing.T) {
	h := New(&mockService{})
	req := httptest.NewRequest(http.MethodPost, "/calendars/cal1/finalise", jsonBody(t, map[string]string{"date": "2026-05-01"}))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.FinaliseCalendar(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestFinaliseCalendar_NotOwner(t *testing.T) {
	h := New(&mockService{
		getCalendarOnly: func(_ context.Context, id string) (model.Calendar, error) {
			return model.Calendar{ID: id, OwnerID: "other"}, nil
		},
	})
	req := withUser(httptest.NewRequest(http.MethodPost, "/calendars/cal1/finalise", jsonBody(t, map[string]string{"date": "2026-05-01"})), testUser)
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.FinaliseCalendar(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestFinaliseCalendar_BadDate(t *testing.T) {
	h := New(&mockService{})
	req := withUser(httptest.NewRequest(http.MethodPost, "/calendars/cal1/finalise", jsonBody(t, map[string]string{"date": "05/01/2026"})), testUser)
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.FinaliseCalendar(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestFinaliseCalendar_NotFound(t *testing.T) {
	h := New(&mockService{
		getCalendarOnly: func(_ context.Context, _ string) (model.Calendar, error) {
			return model.Calendar{}, store.ErrNotFound
		},
	})
	req := withUser(httptest.NewRequest(http.MethodPost, "/calendars/missing/finalise", jsonBody(t, map[string]string{"date": "2026-05-01"})), testUser)
	req.SetPathValue("id", "missing")
	rec := httptest.NewRecorder()
	h.FinaliseCalendar(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUnfinaliseCalendar_Valid(t *testing.T) {
	h := New(&mockService{
		getCalendarOnly: func(_ context.Context, id string) (model.Calendar, error) {
			return model.Calendar{ID: id, OwnerID: "u1"}, nil
		},
		unfinaliseCalendar: func(_ context.Context, calendarID string) (model.Calendar, error) {
			if calendarID != "cal1" {
				t.Fatalf("unexpected calendarID=%q", calendarID)
			}
			return model.Calendar{ID: "cal1", OwnerID: "u1"}, nil
		},
	})
	req := withUser(httptest.NewRequest(http.MethodDelete, "/calendars/cal1/finalise", nil), testUser)
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.UnfinaliseCalendar(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestUnfinaliseCalendar_NotOwner(t *testing.T) {
	h := New(&mockService{
		getCalendarOnly: func(_ context.Context, id string) (model.Calendar, error) {
			return model.Calendar{ID: id, OwnerID: "other"}, nil
		},
	})
	req := withUser(httptest.NewRequest(http.MethodDelete, "/calendars/cal1/finalise", nil), testUser)
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.UnfinaliseCalendar(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// --- RenameCalendar tests ---

func TestRenameCalendar_Valid(t *testing.T) {
	updated := model.Calendar{ID: "cal1", Name: "New Name", OwnerID: "u1"}
	h := New(&mockService{
		getCalendarOnly: func(_ context.Context, id string) (model.Calendar, error) {
			return model.Calendar{ID: id, OwnerID: "u1"}, nil
		},
		renameCalendar: func(_ context.Context, calendarID, name string) (model.Calendar, error) {
			if calendarID != "cal1" || name != "New Name" {
				t.Fatalf("unexpected args: calendarID=%q name=%q", calendarID, name)
			}
			return updated, nil
		},
	})

	req := withUser(httptest.NewRequest(http.MethodPatch, "/calendars/cal1", jsonBody(t, map[string]string{"name": "New Name"})), testUser)
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.UpdateCalendar(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got model.Calendar
	decodeJSON(t, rec, &got)
	if got.Name != "New Name" {
		t.Fatalf("name = %q, want New Name", got.Name)
	}
}

func TestRenameCalendar_Unauthorized(t *testing.T) {
	h := New(&mockService{})
	req := httptest.NewRequest(http.MethodPatch, "/calendars/cal1", jsonBody(t, map[string]string{"name": "X"}))
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.UpdateCalendar(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRenameCalendar_MalformedJSON(t *testing.T) {
	h := New(&mockService{})
	req := withUser(httptest.NewRequest(http.MethodPatch, "/calendars/cal1", bytes.NewReader([]byte("{nope"))), testUser)
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.UpdateCalendar(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRenameCalendar_EmptyName(t *testing.T) {
	h := New(&mockService{})
	req := withUser(httptest.NewRequest(http.MethodPatch, "/calendars/cal1", jsonBody(t, map[string]string{"name": "   "})), testUser)
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.UpdateCalendar(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRenameCalendar_NotOwner(t *testing.T) {
	h := New(&mockService{
		getCalendarOnly: func(_ context.Context, id string) (model.Calendar, error) {
			return model.Calendar{ID: id, OwnerID: "other"}, nil
		},
	})
	req := withUser(httptest.NewRequest(http.MethodPatch, "/calendars/cal1", jsonBody(t, map[string]string{"name": "X"})), testUser)
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.UpdateCalendar(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRenameCalendar_NotFound(t *testing.T) {
	h := New(&mockService{
		getCalendarOnly: func(_ context.Context, _ string) (model.Calendar, error) {
			return model.Calendar{}, store.ErrNotFound
		},
	})
	req := withUser(httptest.NewRequest(http.MethodPatch, "/calendars/missing", jsonBody(t, map[string]string{"name": "X"})), testUser)
	req.SetPathValue("id", "missing")
	rec := httptest.NewRecorder()
	h.UpdateCalendar(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// --- RemoveParticipant tests ---

func TestRemoveParticipant_Valid(t *testing.T) {
	h := New(&mockService{
		getCalendarOnly: func(_ context.Context, id string) (model.Calendar, error) {
			return model.Calendar{ID: id, OwnerID: "u1"}, nil
		},
		removeParticipant: func(_ context.Context, calendarID, participantID string) error {
			if calendarID != "cal1" || participantID != "p1" {
				t.Fatalf("unexpected args: calendarID=%q participantID=%q", calendarID, participantID)
			}
			return nil
		},
	})

	req := withUser(httptest.NewRequest(http.MethodDelete, "/calendars/cal1/participants/p1", nil), testUser)
	req.SetPathValue("id", "cal1")
	req.SetPathValue("participantId", "p1")
	rec := httptest.NewRecorder()
	h.RemoveParticipant(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestRemoveParticipant_Unauthorized(t *testing.T) {
	h := New(&mockService{})
	req := httptest.NewRequest(http.MethodDelete, "/calendars/cal1/participants/p1", nil)
	req.SetPathValue("id", "cal1")
	req.SetPathValue("participantId", "p1")
	rec := httptest.NewRecorder()
	h.RemoveParticipant(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRemoveParticipant_NotOwner(t *testing.T) {
	h := New(&mockService{
		getCalendarOnly: func(_ context.Context, id string) (model.Calendar, error) {
			return model.Calendar{ID: id, OwnerID: "other"}, nil
		},
	})
	req := withUser(httptest.NewRequest(http.MethodDelete, "/calendars/cal1/participants/p1", nil), testUser)
	req.SetPathValue("id", "cal1")
	req.SetPathValue("participantId", "p1")
	rec := httptest.NewRecorder()
	h.RemoveParticipant(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRemoveParticipant_CalendarNotFound(t *testing.T) {
	h := New(&mockService{
		getCalendarOnly: func(_ context.Context, _ string) (model.Calendar, error) {
			return model.Calendar{}, store.ErrNotFound
		},
	})
	req := withUser(httptest.NewRequest(http.MethodDelete, "/calendars/missing/participants/p1", nil), testUser)
	req.SetPathValue("id", "missing")
	req.SetPathValue("participantId", "p1")
	rec := httptest.NewRecorder()
	h.RemoveParticipant(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRemoveParticipant_ParticipantNotFound(t *testing.T) {
	h := New(&mockService{
		getCalendarOnly: func(_ context.Context, id string) (model.Calendar, error) {
			return model.Calendar{ID: id, OwnerID: "u1"}, nil
		},
		removeParticipant: func(_ context.Context, _, _ string) error {
			return store.ErrNotFound
		},
	})
	req := withUser(httptest.NewRequest(http.MethodDelete, "/calendars/cal1/participants/missing", nil), testUser)
	req.SetPathValue("id", "cal1")
	req.SetPathValue("participantId", "missing")
	rec := httptest.NewRecorder()
	h.RemoveParticipant(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// --- UpdateCalendar visibility tests ---

func TestUpdateCalendar_VisibilityOnly(t *testing.T) {
	updated := model.Calendar{ID: "cal1", Name: "X", OwnerID: "u1", Visibility: model.VisibilitySubmitToView}
	h := New(&mockService{
		getCalendarOnly: func(_ context.Context, id string) (model.Calendar, error) {
			return model.Calendar{ID: id, OwnerID: "u1"}, nil
		},
		updateCalendarVisibility: func(_ context.Context, calendarID, visibility string) (model.Calendar, error) {
			if calendarID != "cal1" || visibility != model.VisibilitySubmitToView {
				t.Fatalf("unexpected args: calendarID=%q visibility=%q", calendarID, visibility)
			}
			return updated, nil
		},
	})
	req := withUser(httptest.NewRequest(http.MethodPatch, "/calendars/cal1", jsonBody(t, map[string]string{"visibility": model.VisibilitySubmitToView})), testUser)
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.UpdateCalendar(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got model.Calendar
	decodeJSON(t, rec, &got)
	if got.Visibility != model.VisibilitySubmitToView {
		t.Fatalf("visibility = %q, want %q", got.Visibility, model.VisibilitySubmitToView)
	}
}

func TestUpdateCalendar_InvalidVisibility(t *testing.T) {
	h := New(&mockService{})
	req := withUser(httptest.NewRequest(http.MethodPatch, "/calendars/cal1", jsonBody(t, map[string]string{"visibility": "bogus"})), testUser)
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.UpdateCalendar(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateCalendar_EmptyBody(t *testing.T) {
	h := New(&mockService{})
	req := withUser(httptest.NewRequest(http.MethodPatch, "/calendars/cal1", jsonBody(t, map[string]string{})), testUser)
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.UpdateCalendar(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateCalendar_InvalidVisibility(t *testing.T) {
	h := New(&mockService{})
	req := withUser(httptest.NewRequest(http.MethodPost, "/calendars", jsonBody(t, map[string]string{"name": "Trip", "visibility": "bogus"})), testUser)
	rec := httptest.NewRecorder()
	h.CreateCalendar(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
