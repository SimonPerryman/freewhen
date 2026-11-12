//go:build integration

package server_test

import (
	"net/http"
	"testing"

	"github.com/freewhen/backend/internal/model"
)

// createCalendarAuth creates a calendar as the primary test user and returns
// the new Calendar plus the owner's auto-created participant.
func createCalendarAuth(t *testing.T, ts *testServer, name string) (model.Calendar, model.Participant) {
	t.Helper()
	resp, body := doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": name})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create calendar: status %d, body %s", resp.StatusCode, body)
	}
	var payload struct {
		model.Calendar
		Participant *model.Participant `json:"participant"`
	}
	decodeJSON(t, body, &payload)
	if payload.Participant == nil {
		t.Fatal("expected owner participant on create")
	}
	return payload.Calendar, *payload.Participant
}

func TestUpdateCalendar(t *testing.T) {
	ts := setupTestServer(t)
	cal, _ := createCalendarAuth(t, ts, "Original")

	// Happy path — rename.
	resp, body := doJSONAuth(t, ts, http.MethodPatch, "/calendars/"+cal.ID, map[string]string{"name": "Renamed"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch: status %d, body %s", resp.StatusCode, body)
	}
	var updated model.Calendar
	decodeJSON(t, body, &updated)
	if updated.Name != "Renamed" {
		t.Errorf("name = %q, want Renamed", updated.Name)
	}

	// Empty name → 400.
	resp, _ = doJSONAuth(t, ts, http.MethodPatch, "/calendars/"+cal.ID, map[string]string{"name": "   "})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("blank name: expected 400, got %d", resp.StatusCode)
	}

	// No fields → 400.
	resp, _ = doJSONAuth(t, ts, http.MethodPatch, "/calendars/"+cal.ID, map[string]any{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("no fields: expected 400, got %d", resp.StatusCode)
	}

	// Unauth → 401.
	resp, _ = doJSON(t, ts, http.MethodPatch, "/calendars/"+cal.ID, map[string]string{"name": "X"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauth: expected 401, got %d", resp.StatusCode)
	}

	// Not found → 404.
	resp, _ = doJSONAuth(t, ts, http.MethodPatch, "/calendars/doesnotexist", map[string]string{"name": "X"})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("missing: expected 404, got %d", resp.StatusCode)
	}
}

func TestDuplicateCalendar(t *testing.T) {
	ts := setupTestServer(t)
	source, _ := createCalendarAuth(t, ts, "Source")

	// Add a couple of extra participants so include_participants has something to copy.
	resp, body := doJSON(t, ts, http.MethodPost, "/calendars/"+source.ID+"/participants", map[string]string{"name": "Alice"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add alice: %d %s", resp.StatusCode, body)
	}
	resp, _ = doJSON(t, ts, http.MethodPost, "/calendars/"+source.ID+"/participants", map[string]string{"name": "Bob"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add bob: %d", resp.StatusCode)
	}

	t.Run("DefaultsCopyParticipants", func(t *testing.T) {
		resp, body := doJSONAuth(t, ts, http.MethodPost, "/calendars/"+source.ID+"/duplicate", nil)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("duplicate: %d %s", resp.StatusCode, body)
		}
		var payload struct {
			model.Calendar
			Participant *model.Participant `json:"participant"`
		}
		decodeJSON(t, body, &payload)
		if payload.ID == source.ID {
			t.Error("duplicate reused source ID")
		}

		// Fetch and confirm participants were copied over.
		resp, body = doJSON(t, ts, http.MethodGet, "/calendars/"+payload.ID, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("get dup: %d %s", resp.StatusCode, body)
		}
		var details model.CalendarWithDetails
		decodeJSON(t, body, &details)
		names := map[string]bool{}
		for _, p := range details.Participants {
			names[p.Name] = true
		}
		if !names["Alice"] || !names["Bob"] {
			t.Errorf("expected Alice+Bob copied, got %+v", details.Participants)
		}
	})

	t.Run("OverrideNameAndSkipParticipants", func(t *testing.T) {
		resp, body := doJSONAuth(t, ts, http.MethodPost, "/calendars/"+source.ID+"/duplicate", map[string]any{
			"name":                 "Custom",
			"include_participants": false,
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("duplicate: %d %s", resp.StatusCode, body)
		}
		var payload struct {
			model.Calendar
		}
		decodeJSON(t, body, &payload)
		if payload.Name != "Custom" {
			t.Errorf("name = %q", payload.Name)
		}

		resp, body = doJSON(t, ts, http.MethodGet, "/calendars/"+payload.ID, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("get: %d %s", resp.StatusCode, body)
		}
		var details model.CalendarWithDetails
		decodeJSON(t, body, &details)
		// Should have only the owner participant when not copying.
		if len(details.Participants) != 1 {
			t.Errorf("expected 1 participant (owner only), got %d", len(details.Participants))
		}
	})

	t.Run("Unauth401", func(t *testing.T) {
		resp, _ := doJSON(t, ts, http.MethodPost, "/calendars/"+source.ID+"/duplicate", nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		resp, _ := doJSONAuth(t, ts, http.MethodPost, "/calendars/doesnotexist/duplicate", nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})
}

func TestRemoveParticipant(t *testing.T) {
	ts := setupTestServer(t)
	cal, _ := createCalendarAuth(t, ts, "Trip")

	// Add an anonymous participant.
	resp, body := doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants", map[string]string{"name": "Alice"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add: %d %s", resp.StatusCode, body)
	}
	var alice model.Participant
	decodeJSON(t, body, &alice)

	// Unauth → 401.
	resp, _ = doJSON(t, ts, http.MethodDelete, "/calendars/"+cal.ID+"/participants/"+alice.ID, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauth: expected 401, got %d", resp.StatusCode)
	}

	// Owner → 204.
	resp, _ = doJSONAuth(t, ts, http.MethodDelete, "/calendars/"+cal.ID+"/participants/"+alice.ID, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("owner remove: expected 204, got %d", resp.StatusCode)
	}

	// Confirm it's gone.
	resp, body = doJSON(t, ts, http.MethodGet, "/calendars/"+cal.ID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get: %d %s", resp.StatusCode, body)
	}
	var details model.CalendarWithDetails
	decodeJSON(t, body, &details)
	for _, p := range details.Participants {
		if p.ID == alice.ID {
			t.Errorf("alice still present after delete: %+v", p)
		}
	}

	// Idempotent-ish: re-deleting an already-gone participant → 404.
	resp, _ = doJSONAuth(t, ts, http.MethodDelete, "/calendars/"+cal.ID+"/participants/"+alice.ID, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("second delete: expected 404, got %d", resp.StatusCode)
	}
}

func TestRenameParticipant(t *testing.T) {
	ts := setupTestServer(t)
	cal, _ := createCalendarAuth(t, ts, "Trip")

	resp, body := doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants", map[string]string{"name": "Alice"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add: %d %s", resp.StatusCode, body)
	}
	var alice model.Participant
	decodeJSON(t, body, &alice)

	// Owner renames anyone — send as authed user without participant_id (service allows owner).
	resp, body = doJSONAuth(t, ts, http.MethodPatch, "/calendars/"+cal.ID+"/participants/"+alice.ID, map[string]string{
		"name": "Alice Renamed",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rename: %d %s", resp.StatusCode, body)
	}
	var renamed model.Participant
	decodeJSON(t, body, &renamed)
	if renamed.Name != "Alice Renamed" {
		t.Errorf("name = %q, want Alice Renamed", renamed.Name)
	}

	// Empty name → 400.
	resp, _ = doJSONAuth(t, ts, http.MethodPatch, "/calendars/"+cal.ID+"/participants/"+alice.ID, map[string]string{"name": " "})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("blank: expected 400, got %d", resp.StatusCode)
	}

	// Anonymous caller holding their own participant token → 200 (doJSON
	// replays the token issued at join). Add a second anonymous participant.
	resp, body = doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants", map[string]string{"name": "Bob"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add bob: %d %s", resp.StatusCode, body)
	}
	var bob model.Participant
	decodeJSON(t, body, &bob)

	resp, body = doJSON(t, ts, http.MethodPatch, "/calendars/"+cal.ID+"/participants/"+bob.ID, map[string]string{
		"name": "Robert",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("anon self-rename: %d %s", resp.StatusCode, body)
	}

	// Anonymous caller without a token → 403, even when the body claims to
	// be bob (the legacy participant_id field is ignored).
	resp, _ = doJSONRaw(t, ts, http.MethodPatch, "/calendars/"+cal.ID+"/participants/"+bob.ID, map[string]string{
		"name":           "Hacker",
		"participant_id": bob.ID,
	}, "", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("anon no-token: expected 403, got %d", resp.StatusCode)
	}
}
