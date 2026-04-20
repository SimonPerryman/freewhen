package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/store"
)

// mockStore implements Store for testing. Only GetParticipant and SetAvailability
// are exercised; the rest panic if called unexpectedly.
type mockStore struct {
	createCalendar             func(name, emoji, ownerID, visibility string) (model.Calendar, error)
	getCalendar                func(ctx context.Context, id string) (model.CalendarWithDetails, error)
	getParticipant             func(ctx context.Context, id string) (model.Participant, error)
	setAvailability            func(ctx context.Context, participantID string, dates []string) error
	participantHasAvailability func(ctx context.Context, participantID string) (bool, error)
}

func (m *mockStore) CreateCalendar(_ context.Context, name, emoji, ownerID, visibility string) (model.Calendar, error) {
	if m.createCalendar == nil {
		panic("unexpected call to CreateCalendar")
	}
	return m.createCalendar(name, emoji, ownerID, visibility)
}

func (m *mockStore) GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error) {
	if m.getCalendar == nil {
		panic("unexpected call to GetCalendar")
	}
	return m.getCalendar(ctx, id)
}

func (m *mockStore) GetCalendarOnly(_ context.Context, _ string) (model.Calendar, error) {
	panic("unexpected call to GetCalendarOnly")
}

func (m *mockStore) AddParticipant(_ context.Context, _, _ string, _ *string) (model.Participant, error) {
	panic("unexpected call to AddParticipant")
}

func (m *mockStore) ClaimParticipant(_ context.Context, _, _, _ string) (model.Participant, error) {
	panic("unexpected call to ClaimParticipant")
}

func (m *mockStore) SetFinalisedDate(_ context.Context, _, _ string) (model.Calendar, error) {
	panic("unexpected call to SetFinalisedDate")
}

func (m *mockStore) ClearFinalisedDate(_ context.Context, _ string) (model.Calendar, error) {
	panic("unexpected call to ClearFinalisedDate")
}

func (m *mockStore) GetParticipant(ctx context.Context, id string) (model.Participant, error) {
	return m.getParticipant(ctx, id)
}

func (m *mockStore) SetAvailability(ctx context.Context, participantID string, dates []string) error {
	return m.setAvailability(ctx, participantID, dates)
}

func (m *mockStore) UpdateCalendarName(_ context.Context, _, _ string) (model.Calendar, error) {
	panic("unexpected call to UpdateCalendarName")
}

func (m *mockStore) UpdateCalendarVisibility(_ context.Context, _, _ string) (model.Calendar, error) {
	panic("unexpected call to UpdateCalendarVisibility")
}

func (m *mockStore) DeleteParticipant(_ context.Context, _, _ string) error {
	panic("unexpected call to DeleteParticipant")
}

func (m *mockStore) CountCalendarParticipants(_ context.Context, _ string) (int, error) {
	panic("unexpected call to CountCalendarParticipants")
}

func (m *mockStore) ParticipantHasAvailability(ctx context.Context, participantID string) (bool, error) {
	if m.participantHasAvailability == nil {
		panic("unexpected call to ParticipantHasAvailability")
	}
	return m.participantHasAvailability(ctx, participantID)
}

