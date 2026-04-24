package server

import (
	"bytes"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(corsOrigin string, logBuf io.Writer) *Server {
	if logBuf == nil {
		logBuf = io.Discard
	}
	return &Server{
		logger:     slog.New(slog.NewJSONHandler(logBuf, nil)),
		corsOrigin: corsOrigin,
	}
}

func TestCORS_SetsHeaders(t *testing.T) {
	s := newTestServer("https://example.com", nil)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/foo", nil)
	rr := httptest.NewRecorder()

	s.cors(next).ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("Allow-Origin = %q, want example.com", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "GET") {
		t.Errorf("Allow-Methods = %q, want to contain GET", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Content-Type") {
		t.Errorf("Allow-Headers = %q, want Content-Type", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Allow-Credentials = %q, want true", got)
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestCORS_PreflightShortCircuits(t *testing.T) {
	s := newTestServer("*", nil)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	req := httptest.NewRequest(http.MethodOptions, "/foo", nil)
	rr := httptest.NewRecorder()

	s.cors(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rr.Code)
	}
	if called {
		t.Error("next handler should not be called on OPTIONS preflight")
	}
}

func TestRequireAuth_NoCookie_Returns401(t *testing.T) {
	s := newTestServer("*", nil)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rr := httptest.NewRecorder()

	s.requireAuth(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
	if nextCalled {
		t.Error("next handler should not be called without auth")
	}
	if !strings.Contains(rr.Body.String(), "unauthorized") {
		t.Errorf("body = %q, want to contain 'unauthorized'", rr.Body.String())
	}
}

func TestRequireAdmin_NoCookie_Returns401(t *testing.T) {
	s := newTestServer("*", nil)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rr := httptest.NewRecorder()

	s.requireAdmin(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (requireAdmin wraps requireAuth)", rr.Code)
	}
	if nextCalled {
		t.Error("next handler should not be called without auth")
	}
}

func TestOptionalAuth_NoCookie_CallsNext(t *testing.T) {
	s := newTestServer("*", nil)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	rr := httptest.NewRecorder()

	s.optionalAuth(next).ServeHTTP(rr, req)

	if !nextCalled {
		t.Error("optionalAuth should call next even without cookie")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestRequestLog_SetsRequestIDHeader(t *testing.T) {
	var buf bytes.Buffer
	s := newTestServer("*", &buf)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	rr := httptest.NewRecorder()

	s.requestLog(next).ServeHTTP(rr, req)

	id := rr.Header().Get("X-Request-ID")
	if id == "" {
		t.Fatal("X-Request-ID header not set")
	}
	if len(id) != 16 {
		t.Errorf("X-Request-ID = %q (len %d), want 16 hex chars", id, len(id))
	}
	if _, err := hex.DecodeString(id); err != nil {
		t.Errorf("X-Request-ID is not valid hex: %v", err)
	}
	if !strings.Contains(buf.String(), `"path":"/anything"`) {
		t.Errorf("log output missing path: %s", buf.String())
	}
	if !strings.Contains(buf.String(), `"status":200`) {
		t.Errorf("log output missing status: %s", buf.String())
	}
}

func TestRequestLog_HealthNotLogged(t *testing.T) {
	var buf bytes.Buffer
	s := newTestServer("*", &buf)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	s.requestLog(next).ServeHTTP(rr, req)

	if buf.Len() != 0 {
		t.Errorf("expected no log output for /health, got %q", buf.String())
	}
	if rr.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID should still be set even for /health")
	}
}

func TestResponseWriter_CapturesStatusCode(t *testing.T) {
	rr := httptest.NewRecorder()
	rw := newResponseWriter(rr)

	if rw.statusCode != http.StatusOK {
		t.Errorf("initial statusCode = %d, want 200", rw.statusCode)
	}

	rw.WriteHeader(http.StatusTeapot)
	if rw.statusCode != http.StatusTeapot {
		t.Errorf("after WriteHeader(418), statusCode = %d", rw.statusCode)
	}
	if rr.Code != http.StatusTeapot {
		t.Errorf("underlying writer Code = %d, want 418", rr.Code)
	}
}

func TestGenerateRequestID_UniqueAndHex(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id := generateRequestID()
		if len(id) != 16 {
			t.Fatalf("len(id) = %d, want 16", len(id))
		}
		if _, err := hex.DecodeString(id); err != nil {
			t.Fatalf("id %q not hex: %v", id, err)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}
