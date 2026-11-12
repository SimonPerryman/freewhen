package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/freewhen/backend/internal/model"
)

// TestMe_SignedOutReturns200Null pins that a signed-out /auth/me is a normal
// 200 with a JSON null body, not a 401 (which browsers log as a console
// error on every signed-out page load).
func TestMe_SignedOutReturns200Null(t *testing.T) {
	h := &AuthHandler{}
	rec := httptest.NewRecorder()
	h.Me(rec, httptest.NewRequest(http.MethodGet, "/auth/me", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "null" {
		t.Errorf("body = %q, want null", got)
	}
}

func TestMe_SignedInReturnsUser(t *testing.T) {
	h := &AuthHandler{}
	user := model.User{ID: "u_alice", Email: "alice@example.com", Name: "Alice"}
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, &user))
	rec := httptest.NewRecorder()
	h.Me(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got model.User
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != user.ID || got.Email != user.Email {
		t.Errorf("user = %+v, want %+v", got, user)
	}
}
