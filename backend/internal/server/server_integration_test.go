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

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/server"
	"github.com/freewhen/backend/internal/store"
)

func setupTestServer(t *testing.T) *httptest.Server {
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

	for _, table := range []string{"availability", "participants", "calendars"} {
		if _, err := db.Exec("DELETE FROM " + table); err != nil {
			t.Fatalf("clean %s: %v", table, err)
		}
	}

	ts := httptest.NewServer(server.New(db, logger, "*"))
	t.Cleanup(ts.Close)
	return ts
}

func doJSON(t *testing.T, ts *httptest.Server, method, path string, body any) (*http.Response, []byte) {
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

	// 1. Create calendar
	resp, body := doJSON(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Weekend Trip"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create calendar: status %d, body %s", resp.StatusCode, body)
	}
	var cal model.Calendar
	decodeJSON(t, body, &cal)
	if cal.ID == "" {
		t.Fatal("expected calendar ID, got empty")
	}
	if cal.Name != "Weekend Trip" {
		t.Errorf("expected name %q, got %q", "Weekend Trip", cal.Name)
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
	if !names["Alice"] || !names["Bob"] {
		t.Errorf("expected both Alice and Bob in participants, got %+v", details.Participants)
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

	resp, _ := doJSON(t, ts, http.MethodPost, "/calendars", "not json")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
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

	resp, body := doJSON(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Test"})
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

	resp, body := doJSON(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Test"})
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
	_, body := doJSON(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "A"})
	var calA model.Calendar
	decodeJSON(t, body, &calA)

	// Participant on A
	_, body = doJSON(t, ts, http.MethodPost, "/calendars/"+calA.ID+"/participants", map[string]string{"name": "Alice"})
	var alice model.Participant
	decodeJSON(t, body, &alice)

	// Calendar B
	_, body = doJSON(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "B"})
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

	_, body := doJSON(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Test"})
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

func TestSetAvailability_EmptyParticipantID(t *testing.T) {
	ts := setupTestServer(t)

	_, body := doJSON(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "Test"})
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
