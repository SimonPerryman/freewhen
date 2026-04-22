//go:build integration

package store

import (
	"context"
	"regexp"
	"testing"
	"time"
)

func TestCreateCalendar(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")

	cal, err := s.CreateCalendar(ctx, "Weekend Trip", "✨", owner.ID, "open")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cal.ID) != 8 {
		t.Errorf("expected 8-char ID, got %q", cal.ID)
	}
	if !regexp.MustCompile(`^[a-z0-9]+$`).MatchString(cal.ID) {
		t.Errorf("expected alphanumeric ID, got %q", cal.ID)
	}
	if cal.Name != "Weekend Trip" {
		t.Errorf("expected name %q, got %q", "Weekend Trip", cal.Name)
	}
	if cal.Emoji != "✨" {
		t.Errorf("expected emoji %q, got %q", "✨", cal.Emoji)
	}
	if cal.OwnerID != owner.ID {
		t.Errorf("expected owner_id %q, got %q", owner.ID, cal.OwnerID)
	}
	if time.Since(cal.CreatedAt) > 5*time.Second {
		t.Errorf("expected recent created_at, got %v", cal.CreatedAt)
	}
}

func TestCreateCalendarNoEmoji(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")

	cal, err := s.CreateCalendar(ctx, "No Icon", "", owner.ID, "open")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cal.Emoji != "" {
		t.Errorf("expected empty emoji, got %q", cal.Emoji)
	}

	got, err := s.GetCalendar(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}
	if got.Calendar.Emoji != "" {
		t.Errorf("expected empty emoji on read, got %q", got.Calendar.Emoji)
	}
}

func TestCreateCalendarEmojiRoundTrip(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")

	cal, err := s.CreateCalendar(ctx, "Round Trip", "🪩", owner.ID, "open")
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	got, err := s.GetCalendar(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}
	if got.Calendar.Emoji != "🪩" {
		t.Errorf("expected emoji %q on read, got %q", "🪩", got.Calendar.Emoji)
	}
}

func TestCreateCalendarDefaultName(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")

	cal, err := s.CreateCalendar(ctx, "", "", owner.ID, "open")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cal.Name != "" {
		t.Errorf("expected empty name, got %q", cal.Name)
	}
}

func TestGetCalendar(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")

	cal, err := s.CreateCalendar(ctx, "Test", "🦊", owner.ID, "open")
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	alice, err := s.AddParticipant(ctx, cal.ID, "Alice", nil)
	if err != nil {
		t.Fatalf("add alice: %v", err)
	}
	bob, err := s.AddParticipant(ctx, cal.ID, "Bob", nil)
	if err != nil {
		t.Fatalf("add bob: %v", err)
	}

	if err := s.SetAvailability(ctx, alice.ID, []string{"2026-05-01", "2026-05-02"}, nil, nil); err != nil {
		t.Fatalf("set alice availability: %v", err)
	}
	if err := s.SetAvailability(ctx, bob.ID, []string{"2026-05-02", "2026-05-03"}, nil, nil); err != nil {
		t.Fatalf("set bob availability: %v", err)
	}

	result, err := s.GetCalendar(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}
	if result.Calendar.ID != cal.ID {
		t.Errorf("expected calendar ID %q, got %q", cal.ID, result.Calendar.ID)
	}
	if len(result.Participants) != 2 {
		t.Errorf("expected 2 participants, got %d", len(result.Participants))
	}
	if len(result.Availability) != 4 {
		t.Errorf("expected 4 availability rows, got %d", len(result.Availability))
	}
}

