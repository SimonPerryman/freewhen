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
	createCalendar             func(name, emoji, ownerID string) (model.Calendar, error)
	duplicateCalendar          func(ctx context.Context, sourceID, newOwnerID, finalName, ownerParticipantName string, includeParticipants bool) (model.Calendar, model.Participant, error)
	getCalendar                func(ctx context.Context, id string) (model.CalendarWithDetails, error)
	getParticipant             func(ctx context.Context, id string) (model.Participant, error)
	setAvailability            func(ctx context.Context, participantID string, dates []string, noteOverrides map[string]string, preferredOverrides map[string]bool) error
	touchCalendar              func(ctx context.Context, calendarID string) error
	listCalendarsForUser       func(ctx context.Context, userID string, limit int) ([]store.CalendarListItem, error)
	addParticipant             func(ctx context.Context, calendarID, name string, userID *string) (model.Participant, error)
	countCalendarParticipants  func(ctx context.Context, calendarID string) (int, error)
	countCalendarsForOwner     func(ctx context.Context, ownerID string) (int, error)
	getCalendarOnly            func(ctx context.Context, id string) (model.Calendar, error)
	updateParticipantName      func(ctx context.Context, calendarID, participantID, name string) (model.Participant, error)
	listInvites                func() ([]model.Invite, error)
	getInvite                  func(id string) (model.Invite, error)
	addInvites                 func(names []string) (store.AddInvitesResult, error)
	updateInviteName           func(id, name string) (model.Invite, error)
	deleteInvite               func() error
	setInviteMatch             func(id string, pid *string) (model.Invite, error)
	participantsWithNoAvailability func() ([]model.Participant, error)
}

func (m *mockStore) CreateCalendar(_ context.Context, name, emoji, ownerID string) (model.Calendar, error) {
	if m.createCalendar == nil {
		panic("unexpected call to CreateCalendar")
	}
	return m.createCalendar(name, emoji, ownerID)
}

func (m *mockStore) DuplicateCalendar(ctx context.Context, sourceID, newOwnerID, finalName, ownerParticipantName string, includeParticipants bool) (model.Calendar, model.Participant, error) {
	if m.duplicateCalendar == nil {
		panic("unexpected call to DuplicateCalendar")
	}
	return m.duplicateCalendar(ctx, sourceID, newOwnerID, finalName, ownerParticipantName, includeParticipants)
}

func (m *mockStore) GetCalendar(ctx context.Context, id string) (model.CalendarWithDetails, error) {
	if m.getCalendar == nil {
		panic("unexpected call to GetCalendar")
	}
	return m.getCalendar(ctx, id)
}

func (m *mockStore) GetCalendarOnly(ctx context.Context, id string) (model.Calendar, error) {
	if m.getCalendarOnly == nil {
		panic("unexpected call to GetCalendarOnly")
	}
	return m.getCalendarOnly(ctx, id)
}

