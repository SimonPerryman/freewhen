//go:build integration

package server_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/freewhen/backend/internal/limits"
	"github.com/freewhen/backend/internal/store"
	"github.com/freewhen/backend/internal/testdb"
)

// openTestDB opens a sql.DB against the shared test Postgres instance. The
// setupTestServer wiring already opened its own connection, but the helpers
// below need their own handle for direct seeding.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", testdb.Start(t))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// ownerUserID returns the primary test user's ID by looking up the known seed
// email. setupTestServer inserts a user with email "test@example.com".
func ownerUserID(t *testing.T, db *sql.DB) string {
	t.Helper()
	var id string
	if err := db.QueryRow(`SELECT id FROM users WHERE email = 'test@example.com'`).Scan(&id); err != nil {
		t.Fatalf("lookup owner: %v", err)
	}
	return id
}

// fillCalendarParticipantsToCap adds anonymous participants to a calendar
// until the total count equals MaxParticipantsPerCalendar. The calendar is
// assumed to already have at least one participant (the owner auto-join).
func fillCalendarParticipantsToCap(t *testing.T, db *sql.DB, calendarID string) {
	t.Helper()
	st := store.New(db)
	ctx := t.Context()

	var existing int
	if err := db.QueryRow(`SELECT COUNT(*) FROM participants WHERE calendar_id = $1`, calendarID).Scan(&existing); err != nil {
		t.Fatalf("count participants: %v", err)
	}
	for i := existing; i < limits.MaxParticipantsPerCalendar; i++ {
		name := fmt.Sprintf("seed-%d", i)
		if _, err := st.AddParticipant(ctx, calendarID, name, nil, nil); err != nil {
			t.Fatalf("seed participant %d: %v", i, err)
		}
	}
}

// createCalendarsUpToCap creates calendars directly via the store until the
// owner has exactly MaxCalendarsPerUser. Returns the ID of the first calendar
// created (useful for duplicate tests).
func createCalendarsUpToCap(t *testing.T, db *sql.DB, ownerID string) string {
	t.Helper()
	st := store.New(db)
	ctx := t.Context()

	var existing int
	if err := db.QueryRow(`SELECT COUNT(*) FROM calendars WHERE owner_id = $1`, ownerID).Scan(&existing); err != nil {
		t.Fatalf("count calendars: %v", err)
	}

	var firstID string
	for i := existing; i < limits.MaxCalendarsPerUser; i++ {
		cal, err := st.CreateCalendar(ctx, fmt.Sprintf("seed-cal-%d", i), "", ownerID)
		if err != nil {
			t.Fatalf("seed calendar %d: %v", i, err)
		}
		if firstID == "" {
			firstID = cal.ID
		}
	}
	if firstID == "" {
		// already at cap on entry — fall back to any owned calendar
		if err := db.QueryRow(`SELECT id FROM calendars WHERE owner_id = $1 LIMIT 1`, ownerID).Scan(&firstID); err != nil {
			t.Fatalf("pick fallback calendar: %v", err)
		}
	}
	return firstID
}

// fillInvitesToCap adds invites directly via the store until the calendar has
// MaxInvitesPerCalendar invites.
func fillInvitesToCap(t *testing.T, db *sql.DB, calendarID string) {
	t.Helper()
	st := store.New(db)
	ctx := t.Context()

	var existing int
	if err := db.QueryRow(`SELECT COUNT(*) FROM calendar_invites WHERE calendar_id = $1`, calendarID).Scan(&existing); err != nil {
		t.Fatalf("count invites: %v", err)
	}

	toAdd := limits.MaxInvitesPerCalendar - existing
	if toAdd <= 0 {
		return
	}
	names := make([]string, 0, toAdd)
	for i := 0; i < toAdd; i++ {
		names = append(names, fmt.Sprintf("invite-seed-%d", existing+i))
	}
	res, err := st.AddInvites(ctx, calendarID, names)
	if err != nil {
		t.Fatalf("seed invites: %v", err)
	}
	if len(res.Added) != toAdd {
		t.Fatalf("expected %d invites added, got %d (skipped %d)", toAdd, len(res.Added), len(res.Skipped))
	}
}

// doJSONWithIP is like doJSON but sets X-Forwarded-For so the server-side
// rate limiter sees each request as coming from the supplied client IP.
func doJSONWithIP(t *testing.T, ts *testServer, method, path string, body any, clientIP string) (*http.Response, []byte) {
	t.Helper()
	var buf io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf = bytes.NewBuffer(data)
	}
	req, err := http.NewRequest(method, ts.URL+path, buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if clientIP != "" {
		req.Header.Set("X-Forwarded-For", clientIP)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return resp, data
}

func assertErrorCode(t *testing.T, body []byte, wantCode string) {
	t.Helper()
	var envelope struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode error envelope: %v, body: %s", err, body)
	}
	if envelope.Code != wantCode {
		t.Errorf("error code = %q, want %q, body: %s", envelope.Code, wantCode, body)
	}
}

