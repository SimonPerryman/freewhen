//go:build integration

package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/freewhen/backend/internal/handler"
	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/store"
)

// Regression tests for plans/bugfix/participant-authorization.md (M1/L2):
// knowing a participant's public id must not let a caller act as them.

type authFixture struct {
	ts         *testServer
	cal        model.Calendar
	owner      model.Participant
	alice, bob model.Participant
	aliceToken string
	bobToken   string
	otherSess  string
}

func joinAnon(t *testing.T, ts *testServer, calID string, body map[string]string) (model.Participant, string) {
	t.Helper()
	resp, data := doJSONRaw(t, ts, http.MethodPost, "/calendars/"+calID+"/participants", body, "", nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("join %v: %d %s", body, resp.StatusCode, data)
	}
	tok := resp.Header.Get(handler.ParticipantTokenHeader)
	if tok == "" {
		t.Fatalf("anonymous join must return %s", handler.ParticipantTokenHeader)
	}
	if strings.Contains(string(data), tok) {
		t.Fatal("participant token must not appear in the JSON body")
	}
	var p model.Participant
	decodeJSON(t, data, &p)
	return p, tok
}

func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()
	ts := setupTestServer(t)
	cal, owner := createCalendarAuth(t, ts, "Auth Trip")
	alice, aliceTok := joinAnon(t, ts, cal.ID, map[string]string{"name": "Alice", "email": "alice@example.com"})
	bob, bobTok := joinAnon(t, ts, cal.ID, map[string]string{"name": "Bob"})

	st := store.New(openTestDB(t))
	other, err := st.CreateUser(t.Context(), "mallory@example.com", "Mallory", "google", "mallory-sub")
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}
	sess, err := st.CreateSession(t.Context(), other.ID)
	if err != nil {
		t.Fatalf("create other session: %v", err)
	}
	return &authFixture{ts: ts, cal: cal, owner: owner, alice: alice, bob: bob, aliceToken: aliceTok, bobToken: bobTok, otherSess: sess}
}

func tokenHeader(tok string) map[string]string {
	if tok == "" {
		return nil
	}
	return map[string]string{handler.ParticipantTokenHeader: tok}
}

func availabilityOf(t *testing.T, ts *testServer, calID, pid string) map[string]bool {
	t.Helper()
	resp, body := doJSONRaw(t, ts, http.MethodGet, "/calendars/"+calID, nil, "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get calendar: %d %s", resp.StatusCode, body)
	}
	var d model.CalendarWithDetails
	decodeJSON(t, body, &d)
	return datesByParticipant(d.Availability)[pid]
}

func TestParticipantAuth_SetAvailability(t *testing.T) {
	f := newAuthFixture(t)
	path := "/calendars/" + f.cal.ID + "/availability"
	put := func(pid, session, tok string) int {
		resp, _ := doJSONRaw(t, f.ts, http.MethodPut, path, map[string]any{"participant_id": pid, "dates": []string{"2026-05-01"}}, session, tokenHeader(tok))
		return resp.StatusCode
	}

	if got := put(f.alice.ID, "", f.aliceToken); got != http.StatusOK {
		t.Fatalf("alice own write: %d", got)
	}

	cases := []struct {
		name    string
		pid     string
		session string
		tok     string
	}{
		{"anon no token on alice", f.alice.ID, "", ""},
		{"bob's token on alice", f.alice.ID, "", f.bobToken},
		{"anon on owner's row", f.owner.ID, "", ""},
		{"bob's token on owner's row", f.owner.ID, "", f.bobToken},
		{"other user on owner's row", f.owner.ID, f.otherSess, ""},
		{"other user on alice", f.alice.ID, f.otherSess, ""},
		{"owner on alice (no availability override)", f.alice.ID, f.ts.sessionToken, ""},
	}
	for _, tc := range cases {
		if got := put(tc.pid, tc.session, tc.tok); got != http.StatusForbidden {
			t.Errorf("%s: status %d, want 403", tc.name, got)
		}
	}

	// The wipe attack from the report must not have touched alice's row.
	resp, _ := doJSONRaw(t, f.ts, http.MethodPut, path, map[string]any{"participant_id": f.alice.ID, "dates": []string{}}, "", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("wipe attempt: %d, want 403", resp.StatusCode)
	}
	if got := availabilityOf(t, f.ts, f.cal.ID, f.alice.ID); !got["2026-05-01"] {
		t.Fatalf("alice's availability was modified: %v", got)
	}

	// Owner writes their own linked row by session.
	if got := put(f.owner.ID, f.ts.sessionToken, ""); got != http.StatusOK {
		t.Errorf("owner own write: %d", got)
	}
}

func TestParticipantAuth_ImportAvailability(t *testing.T) {
	f := newAuthFixture(t)
	path := "/calendars/" + f.cal.ID + "/availability/import"
	body := func(pid string) map[string]any {
		return map[string]any{"participant_id": pid, "days": []string{"2026-05-01"}}
	}

	resp, data := doJSONRaw(t, f.ts, http.MethodPost, path, body(f.owner.ID), f.otherSess, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("other user importing into owner's row: %d %s, want 403", resp.StatusCode, data)
	}
	resp, data = doJSONRaw(t, f.ts, http.MethodPost, path, body(f.alice.ID), f.otherSess, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("other user importing into alice's row: %d %s, want 403", resp.StatusCode, data)
	}
	resp, data = doJSONRaw(t, f.ts, http.MethodPost, path, body(f.owner.ID), f.ts.sessionToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("owner importing into own row: %d %s", resp.StatusCode, data)
	}
}