func (m *mockStore) AddParticipant(ctx context.Context, calendarID, name string, userID *string) (model.Participant, error) {
	if m.addParticipant == nil {
		panic("unexpected call to AddParticipant")
	}
	return m.addParticipant(ctx, calendarID, name, userID)
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

func (m *mockStore) SetAvailability(ctx context.Context, participantID string, dates []string, noteOverrides map[string]string, preferredOverrides map[string]bool) error {
	return m.setAvailability(ctx, participantID, dates, noteOverrides, preferredOverrides)
}

func (m *mockStore) UpdateCalendarName(_ context.Context, _, _ string) (model.Calendar, error) {
	panic("unexpected call to UpdateCalendarName")
}

func (m *mockStore) DeleteParticipant(_ context.Context, _, _ string) error {
	panic("unexpected call to DeleteParticipant")
}

func (m *mockStore) UpdateParticipantName(ctx context.Context, calendarID, participantID, name string) (model.Participant, error) {
	if m.updateParticipantName == nil {
		panic("unexpected call to UpdateParticipantName")
	}
	return m.updateParticipantName(ctx, calendarID, participantID, name)
}

func (m *mockStore) SetCalendarPassphrase(_ context.Context, _ string, _ *string) (model.Calendar, error) {
	panic("unexpected call to SetCalendarPassphrase")
}

func (m *mockStore) CountCalendarParticipants(ctx context.Context, calendarID string) (int, error) {
	if m.countCalendarParticipants == nil {
		return 0, nil
	}
	return m.countCalendarParticipants(ctx, calendarID)
}

func (m *mockStore) CountCalendarsForOwner(ctx context.Context, ownerID string) (int, error) {
	if m.countCalendarsForOwner == nil {
		return 0, nil
	}
	return m.countCalendarsForOwner(ctx, ownerID)
}

func (m *mockStore) TouchCalendar(ctx context.Context, calendarID string) error {
	if m.touchCalendar == nil {
		return nil
	}
	return m.touchCalendar(ctx, calendarID)
}

func (m *mockStore) ListCalendarsForUser(ctx context.Context, userID string, limit int) ([]store.CalendarListItem, error) {
	if m.listCalendarsForUser == nil {
		panic("unexpected call to ListCalendarsForUser")
	}
	return m.listCalendarsForUser(ctx, userID, limit)
}

func (m *mockStore) ListInvites(_ context.Context, _ string) ([]model.Invite, error) {
	if m.listInvites == nil {
		return []model.Invite{}, nil
	}
	return m.listInvites()
}

func (m *mockStore) GetInvite(_ context.Context, id string) (model.Invite, error) {
	if m.getInvite == nil {
		return model.Invite{ID: id}, nil
	}
	return m.getInvite(id)
}

func (m *mockStore) AddInvites(_ context.Context, _ string, names []string) (store.AddInvitesResult, error) {
	if m.addInvites == nil {
		return store.AddInvitesResult{}, nil
	}
	return m.addInvites(names)
}

func (m *mockStore) UpdateInviteName(_ context.Context, id, name string) (model.Invite, error) {
	if m.updateInviteName == nil {
		return model.Invite{ID: id, Name: name}, nil
	}
	return m.updateInviteName(id, name)
}

func (m *mockStore) DeleteInvite(_ context.Context, _ string) error {
	if m.deleteInvite == nil {
		return nil
	}
	return m.deleteInvite()
}

func (m *mockStore) SetInviteMatch(_ context.Context, id string, pid *string) (model.Invite, error) {
	if m.setInviteMatch == nil {
		return model.Invite{ID: id, MatchedParticipantID: pid}, nil
	}
	return m.setInviteMatch(id, pid)
}

func (m *mockStore) ParticipantsWithNoAvailability(_ context.Context, _ string) ([]model.Participant, error) {
	if m.participantsWithNoAvailability == nil {
		return []model.Participant{}, nil
	}
	return m.participantsWithNoAvailability()
}

func TestSetAvailability_ParticipantBelongsToCalendar(t *testing.T) {
	var storeCalled bool
	ms := &mockStore{
		getParticipant: func(_ context.Context, id string) (model.Participant, error) {
			return model.Participant{ID: id, CalendarID: "cal1"}, nil
		},
		setAvailability: func(_ context.Context, participantID string, dates []string, _ map[string]string, _ map[string]bool) error {
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

	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})
	err := s.SetAvailability(context.Background(), "cal1", "p1", []string{"2026-05-01"}, nil, nil)
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

	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})
	err := s.SetAvailability(context.Background(), "cal1", "missing", []string{"2026-05-01"}, nil, nil)
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

	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})
	err := s.SetAvailability(context.Background(), "cal1", "p1", []string{"2026-05-01"}, nil, nil)
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

	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})
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
		setAvailability: func(_ context.Context, _ string, _ []string, _ map[string]string, _ map[string]bool) error {
			return fmt.Errorf("db down")
		},
	}

	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})
	err := s.SetAvailability(context.Background(), "cal1", "p1", []string{"2026-05-01"}, nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- DuplicateCalendar tests ---

func TestDuplicateCalendar_DefaultNameSuffix(t *testing.T) {
	var gotFinalName string
	ms := &mockStore{
		countCalendarsForOwner: func(_ context.Context, _ string) (int, error) { return 3, nil },
		getCalendarOnly: func(_ context.Context, id string) (model.Calendar, error) {
			return model.Calendar{ID: id, OwnerID: "u1", Name: "Tennis"}, nil
		},
		countCalendarParticipants: func(_ context.Context, _ string) (int, error) { return 2, nil },
		duplicateCalendar: func(_ context.Context, sourceID, newOwnerID, finalName, ownerName string, include bool) (model.Calendar, model.Participant, error) {
			gotFinalName = finalName
			return model.Calendar{ID: "new", Name: finalName, OwnerID: newOwnerID}, model.Participant{ID: "p", Name: ownerName, UserID: &newOwnerID}, nil
		},
	}
	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})
	_, _, err := s.DuplicateCalendar(context.Background(), "cal1", "u1", "Simon", "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotFinalName != "Tennis (copy)" {
		t.Fatalf("finalName = %q, want %q", gotFinalName, "Tennis (copy)")
	}
}

