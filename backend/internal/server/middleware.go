package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/freewhen/backend/internal/handler"
	"github.com/freewhen/backend/internal/model"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Unwrap exposes the underlying ResponseWriter so http.ResponseController can
// reach Flusher/Hijacker on the real writer (needed by the SSE handler).
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.corsOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Calendar-Access, X-Participant-Token")
		// Expose the minted participant token (anonymous join / legacy
		// trust-on-first-use) and the request id to cross-origin JS.
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, X-Participant-Token")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// requireAuth returns 401 if no valid session cookie is present.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := s.authenticateRequest(r)
		if user == nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), handler.UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// requireAdmin returns 403 if the authenticated user is not an admin.
func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		user := handler.UserFromContext(r)
		if user == nil || !user.IsAdmin {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// optionalAuth attaches the user to context if a valid session exists, but
// does not reject the request if no session is present.
func (s *Server) optionalAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := s.authenticateRequest(r)
		if user != nil {
			ctx := context.WithValue(r.Context(), handler.UserContextKey, user)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	}
}

func (s *Server) authenticateRequest(r *http.Request) *model.User {
	cookie, err := r.Cookie("session")
	if err != nil {
		return nil
	}
	sess, err := s.store.GetSession(r.Context(), cookie.Value)
	if err != nil || sess == nil {
		return nil
	}
	user, err := s.store.GetUser(r.Context(), sess.UserID)
	if err != nil {
		return nil
	}
	// Refresh session expiry in the background. Use the raw token from the
	// cookie — the Session struct intentionally no longer carries it now
	// that storage is hash-only.
	rawToken := cookie.Value
	go func() {
		_ = s.store.RefreshSession(context.Background(), rawToken)
	}()
	return &user
}

func (s *Server) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := generateRequestID()
		ctx := context.WithValue(r.Context(), handler.RequestIDContextKey, id)
		r = r.WithContext(ctx)

		w.Header().Set("X-Request-ID", id)

		wrapped := newResponseWriter(w)
		start := time.Now()

		next.ServeHTTP(wrapped, r)

		if r.URL.Path == "/health" {
			return
		}

		s.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", id,
		)
	})
}

// secureHeaders sets baseline transport-hardening response headers on every
// response.
//
// Audit F16:
//   - X-Content-Type-Options: nosniff — disables MIME sniffing on all
//     responses; the API only ever returns JSON / iCal / SSE, so sniffing
//     adds attack surface with no upside.
//   - Referrer-Policy: no-referrer — keeps any URL the API receives or
//     redirects to from leaking via Referer to third-party assets. Most
//     critical on /auth/* where the magic-link interstitial holds the token
//     in `?token=...`; if any third-party asset (analytics, fonts) is ever
//     added to that page the token would otherwise leak.
//   - Strict-Transport-Security — only when SECURE_COOKIES=true (the same
//     gate we use to assume "this is a public HTTPS deployment, not a local
//     dev server"). Avoids accidentally pinning HSTS for `localhost`.
//   - Cache-Control: no-store, private on /auth/* — token-bearing responses
//     must never end up in a shared cache or browser back/forward cache.
func (s *Server) secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		if s.cfg != nil && s.cfg.SecureCookies {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		if strings.HasPrefix(r.URL.Path, "/auth/") {
			h.Set("Cache-Control", "no-store, private")
		}
		next.ServeHTTP(w, r)
	})
}

// requireJSONContentType returns 415 for any mutating request (anything other
// than GET / HEAD / OPTIONS) whose Content-Type isn't application/json.
//
// Audit F13 — defense-in-depth on top of SameSite=Lax. A `<form>` element can
// only ever set Content-Type to application/x-www-form-urlencoded,
// multipart/form-data, or text/plain, none of which pass this check. Any
// cross-origin POST/PATCH/DELETE/PUT carrying JSON is therefore forced to
// trigger a CORS preflight, which our `cors` middleware only honours from
// the configured origin. It also makes the JSON-only contract on the API
// explicit at the edge rather than implicit per handler.
func requireJSONContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		ct := r.Header.Get("Content-Type")
		mediaType, _, err := mime.ParseMediaType(ct)
		if err != nil || !strings.EqualFold(mediaType, "application/json") {
			http.Error(w, `{"error":"unsupported_media_type","message":"Content-Type must be application/json"}`, http.StatusUnsupportedMediaType)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func generateRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
