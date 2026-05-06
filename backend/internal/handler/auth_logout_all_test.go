package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/freewhen/backend/internal/model"
)

// TestLogoutAll_DeletesAllUserSessionsAndIssuesFresh pins the F12 contract:
// every existing session row for the user is gone after the call, and the
// caller leaves with a brand-new session cookie. Other users are untouched.
func TestLogoutAll_DeletesAllUserSessionsAndIssuesFresh(t *testing.T) {
	st := newFakeAuthStore()
	user := model.User{ID: "u_alice", Email: "alice@example.com"}
	st.usersByID[user.ID] = user
	other := model.User{ID: "u_bob", Email: "bob@example.com"}
	st.usersByID[other.ID] = other

	// Three existing sessions for alice (phone, laptop, desktop) + one for bob.
	for i := 0; i < 3; i++ {
		if _, err := st.CreateSession(context.Background(), user.ID); err != nil {
			t.Fatalf("seed alice session %d: %v", i, err)
		}
	}
	if _, err := st.CreateSession(context.Background(), other.ID); err != nil {
		t.Fatalf("seed bob session: %v", err)
	}

	h := newTestAuthHandler(t, st, nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout-all", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, &user))
	rec := httptest.NewRecorder()
	h.LogoutAll(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}

	// Exactly one alice session remains — the freshly issued one.
	aliceSessions := 0
	for _, s := range st.sessions {
		if s.UserID == user.ID {
			aliceSessions++
		}
	}
	if aliceSessions != 1 {
		t.Fatalf("alice sessions remaining: got %d, want 1", aliceSessions)
	}

	// Bob's session is untouched.
	bobSessions := 0
	for _, s := range st.sessions {
		if s.UserID == other.ID {
			bobSessions++
		}
	}
	if bobSessions != 1 {
		t.Fatalf("bob sessions remaining: got %d, want 1", bobSessions)
	}

	// Response sets a session cookie matching the new row.
	var sessCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			sessCookie = c
		}
	}
	if sessCookie == nil || sessCookie.Value == "" {
		t.Fatalf("session cookie not set on response")
	}
	if _, ok := st.sessions[sessCookie.Value]; !ok {
		t.Fatalf("cookie token %q does not match any session row", sessCookie.Value)
	}
}

func TestLogoutAll_Unauthenticated(t *testing.T) {
	h := newTestAuthHandler(t, newFakeAuthStore(), nil)
	req := httptest.NewRequest(http.MethodPost, "/auth/logout-all", nil)
	rec := httptest.NewRecorder()
	h.LogoutAll(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", rec.Code)
	}
}