func TestDuplicateCalendar_OverrideNameHonoured(t *testing.T) {
	var gotFinalName string
	ms := &mockStore{
		countCalendarsForOwner: func(_ context.Context, _ string) (int, error) { return 0, nil },
		getCalendarOnly: func(_ context.Context, id string) (model.Calendar, error) {
			return model.Calendar{ID: id, OwnerID: "u1", Name: "Original"}, nil
		},
		countCalendarParticipants: func(_ context.Context, _ string) (int, error) { return 0, nil },
		duplicateCalendar: func(_ context.Context, _, _, finalName, _ string, _ bool) (model.Calendar, model.Participant, error) {
			gotFinalName = finalName
			return model.Calendar{ID: "n"}, model.Participant{}, nil
		},
	}
	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})
	_, _, err := s.DuplicateCalendar(context.Background(), "cal1", "u1", "Simon", "Tennis May", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotFinalName != "Tennis May" {
		t.Fatalf("finalName = %q, want %q", gotFinalName, "Tennis May")
	}
}

func TestDuplicateCalendar_LimitReached(t *testing.T) {
	ms := &mockStore{
		countCalendarsForOwner: func(_ context.Context, _ string) (int, error) { return 200, nil },
	}
	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})
	_, _, err := s.DuplicateCalendar(context.Background(), "cal1", "u1", "Simon", "", true)
	if !errors.Is(err, store.ErrCalendarLimitReached) {
		t.Fatalf("err = %v, want ErrCalendarLimitReached", err)
	}
}

func TestDuplicateCalendar_CalendarFull(t *testing.T) {
	ms := &mockStore{
		countCalendarsForOwner: func(_ context.Context, _ string) (int, error) { return 0, nil },
		getCalendarOnly: func(_ context.Context, id string) (model.Calendar, error) {
			return model.Calendar{ID: id, OwnerID: "u1", Name: "x"}, nil
		},
		countCalendarParticipants: func(_ context.Context, _ string) (int, error) { return 100, nil },
	}
	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})
	_, _, err := s.DuplicateCalendar(context.Background(), "cal1", "u1", "Simon", "", true)
	if !errors.Is(err, store.ErrCalendarFull) {
		t.Fatalf("err = %v, want ErrCalendarFull", err)
	}
}

