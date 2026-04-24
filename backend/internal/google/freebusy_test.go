package google

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func withStubServer(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	prev := freeBusyURL
	freeBusyURL = srv.URL
	t.Cleanup(func() { freeBusyURL = prev })
	return srv.URL
}

func TestFreeBusy_SuccessReturnsBusyRanges(t *testing.T) {
	var gotBody map[string]any
	var gotAuth string
	withStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		gotAuth = r.Header.Get("Authorization")
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{
			"calendars": {
				"primary": {
					"busy": [
						{"start":"2026-05-01T09:00:00Z","end":"2026-05-01T10:00:00Z"},
						{"start":"2026-05-01T14:00:00Z","end":"2026-05-01T15:30:00Z"}
					]
				}
			}
		}`)
	})

	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	ranges, err := FreeBusy(context.Background(), nil, "tok123", start, end, "UTC")
	if err != nil {
		t.Fatalf("FreeBusy: %v", err)
	}
	if len(ranges) != 2 {
		t.Fatalf("len(ranges) = %d, want 2", len(ranges))
	}
	if gotAuth != "Bearer tok123" {
		t.Errorf("Authorization = %q, want Bearer tok123", gotAuth)
	}
	if gotBody["timeZone"] != "UTC" {
		t.Errorf("timeZone = %v, want UTC", gotBody["timeZone"])
	}
	wantStart := "2026-05-01T09:00:00Z"
	if got := ranges[0].Start.UTC().Format(time.RFC3339); got != wantStart {
		t.Errorf("range[0].Start = %s, want %s", got, wantStart)
	}
}

func TestFreeBusy_PrimaryMissingReturnsNil(t *testing.T) {
	withStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"calendars":{}}`)
	})

	ranges, err := FreeBusy(context.Background(), nil, "tok", time.Now(), time.Now().Add(time.Hour), "UTC")
	if err != nil {
		t.Fatalf("FreeBusy: %v", err)
	}
	if ranges != nil {
		t.Errorf("ranges = %v, want nil", ranges)
	}
}

func TestFreeBusy_CalendarErrorsReturnsError(t *testing.T) {
	withStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"calendars":{"primary":{"errors":[{"domain":"global","reason":"notFound"}]}}}`)
	})

	_, err := FreeBusy(context.Background(), nil, "tok", time.Now(), time.Now().Add(time.Hour), "UTC")
	if err == nil {
		t.Fatal("expected error when calendar errors present")
	}
	if !strings.Contains(err.Error(), "notFound") {
		t.Errorf("err = %v, want to mention reason", err)
	}
}

func TestFreeBusy_Non200ReturnsError(t *testing.T) {
	withStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, "forbidden")
	})

	_, err := FreeBusy(context.Background(), nil, "tok", time.Now(), time.Now().Add(time.Hour), "UTC")
	if err == nil {
		t.Fatal("expected error on non-200")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("err = %v, want to mention status code", err)
	}
}

func TestFreeBusy_InvalidJSONReturnsError(t *testing.T) {
	withStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "not json")
	})

	_, err := FreeBusy(context.Background(), nil, "tok", time.Now(), time.Now().Add(time.Hour), "UTC")
	if err == nil {
		t.Fatal("expected decode error")
	}
}
