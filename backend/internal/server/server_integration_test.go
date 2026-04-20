//go:build integration

package server_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"github.com/freewhen/backend/internal/config"
	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/server"
	"github.com/freewhen/backend/internal/store"
)

type testServer struct {
	*httptest.Server
	sessionToken string
}

func setupTestServer(t *testing.T) *testServer {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := store.Migrate(db, logger); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	for _, table := range []string{"availability", "participants", "sessions", "magic_link_tokens", "calendars", "users"} {
		if _, err := db.Exec("DELETE FROM " + table); err != nil {
			t.Fatalf("clean %s: %v", table, err)
		}
	}

	cfg := &config.Config{
		CORSOrigin: "*",
	}
	ts := httptest.NewServer(server.New(db, logger, cfg))
	t.Cleanup(ts.Close)

	// Create a test user and session for authenticated requests
	st := store.New(db)
	ctx := t.Context()
	user, err := st.CreateUser(ctx, "test@example.com", "Test User", "google", "test-sub")
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	token, err := st.CreateSession(ctx, user.ID)
	if err != nil {
		t.Fatalf("create test session: %v", err)
	}

	return &testServer{Server: ts, sessionToken: token}
}

func doJSON(t *testing.T, ts *testServer, method, path string, body any) (*http.Response, []byte) {
	return doJSONWithSession(t, ts, method, path, body, "")
}

func doJSONAuth(t *testing.T, ts *testServer, method, path string, body any) (*http.Response, []byte) {
	return doJSONWithSession(t, ts, method, path, body, ts.sessionToken)
}

func doJSONWithSession(t *testing.T, ts *testServer, method, path string, body any, sessionToken string) (*http.Response, []byte) {
	t.Helper()

	var buf io.Reader
	if body != nil {
		switch b := body.(type) {
		case string:
			buf = bytes.NewBufferString(b)
		case []byte:
			buf = bytes.NewBuffer(b)
		default:
			data, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("marshal body: %v", err)
			}
			buf = bytes.NewBuffer(data)
		}
	}

	req, err := http.NewRequest(method, ts.URL+path, buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if sessionToken != "" {
		req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	}

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return resp, data
}

func decodeJSON(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("decode JSON: %v\nbody: %s", err, string(data))
	}
}

