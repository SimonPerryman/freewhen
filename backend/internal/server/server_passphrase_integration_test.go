//go:build integration

package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/freewhen/backend/internal/handler"
)

// doJSONWithAccess is like doJSON but also sends the X-Calendar-Access header.
func doJSONWithAccess(t *testing.T, ts *testServer, method, path string, body any, accessToken string) (*http.Response, []byte) {
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
	if accessToken != "" {
		req.Header.Set(handler.AccessHeader, accessToken)
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

func TestPassphrase_SetGetUnlockFlow(t *testing.T) {
	ts := setupTestServer(t)
	cal, _ := createCalendarAuth(t, ts, "Secret Trip")

	// Before passphrase: GET returns full details (no gate).
	resp, body := doJSON(t, ts, http.MethodGet, "/calendars/"+cal.ID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pre-gate get: %d %s", resp.StatusCode, body)
	}

	// Set passphrase as owner.
	resp, body = doJSONAuth(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/passphrase", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set passphrase: %d %s", resp.StatusCode, body)
	}
	var setResp struct {
		Passphrase string `json:"passphrase"`
	}
	decodeJSON(t, body, &setResp)
	if setResp.Passphrase == "" {
		t.Fatal("expected plaintext passphrase in response")
	}

	// Anonymous GET now → gated response with requires_passphrase=true.
	resp, body = doJSON(t, ts, http.MethodGet, "/calendars/"+cal.ID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("gated get: %d %s", resp.StatusCode, body)
	}
	var gated struct {
		RequiresPassphrase bool `json:"requires_passphrase"`
	}
	decodeJSON(t, body, &gated)
	if !gated.RequiresPassphrase {
		t.Errorf("expected requires_passphrase=true; body: %s", body)
	}

	// Wrong passphrase → 401.
	resp, _ = doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/unlock", map[string]string{"passphrase": "nope"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong passphrase: expected 401, got %d", resp.StatusCode)
	}

	// Empty passphrase → 400.
	resp, _ = doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/unlock", map[string]string{"passphrase": "  "})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty passphrase: expected 400, got %d", resp.StatusCode)
	}

	// Correct passphrase → token.
	resp, body = doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/unlock", map[string]string{"passphrase": setResp.Passphrase})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unlock: %d %s", resp.StatusCode, body)
	}
	var unlock struct {
		Token string `json:"token"`
	}
	decodeJSON(t, body, &unlock)
	if unlock.Token == "" {
		t.Fatal("expected access token")
	}

	// With the access token, anon GET returns full details.
	resp, body = doJSONWithAccess(t, ts, http.MethodGet, "/calendars/"+cal.ID, nil, unlock.Token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("gated get with token: %d %s", resp.StatusCode, body)
	}
	var withToken struct {
		RequiresPassphrase bool `json:"requires_passphrase"`
		Participants       []any `json:"participants"`
	}
	decodeJSON(t, body, &withToken)
	if withToken.RequiresPassphrase {
		t.Errorf("token should bypass gate; body: %s", body)
	}
	if len(withToken.Participants) == 0 {
		t.Errorf("expected participants in gated response; body: %s", body)
	}

	// Rotating the passphrase bumps the epoch, invalidating the old token.
	resp, body = doJSONAuth(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/passphrase", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rotate: %d %s", resp.StatusCode, body)
	}
	var newSet struct {
		Passphrase string `json:"passphrase"`
	}
	decodeJSON(t, body, &newSet)
	if newSet.Passphrase == setResp.Passphrase {
		t.Error("rotated passphrase equals previous")
	}

	resp, body = doJSONWithAccess(t, ts, http.MethodGet, "/calendars/"+cal.ID, nil, unlock.Token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("after rotate: %d %s", resp.StatusCode, body)
	}
	decodeJSON(t, body, &gated)
	if !gated.RequiresPassphrase {
		t.Error("old token should be invalidated after epoch bump")
	}

	// Clear passphrase as owner → 204, gate lifted.
	resp, _ = doJSONAuth(t, ts, http.MethodDelete, "/calendars/"+cal.ID+"/passphrase", nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("clear: expected 204, got %d", resp.StatusCode)
	}

	resp, body = doJSON(t, ts, http.MethodGet, "/calendars/"+cal.ID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("post-clear get: %d %s", resp.StatusCode, body)
	}
	var postClear struct {
		RequiresPassphrase bool                `json:"requires_passphrase"`
		Participants       []map[string]any    `json:"participants"`
	}
	decodeJSON(t, body, &postClear)
	if postClear.RequiresPassphrase {
		t.Errorf("expected gate lifted after clear; body: %s", body)
	}
	if len(postClear.Participants) == 0 {
		t.Errorf("expected participants visible after clear; body: %s", body)
	}
}

func TestPassphrase_OwnerOnly(t *testing.T) {
	ts := setupTestServer(t)
	cal, _ := createCalendarAuth(t, ts, "Trip")

	// Unauth set → 401.
	resp, _ := doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/passphrase", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauth set: expected 401, got %d", resp.StatusCode)
	}

	// Unauth clear → 401.
	resp, _ = doJSON(t, ts, http.MethodDelete, "/calendars/"+cal.ID+"/passphrase", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauth clear: expected 401, got %d", resp.StatusCode)
	}
}

func TestUnlock_NoPassphrase(t *testing.T) {
	ts := setupTestServer(t)
	cal, _ := createCalendarAuth(t, ts, "Trip")

	// No passphrase set → unlock responds 400 no_passphrase.
	resp, _ := doJSON(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/unlock", map[string]string{"passphrase": "whatever"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestUnlock_CalendarNotFound(t *testing.T) {
	ts := setupTestServer(t)

	resp, _ := doJSON(t, ts, http.MethodPost, "/calendars/doesnotexist/unlock", map[string]string{"passphrase": "x"})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}
