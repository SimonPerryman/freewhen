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
	"github.com/freewhen/backend/internal/store"
)

// mockService implements CalendarService for testing.
type mockService struct {
	createCalendar  func(ctx context.Context, name, ownerID string) (model.Calendar, error)
	getCalendar     func(ctx context.Context, id string) (model.CalendarWithDetails, error)
	addParticipant  func(ctx context.Context, calendarID, name string) (model.Participant, error)
	setAvailability func(ctx context.Context, calendarID, participantID string, dates []string) error
}

func (m *mockService) CreateCalendar(ctx context.Context, name, ownerID string) (model.Calendar, error) {
	return m.createCalendar(ctx, name, ownerID)
}

func (m *mockService) GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error) {
	return m.getCalendar(ctx, id)
}

func (m *mockService) AddParticipant(ctx context.Context, calendarID, name string) (model.Participant, error) {
	return m.addParticipant(ctx, calendarID, name)
}

func (m *mockService) SetAvailability(ctx context.Context, calendarID, participantID string, dates []string) error {
	return m.setAvailability(ctx, calendarID, participantID, dates)
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
	h := New(&mockService{
		createCalendar: func(_ context.Context, name, ownerID string) (model.Calendar, error) {
			if name != "Trip" {
				t.Fatalf("expected name %q, got %q", "Trip", name)
			}
			if ownerID != "u1" {
				t.Fatalf("expected ownerID %q, got %q", "u1", ownerID)
			}
			return cal, nil
		},
	})

	req := withUser(httptest.NewRequest(http.MethodPost, "/calendars", jsonBody(t, map[string]string{"name": "Trip"})), testUser)
	rec := httptest.NewRecorder()
	h.CreateCalendar(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var got model.Calendar
	decodeJSON(t, rec, &got)
	if got.ID != cal.ID || got.Name != cal.Name {
		t.Fatalf("body = %+v, want id=%s name=%s", got, cal.ID, cal.Name)
	}
}

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
		createCalendar: func(_ context.Context, _, _ string) (model.Calendar, error) {
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
		getCalendar: func(_ context.Context, id string) (model.CalendarWithDetails, error) {
			if id != "abc12345" {
				t.Fatalf("expected id %q, got %q", "abc12345", id)
			}
			return details, nil
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
		getCalendar: func(_ context.Context, _ string) (model.CalendarWithDetails, error) {
			return model.CalendarWithDetails{}, store.ErrNotFound
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
		getCalendar: func(_ context.Context, _ string) (model.CalendarWithDetails, error) {
			return model.CalendarWithDetails{}, fmt.Errorf("db down")
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

// --- AddParticipant tests ---

func TestAddParticipant_Valid(t *testing.T) {
	p := model.Participant{ID: "p1", CalendarID: "cal1", Name: "Alice", Colour: "#E63946"}
	h := New(&mockService{
		addParticipant: func(_ context.Context, calendarID, name string) (model.Participant, error) {
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
		addParticipant: func(_ context.Context, _, _ string) (model.Participant, error) {
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
		addParticipant: func(_ context.Context, _, _ string) (model.Participant, error) {
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
		addParticipant: func(_ context.Context, _, _ string) (model.Participant, error) {
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
