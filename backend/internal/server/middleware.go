package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/freewhen/backend/internal/handler"
	"github.com/freewhen/backend/internal/model"
)

type contextKey string

const requestIDKey contextKey = "request_id"

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

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.corsOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Calendar-Access")
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
	// Refresh session expiry in the background
	go func() {
		_ = s.store.RefreshSession(context.Background(), sess.Token)
	}()
	return &user
}

func (s *Server) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := generateRequestID()
		ctx := context.WithValue(r.Context(), requestIDKey, id)
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

func generateRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
