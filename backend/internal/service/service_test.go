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
	getParticipant  func(ctx context.Context, id string) (model.Participant, error)
	setAvailability func(ctx context.Context, participantID string, dates []string) error
}

func (m *mockStore) CreateCalendar(_ context.Context, _ string) (model.Calendar, error) {
	panic("unexpected call to CreateCalendar")
}

func (m *mockStore) GetCalendar(_ context.Context, _ string) (model.CalendarWithDetails, error) {
	panic("unexpected call to GetCalendar")
}

func (m *mockStore) AddParticipant(_ context.Context, _, _ string) (model.Participant, error) {
	panic("unexpected call to AddParticipant")
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
