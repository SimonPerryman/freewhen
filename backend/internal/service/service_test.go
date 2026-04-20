package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/store"
)

// mockStore implements Store for testing. Only GetParticipant and SetAvailability
// are exercised; the rest panic if called unexpectedly.
type mockStore struct {
	createCalendar  func(name, emoji, ownerID string) (model.Calendar, error)
	getParticipant  func(ctx context.Context, id string) (model.Participant, error)
	setAvailability func(ctx context.Context, participantID string, dates []string) error
}

func (m *mockStore) CreateCalendar(_ context.Context, name, emoji, ownerID string) (model.Calendar, error) {
	if m.createCalendar == nil {
		panic("unexpected call to CreateCalendar")
	}
	return m.createCalendar(name, emoji, ownerID)
}

func (m *mockStore) GetCalendar(_ context.Context, _ string) (model.CalendarWithDetails, error) {
	panic("unexpected call to GetCalendar")
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
	var gotEmoji string
	ms := &mockStore{
		createCalendar: func(_, emoji, _ string) (model.Calendar, error) {
			gotEmoji = emoji
			return model.Calendar{ID: "c1", Name: "x", Emoji: emoji}, nil
		},
	}

	s := &Service{store: ms}
	cal, err := s.CreateCalendar(context.Background(), "x", "owner1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotEmoji == "" {
		t.Fatal("expected service to pass an emoji to the store, got empty string")
	}
	if cal.Emoji != gotEmoji {
		t.Errorf("returned emoji %q, want %q", cal.Emoji, gotEmoji)
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