func TestAddParticipant_CalendarFull(t *testing.T) {
	ts := setupTestServer(t)
	db := openTestDB(t)

	cal, _ := createCalendarAuth(t, ts, "Packed")
	fillCalendarParticipantsToCap(t, db, cal.ID)

	resp, body := doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants", map[string]string{"name": "One-Too-Many"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d; body: %s", resp.StatusCode, body)
	}
	assertErrorCode(t, body, "calendar_full")
}

func TestCreateCalendar_LimitReached(t *testing.T) {
	ts := setupTestServer(t)
	db := openTestDB(t)

	ownerID := ownerUserID(t, db)
	createCalendarsUpToCap(t, db, ownerID)

	resp, body := doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Just One More"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d; body: %s", resp.StatusCode, body)
	}
	assertErrorCode(t, body, "calendar_limit_reached")
}

func TestDuplicateCalendar_LimitReached(t *testing.T) {
	ts := setupTestServer(t)
	db := openTestDB(t)

	ownerID := ownerUserID(t, db)
	sourceID := createCalendarsUpToCap(t, db, ownerID)

	resp, body := doJSONAuth(t, ts, http.MethodPost, "/calendars/"+sourceID+"/duplicate", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d; body: %s", resp.StatusCode, body)
	}
	assertErrorCode(t, body, "calendar_limit_reached")
}

func TestDuplicateCalendar_CalendarFull(t *testing.T) {
	ts := setupTestServer(t)
	db := openTestDB(t)

	// Only signed-in participants other than the new owner are copied, and
	// the owner's own row is the +1. So the copy only overflows when the
	// owner has no row on the source (removed) and the cap is filled with
	// other signed-in users: copied (cap) + owner (1) > cap → calendar_full.
	// (Anonymous rows don't count — see
	// TestDuplicateCalendar_AnonymousParticipantsDontCountTowardsCap.)
	cal, owner := createCalendarAuth(t, ts, "Packed")
	if _, err := db.Exec(`UPDATE participants SET deleted_at = now() WHERE id = $1`, owner.ID); err != nil {
		t.Fatalf("remove owner row: %v", err)
	}
	st := store.New(db)
	for i := 0; i < limits.MaxParticipantsPerCalendar; i++ {
		u, err := st.CreateUser(t.Context(), fmt.Sprintf("packed-%d@example.com", i), "", "email", "")
		if err != nil {
			t.Fatalf("create user %d: %v", i, err)
		}
		uid := u.ID
		if _, err := st.AddParticipant(t.Context(), cal.ID, fmt.Sprintf("member-%d", i), &uid, nil); err != nil {
			t.Fatalf("add member %d: %v", i, err)
		}
	}

	resp, body := doJSONAuth(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/duplicate", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d; body: %s", resp.StatusCode, body)
	}
	assertErrorCode(t, body, "calendar_full")
}

func TestAddInvites_LimitReached(t *testing.T) {
	ts := setupTestServer(t)
	db := openTestDB(t)

	cal, _ := createCalendarAuth(t, ts, "Guest List")
	fillInvitesToCap(t, db, cal.ID)

	resp, body := doJSONAuth(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/invites", map[string]any{
		"names": []string{"Overflow"},
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d; body: %s", resp.StatusCode, body)
	}
	assertErrorCode(t, body, "invite_limit_reached")
}

func TestUnlock_RateLimited(t *testing.T) {
	ts := setupTestServer(t)

	// Set a passphrase so /unlock has something to reject.
	cal, _ := createCalendarAuth(t, ts, "Locked")
	resp, body := doJSONAuth(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/passphrase", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set passphrase: %d %s", resp.StatusCode, body)
	}
	var set struct {
		Passphrase string `json:"passphrase"`
	}
	decodeJSON(t, body, &set)

	const attackerIP = "203.0.113.7"
	// The limiter allows 5 attempts in the window; the 6th from the same IP
	// against the same calendar should be rejected as rate-limited.
	for i := 0; i < 5; i++ {
		resp, _ := doJSONWithIP(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/unlock",
			map[string]string{"passphrase": "wrong"}, attackerIP)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i+1, resp.StatusCode)
		}
	}
	resp, body = doJSONWithIP(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/unlock",
		map[string]string{"passphrase": "wrong"}, attackerIP)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("6th attempt: expected 429, got %d; body: %s", resp.StatusCode, body)
	}
	if ra := resp.Header.Get("Retry-After"); ra == "" {
		t.Error("expected Retry-After header on 429")
	}
	assertErrorCode(t, body, "rate_limited")

	// A correct passphrase from a different IP must still unlock: the limiter
	// is keyed per (calendar, IP).
	resp, body = doJSONWithIP(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/unlock",
		map[string]string{"passphrase": set.Passphrase}, "198.51.100.42")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("different IP unlock: %d %s", resp.StatusCode, body)
	}
	var unlock struct {
		Token string `json:"token"`
	}
	decodeJSON(t, body, &unlock)
	if unlock.Token == "" {
		t.Error("expected access token on successful unlock from fresh IP")
	}
}