func TestParticipantAuth_RenameAndEmail(t *testing.T) {
	f := newAuthFixture(t)
	alicePath := "/calendars/" + f.cal.ID + "/participants/" + f.alice.ID

	// Rename alice with bob's token (and a spoofed body participant_id) → 403.
	resp, _ := doJSONRaw(t, f.ts, http.MethodPatch, alicePath, map[string]string{"name": "Pwned", "participant_id": f.alice.ID}, "", tokenHeader(f.bobToken))
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("rename with bob's token: %d, want 403", resp.StatusCode)
	}
	// Rename the owner's row anonymously → 403.
	resp, _ = doJSONRaw(t, f.ts, http.MethodPatch, "/calendars/"+f.cal.ID+"/participants/"+f.owner.ID, map[string]string{"name": "Pwned"}, "", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("anon rename of owner: %d, want 403", resp.StatusCode)
	}
	// Email update on alice with bob's token → 403.
	resp, _ = doJSONRaw(t, f.ts, http.MethodPatch, alicePath, map[string]string{"email": "evil@example.com"}, "", tokenHeader(f.bobToken))
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("email update with bob's token: %d, want 403", resp.StatusCode)
	}
	// Email update without a token → 403.
	resp, _ = doJSONRaw(t, f.ts, http.MethodPatch, alicePath, map[string]string{"email": ""}, "", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("email clear without token: %d, want 403", resp.StatusCode)
	}

	// Alice with her own token can do both.
	resp, data := doJSONRaw(t, f.ts, http.MethodPatch, alicePath, map[string]string{"name": "Alicia"}, "", tokenHeader(f.aliceToken))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("alice self-rename: %d %s", resp.StatusCode, data)
	}
	resp, data = doJSONRaw(t, f.ts, http.MethodPatch, alicePath, map[string]string{"email": ""}, "", tokenHeader(f.aliceToken))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("alice email clear: %d %s", resp.StatusCode, data)
	}
}

func TestParticipantAuth_NoTokenHashForbidden(t *testing.T) {
	ts := setupTestServer(t)
	cal, _ := createCalendarAuth(t, ts, "No Token")

	// An anonymous row with token_hash NULL (never issued a token) can never
	// be written — there is no trust-on-first-use.
	p, err := store.New(openTestDB(t)).AddParticipant(t.Context(), cal.ID, "Ghost", nil, nil)
	if err != nil {
		t.Fatalf("seed participant: %v", err)
	}
	path := "/calendars/" + cal.ID + "/availability"
	body := map[string]any{"participant_id": p.ID, "dates": []string{"2026-05-01"}}

	resp, data := doJSONRaw(t, ts, http.MethodPut, path, body, "", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("write to tokenless row: %d %s, want 403", resp.StatusCode, data)
	}
	if got := resp.Header.Get(handler.ParticipantTokenHeader); got != "" {
		t.Errorf("no token may be issued outside join, got %q", got)
	}
}

func TestParticipantAuth_Claim(t *testing.T) {
	f := newAuthFixture(t)
	claimPath := "/calendars/" + f.cal.ID + "/participants/" + f.alice.ID + "/claim"

	// Signed-in stranger without alice's token → 403.
	resp, data := doJSONRaw(t, f.ts, http.MethodPost, claimPath, nil, f.otherSess, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("claim without token: %d %s, want 403", resp.StatusCode, data)
	}
	// With bob's token → 403.
	resp, _ = doJSONRaw(t, f.ts, http.MethodPost, claimPath, nil, f.otherSess, tokenHeader(f.bobToken))
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("claim with bob's token: %d, want 403", resp.StatusCode)
	}
	// With alice's token → 200, and alice's stored email is not echoed.
	resp, data = doJSONRaw(t, f.ts, http.MethodPost, claimPath, nil, f.otherSess, tokenHeader(f.aliceToken))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("claim with token: %d %s", resp.StatusCode, data)
	}
	if strings.Contains(string(data), "alice@example.com") || strings.Contains(string(data), `"email"`) {
		t.Errorf("claim response leaked email: %s", data)
	}
}

func TestParticipantAuth_ClaimRespectsPassphraseGate(t *testing.T) {
	f := newAuthFixture(t)
	resp, body := doJSONAuth(t, f.ts, http.MethodPost, "/calendars/"+f.cal.ID+"/passphrase", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set passphrase: %d %s", resp.StatusCode, body)
	}
	claimPath := "/calendars/" + f.cal.ID + "/participants/" + f.alice.ID + "/claim"
	resp, body = doJSONRaw(t, f.ts, http.MethodPost, claimPath, nil, f.otherSess, tokenHeader(f.aliceToken))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("claim on gated calendar without access token: %d %s, want 401", resp.StatusCode, body)
	}
}