func TestSetAvailability_ParticipantBelongsToCalendar(t *testing.T) {
	var storeCalled bool
	ms := &mockStore{
		getParticipant: func(_ context.Context, id string) (model.Participant, error) {
			return model.Participant{ID: id, CalendarID: "cal1"}, nil
		},
		setAvailability: func(_ context.Context, participantID string, dates []string) error {
			storeCalled = true
			if participantID != "p1" {
				t.Fatalf("participantID = %q, want p1", participantID)
			}
			if len(dates) != 1 || dates[0] != "2026-05-01" {
				t.Fatalf("dates = %v, want [2026-05-01]", dates)
			}
			return nil
		},
	}

	s := &Service{store: ms}
	err := s.SetAvailability(context.Background(), "cal1", "p1", []string{"2026-05-01"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !storeCalled {
		t.Fatal("expected store.SetAvailability to be called")
	}
}

func TestSetAvailability_ParticipantNotFound(t *testing.T) {
	ms := &mockStore{
		getParticipant: func(_ context.Context, _ string) (model.Participant, error) {
			return model.Participant{}, store.ErrNotFound
		},
	}

	s := &Service{store: ms}
	err := s.SetAvailability(context.Background(), "cal1", "missing", []string{"2026-05-01"})
	if err != store.ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestSetAvailability_ParticipantWrongCalendar(t *testing.T) {
	ms := &mockStore{
		getParticipant: func(_ context.Context, id string) (model.Participant, error) {
			return model.Participant{ID: id, CalendarID: "other-cal"}, nil
		},
	}

	s := &Service{store: ms}
	err := s.SetAvailability(context.Background(), "cal1", "p1", []string{"2026-05-01"})
	if err != store.ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPickRandomEmoji_ReturnsFromCuratedSet(t *testing.T) {
	allowed := make(map[string]bool, len(defaultEmojis))
	for _, e := range defaultEmojis {
		allowed[e] = true
	}
	// Run a handful of iterations to make sure every result is in the set.
	for i := 0; i < 50; i++ {
		got := pickRandomEmoji()
		if !allowed[got] {
			t.Fatalf("pickRandomEmoji returned %q, not in curated set", got)
		}
	}
}

func TestCreateCalendar_PassesEmojiToStore(t *testing.T) {
	var gotEmoji, gotVisibility string
	ms := &mockStore{
		createCalendar: func(_, emoji, _, visibility string) (model.Calendar, error) {
			gotEmoji = emoji
			gotVisibility = visibility
			return model.Calendar{ID: "c1", Name: "x", Emoji: emoji, Visibility: visibility}, nil
		},
	}

	s := &Service{store: ms}
	cal, err := s.CreateCalendar(context.Background(), "x", "owner1", model.VisibilityOpen)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotEmoji == "" {
		t.Fatal("expected service to pass an emoji to the store, got empty string")
	}
	if cal.Emoji != gotEmoji {
		t.Errorf("returned emoji %q, want %q", cal.Emoji, gotEmoji)
	}
	if gotVisibility != model.VisibilityOpen {
		t.Errorf("visibility = %q, want %q", gotVisibility, model.VisibilityOpen)
	}
}

func TestSetAvailability_StoreError(t *testing.T) {
	ms := &mockStore{
		getParticipant: func(_ context.Context, id string) (model.Participant, error) {
			return model.Participant{ID: id, CalendarID: "cal1"}, nil
		},
		setAvailability: func(_ context.Context, _ string, _ []string) error {
			return fmt.Errorf("db down")
		},
	}

	s := &Service{store: ms}
	err := s.SetAvailability(context.Background(), "cal1", "p1", []string{"2026-05-01"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- GetCalendarView tests ---

func TestGetCalendarView_OpenAlwaysRevealed(t *testing.T) {
	details := model.CalendarWithDetails{
		Calendar:         model.Calendar{ID: "c1", OwnerID: "owner", Visibility: model.VisibilityOpen},
		Participants:     []model.Participant{{ID: "p1", CalendarID: "c1"}},
		Availability:     []model.Availability{{ParticipantID: "p1", CalendarID: "c1", Date: "2026-05-01"}},
		ParticipantCount: 1,
	}
	ms := &mockStore{
		getCalendar: func(_ context.Context, _ string) (model.CalendarWithDetails, error) {
			return details, nil
		},
	}
	s := &Service{store: ms}

	view, err := s.GetCalendarView(context.Background(), "c1", Viewer{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !view.Revealed {
		t.Fatal("open calendar should always be revealed")
	}
	if len(view.Details.Participants) != 1 {
		t.Fatalf("participants = %d, want 1", len(view.Details.Participants))
	}
}

func TestGetCalendarView_SubmitToView_OwnerRevealed(t *testing.T) {
	details := model.CalendarWithDetails{
		Calendar:         model.Calendar{ID: "c1", OwnerID: "owner", Visibility: model.VisibilitySubmitToView},
		Participants:     []model.Participant{{ID: "p1", CalendarID: "c1"}},
		Availability:     []model.Availability{{ParticipantID: "p1", CalendarID: "c1", Date: "2026-05-01"}},
		ParticipantCount: 1,
	}
	ms := &mockStore{
		getCalendar: func(_ context.Context, _ string) (model.CalendarWithDetails, error) {
			return details, nil
		},
	}
	s := &Service{store: ms}

	ownerID := "owner"
	view, err := s.GetCalendarView(context.Background(), "c1", Viewer{UserID: &ownerID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !view.Revealed {
		t.Fatal("owner should see the full view")
	}
}

func TestGetCalendarView_SubmitToView_UnsubmittedAnonymousHidden(t *testing.T) {
	details := model.CalendarWithDetails{
		Calendar:         model.Calendar{ID: "c1", OwnerID: "owner", Visibility: model.VisibilitySubmitToView},
		Participants:     []model.Participant{{ID: "p1", CalendarID: "c1"}, {ID: "p2", CalendarID: "c1"}},
		Availability:     []model.Availability{{ParticipantID: "p1", CalendarID: "c1", Date: "2026-05-01"}},
		ParticipantCount: 2,
	}
	ms := &mockStore{
		getCalendar: func(_ context.Context, _ string) (model.CalendarWithDetails, error) {
			return details, nil
		},
		participantHasAvailability: func(_ context.Context, pid string) (bool, error) {
			return pid == "p1", nil
		},
	}
	s := &Service{store: ms}

	pid := "p2"
	view, err := s.GetCalendarView(context.Background(), "c1", Viewer{ParticipantID: &pid})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view.Revealed {
		t.Fatal("unsubmitted participant should be hidden")
	}
	if view.Details.ParticipantCount != 2 {
		t.Fatalf("participantCount = %d, want 2", view.Details.ParticipantCount)
	}
	if len(view.Details.Participants) != 0 || len(view.Details.Availability) != 0 {
		t.Fatal("hidden view must not leak participants or availability")
	}
}

func TestGetCalendarView_SubmitToView_SubmittedParticipantRevealed(t *testing.T) {
	details := model.CalendarWithDetails{
		Calendar:         model.Calendar{ID: "c1", OwnerID: "owner", Visibility: model.VisibilitySubmitToView},
		Participants:     []model.Participant{{ID: "p1", CalendarID: "c1"}},
		Availability:     []model.Availability{{ParticipantID: "p1", CalendarID: "c1", Date: "2026-05-01"}},
		ParticipantCount: 1,
	}
	ms := &mockStore{
		getCalendar: func(_ context.Context, _ string) (model.CalendarWithDetails, error) {
			return details, nil
		},
		participantHasAvailability: func(_ context.Context, _ string) (bool, error) {
			return true, nil
		},
	}
	s := &Service{store: ms}

	pid := "p1"
	view, err := s.GetCalendarView(context.Background(), "c1", Viewer{ParticipantID: &pid})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !view.Revealed {
		t.Fatal("submitted participant should see the full view")
	}
}

func TestGetCalendarView_SubmitToView_SubmittedLinkedUserRevealed(t *testing.T) {
	userID := "user-1"
	details := model.CalendarWithDetails{
		Calendar:         model.Calendar{ID: "c1", OwnerID: "owner", Visibility: model.VisibilitySubmitToView},
		Participants:     []model.Participant{{ID: "p1", CalendarID: "c1", UserID: &userID}},
		Availability:     []model.Availability{},
		ParticipantCount: 1,
	}
	ms := &mockStore{
		getCalendar: func(_ context.Context, _ string) (model.CalendarWithDetails, error) {
			return details, nil
		},
		participantHasAvailability: func(_ context.Context, _ string) (bool, error) {
			return true, nil
		},
	}
	s := &Service{store: ms}

	view, err := s.GetCalendarView(context.Background(), "c1", Viewer{UserID: &userID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !view.Revealed {
		t.Fatal("linked user who has submitted should see the full view")
	}
}

func TestUpdateCalendarVisibility_InvalidValue(t *testing.T) {
	s := &Service{store: &mockStore{}}
	_, err := s.UpdateCalendarVisibility(context.Background(), "c1", "bogus")
	if !errors.Is(err, ErrInvalidVisibility) {
		t.Fatalf("err = %v, want ErrInvalidVisibility", err)
	}
}
