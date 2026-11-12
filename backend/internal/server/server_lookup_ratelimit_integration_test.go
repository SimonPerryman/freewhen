//go:build integration

package server_test

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/freewhen/backend/internal/limits"
	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/store"
	"github.com/freewhen/backend/internal/testdb"
)

// New calendars get 12-char IDs; a pre-existing 8-char calendar must still
// resolve through the public GET.
func TestCalendarIDs_NewAreLongAndLegacyStillResolve(t *testing.T) {
	ts := setupTestServer(t)

	resp, body := doJSONAuth(t, ts, http.MethodPost, "/calendars", map[string]string{"name": "New"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d %s", resp.StatusCode, body)
	}
	var created model.Calendar
	decodeJSON(t, body, &created)
	if len(created.ID) != store.CalendarIDLength {
		t.Fatalf("expected %d-char id, got %q", store.CalendarIDLength, created.ID)
	}

	// Insert a legacy 8-char calendar directly, owned by the same user.
	db, err := sql.Open("pgx", testdb.Start(t))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	const legacyID = "leg4cy01"
	if _, err := db.Exec(
		`INSERT INTO calendars (id, name, owner_id) SELECT $1, 'Legacy', owner_id FROM calendars WHERE id = $2`,
		legacyID, created.ID,
	); err != nil {
		t.Fatalf("insert legacy calendar: %v", err)
	}

	resp, body = doJSON(t, ts, http.MethodGet, "/calendars/"+legacyID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get legacy: %d %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `"Legacy"`) {
		t.Fatalf("legacy calendar body missing name: %s", body)
	}
}

// Unknown IDs from one client IP are throttled after the burst; another IP is
// unaffected; the SSE route has its own budget.
func TestCalendarLookup_RateLimitedPerIP(t *testing.T) {
	ts := setupTestServer(t)

	attacker := map[string]string{"X-Forwarded-For": "203.0.113.50"}
	for i := 0; i < limits.CalendarLookupsPerIPPerMinute; i++ {
		resp, body := doJSONRaw(t, ts, http.MethodGet, fmt.Sprintf("/calendars/zzzz%04d", i), nil, "", attacker)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("probe %d: status %d %s, want 404", i+1, resp.StatusCode, body)
		}
	}

	resp, body := doJSONRaw(t, ts, http.MethodGet, "/calendars/zzzzzzzz", nil, "", attacker)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status %d %s, want 429", resp.StatusCode, body)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Fatal("429 missing Retry-After")
	}
	if !strings.Contains(string(body), `"code":"rate_limited"`) {
		t.Fatalf("unexpected 429 body: %s", body)
	}

	// Other by-id routes share the budget, so an attacker can't switch oracle.
	resp, _ = doJSONRaw(t, ts, http.MethodPost, "/calendars/zzzzzzzz/unlock", map[string]string{"passphrase": "x"}, "", attacker)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("unlock: status %d, want 429", resp.StatusCode)
	}

	// A different client IP is unaffected.
	resp, body = doJSONRaw(t, ts, http.MethodGet, "/calendars/zzzzzzzz", nil, "", map[string]string{"X-Forwarded-For": "198.51.100.60"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("other IP: status %d %s, want 404", resp.StatusCode, body)
	}

	// The SSE route has a separate bucket: the drained lookup budget doesn't
	// block the stream (404 here because the calendar doesn't exist).
	resp, body = doJSONRaw(t, ts, http.MethodGet, "/calendars/zzzzzzzz/events", nil, "", attacker)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("events: status %d %s, want 404 (not throttled)", resp.StatusCode, body)
	}
}
