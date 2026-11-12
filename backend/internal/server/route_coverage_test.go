package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/freewhen/backend/internal/handler"
)

// TestRouteCoverage_AuthClassMatchesAllowList is the default-deny-via-test
// guard for the opt-in middleware pattern. Every registered route must show
// up in the expected map below. New routes default to authRequired in the
// reviewer's mind — adding to authPublic or authOptional is a deliberate
// decision recorded here, not a quiet default.
//
// If this test fails because you added a route, classify it explicitly:
//   - authRequired/authAdmin: register with handleAuth/handleAdmin
//   - authOptional: handler must enforce its own access rules; add a
//     comment near registration explaining the gate
//   - authPublic: must be safe to call anonymously from any origin
func TestRouteCoverage_AuthClassMatchesAllowList(t *testing.T) {
	expected := map[string]authKind{
		// Health
		"GET /health": authPublic,

		// Auth — the login/session-mint paths are intentionally public.
		"GET /auth/{provider}":          authPublic,
		"GET /auth/{provider}/callback": authPublic,
		"GET /auth/me":                  authOptional, // 200 null when unauth'd
		"PATCH /auth/me":                authRequired,
		"POST /auth/logout":             authPublic, // no-op without session
		"POST /auth/logout-all":         authRequired,
		"POST /auth/magic-link":         authPublic,
		"POST /auth/magic-link/verify":  authPublic,

		// User
		"GET /users/me/calendars": authRequired,

		// Admin
		"GET /admin/metrics": authAdmin,

		// Calendars
		"POST /calendars":                                         authRequired,
		"POST /calendars/{id}/duplicate":                          authRequired,
		"GET /calendars/{id}":                                     authOptional,
		"PATCH /calendars/{id}":                                   authRequired,
		"DELETE /calendars/{id}":                                  authRequired,
		"DELETE /calendars/{id}/participants/me":                  authRequired,
		"POST /calendars/{id}/participants":                       authOptional,
		"POST /calendars/{id}/participants/{participantId}/claim": authRequired,
		"DELETE /calendars/{id}/participants/{participantId}":     authRequired,
		"PATCH /calendars/{id}/participants/{participantId}":      authOptional,
		"PUT /calendars/{id}/availability":                        authOptional,
		"POST /calendars/{id}/finalise":                           authRequired,
		"DELETE /calendars/{id}/finalise":                         authRequired,
		"POST /calendars/{id}/unlock":                             authOptional,
		"POST /calendars/{id}/passphrase":                         authRequired,
		"DELETE /calendars/{id}/passphrase":                       authRequired,
		"GET /calendars/{id}/missing":                             authRequired,
		"POST /calendars/{id}/invites":                            authRequired,
		"PUT /calendars/{id}/invites/{inviteId}":                  authRequired,
		"DELETE /calendars/{id}/invites/{inviteId}":               authRequired,
		"POST /calendars/{id}/invites/{inviteId}/match":           authRequired,
		"GET /calendars/{id}/export.ics":                          authOptional,
		"GET /calendars/{id}/export/{provider}":                   authOptional,
		"POST /auth/{provider}/calendar/consent-ack":              authRequired,
		"GET /auth/{provider}/calendar/connect":                   authRequired,
		"GET /auth/{provider}/calendar/callback":                  authRequired,
		"GET /calendars/{id}/import/suggestions":                  authRequired,
		"POST /calendars/{id}/availability/import":                authRequired,
		"GET /calendars/{id}/events":                              authOptional,
	}

	s := newRouteCoverageServer()
	s.routes()

	got := map[string]authKind{}
	for _, r := range s.routeSpecs {
		got[r.pattern] = r.kind
	}

	// Detect orphans (registered but not in expected): forces the developer
	// to make an explicit classification decision.
	for pattern, kind := range got {
		want, ok := expected[pattern]
		if !ok {
			t.Errorf("route %q registered with kind %v but not in expected allow-list — add it to TestRouteCoverage_AuthClassMatchesAllowList with an explicit auth class", pattern, kind)
			continue
		}
		if want != kind {
			t.Errorf("route %q: registered as %v, expected %v — classification changed; verify intentional and update the allow-list", pattern, kind, want)
		}
	}

	// Detect stale entries (in expected but no longer registered).
	for pattern := range expected {
		if _, ok := got[pattern]; !ok {
			t.Errorf("route %q in expected allow-list but not registered — remove if intentional", pattern)
		}
	}
}

// TestRouteCoverage_OptionalRoutesRateLimited guards L6: every route reachable
// without a session that resolves a calendar by ID must carry the per-IP
// lookup limit. The only exemptions are optional routes that never take a
// calendar ID.
func TestRouteCoverage_OptionalRoutesRateLimited(t *testing.T) {
	notCalendarLookup := map[string]bool{
		"GET /auth/me": true, // session probe; 200 null when signed out
	}
	s := newRouteCoverageServer()
	s.routes()
	for _, spec := range s.routeSpecs {
		if notCalendarLookup[spec.pattern] {
			if spec.rateLimited {
				t.Errorf("route %q is exempt but registered with the calendar lookup limit", spec.pattern)
			}
			continue
		}
		if spec.kind == authOptional && !spec.rateLimited {
			t.Errorf("optional-auth route %q is not rate limited", spec.pattern)
		}
	}
}

// TestRouteCoverage_RequiredRoutesReject401 hits every authRequired/authAdmin
// route with no session cookie and asserts 401. Catches the case where a
// handler is registered with handleAuth but the wrap was bypassed somewhere.
func TestRouteCoverage_RequiredRoutesReject401(t *testing.T) {
	s := newRouteCoverageServer()
	s.routes()

	// Distinct patterns we've already exercised; report once per pattern.
	for _, spec := range s.routeSpecs {
		if spec.kind != authRequired && spec.kind != authAdmin {
			continue
		}

		method, path := splitPattern(spec.pattern)
		// Replace path wildcards with placeholder values so the mux matches.
		path = strings.ReplaceAll(path, "{id}", "00000000-0000-0000-0000-000000000000")
		path = strings.ReplaceAll(path, "{participantId}", "00000000-0000-0000-0000-000000000000")
		path = strings.ReplaceAll(path, "{inviteId}", "00000000-0000-0000-0000-000000000000")

		req := httptest.NewRequest(method, path, nil)
		rr := httptest.NewRecorder()
		s.mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401 (no cookie should be rejected before handler runs)", spec.pattern, rr.Code)
		}
	}
}

// newRouteCoverageServer constructs a Server with nil handler fields. Method
// values bound from nil pointer-receiver handlers are safe to register; the
// tests in this file never invoke the underlying handlers, only the auth
// wrappers, so the nils never get dereferenced.
func newRouteCoverageServer() *Server {
	return &Server{
		handler:        (*handler.Handler)(nil),
		events:         (*handler.EventsHandler)(nil),
		auth:           (*handler.AuthHandler)(nil),
		metrics:        (*handler.MetricsHandler)(nil),
		calIntegration: (*handler.CalendarIntegrationHandler)(nil),
		lookupLimiter:  handler.NewCalendarLookupLimiter(0),
		mux:            http.NewServeMux(),
		logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
}

func splitPattern(p string) (method, path string) {
	parts := strings.SplitN(p, " ", 2)
	return parts[0], parts[1]
}
