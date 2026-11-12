package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/freewhen/backend/internal/limits"
	"github.com/freewhen/backend/internal/ratelimit"
)

func okHandler(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

func lookupReq(xff string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/calendars/abcd1234", nil)
	r.RemoteAddr = "10.0.0.1:1234"
	if xff != "" {
		r.Header.Set("X-Forwarded-For", xff)
	}
	return r
}

func TestCalendarLookupLimiter_429AfterBurst(t *testing.T) {
	l := NewCalendarLookupLimiter(0)
	h := l.Lookups(okHandler)

	for i := 0; i < limits.CalendarLookupsPerIPPerMinute; i++ {
		rr := httptest.NewRecorder()
		h(rr, lookupReq("203.0.113.7"))
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: status %d, want 200", i+1, rr.Code)
		}
	}

	rr := httptest.NewRecorder()
	h(rr, lookupReq("203.0.113.7"))
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429", rr.Code)
	}
	if got := rr.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After = %q, want \"1\"", got)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["code"] != "rate_limited" || body["error"] == "" {
		t.Fatalf("unexpected body %v", body)
	}
}

func TestCalendarLookupLimiter_SeparateIPsIndependent(t *testing.T) {
	l := newCalendarLookupLimiter(ratelimit.New(2, time.Minute), ratelimit.New(2, time.Minute), 0)
	h := l.Lookups(okHandler)

	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		h(rr, lookupReq("198.51.100.1"))
	}
	rr := httptest.NewRecorder()
	h(rr, lookupReq("198.51.100.1"))
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("first IP: status %d, want 429", rr.Code)
	}

	rr = httptest.NewRecorder()
	h(rr, lookupReq("198.51.100.2"))
	if rr.Code != http.StatusOK {
		t.Fatalf("second IP: status %d, want 200", rr.Code)
	}
}

func TestCalendarLookupLimiter_UsesTrustedXFFHop(t *testing.T) {
	// Depth 1: the rightmost entry is our load balancer; the client is the
	// one to its left. A spoofed leftmost entry must not change the key.
	l := newCalendarLookupLimiter(ratelimit.New(1, time.Minute), ratelimit.New(1, time.Minute), 1)
	h := l.Lookups(okHandler)

	rr := httptest.NewRecorder()
	h(rr, lookupReq("1.1.1.1, 203.0.113.9, 35.0.0.1"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rr.Code)
	}
	rr = httptest.NewRecorder()
	h(rr, lookupReq("2.2.2.2, 203.0.113.9, 35.0.0.1"))
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("spoofed leftmost XFF bypassed the limit: status %d", rr.Code)
	}
}

func TestCalendarLookupLimiter_IPv6GroupedBySlash64(t *testing.T) {
	l := newCalendarLookupLimiter(ratelimit.New(1, time.Minute), ratelimit.New(1, time.Minute), 0)
	h := l.Lookups(okHandler)

	rr := httptest.NewRecorder()
	h(rr, lookupReq("2001:db8:1:2::1"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rr.Code)
	}
	rr = httptest.NewRecorder()
	h(rr, lookupReq("2001:db8:1:2:ffff::99"))
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("same /64 should share a bucket: status %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	h(rr, lookupReq("2001:db8:1:3::1"))
	if rr.Code != http.StatusOK {
		t.Fatalf("different /64 should be independent: status %d", rr.Code)
	}
}

func TestRateLimitKey(t *testing.T) {
	cases := map[string]string{
		"203.0.113.7":          "203.0.113.7",
		"::ffff:203.0.113.7":   "203.0.113.7",
		"2001:db8:1:2:3:4:5:6": "2001:db8:1:2::/64",
		"not-an-ip":            "not-an-ip",
		"":                     "",
	}
	for in, want := range cases {
		if got := rateLimitKey(in); got != want {
			t.Errorf("rateLimitKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCalendarLookupLimiter_StreamBucketIndependentOfLookups(t *testing.T) {
	l := NewCalendarLookupLimiter(0)
	lookups := l.Lookups(okHandler)
	streams := l.Streams(okHandler)

	// Drain the lookup budget completely.
	for i := 0; i <= limits.CalendarLookupsPerIPPerMinute; i++ {
		lookups(httptest.NewRecorder(), lookupReq("203.0.113.8"))
	}
	rr := httptest.NewRecorder()
	streams(rr, lookupReq("203.0.113.8"))
	if rr.Code != http.StatusOK {
		t.Fatalf("SSE connect starved by lookups: status %d", rr.Code)
	}
}

// TestCalendarLookupLimiter_SSEReconnectLoopNeverTrips simulates the worst
// realistic reconnect loop: the browser's EventSource retrying every 3s
// (its default) for an hour, plus the refetch the client fires on every
// reconnect, across three tabs from the same IP. None of it may be throttled.
func TestCalendarLookupLimiter_SSEReconnectLoopNeverTrips(t *testing.T) {
	now := time.Unix(0, 0)
	clock := func() time.Time { return now }
	lb := ratelimit.New(limits.CalendarLookupsPerIPPerMinute, time.Minute)
	sb := ratelimit.New(limits.CalendarStreamsPerIPPerMinute, time.Minute)
	lb.WithClock(clock)
	sb.WithClock(clock)
	l := newCalendarLookupLimiter(lb, sb, 0)
	lookups := l.Lookups(okHandler)
	streams := l.Streams(okHandler)

	const tabs = 3
	for tick := 0; tick < 1200; tick++ { // 1200 x 3s = 1 hour
		for tab := 0; tab < tabs; tab++ {
			rr := httptest.NewRecorder()
			streams(rr, lookupReq("203.0.113.10"))
			if rr.Code != http.StatusOK {
				t.Fatalf("tick %d tab %d: SSE reconnect throttled (%d)", tick, tab, rr.Code)
			}
			rr = httptest.NewRecorder()
			lookups(rr, lookupReq("203.0.113.10"))
			if rr.Code != http.StatusOK {
				t.Fatalf("tick %d tab %d: post-reconnect refetch throttled (%d)", tick, tab, rr.Code)
			}
		}
		now = now.Add(3 * time.Second)
	}
}

// TestCalendarLookupLimiter_EnumerationIsThrottled shows the sustained rate a
// single IP gets once its burst is spent: one guess per refill interval.
func TestCalendarLookupLimiter_EnumerationIsThrottled(t *testing.T) {
	now := time.Unix(0, 0)
	lb := ratelimit.New(limits.CalendarLookupsPerIPPerMinute, time.Minute)
	lb.WithClock(func() time.Time { return now })
	l := newCalendarLookupLimiter(lb, ratelimit.New(1, time.Minute), 0)
	h := l.Lookups(okHandler)

	allowed := 0
	// 10 minutes of hammering, 100 requests per second.
	for step := 0; step < 60_000; step++ {
		rr := httptest.NewRecorder()
		h(rr, lookupReq("203.0.113.11"))
		if rr.Code == http.StatusOK {
			allowed++
		}
		now = now.Add(10 * time.Millisecond)
	}
	maxAllowed := limits.CalendarLookupsPerIPPerMinute * 11 // burst + 10 minutes of refill
	if allowed > maxAllowed {
		t.Fatalf("allowed %d lookups in 10 minutes, want <= %d", allowed, maxAllowed)
	}
}
