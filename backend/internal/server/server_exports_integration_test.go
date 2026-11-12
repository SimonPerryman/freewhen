//go:build integration

package server_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/freewhen/backend/internal/model"
)

func TestHealth_OK(t *testing.T) {
	ts := setupTestServer(t)

	resp, body := doJSON(t, ts, http.MethodGet, "/health", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d, body: %s", resp.StatusCode, body)
	}
}

func TestAuth_Me(t *testing.T) {
	ts := setupTestServer(t)

	// Signed out is a normal state, not an error: 200 with a JSON null body
	// for both a missing cookie and an unknown/expired session token.
	resp, body := doJSON(t, ts, http.MethodGet, "/auth/me", nil)
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != "null" {
		t.Errorf("no cookie: expected 200 null, got %d %q", resp.StatusCode, body)
	}
	resp, body = doJSONWithSession(t, ts, http.MethodGet, "/auth/me", nil, "not-a-real-session")
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != "null" {
		t.Errorf("invalid cookie: expected 200 null, got %d %q", resp.StatusCode, body)
	}

	// Other authenticated endpoints still 401 without a session.
	resp, _ = doJSON(t, ts, http.MethodGet, "/users/me/calendars", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("/users/me/calendars no cookie: expected 401, got %d", resp.StatusCode)
	}

	// Authenticated → 200 + user JSON.
	resp, body = doJSONAuth(t, ts, http.MethodGet, "/auth/me", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("auth: expected 200, got %d, body: %s", resp.StatusCode, body)
	}
	var u model.User
	decodeJSON(t, body, &u)
	if u.Email != "test@example.com" {
		t.Errorf("email = %q, want test@example.com", u.Email)
	}
}

