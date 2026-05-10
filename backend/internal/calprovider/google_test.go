package calprovider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// withGoogle builds a Google provider whose freeBusy endpoint points at the
// given handler. Returns the provider and the test server's URL.
func withGoogle(t *testing.T, handler http.HandlerFunc) (*Google, string) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	g := NewGoogle("client", "secret", "https://app.test/callback",
		WithGoogleFreeBusyURL(srv.URL),
	)
	return g, srv.URL
}

func TestGoogleFreeBusy_SuccessReturnsBusyRanges(t *testing.T) {
	var gotBody map[string]any
	var gotAuth string
	g, _ := withGoogle(t, func(w http.ResponseWriter, r *http.Request) {
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
	ranges, err := g.FreeBusy(context.Background(), "tok123", start, end, "UTC")
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

func TestGoogleFreeBusy_PrimaryMissingReturnsNil(t *testing.T) {
	g, _ := withGoogle(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"calendars":{}}`)
	})

	ranges, err := g.FreeBusy(context.Background(), "tok", time.Now(), time.Now().Add(time.Hour), "UTC")
	if err != nil {
		t.Fatalf("FreeBusy: %v", err)
	}
	if ranges != nil {
		t.Errorf("ranges = %v, want nil", ranges)
	}
}

func TestGoogleFreeBusy_CalendarErrorsReturnsError(t *testing.T) {
	g, _ := withGoogle(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"calendars":{"primary":{"errors":[{"domain":"global","reason":"notFound"}]}}}`)
	})

	_, err := g.FreeBusy(context.Background(), "tok", time.Now(), time.Now().Add(time.Hour), "UTC")
	if err == nil {
		t.Fatal("expected error when calendar errors present")
	}
	if !strings.Contains(err.Error(), "notFound") {
		t.Errorf("err = %v, want to mention reason", err)
	}
}

func TestGoogleFreeBusy_Non200ReturnsError(t *testing.T) {
	g, _ := withGoogle(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, "forbidden")
	})

	_, err := g.FreeBusy(context.Background(), "tok", time.Now(), time.Now().Add(time.Hour), "UTC")
	if err == nil {
		t.Fatal("expected error on non-200")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("err = %v, want to mention status code", err)
	}
}

func TestGoogleFreeBusy_InvalidJSONReturnsError(t *testing.T) {
	g, _ := withGoogle(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "not json")
	})

	_, err := g.FreeBusy(context.Background(), "tok", time.Now(), time.Now().Add(time.Hour), "UTC")
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestGoogleAddEventURL_AllDay(t *testing.T) {
	g := NewGoogle("client", "secret", "https://app.test/cb")
	got := g.AddEventURL(ExportEvent{
		Title:       "🎉 Trip",
		Description: "Going: Alice, Bob",
		Date:        time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		AllDay:      true,
	})
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Host != "calendar.google.com" || parsed.Path != "/calendar/render" {
		t.Errorf("host/path = %s%s", parsed.Host, parsed.Path)
	}
	q := parsed.Query()
	if q.Get("action") != "TEMPLATE" {
		t.Errorf("action = %q", q.Get("action"))
	}
	if q.Get("dates") != "20260601/20260602" {
		t.Errorf("dates = %q, want 20260601/20260602 (end-exclusive)", q.Get("dates"))
	}
	if q.Get("text") != "🎉 Trip" {
		t.Errorf("text = %q", q.Get("text"))
	}
	if q.Get("details") != "Going: Alice, Bob" {
		t.Errorf("details = %q", q.Get("details"))
	}
}

func TestRegistry_LookupAndIterationOrder(t *testing.T) {
	g1 := NewGoogle("a", "s", "https://x/cb")
	r := NewRegistry(nil, g1, nil)
	got, ok := r.Get("google")
	if !ok || got != g1 {
		t.Fatalf("Get(google) = %v, %v", got, ok)
	}
	if _, ok := r.Get("missing"); ok {
		t.Errorf("Get(missing) ok=true")
	}
	all := r.All()
	if len(all) != 1 || all[0].Name() != "google" {
		t.Errorf("All() = %+v", all)
	}
}

func TestRegistry_NilSafe(t *testing.T) {
	var r *Registry
	if _, ok := r.Get("google"); ok {
		t.Errorf("nil registry Get ok=true")
	}
	if all := r.All(); all != nil {
		t.Errorf("nil registry All = %v", all)
	}
}
