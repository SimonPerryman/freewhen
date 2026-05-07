package handler

import (
	"net/http/httptest"
	"testing"
)

// TestRemoteIPRejectsForgedXFF pins the rate-limiter input invariant from
// auth-audit F17: a malicious client cannot poison the per-IP bucket by
// supplying its own X-Forwarded-For header. Cloud Run / GCP front-end
// appends the real client IP (and any LB hops) to the right of whatever
// the client sent, so the trusted client IP lives at len-1-trustedProxyDepth.
func TestRemoteIPRejectsForgedXFF(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		xff       string
		remote    string
		depth     int
		want      string
	}{
		// Cloud Run direct (no LB): GFE appends the real client IP.
		{name: "direct: no xff falls back to RemoteAddr", xff: "", remote: "203.0.113.10:54321", depth: 0, want: "203.0.113.10"},
		{name: "direct: honest client", xff: "203.0.113.10", remote: "10.0.0.1:0", depth: 0, want: "203.0.113.10"},
		{name: "direct: forged leftmost ignored", xff: "1.2.3.4, 203.0.113.10", remote: "10.0.0.1:0", depth: 0, want: "203.0.113.10"},
		{name: "direct: multiple forged entries ignored", xff: "1.2.3.4, 5.6.7.8, 9.10.11.12, 203.0.113.10", remote: "10.0.0.1:0", depth: 0, want: "203.0.113.10"},
		{name: "direct: IPv6 last entry", xff: "1.2.3.4, 2001:db8::1", remote: "10.0.0.1:0", depth: 0, want: "2001:db8::1"},
		{name: "direct: trims whitespace", xff: "  203.0.113.10  ", remote: "10.0.0.1:0", depth: 0, want: "203.0.113.10"},

		// Behind external HTTPS LB: depth=1 skips the LB hop.
		{name: "lb: honest client", xff: "203.0.113.10, 130.211.0.1", remote: "10.0.0.1:0", depth: 1, want: "203.0.113.10"},
		{name: "lb: forged leftmost ignored", xff: "1.2.3.4, 203.0.113.10, 130.211.0.1", remote: "10.0.0.1:0", depth: 1, want: "203.0.113.10"},

		// Misconfigured / malicious: fewer entries than depth+1 → fall back to RemoteAddr.
		// This is the safe failure mode: don't trust attacker input when the proxy
		// chain doesn't match the configured depth.
		{name: "depth too high falls back to RemoteAddr", xff: "1.2.3.4", remote: "10.0.0.5:0", depth: 1, want: "10.0.0.5"},
		{name: "RemoteAddr without port", xff: "", remote: "10.0.0.5", depth: 0, want: "10.0.0.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequest("POST", "/", nil)
			r.RemoteAddr = tt.remote
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			got := remoteIP(r, tt.depth)
			if got != tt.want {
				t.Fatalf("remoteIP(depth=%d, xff=%q, remote=%q) = %q, want %q",
					tt.depth, tt.xff, tt.remote, got, tt.want)
			}
		})
	}
}
