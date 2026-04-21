package server

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/freewhen/backend/internal/config"
	"github.com/freewhen/backend/internal/email"
	"github.com/freewhen/backend/internal/handler"
	"github.com/freewhen/backend/internal/realtime"
	"github.com/freewhen/backend/internal/service"
	"github.com/freewhen/backend/internal/store"
)

type Server struct {
	handler    *handler.Handler
	events     *handler.EventsHandler
	auth       *handler.AuthHandler
	metrics    *handler.MetricsHandler
	hub        *realtime.Hub
	store      *store.Store
	mux        *http.ServeMux
	logger     *slog.Logger
	db         *sql.DB
	corsOrigin string
	cfg        *config.Config
}

func New(db *sql.DB, logger *slog.Logger, cfg *config.Config) *Server {
	st := store.New(db)
	hub := realtime.NewHub(logger)
	svc := service.New(st, hub, service.AccessConfig{
		Secret: cfg.CalendarAccessSecret,
		TTL:    30 * 24 * time.Hour,
	})
	h := handler.New(svc)
	eh := handler.NewEvents(hub, svc, logger)

	var emailSender handler.EmailSender
	if cfg.ResendAPIKey != "" {
		emailSender = email.NewSender(cfg.ResendAPIKey, cfg.MagicLinkFromEmail)
	}
	ah := handler.NewAuth(st, emailSender, cfg, logger)
	mh := handler.NewMetrics(st, hub)

	srv := &Server{
		handler:    h,
		events:     eh,
		auth:       ah,
		metrics:    mh,
		hub:        hub,
		store:      st,
		mux:        http.NewServeMux(),
		logger:     logger,
		db:         db,
		corsOrigin: cfg.CORSOrigin,
		cfg:        cfg,
	}
	srv.routes()
	return srv
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)

	// Auth routes
	s.mux.HandleFunc("GET /auth/google", s.auth.GoogleLogin)
	s.mux.HandleFunc("GET /auth/google/callback", s.auth.GoogleCallback)
	s.mux.HandleFunc("GET /auth/me", s.auth.Me)
	s.mux.HandleFunc("PATCH /auth/me", s.requireAuth(s.auth.UpdateMe))
	s.mux.HandleFunc("POST /auth/logout", s.auth.Logout)
	s.mux.HandleFunc("POST /auth/magic-link", s.auth.SendMagicLink)
	s.mux.HandleFunc("GET /auth/magic-link/callback", s.auth.MagicLinkCallback)

	// User routes
	s.mux.HandleFunc("GET /users/me/calendars", s.requireAuth(s.handler.ListUserCalendars))

	// Admin routes
	s.mux.HandleFunc("GET /admin/metrics", s.requireAdmin(s.metrics.GetMetrics))

	// Calendar routes — POST requires auth
	s.mux.HandleFunc("POST /calendars", s.requireAuth(s.handler.CreateCalendar))
	s.mux.HandleFunc("GET /calendars/{id}", s.optionalAuth(s.handler.GetCalendar))
	s.mux.HandleFunc("PATCH /calendars/{id}", s.requireAuth(s.handler.UpdateCalendar))
	s.mux.HandleFunc("POST /calendars/{id}/participants", s.optionalAuth(s.handler.AddParticipant))
	s.mux.HandleFunc("POST /calendars/{id}/participants/{participantId}/claim", s.requireAuth(s.handler.ClaimParticipant))
	s.mux.HandleFunc("DELETE /calendars/{id}/participants/{participantId}", s.requireAuth(s.handler.RemoveParticipant))
	s.mux.HandleFunc("PUT /calendars/{id}/availability", s.optionalAuth(s.handler.SetAvailability))
	s.mux.HandleFunc("POST /calendars/{id}/finalise", s.requireAuth(s.handler.FinaliseCalendar))
	s.mux.HandleFunc("DELETE /calendars/{id}/finalise", s.requireAuth(s.handler.UnfinaliseCalendar))
	s.mux.HandleFunc("POST /calendars/{id}/unlock", s.optionalAuth(s.handler.Unlock))
	s.mux.HandleFunc("POST /calendars/{id}/passphrase", s.requireAuth(s.handler.SetPassphrase))
	s.mux.HandleFunc("DELETE /calendars/{id}/passphrase", s.requireAuth(s.handler.ClearPassphrase))

	// Server-sent events stream for live updates.
	s.mux.HandleFunc("GET /calendars/{id}/events", s.optionalAuth(s.events.Stream))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.cors(s.requestLog(s.mux)).ServeHTTP(w, r)
}