func TestDuplicateCalendar_SourceNotFound(t *testing.T) {
	ms := &mockStore{
		countCalendarsForOwner: func(_ context.Context, _ string) (int, error) { return 0, nil },
		getCalendarOnly: func(_ context.Context, _ string) (model.Calendar, error) {
			return model.Calendar{}, store.ErrNotFound
		},
	}
	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})
	_, _, err := s.DuplicateCalendar(context.Background(), "missing", "u1", "Simon", "", true)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// --- ListUserCalendars tests ---

func TestListUserCalendars_UnderCap(t *testing.T) {
	ms := &mockStore{
		listCalendarsForUser: func(_ context.Context, userID string, limit int) ([]store.CalendarListItem, error) {
			if userID != "u1" {
				t.Fatalf("userID = %q, want u1", userID)
			}
			if limit != MyCalendarsPageSize+1 {
				t.Fatalf("limit = %d, want %d", limit, MyCalendarsPageSize+1)
			}
			items := make([]store.CalendarListItem, 3)
			return items, nil
		},
	}
	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})

	items, hasMore, err := s.ListUserCalendars(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("len = %d, want 3", len(items))
	}
	if hasMore {
		t.Fatal("has_more = true, want false")
	}
}

func TestListUserCalendars_AtCapNoOverflow(t *testing.T) {
	ms := &mockStore{
		listCalendarsForUser: func(_ context.Context, _ string, _ int) ([]store.CalendarListItem, error) {
			return make([]store.CalendarListItem, MyCalendarsPageSize), nil
		},
	}
	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})

	items, hasMore, err := s.ListUserCalendars(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != MyCalendarsPageSize {
		t.Fatalf("len = %d, want %d", len(items), MyCalendarsPageSize)
	}
	if hasMore {
		t.Fatal("has_more = true, want false")
	}
}

func TestListUserCalendars_OverCapTrims(t *testing.T) {
	ms := &mockStore{
		listCalendarsForUser: func(_ context.Context, _ string, _ int) ([]store.CalendarListItem, error) {
			return make([]store.CalendarListItem, MyCalendarsPageSize+1), nil
		},
	}
	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})

	items, hasMore, err := s.ListUserCalendars(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != MyCalendarsPageSize {
		t.Fatalf("len = %d, want %d", len(items), MyCalendarsPageSize)
	}
	if !hasMore {
		t.Fatal("has_more = false, want true")
	}
}

// --- TouchCalendar wiring tests ---

func TestAddParticipant_TouchesCalendar(t *testing.T) {
	var touchCalendarID string
	ms := &mockStore{
		touchCalendar: func(_ context.Context, calendarID string) error {
			touchCalendarID = calendarID
			return nil
		},
	}
	ms.addParticipant = func(_ context.Context, calendarID, _ string, _ *string) (model.Participant, error) {
		return model.Participant{ID: "p1", CalendarID: calendarID}, nil
	}

	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})
	if _, err := s.AddParticipant(context.Background(), "cal1", "Alice", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if touchCalendarID != "cal1" {
		t.Fatalf("touchCalendarID = %q, want cal1", touchCalendarID)
	}
}

func TestSetAvailability_TouchesCalendar(t *testing.T) {
	var touched bool
	ms := &mockStore{
		getParticipant: func(_ context.Context, id string) (model.Participant, error) {
			return model.Participant{ID: id, CalendarID: "cal1"}, nil
		},
		setAvailability: func(_ context.Context, _ string, _ []string, _ map[string]string, _ map[string]bool) error { return nil },
		touchCalendar: func(_ context.Context, calendarID string) error {
			if calendarID != "cal1" {
				t.Fatalf("touchCalendar calendarID = %q, want cal1", calendarID)
			}
			touched = true
			return nil
		},
	}
	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})

	if err := s.SetAvailability(context.Background(), "cal1", "p1", []string{"2026-05-01"}, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !touched {
		t.Fatal("expected TouchCalendar to be called")
	}
}

