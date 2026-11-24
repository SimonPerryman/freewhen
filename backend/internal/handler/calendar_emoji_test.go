package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/freewhen/backend/internal/model"
)

func ownedCalendar(_ context.Context, id string) (model.Calendar, error) {
	return model.Calendar{ID: id, OwnerID: "u1"}, nil
}

func patchCalendar(t *testing.T, h *Handler, body map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := withUser(httptest.NewRequest(http.MethodPatch, "/calendars/cal1", jsonBody(t, body)), testUser)
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.UpdateCalendar(rec, req)
	return rec
}

func TestUpdateCalendar_Emoji(t *testing.T) {
	var gotEmoji string
	h := New(&mockService{
		getCalendarOnly: ownedCalendar,
		setCalendarEmoji: func(_ context.Context, calendarID, emoji string) (model.Calendar, error) {
			gotEmoji = emoji
			return model.Calendar{ID: calendarID, Emoji: emoji, OwnerID: "u1"}, nil
		},
	})

	rec := patchCalendar(t, h, map[string]string{"emoji": "🎉"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if gotEmoji != "🎉" {
		t.Fatalf("service got emoji %q, want 🎉", gotEmoji)
	}
	var got model.Calendar
	decodeResponse(t, rec, &got)
	if got.Emoji != "🎉" {
		t.Fatalf("response emoji = %q, want 🎉", got.Emoji)
	}
}

func TestUpdateCalendar_InvalidEmojiRejectedBeforeAnyWrite(t *testing.T) {
	h := New(&mockService{
		getCalendarOnly: ownedCalendar,
		renameCalendar: func(context.Context, string, string) (model.Calendar, error) {
			t.Fatal("rename must not run when the emoji is invalid")
			return model.Calendar{}, nil
		},
		setCalendarEmoji: func(context.Context, string, string) (model.Calendar, error) {
			t.Fatal("emoji update must not run for an invalid emoji")
			return model.Calendar{}, nil
		},
	})

	for _, emoji := range []string{"", "x", "📅", "🎉🎉"} {
		rec := patchCalendar(t, h, map[string]string{"name": "New", "emoji": emoji})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("emoji %q: status = %d, want 400", emoji, rec.Code)
		}
		assertErrorCode(t, rec, "invalid_emoji")
	}
}

func TestUpdateCalendar_NameAndEmoji(t *testing.T) {
	var renamed, emojied bool
	h := New(&mockService{
		getCalendarOnly: ownedCalendar,
		renameCalendar: func(_ context.Context, id, name string) (model.Calendar, error) {
			renamed = true
			return model.Calendar{ID: id, Name: name, OwnerID: "u1"}, nil
		},
		setCalendarEmoji: func(_ context.Context, id, emoji string) (model.Calendar, error) {
			emojied = true
			return model.Calendar{ID: id, Name: "New", Emoji: emoji, OwnerID: "u1"}, nil
		},
	})

	rec := patchCalendar(t, h, map[string]string{"name": "New", "emoji": "🦊"})

	if rec.Code != http.StatusOK || !renamed || !emojied {
		t.Fatalf("status = %d, renamed = %v, emoji set = %v; want 200, both applied", rec.Code, renamed, emojied)
	}
}

func TestUpdateCalendar_EmojiNotOwner(t *testing.T) {
	h := New(&mockService{
		getCalendarOnly: func(_ context.Context, id string) (model.Calendar, error) {
			return model.Calendar{ID: id, OwnerID: "someone-else"}, nil
		},
	})

	rec := patchCalendar(t, h, map[string]string{"emoji": "🎉"})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}