func TestGetCalendarNotFound(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	_, err := s.GetCalendar(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateCalendarName(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")

	cal, err := s.CreateCalendar(ctx, "Old Name", "🌿", owner.ID, "open")
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	updated, err := s.UpdateCalendarName(ctx, cal.ID, "New Name")
	if err != nil {
		t.Fatalf("update name: %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("expected name %q, got %q", "New Name", updated.Name)
	}
	if updated.Emoji != "🌿" {
		t.Errorf("expected emoji to be preserved, got %q", updated.Emoji)
	}

	got, err := s.GetCalendarOnly(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}
	if got.Name != "New Name" {
		t.Errorf("expected name on re-read %q, got %q", "New Name", got.Name)
	}
}

func TestUpdateCalendarNameNotFound(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	_, err := s.UpdateCalendarName(ctx, "nonexistent", "New Name")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetCalendarEmpty(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")

	cal, err := s.CreateCalendar(ctx, "Empty", "🌿", owner.ID, "open")
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}

	result, err := s.GetCalendar(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}
	if len(result.Participants) != 0 {
		t.Errorf("expected 0 participants, got %d", len(result.Participants))
	}
	if len(result.Availability) != 0 {
		t.Errorf("expected 0 availability, got %d", len(result.Availability))
	}
}

func TestUpdateCalendarVisibility(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")

	cal, err := s.CreateCalendar(ctx, "Visibility", "", owner.ID, "open")
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}
	if cal.Visibility != "open" {
		t.Fatalf("visibility = %q, want open", cal.Visibility)
	}

	updated, err := s.UpdateCalendarVisibility(ctx, cal.ID, "submit_to_view")
	if err != nil {
		t.Fatalf("update visibility: %v", err)
	}
	if updated.Visibility != "submit_to_view" {
		t.Fatalf("visibility = %q, want submit_to_view", updated.Visibility)
	}

	got, err := s.GetCalendar(ctx, cal.ID)
	if err != nil {
		t.Fatalf("get calendar: %v", err)
	}
	if got.Calendar.Visibility != "submit_to_view" {
		t.Fatalf("re-read visibility = %q, want submit_to_view", got.Calendar.Visibility)
	}
}

func TestTouchCalendar_BumpsUpdatedAt(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")

	cal, err := s.CreateCalendar(ctx, "Touch", "", owner.ID, "open")
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}
	before := cal.UpdatedAt

	time.Sleep(10 * time.Millisecond)
	if err := s.TouchCalendar(ctx, cal.ID); err != nil {
		t.Fatalf("touch: %v", err)
	}

	after, err := s.GetCalendarOnly(ctx, cal.ID)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if !after.UpdatedAt.After(before) {
		t.Fatalf("updated_at did not advance: before=%v after=%v", before, after.UpdatedAt)
	}
}

func TestTouchCalendar_NotFound(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	if err := s.TouchCalendar(ctx, "missing"); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestListCalendarsForUser(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	joiner := createTestUser(t, s, "Joiner")

	calA, err := s.CreateCalendar(ctx, "Alpha", "", owner.ID, "open")
	if err != nil {
		t.Fatalf("create alpha: %v", err)
	}
	if _, err := s.AddParticipant(ctx, calA.ID, "Owner", &owner.ID); err != nil {
		t.Fatalf("owner joins alpha: %v", err)
	}

	calB, err := s.CreateCalendar(ctx, "Beta", "", owner.ID, "open")
	if err != nil {
		t.Fatalf("create beta: %v", err)
	}
	if _, err := s.AddParticipant(ctx, calB.ID, "Owner", &owner.ID); err != nil {
		t.Fatalf("owner joins beta: %v", err)
	}

	// joiner joins only Alpha.
	if _, err := s.AddParticipant(ctx, calA.ID, "Joiner", &joiner.ID); err != nil {
		t.Fatalf("joiner joins alpha: %v", err)
	}

	// Bump Beta so it sorts above Alpha for the owner.
	time.Sleep(10 * time.Millisecond)
	if err := s.TouchCalendar(ctx, calB.ID); err != nil {
		t.Fatalf("touch beta: %v", err)
	}

	ownerList, err := s.ListCalendarsForUser(ctx, owner.ID, 10)
	if err != nil {
		t.Fatalf("list owner: %v", err)
	}
	if len(ownerList) != 2 {
		t.Fatalf("owner list len = %d, want 2", len(ownerList))
	}
	if ownerList[0].ID != calB.ID {
		t.Errorf("owner row[0] id = %q, want %q (Beta)", ownerList[0].ID, calB.ID)
	}
	// Alpha has 2 participants (owner + joiner).
	var alphaCount int
	for _, item := range ownerList {
		if item.ID == calA.ID {
			alphaCount = item.ParticipantCount
		}
	}
	if alphaCount != 2 {
		t.Errorf("alpha participant_count = %d, want 2", alphaCount)
	}

	joinerList, err := s.ListCalendarsForUser(ctx, joiner.ID, 10)
	if err != nil {
		t.Fatalf("list joiner: %v", err)
	}
	if len(joinerList) != 1 {
		t.Fatalf("joiner list len = %d, want 1", len(joinerList))
	}
	if joinerList[0].ID != calA.ID {
		t.Errorf("joiner row[0] id = %q, want %q", joinerList[0].ID, calA.ID)
	}
}

func TestListCalendarsForUser_RespectsLimit(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")

	for i := 0; i < 5; i++ {
		cal, err := s.CreateCalendar(ctx, "Cal", "", owner.ID, "open")
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := s.AddParticipant(ctx, cal.ID, "Owner", &owner.ID); err != nil {
			t.Fatalf("join: %v", err)
		}
	}

	rows, err := s.ListCalendarsForUser(ctx, owner.ID, 3)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("len = %d, want 3", len(rows))
	}
}

func TestParticipantHasAvailability(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")

	cal, err := s.CreateCalendar(ctx, "Avail", "", owner.ID, "submit_to_view")
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}
	p, err := s.AddParticipant(ctx, cal.ID, "Alice", nil)
	if err != nil {
		t.Fatalf("add participant: %v", err)
	}

	has, err := s.ParticipantHasAvailability(ctx, p.ID)
	if err != nil {
		t.Fatalf("has availability (none): %v", err)
	}
	if has {
		t.Fatal("expected no availability, got some")
	}

	if err := s.SetAvailability(ctx, p.ID, []string{"2026-05-01"}, nil, nil); err != nil {
		t.Fatalf("set availability: %v", err)
	}
	has, err = s.ParticipantHasAvailability(ctx, p.ID)
	if err != nil {
		t.Fatalf("has availability (one): %v", err)
	}
	if !has {
		t.Fatal("expected availability, got none")
	}
}