func TestAddParticipant_CapReached(t *testing.T) {
	ms := &mockStore{
		countCalendarParticipants: func(_ context.Context, _ string) (int, error) {
			return 100, nil
		},
	}
	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})

	_, err := s.AddParticipant(context.Background(), "cal1", "Alice", nil)
	if !errors.Is(err, store.ErrCalendarFull) {
		t.Fatalf("err = %v, want ErrCalendarFull", err)
	}
}

func TestAddParticipant_UnderCap(t *testing.T) {
	ms := &mockStore{
		countCalendarParticipants: func(_ context.Context, _ string) (int, error) {
			return 99, nil
		},
		addParticipant: func(_ context.Context, calendarID, _ string, _ *string) (model.Participant, error) {
			return model.Participant{ID: "p1", CalendarID: calendarID}, nil
		},
	}
	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})

	p, err := s.AddParticipant(context.Background(), "cal1", "Alice", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID != "p1" {
		t.Fatalf("id = %q, want p1", p.ID)
	}
}

func TestCreateCalendar_CapReached(t *testing.T) {
	ms := &mockStore{
		countCalendarsForOwner: func(_ context.Context, _ string) (int, error) {
			return 200, nil
		},
	}
	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})

	_, err := s.CreateCalendar(context.Background(), "Trip", "owner1")
	if !errors.Is(err, store.ErrCalendarLimitReached) {
		t.Fatalf("err = %v, want ErrCalendarLimitReached", err)
	}
}

func TestCreateCalendar_UnderCap(t *testing.T) {
	ms := &mockStore{
		countCalendarsForOwner: func(_ context.Context, _ string) (int, error) {
			return 199, nil
		},
		createCalendar: func(_, _, _ string) (model.Calendar, error) {
			return model.Calendar{ID: "c1", Name: "Trip"}, nil
		},
	}
	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})

	cal, err := s.CreateCalendar(context.Background(), "Trip", "owner1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cal.ID != "c1" {
		t.Fatalf("id = %q, want c1", cal.ID)
	}
}

func TestRenameParticipant_OwnerAllowed(t *testing.T) {
	ownerID := "owner1"
	ms := &mockStore{
		getCalendarOnly: func(_ context.Context, _ string) (model.Calendar, error) {
			return model.Calendar{ID: "c1", OwnerID: ownerID}, nil
		},
		updateParticipantName: func(_ context.Context, calendarID, pid, name string) (model.Participant, error) {
			return model.Participant{ID: pid, CalendarID: calendarID, Name: name}, nil
		},
	}
	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})

	p, err := s.RenameParticipant(context.Background(), "c1", "p1", &ownerID, "", "Alicia")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "Alicia" {
		t.Fatalf("name = %q, want Alicia", p.Name)
	}
}