func TestAuth_UpdateMe(t *testing.T) {
	ts := setupTestServer(t)

	// Happy path — change display name.
	resp, body := doJSONAuth(t, ts, http.MethodPatch, "/auth/me", map[string]string{"name": "Renamed"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch: status %d, body %s", resp.StatusCode, body)
	}
	var u model.User
	decodeJSON(t, body, &u)
	if u.Name != "Renamed" {
		t.Errorf("name = %q, want Renamed", u.Name)
	}

	// Empty body → 400 (no fields).
	resp, _ = doJSONAuth(t, ts, http.MethodPatch, "/auth/me", map[string]any{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty: expected 400, got %d", resp.StatusCode)
	}

	// Empty name → 400.
	resp, _ = doJSONAuth(t, ts, http.MethodPatch, "/auth/me", map[string]string{"name": ""})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("blank name: expected 400, got %d", resp.StatusCode)
	}

	// Unauth → 401.
	resp, _ = doJSON(t, ts, http.MethodPatch, "/auth/me", map[string]string{"name": "X"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauth: expected 401, got %d", resp.StatusCode)
	}
}

func TestAuth_Logout(t *testing.T) {
	ts := setupTestServer(t)

	// Logout with session present.
	resp, _ := doJSONAuth(t, ts, http.MethodPost, "/auth/logout", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("logout: expected 200, got %d", resp.StatusCode)
	}
	// Cookie should be cleared (Set-Cookie with MaxAge < 0).
	var sessionClear bool
	for _, c := range resp.Cookies() {
		if c.Name == "session" && c.MaxAge < 0 {
			sessionClear = true
		}
	}
	if !sessionClear {
		t.Error("expected session cookie to be cleared")
	}

	// After logout, the original session token is invalidated; /auth/me
	// reports signed out (200 null) and authenticated routes reject it.
	resp, body := doJSONAuth(t, ts, http.MethodGet, "/auth/me", nil)
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != "null" {
		t.Errorf("after logout: expected 200 null, got %d %q", resp.StatusCode, body)
	}
	resp, _ = doJSONAuth(t, ts, http.MethodGet, "/users/me/calendars", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("after logout /users/me/calendars: expected 401, got %d", resp.StatusCode)
	}

	// Logout without session → still 200 (idempotent).
	resp, _ = doJSON(t, ts, http.MethodPost, "/auth/logout", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("idempotent logout: expected 200, got %d", resp.StatusCode)
	}
}

func TestExportICS(t *testing.T) {
	ts := setupTestServer(t)

	_, body := doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Ski Trip"})
	var cal model.Calendar
	decodeJSON(t, body, &cal)

	// Not finalised → 404 (distinct error code but same status).
	resp, _ := doJSON(t, ts, http.MethodGet, "/calendars/"+cal.ID+"/export.ics", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("pre-finalise: expected 404, got %d", resp.StatusCode)
	}

	// Finalise it.
	resp, body = doJSONAuth(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/finalise", map[string]string{"date": "2026-06-01"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("finalise: status %d, body %s", resp.StatusCode, body)
	}

	// Now export — public (no auth needed).
	resp, body = doJSON(t, ts, http.MethodGet, "/calendars/"+cal.ID+"/export.ics", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("export: status %d, body %s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/calendar") {
		t.Errorf("Content-Type = %q, want text/calendar", ct)
	}
	s := string(body)
	for _, want := range []string{"BEGIN:VCALENDAR", "BEGIN:VEVENT", "DTSTART;VALUE=DATE:20260601", "Ski Trip", "END:VCALENDAR"} {
		if !strings.Contains(s, want) {
			t.Errorf("ICS missing %q; body:\n%s", want, s)
		}
	}

	// Missing calendar → 404.
	resp, _ = doJSON(t, ts, http.MethodGet, "/calendars/nope/export.ics", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("missing: expected 404, got %d", resp.StatusCode)
	}
}

// TestExportEnrichedDescription covers the per-day attendees + notes + return
// link embedded in both .ics and Google export.
func TestExportEnrichedDescription(t *testing.T) {
	ts := setupTestServer(t)

	_, body := doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Ski Trip"})
	var createResp struct {
		model.Calendar
		Participant *model.Participant `json:"participant"`
	}
	decodeJSON(t, body, &createResp)
	cal := createResp.Calendar
	owner := createResp.Participant

	addParticipant := func(name string) model.Participant {
		t.Helper()
		resp, body := doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants", map[string]string{"name": name})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("add %s: %d %s", name, resp.StatusCode, body)
		}
		var p model.Participant
		decodeJSON(t, body, &p)
		return p
	}
	alice := addParticipant("Alice")
	bob := addParticipant("Bob")

	setAvail := func(p model.Participant, dates []string, notes map[string]string) {
		t.Helper()
		payload := map[string]any{"participant_id": p.ID, "dates": dates}
		if notes != nil {
			payload["notes"] = notes
		}
		// Session authorizes the owner's linked row; anonymous rows are
		// authorized by the participant token doJSON* replays.
		resp, body := doJSONAuth(t, ts, http.MethodPut, "/calendars/"+cal.ID+"/availability", payload)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("set availability for %s: %d %s", p.Name, resp.StatusCode, body)
		}
	}
	// Test User (owner), Alice, Bob all available on 2026-06-01. Alice and
	// Bob leave notes; the owner does not.
	setAvail(*owner, []string{"2026-06-01"}, nil)
	setAvail(alice, []string{"2026-06-01"}, map[string]string{"2026-06-01": "Have to leave by 9pm"})
	setAvail(bob, []string{"2026-06-01"}, map[string]string{"2026-06-01": "Can host"})

	resp, body := doJSONAuth(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/finalise", map[string]string{"date": "2026-06-01"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("finalise: %d %s", resp.StatusCode, body)
	}

	// .ics
	resp, body = doJSON(t, ts, http.MethodGet, "/calendars/"+cal.ID+"/export.ics", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ics: %d %s", resp.StatusCode, body)
	}
	// Unfold per RFC 5545 §3.1 before content assertions; folded continuations
	// are presentational, not semantic.
	icsBody := strings.ReplaceAll(string(body), "\r\n ", "")
	// Description shape — note ICS escapes commas as `\,` and newlines as `\n`.
	for _, want := range []string{
		"DESCRIPTION:Going: Alice\\, Bob\\, Test User",
		`Notes:`,
		`Alice — Have to leave by 9pm`,
		`Bob — Can host`,
		"https://freewhen.test/calendars/" + cal.ID,
		"Created with FreeWhen",
	} {
		if !strings.Contains(icsBody, want) {
			t.Errorf("ICS missing %q; body:\n%s", want, icsBody)
		}
	}

	// Google redirect
	client := ts.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/calendars/"+cal.ID+"/export/google", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	gResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer gResp.Body.Close()
	if gResp.StatusCode != http.StatusFound {
		t.Fatalf("google: status %d", gResp.StatusCode)
	}
	loc := gResp.Header.Get("Location")
	parsed, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse google url: %v", err)
	}
	details := parsed.Query().Get("details")
	for _, want := range []string{
		"Going: Alice, Bob, Test User",
		"Notes:",
		"Alice — Have to leave by 9pm",
		"Bob — Can host",
		"https://freewhen.test/calendars/" + cal.ID,
		"Created with FreeWhen",
	} {
		if !strings.Contains(details, want) {
			t.Errorf("google details missing %q; got:\n%s", want, details)
		}
	}
}

