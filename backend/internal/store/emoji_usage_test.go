//go:build integration

package store

import (
	"context"
	"testing"
	"time"

	"github.com/freewhen/backend/internal/model"
)

func TestListEmojiUsageForOwner(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	owner := createTestUser(t, s, "Owner")
	other := createTestUser(t, s, "Other")

	mustCreate := func(name, emoji, ownerID string) model.Calendar {
		t.Helper()
		cal, err := s.CreateCalendar(ctx, name, emoji, ownerID)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		return cal
	}
	lemon := mustCreate("Lemon", "🍋", owner.ID)
	fox := mustCreate("Fox", "🦊", owner.ID)
	mustCreate("No emoji", "", owner.ID)            // NULL emoji: skipped
	deleted := mustCreate("Deleted", "🌊", owner.ID) // soft-deleted: skipped
	mustCreate("Someone else's", "🍒", other.ID)     // other owner: skipped
	if err := s.SoftDeleteCalendar(ctx, deleted.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	// LastActive is updated_at, which activity on the calendar bumps.
	time.Sleep(10 * time.Millisecond)
	if err := s.TouchCalendar(ctx, fox.ID); err != nil {
		t.Fatalf("touch: %v", err)
	}
	fox, err := s.GetCalendarOnly(ctx, fox.ID)
	if err != nil {
		t.Fatalf("re-read fox: %v", err)
	}
	if !fox.UpdatedAt.After(lemon.UpdatedAt) {
		t.Fatalf("touch did not advance fox's updated_at")
	}

	usage, err := s.ListEmojiUsageForOwner(ctx, owner.ID)
	if err != nil {
		t.Fatalf("list emoji usage: %v", err)
	}
	want := map[string]time.Time{"🍋": lemon.UpdatedAt, "🦊": fox.UpdatedAt}
	if len(usage) != len(want) {
		t.Fatalf("usage = %v, want emojis %v", usage, want)
	}
	for _, u := range usage {
		at, ok := want[u.Emoji]
		if !ok {
			t.Fatalf("unexpected emoji %q in usage %v", u.Emoji, usage)
		}
		if !u.LastActive.Equal(at) {
			t.Errorf("%s LastActive = %v, want %v", u.Emoji, u.LastActive, at)
		}
	}

	none, err := s.ListEmojiUsageForOwner(ctx, "no-such-user")
	if err != nil {
		t.Fatalf("list for unknown owner: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("unknown owner usage = %v, want empty", none)
	}
}