func TestRenameParticipant_SelfViaParticipantID(t *testing.T) {
	ms := &mockStore{
		getCalendarOnly: func(_ context.Context, _ string) (model.Calendar, error) {
			return model.Calendar{ID: "c1", OwnerID: "owner1"}, nil
		},
		updateParticipantName: func(_ context.Context, calendarID, pid, name string) (model.Participant, error) {
			return model.Participant{ID: pid, CalendarID: calendarID, Name: name}, nil
		},
	}
	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})

	_, err := s.RenameParticipant(context.Background(), "c1", "p1", nil, "p1", "Alicia")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenameParticipant_SelfViaLinkedUser(t *testing.T) {
	userID := "u1"
	ms := &mockStore{
		getCalendarOnly: func(_ context.Context, _ string) (model.Calendar, error) {
			return model.Calendar{ID: "c1", OwnerID: "owner1"}, nil
		},
		getParticipant: func(_ context.Context, id string) (model.Participant, error) {
			return model.Participant{ID: id, CalendarID: "c1", UserID: &userID}, nil
		},
		updateParticipantName: func(_ context.Context, calendarID, pid, name string) (model.Participant, error) {
			return model.Participant{ID: pid, CalendarID: calendarID, Name: name}, nil
		},
	}
	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})

	_, err := s.RenameParticipant(context.Background(), "c1", "p1", &userID, "", "Alicia")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenameParticipant_StrangerForbidden(t *testing.T) {
	callerID := "stranger"
	targetUser := "someone-else"
	ms := &mockStore{
		getCalendarOnly: func(_ context.Context, _ string) (model.Calendar, error) {
			return model.Calendar{ID: "c1", OwnerID: "owner1"}, nil
		},
		getParticipant: func(_ context.Context, id string) (model.Participant, error) {
			return model.Participant{ID: id, CalendarID: "c1", UserID: &targetUser}, nil
		},
	}
	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})

	_, err := s.RenameParticipant(context.Background(), "c1", "p1", &callerID, "", "Alicia")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestRenameParticipant_FinalisedRejected(t *testing.T) {
	ownerID := "owner1"
	date := "2026-05-01"
	ms := &mockStore{
		getCalendarOnly: func(_ context.Context, _ string) (model.Calendar, error) {
			return model.Calendar{ID: "c1", OwnerID: ownerID, FinalisedDate: &date}, nil
		},
	}
	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})

	_, err := s.RenameParticipant(context.Background(), "c1", "p1", &ownerID, "", "Alicia")
	if !errors.Is(err, store.ErrCalendarFinalised) {
		t.Fatalf("err = %v, want ErrCalendarFinalised", err)
	}
}

func TestRenameParticipant_EmptyNameRejected(t *testing.T) {
	ownerID := "owner1"
	ms := &mockStore{
		getCalendarOnly: func(_ context.Context, _ string) (model.Calendar, error) {
			return model.Calendar{ID: "c1", OwnerID: ownerID}, nil
		},
	}
	s := New(ms, nil, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})

	_, err := s.RenameParticipant(context.Background(), "c1", "p1", &ownerID, "", "   ")
	if !errors.Is(err, ErrInvalidName) {
		t.Fatalf("err = %v, want ErrInvalidName", err)
	}
}

// --- Publisher wiring tests ---

type recordingPublisher struct {
	events []pubCall
}

type pubCall struct {
	CalendarID string
	Type       string
}

func (r *recordingPublisher) Publish(calendarID, eventType string) {
	r.events = append(r.events, pubCall{CalendarID: calendarID, Type: eventType})
}

func TestPublisher_SetAvailability_PublishesOnSuccess(t *testing.T) {
	ms := &mockStore{
		getParticipant: func(_ context.Context, id string) (model.Participant, error) {
			return model.Participant{ID: id, CalendarID: "cal1"}, nil
		},
		setAvailability: func(_ context.Context, _ string, _ []string, _ map[string]string, _ map[string]bool) error { return nil },
	}
	rp := &recordingPublisher{}
	s := New(ms, rp, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})

	if err := s.SetAvailability(context.Background(), "cal1", "p1", []string{"2026-05-01"}, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rp.events) != 1 ||
		rp.events[0].CalendarID != "cal1" ||
		rp.events[0].Type != "availability.changed" {
		t.Fatalf("publisher events = %+v", rp.events)
	}
}

func TestPublisher_SetAvailability_NoPublishOnStoreError(t *testing.T) {
	ms := &mockStore{
		getParticipant: func(_ context.Context, id string) (model.Participant, error) {
			return model.Participant{ID: id, CalendarID: "cal1"}, nil
		},
		setAvailability: func(_ context.Context, _ string, _ []string, _ map[string]string, _ map[string]bool) error {
			return errors.New("db down")
		},
	}
	rp := &recordingPublisher{}
	s := New(ms, rp, AccessConfig{Secret: "test-secret-at-least-32-bytes-long", TTL: 0})

	if err := s.SetAvailability(context.Background(), "cal1", "p1", []string{"2026-05-01"}, nil, nil); err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(rp.events) != 0 {
		t.Fatalf("expected no publish on error, got %+v", rp.events)
	}
}
