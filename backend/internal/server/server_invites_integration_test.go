//go:build integration

package server_test

import (
	"net/http"
	"testing"

	"github.com/freewhen/backend/internal/model"
)

func TestInvites_CRUDFlow(t *testing.T) {
	ts := setupTestServer(t)
	cal, _ := createCalendarAuth(t, ts, "Party")

	// Add two invites.
	resp, body := doJSONAuth(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/invites", map[string]any{
		"names": []string{"Alice", "Bob"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add invites: %d %s", resp.StatusCode, body)
	}
	var addResult struct {
		Added []model.Invite `json:"added"`
	}
	decodeJSON(t, body, &addResult)
	if len(addResult.Added) != 2 {
		t.Fatalf("expected 2 invites added, got %d", len(addResult.Added))
	}
	aliceInvite := addResult.Added[0]

	// Duplicate name → skipped (not an error).
	resp, body = doJSONAuth(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/invites", map[string]any{
		"names": []string{"Alice"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add dup: %d %s", resp.StatusCode, body)
	}
	var dupResult struct {
		Added   []model.Invite `json:"added"`
		Skipped []struct {
			Name string `json:"name"`
		} `json:"skipped"`
	}
	decodeJSON(t, body, &dupResult)
	if len(dupResult.Added) != 0 {
		t.Errorf("dup should not add, got %+v", dupResult.Added)
	}
	if len(dupResult.Skipped) != 1 {
		t.Errorf("dup should be skipped, got %+v", dupResult.Skipped)
	}

	// Empty names → 400.
	resp, _ = doJSONAuth(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/invites", map[string]any{"names": []string{}})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty: expected 400, got %d", resp.StatusCode)
	}

	// List missing — should show both invites in not_joined.
	resp, body = doJSONAuth(t, ts, http.MethodGet, "/calendars/"+cal.ID+"/missing", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("missing: %d %s", resp.StatusCode, body)
	}
	var missing struct {
		Invites   []model.Invite `json:"invites"`
		NotJoined []struct {
			InviteID string `json:"invite_id"`
			Name     string `json:"name"`
		} `json:"not_joined"`
		JoinedNotResponded []any `json:"joined_not_responded"`
	}
	decodeJSON(t, body, &missing)
	if len(missing.Invites) != 2 {
		t.Errorf("invites count = %d, want 2", len(missing.Invites))
	}
	if len(missing.NotJoined) != 2 {
		t.Errorf("not_joined count = %d, want 2", len(missing.NotJoined))
	}

	// Rename the Alice invite.
	resp, body = doJSONAuth(t, ts, http.MethodPut, "/calendars/"+cal.ID+"/invites/"+aliceInvite.ID, map[string]string{"name": "Alicia"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rename: %d %s", resp.StatusCode, body)
	}
	var renamed model.Invite
	decodeJSON(t, body, &renamed)
	if renamed.Name != "Alicia" {
		t.Errorf("name = %q, want Alicia", renamed.Name)
	}

	// Manual match: add a participant and link them to the invite.
	resp, body = doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants", map[string]string{"name": "Alicia Smith"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add participant: %d %s", resp.StatusCode, body)
	}
	var p model.Participant
	decodeJSON(t, body, &p)

	resp, body = doJSONAuth(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/invites/"+aliceInvite.ID+"/match", map[string]any{
		"participant_id": p.ID,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("match: %d %s", resp.StatusCode, body)
	}
	var matched model.Invite
	decodeJSON(t, body, &matched)
	if matched.MatchedParticipantID == nil || *matched.MatchedParticipantID != p.ID {
		t.Errorf("matched_participant_id = %v, want %q", matched.MatchedParticipantID, p.ID)
	}

	// Unmatch (participant_id: null).
	resp, body = doJSONAuth(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/invites/"+aliceInvite.ID+"/match", map[string]any{
		"participant_id": nil,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unmatch: %d %s", resp.StatusCode, body)
	}
	decodeJSON(t, body, &matched)
	if matched.MatchedParticipantID != nil {
		t.Errorf("expected unmatched, got %v", matched.MatchedParticipantID)
	}

	// Delete the other invite.
	bobInvite := addResult.Added[1]
	resp, _ = doJSONAuth(t, ts, http.MethodDelete, "/calendars/"+cal.ID+"/invites/"+bobInvite.ID, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete: expected 204, got %d", resp.StatusCode)
	}

	// Deleting again → 204 (idempotent) or 404 depending on lookup path.
	// Per handler: second lookup returns 404 not_found from checkInviteOnCalendar.
	resp, _ = doJSONAuth(t, ts, http.MethodDelete, "/calendars/"+cal.ID+"/invites/"+bobInvite.ID, nil)
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusNoContent {
		t.Errorf("second delete: got %d, want 404 or 204", resp.StatusCode)
	}
}

func TestInvites_OwnerOnly(t *testing.T) {
	ts := setupTestServer(t)
	cal, _ := createCalendarAuth(t, ts, "Party")

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"list_missing", http.MethodGet, "/calendars/" + cal.ID + "/missing", nil},
		{"add_invites", http.MethodPost, "/calendars/" + cal.ID + "/invites", map[string]any{"names": []string{"A"}}},
		{"rename_invite", http.MethodPut, "/calendars/" + cal.ID + "/invites/x", map[string]string{"name": "B"}},
		{"delete_invite", http.MethodDelete, "/calendars/" + cal.ID + "/invites/x", nil},
		{"match_invite", http.MethodPost, "/calendars/" + cal.ID + "/invites/x/match", map[string]any{"participant_id": nil}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, _ := doJSON(t, ts, tc.method, tc.path, tc.body)
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("expected 401, got %d", resp.StatusCode)
			}
		})
	}
}

func TestInvites_CrossCalendarForbidden(t *testing.T) {
	ts := setupTestServer(t)
	calA, _ := createCalendarAuth(t, ts, "A")
	calB, _ := createCalendarAuth(t, ts, "B")

	// Create an invite on A.
	resp, body := doJSONAuth(t, ts, http.MethodPost, "/calendars/"+calA.ID+"/invites", map[string]any{"names": []string{"Alice"}})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add: %d %s", resp.StatusCode, body)
	}
	var added struct {
		Added []model.Invite `json:"added"`
	}
	decodeJSON(t, body, &added)
	inv := added.Added[0]

	// Rename via calendar B's path → 404 (invite not on this calendar).
	resp, _ = doJSONAuth(t, ts, http.MethodPut, "/calendars/"+calB.ID+"/invites/"+inv.ID, map[string]string{"name": "X"})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("cross-calendar rename: expected 404, got %d", resp.StatusCode)
	}
}
