package server

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/freewhen/backend/internal/config"
	"github.com/freewhen/backend/internal/email"
	"github.com/freewhen/backend/internal/handler"
	"github.com/freewhen/backend/internal/service"
	"github.com/freewhen/backend/internal/store"
)

type Server struct {
	handler    *handler.Handler
	auth       *handler.AuthHandler
	store      *store.Store
	mux        *http.ServeMux
	logger     *slog.Logger
	db         *sql.DB
	corsOrigin string
	cfg        *config.Config
}

func New(db *sql.DB, logger *slog.Logger, cfg *config.Config) *Server {
	st := store.New(db)
	svc := service.New(st)
	h := handler.New(svc)

	var emailSender handler.EmailSender
	if cfg.ResendAPIKey != "" {
		emailSender = email.NewSender(cfg.ResendAPIKey, cfg.MagicLinkFromEmail)
	}
	ah := handler.NewAuth(st, emailSender, cfg, logger)

	srv := &Server{
		handler:    h,
		auth:       ah,
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
	s.mux.HandleFunc("POST /auth/logout", s.auth.Logout)
	s.mux.HandleFunc("POST /auth/magic-link", s.auth.SendMagicLink)
	s.mux.HandleFunc("GET /auth/magic-link/callback", s.auth.MagicLinkCallback)

	// Calendar routes — POST requires auth
	s.mux.HandleFunc("POST /calendars", s.requireAuth(s.handler.CreateCalendar))
	s.mux.HandleFunc("GET /calendars/{id}", s.handler.GetCalendar)
	s.mux.HandleFunc("POST /calendars/{id}/participants", s.handler.AddParticipant)
	s.mux.HandleFunc("PUT /calendars/{id}/availability", s.handler.SetAvailability)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.cors(s.requestLog(s.mux)).ServeHTTP(w, r)
}