func TestDuplicateCalendar_CopiesCoreFields(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")

	src, err := s.CreateCalendar(ctx, "Tennis", "🎾", owner.ID, "submit_to_view")
	if err != nil {
		t.Fatalf("create src: %v", err)
	}
	if _, err := s.AddParticipant(ctx, src.ID, "Owner", &owner.ID); err != nil {
		t.Fatalf("owner join: %v", err)
	}
	if _, err := s.AddParticipant(ctx, src.ID, "Alice", nil); err != nil {
		t.Fatalf("add alice: %v", err)
	}
	if _, err := s.AddParticipant(ctx, src.ID, "Bob", nil); err != nil {
		t.Fatalf("add bob: %v", err)
	}

	dup, ownerP, err := s.DuplicateCalendar(ctx, src.ID, owner.ID, "Tennis (copy)", "Owner", true)
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	if dup.ID == src.ID {
		t.Fatal("expected new calendar id, got same as source")
	}
	if dup.Name != "Tennis (copy)" || dup.Emoji != "🎾" || dup.Visibility != "submit_to_view" {
		t.Fatalf("unexpected duplicate fields: %+v", dup)
	}
	if ownerP.Name != "Owner" || ownerP.UserID == nil || *ownerP.UserID != owner.ID {
		t.Fatalf("unexpected owner participant: %+v", ownerP)
	}

	got, err := s.GetCalendar(ctx, dup.ID)
	if err != nil {
		t.Fatalf("get dup: %v", err)
	}
	if len(got.Participants) != 3 {
		t.Fatalf("participants len = %d, want 3 (owner + alice + bob)", len(got.Participants))
	}
	if len(got.Availability) != 0 {
		t.Fatalf("expected no availability on duplicate, got %d rows", len(got.Availability))
	}
	if got.Calendar.FinalisedDate != nil {
		t.Fatal("expected duplicate to be unfinalised")
	}
}

func TestDuplicateCalendar_SkipParticipants(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")

	src, err := s.CreateCalendar(ctx, "Tennis", "", owner.ID, "open")
	if err != nil {
		t.Fatalf("create src: %v", err)
	}
	if _, err := s.AddParticipant(ctx, src.ID, "Owner", &owner.ID); err != nil {
		t.Fatalf("owner join: %v", err)
	}
	if _, err := s.AddParticipant(ctx, src.ID, "Alice", nil); err != nil {
		t.Fatalf("add alice: %v", err)
	}

	dup, _, err := s.DuplicateCalendar(ctx, src.ID, owner.ID, "Tennis (copy)", "Owner", false)
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	got, err := s.GetCalendar(ctx, dup.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.Participants) != 1 {
		t.Fatalf("participants len = %d, want 1 (owner only)", len(got.Participants))
	}
}

func TestDuplicateCalendar_OwnerDedupedOnIncludeParticipants(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")

	src, err := s.CreateCalendar(ctx, "Tennis", "", owner.ID, "open")
	if err != nil {
		t.Fatalf("create src: %v", err)
	}
	// Owner is a participant on the source (standard auto-join shape).
	if _, err := s.AddParticipant(ctx, src.ID, "Owner", &owner.ID); err != nil {
		t.Fatalf("owner join: %v", err)
	}

	dup, _, err := s.DuplicateCalendar(ctx, src.ID, owner.ID, "Tennis (copy)", "Owner", true)
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	got, err := s.GetCalendar(ctx, dup.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.Participants) != 1 {
		t.Fatalf("participants len = %d, want 1 (owner, not duplicated)", len(got.Participants))
	}
}
