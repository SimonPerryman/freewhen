//go:build integration

package server_test

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/freewhen/backend/internal/config"
	"github.com/freewhen/backend/internal/server"
	"github.com/freewhen/backend/internal/store"
	"github.com/freewhen/backend/internal/testdb"
)

// setupTestServerWithSecondUser stands up a server (mirroring setupTestServer)
// and provisions a second authenticated user, returning their session token
// and id. Used by tests that need two distinct authenticated callers.
func setupTestServerWithSecondUser(t *testing.T) (ts *testServer, secondToken, secondUserID string) {
	t.Helper()
	dsn := testdb.Start(t)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := store.Migrate(db, logger); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, table := range []string{"availability", "calendar_invites", "participants", "sessions", "magic_link_tokens", "calendars", "users"} {
		if _, err := db.Exec("DELETE FROM " + table); err != nil {
			t.Fatalf("clean %s: %v", table, err)
		}
	}
	cfg := &config.Config{
		CORSOrigin:           "*",
		CalendarAccessSecret: "test-secret-at-least-32-bytes-long-xx",
	}
	hs := httptest.NewServer(server.New(db, logger, cfg))
	t.Cleanup(hs.Close)

	st := store.New(db)
	ctx := t.Context()
	primary, err := st.CreateUser(ctx, "primary@example.com", "Primary", "google", "primary-sub")
	if err != nil {
		t.Fatalf("create primary user: %v", err)
	}
	primaryToken, err := st.CreateSession(ctx, primary.ID)
	if err != nil {
		t.Fatalf("create primary session: %v", err)
	}
	second, err := st.CreateUser(ctx, "second@example.com", "Second", "google", "second-sub")
	if err != nil {
		t.Fatalf("create second user: %v", err)
	}
	tok, err := st.CreateSession(ctx, second.ID)
	if err != nil {
		t.Fatalf("create second session: %v", err)
	}
	return &testServer{Server: hs, sessionToken: primaryToken}, tok, second.ID
}

// --- DELETE /calendars/{id} -----------------------------------------------

func TestDeleteCalendar_OwnerSucceedsAndIsIdempotent(t *testing.T) {
	ts := setupTestServer(t)
	cal, _ := createCalendarAuth(t, ts, "Doomed")

	resp, body := doJSONAuth(t, ts, http.MethodDelete, "/calendars/"+cal.ID, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("first delete: %d %s", resp.StatusCode, body)
	}

	resp, body = doJSONAuth(t, ts, http.MethodDelete, "/calendars/"+cal.ID, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("second (idempotent) delete: %d %s", resp.StatusCode, body)
	}

	// /users/me/calendars no longer surfaces the calendar.
	resp, body = doJSONAuth(t, ts, http.MethodGet, "/users/me/calendars", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list calendars: %d %s", resp.StatusCode, body)
	}
	var listed struct {
		Calendars []map[string]any `json:"calendars"`
	}
	if err := json.Unmarshal(body, &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed.Calendars) != 0 {
		t.Fatalf("expected 0 calendars after delete, got %d", len(listed.Calendars))
	}

	// GET on the deleted calendar returns 404 (read endpoints stay 404).
	resp, _ = doJSONAuth(t, ts, http.MethodGet, "/calendars/"+cal.ID, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("get after delete: expected 404, got %d", resp.StatusCode)
	}

	// Mutating writes return 410 (calendar_unavailable).
	resp, body = doJSONAuth(t, ts, http.MethodPatch, "/calendars/"+cal.ID, map[string]string{"name": "Zombie"})
	if resp.StatusCode != http.StatusGone {
		t.Fatalf("patch after delete: expected 410, got %d body=%s", resp.StatusCode, body)
	}
	var errBody struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &errBody); err != nil {
		t.Fatalf("decode err body: %v", err)
	}
	if errBody.Code != "calendar_unavailable" {
		t.Fatalf("expected code calendar_unavailable, got %q", errBody.Code)
	}
}

func TestDeleteCalendar_NonOwnerForbidden(t *testing.T) {
	ts, otherToken, _ := setupTestServerWithSecondUser(t)
	cal, _ := createCalendarAuth(t, ts, "Owned")

	resp, body := doJSONWithSession(t, ts, http.MethodDelete, "/calendars/"+cal.ID, nil, otherToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-owner delete: expected 403, got %d body=%s", resp.StatusCode, body)
	}
}

func TestDeleteCalendar_Unauthorized(t *testing.T) {
	ts := setupTestServer(t)
	cal, _ := createCalendarAuth(t, ts, "Owned")

	resp, _ := doJSON(t, ts, http.MethodDelete, "/calendars/"+cal.ID, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// --- DELETE /calendars/{id}/participants/me --------------------------------

func TestLeaveCalendar_MemberLeavesIdempotently(t *testing.T) {
	ts, secondToken, _ := setupTestServerWithSecondUser(t)
	cal, _ := createCalendarAuth(t, ts, "Group Trip")

	resp, body := doJSONWithSession(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants",
		map[string]any{"name": "Second"}, secondToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("join: %d %s", resp.StatusCode, body)
	}

	resp, body = doJSONWithSession(t, ts, http.MethodDelete, "/calendars/"+cal.ID+"/participants/me", nil, secondToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("leave: %d %s", resp.StatusCode, body)
	}

	resp, _ = doJSONWithSession(t, ts, http.MethodDelete, "/calendars/"+cal.ID+"/participants/me", nil, secondToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("idempotent leave: %d", resp.StatusCode)
	}

	resp, body = doJSONWithSession(t, ts, http.MethodGet, "/users/me/calendars", nil, secondToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list after leave: %d %s", resp.StatusCode, body)
	}
	var listed struct {
		Calendars []map[string]any `json:"calendars"`
	}
	_ = json.Unmarshal(body, &listed)
	if len(listed.Calendars) != 0 {
		t.Fatalf("expected 0 calendars after leave, got %d", len(listed.Calendars))
	}
}

func TestLeaveCalendar_OwnerBlocked(t *testing.T) {
	ts := setupTestServer(t)
	cal, _ := createCalendarAuth(t, ts, "Owned")

	resp, body := doJSONAuth(t, ts, http.MethodDelete, "/calendars/"+cal.ID+"/participants/me", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("owner leave: expected 409, got %d body=%s", resp.StatusCode, body)
	}
	var errBody struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(body, &errBody)
	if errBody.Code != "cannot_leave_owned_calendar" {
		t.Fatalf("expected code cannot_leave_owned_calendar, got %q", errBody.Code)
	}
}