// TestExportDescriptionNoNotes covers the branch where everyone's available
// but no one left a note — the "Notes:" block must be omitted entirely.
func TestExportDescriptionNoNotes(t *testing.T) {
	ts := setupTestServer(t)

	_, body := doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Lunch"})
	var createResp struct {
		model.Calendar
		Participant *model.Participant `json:"participant"`
	}
	decodeJSON(t, body, &createResp)
	cal := createResp.Calendar

	// The owner's participant row is user-linked, so the write needs the
	// owner's session.
	resp, _ := doJSONAuth(t, ts, http.MethodPut, "/calendars/"+cal.ID+"/availability", map[string]any{
		"participant_id": createResp.Participant.ID,
		"dates":          []string{"2026-06-01"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set avail: %d", resp.StatusCode)
	}
	resp, _ = doJSONAuth(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/finalise", map[string]string{"date": "2026-06-01"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("finalise: %d", resp.StatusCode)
	}

	resp, body = doJSON(t, ts, http.MethodGet, "/calendars/"+cal.ID+"/export.ics", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ics: %d", resp.StatusCode)
	}
	s := string(body)
	if strings.Contains(s, "Notes:") {
		t.Errorf("ICS should not contain Notes section; got:\n%s", s)
	}
	if !strings.Contains(s, "Going: Test User") {
		t.Errorf("ICS missing Going line; got:\n%s", s)
	}
}

func TestExportGoogle_Redirect(t *testing.T) {
	ts := setupTestServer(t)

	_, body := doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Trip"})
	var cal model.Calendar
	decodeJSON(t, body, &cal)

	// Pre-finalise → 404.
	resp, _ := doJSON(t, ts, http.MethodGet, "/calendars/"+cal.ID+"/export/google", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("pre-finalise: expected 404, got %d", resp.StatusCode)
	}

	resp, _ = doJSONAuth(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/finalise", map[string]string{"date": "2026-06-01"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("finalise: %d", resp.StatusCode)
	}

	// Don't follow redirect automatically.
	client := ts.Client()
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/calendars/"+cal.ID+"/export/google", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want 302", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "https://calendar.google.com/calendar/render?") {
		t.Errorf("Location = %q, want Google render URL", loc)
	}
	for _, want := range []string{"action=TEMPLATE", "dates=20260601", "Trip"} {
		if !strings.Contains(loc, want) {
			t.Errorf("Location missing %q: %s", want, loc)
		}
	}
}

// TestExport_PassphraseGate verifies both export endpoints apply the same
// passphrase gate as GET /calendars/{id}: anonymous callers without a valid
// access token get 401; the token works via header or `?access=`; the owner
// bypasses the gate via session.
func TestExport_PassphraseGate(t *testing.T) {
	ts := setupTestServer(t)
	cal, _ := createCalendarAuth(t, ts, "Secret Trip")

	resp, body := doJSONAuth(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/finalise", map[string]string{"date": "2026-06-01"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("finalise: %d %s", resp.StatusCode, body)
	}
	resp, body = doJSONAuth(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/passphrase", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set passphrase: %d %s", resp.StatusCode, body)
	}
	var setResp struct {
		Passphrase string `json:"passphrase"`
	}
	decodeJSON(t, body, &setResp)

	for _, path := range []string{"/export.ics", "/export/google"} {
		resp, body = doJSON(t, ts, http.MethodGet, "/calendars/"+cal.ID+path, nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s anon: expected 401, got %d", path, resp.StatusCode)
		}
		if strings.Contains(string(body), "Secret Trip") {
			t.Errorf("%s anon: response leaked calendar title", path)
		}
		resp, _ = doJSON(t, ts, http.MethodGet, "/calendars/"+cal.ID+path+"?access=bogus", nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s bogus token: expected 401, got %d", path, resp.StatusCode)
		}
	}

	resp, body = doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/unlock", map[string]string{"passphrase": setResp.Passphrase})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unlock: %d %s", resp.StatusCode, body)
	}
	var unlock struct {
		Token string `json:"token"`
	}
	decodeJSON(t, body, &unlock)

	// Header token.
	resp, body = doJSONWithAccess(t, ts, http.MethodGet, "/calendars/"+cal.ID+"/export.ics", nil, unlock.Token)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "Secret Trip") {
		t.Errorf("header token: status %d body %s", resp.StatusCode, body)
	}
	// Query-param token (plain link navigation can't set headers).
	resp, body = doJSON(t, ts, http.MethodGet, "/calendars/"+cal.ID+"/export.ics?access="+url.QueryEscape(unlock.Token), nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "Secret Trip") {
		t.Errorf("query token: status %d body %s", resp.StatusCode, body)
	}
	// Owner session bypasses the gate.
	resp, body = doJSONAuth(t, ts, http.MethodGet, "/calendars/"+cal.ID+"/export.ics", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("owner: status %d body %s", resp.StatusCode, body)
	}

	client := ts.Client()
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/calendars/"+cal.ID+"/export/google?access="+url.QueryEscape(unlock.Token), nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	gresp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer gresp.Body.Close()
	if gresp.StatusCode != http.StatusFound {
		t.Errorf("google with token: status = %d, want 302", gresp.StatusCode)
	}
}