func TestFullFlow(t *testing.T) {
	ts := setupTestServer(t)

	// 1. Create calendar (owner auto-joins as participant)
	resp, body := doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Weekend Trip"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create calendar: status %d, body %s", resp.StatusCode, body)
	}
	var createResp struct {
		model.Calendar
		Participant *model.Participant `json:"participant"`
	}
	decodeJSON(t, body, &createResp)
	cal := createResp.Calendar
	if cal.ID == "" {
		t.Fatal("expected calendar ID, got empty")
	}
	if cal.Name != "Weekend Trip" {
		t.Errorf("expected name %q, got %q", "Weekend Trip", cal.Name)
	}
	if createResp.Participant == nil {
		t.Fatal("expected owner participant in create response")
	}
	if createResp.Participant.UserID == nil {
		t.Fatal("expected owner participant to have user_id set")
	}

	// 2. Add Alice
	resp, body = doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants", map[string]string{"name": "Alice"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add alice: status %d, body %s", resp.StatusCode, body)
	}
	var alice model.Participant
	decodeJSON(t, body, &alice)
	if alice.ID == "" {
		t.Fatal("expected alice ID, got empty")
	}
	if alice.UserID != nil {
		t.Error("expected anonymous alice to have nil user_id")
	}

	// 3. Add Bob
	resp, body = doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants", map[string]string{"name": "Bob"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add bob: status %d, body %s", resp.StatusCode, body)
	}
	var bob model.Participant
	decodeJSON(t, body, &bob)
	if bob.ID == "" {
		t.Fatal("expected bob ID, got empty")
	}

	// 4. Set Alice's availability
	aliceDates := []string{"2026-05-01", "2026-05-02", "2026-05-03"}
	resp, body = doJSON(t, ts, http.MethodPut, "/calendars/"+cal.ID+"/availability", map[string]any{
		"participant_id": alice.ID,
		"dates":          aliceDates,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set alice availability: status %d, body %s", resp.StatusCode, body)
	}

	// 5. Set Bob's availability
	bobDates := []string{"2026-05-02", "2026-05-03", "2026-05-04"}
	resp, body = doJSON(t, ts, http.MethodPut, "/calendars/"+cal.ID+"/availability", map[string]any{
		"participant_id": bob.ID,
		"dates":          bobDates,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set bob availability: status %d, body %s", resp.StatusCode, body)
	}

	// 6. Fetch and assert
	resp, body = doJSON(t, ts, http.MethodGet, "/calendars/"+cal.ID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get calendar: status %d, body %s", resp.StatusCode, body)
	}
	var details model.CalendarWithDetails
	decodeJSON(t, body, &details)

	if details.Calendar.ID != cal.ID {
		t.Errorf("expected calendar ID %q, got %q", cal.ID, details.Calendar.ID)
	}

	names := map[string]bool{}
	for _, p := range details.Participants {
		names[p.Name] = true
	}
	if !names["Alice"] || !names["Bob"] || !names["Test User"] {
		t.Errorf("expected Test User, Alice and Bob in participants, got %+v", details.Participants)
	}

	got := datesByParticipant(details.Availability)
	assertDateSet(t, "alice", got[alice.ID], aliceDates)
	assertDateSet(t, "bob", got[bob.ID], bobDates)

	// Sub-test: clearing availability with empty array
	t.Run("ClearAvailability", func(t *testing.T) {
		resp, body := doJSON(t, ts, http.MethodPut, "/calendars/"+cal.ID+"/availability", map[string]any{
			"participant_id": alice.ID,
			"dates":          []string{},
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("clear alice availability: status %d, body %s", resp.StatusCode, body)
		}

		resp, body = doJSON(t, ts, http.MethodGet, "/calendars/"+cal.ID, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("get calendar: status %d, body %s", resp.StatusCode, body)
		}
		var details model.CalendarWithDetails
		decodeJSON(t, body, &details)

		got := datesByParticipant(details.Availability)
		if len(got[alice.ID]) != 0 {
			t.Errorf("expected no availability for alice, got %v", got[alice.ID])
		}
		assertDateSet(t, "bob", got[bob.ID], bobDates)
	})
}

func datesByParticipant(rows []model.Availability) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, a := range rows {
		if _, ok := out[a.ParticipantID]; !ok {
			out[a.ParticipantID] = map[string]bool{}
		}
		out[a.ParticipantID][a.Date] = true
	}
	return out
}

func assertDateSet(t *testing.T, who string, got map[string]bool, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: expected %d dates, got %d (%v)", who, len(want), len(got), got)
	}
	for _, d := range want {
		if !got[d] {
			t.Errorf("%s: missing date %q in %v", who, d, got)
		}
	}
}

func TestCreateCalendar_BadJSON(t *testing.T) {
	ts := setupTestServer(t)

	resp, _ := doJSONAuth(t, ts, http.MethodPost, "/calendars", "not json")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateCalendar_Unauthorized(t *testing.T) {
	ts := setupTestServer(t)

	resp, _ := doJSON(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Test"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestGetCalendar_NotFound(t *testing.T) {
	ts := setupTestServer(t)

	resp, _ := doJSON(t, ts, http.MethodGet, "/calendars/doesnotexist", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAddParticipant_CalendarNotFound(t *testing.T) {
	ts := setupTestServer(t)

	resp, _ := doJSON(t, ts, http.MethodPost, "/calendars/doesnotexist/participants", map[string]string{"name": "Alice"})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAddParticipant_EmptyName(t *testing.T) {
	ts := setupTestServer(t)

	resp, body := doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Test"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create calendar: %d %s", resp.StatusCode, body)
	}
	var cal model.Calendar
	decodeJSON(t, body, &cal)

	resp, _ = doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants", map[string]string{"name": ""})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAddParticipant_DuplicateName(t *testing.T) {
	ts := setupTestServer(t)

	resp, body := doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Test"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create calendar: %d %s", resp.StatusCode, body)
	}
	var cal model.Calendar
	decodeJSON(t, body, &cal)

	resp, _ = doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants", map[string]string{"name": "Alice"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first alice: status %d", resp.StatusCode)
	}

	resp, _ = doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants", map[string]string{"name": "Alice"})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409, got %d", resp.StatusCode)
	}
}

