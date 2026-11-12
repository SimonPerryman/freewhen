package handler

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/freewhen/backend/internal/limits"
	"github.com/freewhen/backend/internal/ratelimit"
)

// CalendarLookupLimiter rate-limits, per client IP, the /calendars/{id}
// routes that resolve a calendar by ID without requiring a session. For a
// calendar without a passphrase the ID is the only secret, and every one of
// those routes answers 404 for an unknown ID, so without a limit the ID space
// could be probed for free. Budgets live in internal/limits; see
// plans/tech-debt/calendar-id-entropy-and-lookup-rate-limit.md.
//
// SSE connects get their own bucket so a burst of refetches never starves the
// stream (and vice versa).
type CalendarLookupLimiter struct {
	lookups           *ratelimit.Bucket
	streams           *ratelimit.Bucket
	trustedProxyDepth int
}

// NewCalendarLookupLimiter builds the limiter. trustedProxyDepth has the same
// meaning as for magic-link and unlock limiting (see remoteIP).
func NewCalendarLookupLimiter(trustedProxyDepth int) *CalendarLookupLimiter {
	return newCalendarLookupLimiter(
		ratelimit.New(limits.CalendarLookupsPerIPPerMinute, time.Minute),
		ratelimit.New(limits.CalendarStreamsPerIPPerMinute, time.Minute),
		trustedProxyDepth,
	)
}

func newCalendarLookupLimiter(lookups, streams *ratelimit.Bucket, trustedProxyDepth int) *CalendarLookupLimiter {
	if trustedProxyDepth < 0 {
		trustedProxyDepth = 0
	}
	return &CalendarLookupLimiter{lookups: lookups, streams: streams, trustedProxyDepth: trustedProxyDepth}
}

// Lookups wraps a by-id calendar route (read, join, vote, unlock, export).
func (l *CalendarLookupLimiter) Lookups(next http.HandlerFunc) http.HandlerFunc {
	return l.wrap(l.lookups, next)
}

// Streams wraps the SSE events route.
func (l *CalendarLookupLimiter) Streams(next http.HandlerFunc) http.HandlerFunc {
	return l.wrap(l.streams, next)
}

func (l *CalendarLookupLimiter) wrap(b *ratelimit.Bucket, next http.HandlerFunc) http.HandlerFunc {
	retryAfter := strconv.Itoa(int(math.Ceil(b.RefillInterval().Seconds())))
	if retryAfter == "0" {
		retryAfter = "1"
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !b.Allow(rateLimitKey(remoteIP(r, l.trustedProxyDepth))) {
			w.Header().Set("Retry-After", retryAfter)
			respondError(w, http.StatusTooManyRequests, "rate_limited", "too many requests, try again shortly")
			return
		}
		next(w, r)
	}
}

// rateLimitKey normalises a client IP into a bucket key. IPv6 clients are
// grouped by their /64 — a single host is typically handed a whole /64, so
// keying on the full address would let it rotate through 2^64 buckets.
// Anything that doesn't parse as an IP is used verbatim.
func rateLimitKey(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ip
	}
	if v4 := parsed.To4(); v4 != nil {
		return v4.String()
	}
	return parsed.Mask(net.CIDRMask(64, 128)).String() + "/64"
}
