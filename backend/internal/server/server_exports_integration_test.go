//go:build integration

package server_test

import (
	"net/http"
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

	// Unauth → 401.
	resp, _ := doJSON(t, ts, http.MethodGet, "/auth/me", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no cookie: expected 401, got %d", resp.StatusCode)
	}

	// Authenticated → 200 + user JSON.
	resp, body := doJSONAuth(t, ts, http.MethodGet, "/auth/me", nil)
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

	// After logout, the original session token is invalidated; /auth/me should reject it.
	resp, _ = doJSONAuth(t, ts, http.MethodGet, "/auth/me", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("after logout: expected 401, got %d", resp.StatusCode)
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