func TestSetAvailability_ParticipantWrongCalendar(t *testing.T) {
	ts := setupTestServer(t)

	// Calendar A
	_, body := doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "A"})
	var calA model.Calendar
	decodeJSON(t, body, &calA)

	// Participant on A
	_, body = doJSON(t, ts, http.MethodPost, "/calendars/"+calA.ID+"/participants", map[string]string{"name": "Alice"})
	var alice model.Participant
	decodeJSON(t, body, &alice)

	// Calendar B
	_, body = doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "B"})
	var calB model.Calendar
	decodeJSON(t, body, &calB)

	// PUT availability on B with A's participant ID
	resp, _ := doJSON(t, ts, http.MethodPut, "/calendars/"+calB.ID+"/availability", map[string]any{
		"participant_id": alice.ID,
		"dates":          []string{"2026-05-01"},
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestSetAvailability_InvalidDate(t *testing.T) {
	ts := setupTestServer(t)

	_, body := doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Test"})
	var cal model.Calendar
	decodeJSON(t, body, &cal)

	_, body = doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants", map[string]string{"name": "Alice"})
	var alice model.Participant
	decodeJSON(t, body, &alice)

	resp, _ := doJSON(t, ts, http.MethodPut, "/calendars/"+cal.ID+"/availability", map[string]any{
		"participant_id": alice.ID,
		"dates":          []string{"not-a-date"},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAdminMetrics_Forbidden(t *testing.T) {
	ts := setupTestServer(t)

	// Unauthenticated → 401
	resp, _ := doJSON(t, ts, http.MethodGet, "/admin/metrics", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}

	// Authenticated but not admin → 403
	resp, _ = doJSONAuth(t, ts, http.MethodGet, "/admin/metrics", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestAdminMetrics_OK(t *testing.T) {
	ts := setupTestServer(t)

	// Promote the test user to admin
	dsn := os.Getenv("TEST_DATABASE_URL")
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE users SET is_admin = true WHERE email = 'test@example.com'`); err != nil {
		t.Fatalf("promote admin: %v", err)
	}

	resp, body := doJSONAuth(t, ts, http.MethodGet, "/admin/metrics", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", resp.StatusCode, body)
	}

	var metrics struct {
		Users struct {
			Total   int `json:"total"`
			Last7d  int `json:"last_7d"`
			Last30d int `json:"last_30d"`
		} `json:"users"`
		Calendars struct {
			Total     int `json:"total"`
			Last7d    int `json:"last_7d"`
			Active30d int `json:"active_30d"`
		} `json:"calendars"`
		Participants struct {
			Total          int     `json:"total"`
			AvgPerCalendar float64 `json:"avg_per_calendar"`
		} `json:"participants"`
	}
	decodeJSON(t, body, &metrics)

	// We created one test user in setupTestServer
	if metrics.Users.Total < 1 {
		t.Errorf("expected at least 1 user, got %d", metrics.Users.Total)
	}
}

func TestSetAvailability_EmptyParticipantID(t *testing.T) {
	ts := setupTestServer(t)

	_, body := doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Test"})
	var cal model.Calendar
	decodeJSON(t, body, &cal)

	resp, _ := doJSON(t, ts, http.MethodPut, "/calendars/"+cal.ID+"/availability", map[string]any{
		"participant_id": "",
		"dates":          []string{"2026-05-01"},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestFinaliseFlow(t *testing.T) {
	ts := setupTestServer(t)

	// Create calendar as owner
	_, body := doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Trip"})
	var cal model.Calendar
	decodeJSON(t, body, &cal)

	// Add a participant (unauth: allowed while open)
	_, body = doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants", map[string]string{"name": "Alice"})
	var alice model.Participant
	decodeJSON(t, body, &alice)

	// Finalise as owner
	resp, body := doJSONAuth(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/finalise", map[string]string{"date": "2026-05-02"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("finalise: status %d, body %s", resp.StatusCode, body)
	}
	var finalised model.Calendar
	decodeJSON(t, body, &finalised)
	if finalised.FinalisedDate == nil || *finalised.FinalisedDate != "2026-05-02" {
		t.Fatalf("finalised_date = %v, want 2026-05-02", finalised.FinalisedDate)
	}
	if finalised.FinalisedAt == nil {
		t.Fatalf("finalised_at should be set")
	}

	// Attempt availability write while locked → 409
	resp, _ = doJSON(t, ts, http.MethodPut, "/calendars/"+cal.ID+"/availability", map[string]any{
		"participant_id": alice.ID,
		"dates":          []string{"2026-05-02"},
	})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("availability while finalised: status %d, want 409", resp.StatusCode)
	}

	// Attempt to add participant while locked → 409
	resp, _ = doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants", map[string]string{"name": "Bob"})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("add participant while finalised: status %d, want 409", resp.StatusCode)
	}

	// Re-finalise with a different date (replaces)
	resp, body = doJSONAuth(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/finalise", map[string]string{"date": "2026-05-03"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("re-finalise: status %d, body %s", resp.StatusCode, body)
	}
	decodeJSON(t, body, &finalised)
	if finalised.FinalisedDate == nil || *finalised.FinalisedDate != "2026-05-03" {
		t.Fatalf("re-finalised_date = %v, want 2026-05-03", finalised.FinalisedDate)
	}

	// Unfinalise as owner
	resp, body = doJSONAuth(t, ts, http.MethodDelete, "/calendars/"+cal.ID+"/finalise", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unfinalise: status %d, body %s", resp.StatusCode, body)
	}
	decodeJSON(t, body, &finalised)
	if finalised.FinalisedDate != nil {
		t.Fatalf("expected finalised_date cleared, got %v", *finalised.FinalisedDate)
	}

	// After unfinalise, writes succeed again
	resp, _ = doJSON(t, ts, http.MethodPut, "/calendars/"+cal.ID+"/availability", map[string]any{
		"participant_id": alice.ID,
		"dates":          []string{"2026-05-02"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("availability after unfinalise: status %d, want 200", resp.StatusCode)
	}
}

func TestFinalise_Unauthorized(t *testing.T) {
	ts := setupTestServer(t)

	_, body := doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Trip"})
	var cal model.Calendar
	decodeJSON(t, body, &cal)

	// Unauthenticated → 401
	resp, _ := doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/finalise", map[string]string{"date": "2026-05-02"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestFinalise_NotOwner(t *testing.T) {
	ts := setupTestServer(t)

	// Owner creates calendar
	_, body := doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Trip"})
	var cal model.Calendar
	decodeJSON(t, body, &cal)

	// Second user tries to finalise
	dsn := os.Getenv("TEST_DATABASE_URL")
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	st := store.New(db)
	otherUser, err := st.CreateUser(t.Context(), "other@example.com", "Other User", "google", "other-sub")
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}
	otherToken, err := st.CreateSession(t.Context(), otherUser.ID)
	if err != nil {
		t.Fatalf("create other session: %v", err)
	}

	resp, _ := doJSONWithSession(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/finalise", map[string]string{"date": "2026-05-02"}, otherToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for non-owner, got %d", resp.StatusCode)
	}
}

func TestClaimParticipant(t *testing.T) {
	ts := setupTestServer(t)

	// Create calendar
	_, body := doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Test"})
	var createResp struct {
		model.Calendar
	}
	decodeJSON(t, body, &createResp)
	cal := createResp.Calendar

	// Add anonymous participant
	_, body = doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants", map[string]string{"name": "Alice"})
	var alice model.Participant
	decodeJSON(t, body, &alice)
	if alice.UserID != nil {
		t.Fatal("expected nil user_id before claim")
	}

	// Create a second user to claim
	dsn := os.Getenv("TEST_DATABASE_URL")
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	st := store.New(db)
	claimUser, err := st.CreateUser(t.Context(), "alice@example.com", "Alice Real", "google", "alice-sub")
	if err != nil {
		t.Fatalf("create claim user: %v", err)
	}
	claimToken, err := st.CreateSession(t.Context(), claimUser.ID)
	if err != nil {
		t.Fatalf("create claim session: %v", err)
	}

	// Claim without auth → 401
	resp, _ := doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants/"+alice.ID+"/claim", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}

	// Claim with auth → 200
	resp, body = doJSONWithSession(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants/"+alice.ID+"/claim", nil, claimToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("claim: status %d, body %s", resp.StatusCode, body)
	}
	var claimed model.Participant
	decodeJSON(t, body, &claimed)
	if claimed.UserID == nil {
		t.Fatal("expected user_id after claim")
	}
	if *claimed.UserID != claimUser.ID {
		t.Errorf("expected user_id %q, got %q", claimUser.ID, *claimed.UserID)
	}

	// Claim again → 409 (already linked)
	resp, _ = doJSONWithSession(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants/"+alice.ID+"/claim", nil, claimToken)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 for already linked, got %d", resp.StatusCode)
	}
}

func TestAddParticipant_AuthLinksUser(t *testing.T) {
	ts := setupTestServer(t)

	// Create calendar
	_, body := doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Test"})
	var createResp struct {
		model.Calendar
	}
	decodeJSON(t, body, &createResp)
	cal := createResp.Calendar

	// Create second user
	dsn := os.Getenv("TEST_DATABASE_URL")
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	st := store.New(db)
	joiner, err := st.CreateUser(t.Context(), "joiner@example.com", "Joiner", "google", "joiner-sub")
	if err != nil {
		t.Fatalf("create joiner: %v", err)
	}
	joinerToken, err := st.CreateSession(t.Context(), joiner.ID)
	if err != nil {
		t.Fatalf("create joiner session: %v", err)
	}

	// Add participant while authenticated → auto-links user_id
	resp, body := doJSONWithSession(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants", map[string]string{"name": "Joiner"}, joinerToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add joiner: status %d, body %s", resp.StatusCode, body)
	}
	var p model.Participant
	decodeJSON(t, body, &p)
	if p.UserID == nil {
		t.Fatal("expected user_id to be set when joining authenticated")
	}
	if *p.UserID != joiner.ID {
		t.Errorf("expected user_id %q, got %q", joiner.ID, *p.UserID)
	}

	// Try to join again as same user → 409
	resp, _ = doJSONWithSession(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/participants", map[string]string{"name": "Different Name"}, joinerToken)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 for duplicate user join, got %d", resp.StatusCode)
	}
}

func TestListUserCalendars(t *testing.T) {
	ts := setupTestServer(t)

	// No calendars yet → empty list.
	resp, body := doJSONAuth(t, ts, http.MethodGet, "/users/me/calendars", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: status %d, body %s", resp.StatusCode, body)
	}
	var empty struct {
		Calendars []map[string]any `json:"calendars"`
		HasMore   bool             `json:"has_more"`
	}
	decodeJSON(t, body, &empty)
	if len(empty.Calendars) != 0 {
		t.Fatalf("expected empty calendars, got %v", empty.Calendars)
	}
	if empty.HasMore {
		t.Fatal("expected has_more = false on empty list")
	}

	// Unauthenticated → 401.
	resp, _ = doJSON(t, ts, http.MethodGet, "/users/me/calendars", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", resp.StatusCode)
	}

	// Create two calendars as the primary user.
	_, body = doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Alpha"})
	var first struct {
		model.Calendar
	}
	decodeJSON(t, body, &first)

	_, body = doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Beta"})
	var second struct {
		model.Calendar
	}
	decodeJSON(t, body, &second)

	// Create a second user who joins Alpha.
	dsn := os.Getenv("TEST_DATABASE_URL")
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	st := store.New(db)
	joiner, err := st.CreateUser(t.Context(), "joiner@listtest.example", "Joiner", "google", "list-joiner-sub")
	if err != nil {
		t.Fatalf("create joiner: %v", err)
	}
	joinerToken, err := st.CreateSession(t.Context(), joiner.ID)
	if err != nil {
		t.Fatalf("create joiner session: %v", err)
	}

	resp, body = doJSONWithSession(t, ts, http.MethodPost, "/calendars/"+first.ID+"/participants", map[string]string{"name": "Joiner"}, joinerToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("joiner add participant: status %d, body %s", resp.StatusCode, body)
	}

	// Primary user's list: two calendars (both owner), Alpha moved to top by the
	// joiner's activity, Beta second.
	resp, body = doJSONAuth(t, ts, http.MethodGet, "/users/me/calendars", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("primary list: status %d, body %s", resp.StatusCode, body)
	}
	var primary struct {
		Calendars []struct {
			ID               string `json:"id"`
			Name             string `json:"name"`
			MyRole           string `json:"my_role"`
			ParticipantCount int    `json:"participant_count"`
		} `json:"calendars"`
		HasMore bool `json:"has_more"`
	}
	decodeJSON(t, body, &primary)
	if len(primary.Calendars) != 2 {
		t.Fatalf("expected 2 calendars, got %d", len(primary.Calendars))
	}
	if primary.Calendars[0].ID != first.ID {
		t.Errorf("expected Alpha first after join activity, got %q", primary.Calendars[0].ID)
	}
	for _, row := range primary.Calendars {
		if row.MyRole != "owner" {
			t.Errorf("row %q my_role = %q, want owner", row.ID, row.MyRole)
		}
	}

	// Joiner's list: only Alpha, as joiner.
	resp, body = doJSONWithSession(t, ts, http.MethodGet, "/users/me/calendars", nil, joinerToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("joiner list: status %d, body %s", resp.StatusCode, body)
	}
	var joinerList struct {
		Calendars []struct {
			ID     string `json:"id"`
			MyRole string `json:"my_role"`
		} `json:"calendars"`
	}
	decodeJSON(t, body, &joinerList)
	if len(joinerList.Calendars) != 1 {
		t.Fatalf("joiner expected 1 calendar, got %d", len(joinerList.Calendars))
	}
	if joinerList.Calendars[0].ID != first.ID {
		t.Errorf("joiner got id=%q, want %q", joinerList.Calendars[0].ID, first.ID)
	}
	if joinerList.Calendars[0].MyRole != "joiner" {
		t.Errorf("joiner my_role = %q, want joiner", joinerList.Calendars[0].MyRole)
	}
}

func TestCORS_Preflight(t *testing.T) {
	ts := setupTestServer(t)

	resp, _ := doJSON(t, ts, http.MethodOptions, "/calendars", nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") == "" {
		t.Error("missing Access-Control-Allow-Origin header")
	}
	if resp.Header.Get("Access-Control-Allow-Methods") == "" {
		t.Error("missing Access-Control-Allow-Methods header")
	}
	if resp.Header.Get("Access-Control-Allow-Headers") == "" {
		t.Error("missing Access-Control-Allow-Headers header")
	}
}
