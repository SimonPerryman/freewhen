package server

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/freewhen/backend/internal/config"
	"github.com/freewhen/backend/internal/email"
	"github.com/freewhen/backend/internal/flags"
	"github.com/freewhen/backend/internal/handler"
	"github.com/freewhen/backend/internal/realtime"
	"github.com/freewhen/backend/internal/service"
	"github.com/freewhen/backend/internal/store"
)

type Server struct {
	handler        *handler.Handler
	events         *handler.EventsHandler
	auth           *handler.AuthHandler
	metrics        *handler.MetricsHandler
	calIntegration *handler.CalendarIntegrationHandler
	testLogin      *handler.TestLoginHandler
	testEmail      *handler.CapturingEmailSender
	hub            *realtime.Hub
	store          *store.Store
	mux            *http.ServeMux
	logger         *slog.Logger
	db             *sql.DB
	corsOrigin     string
	cfg            *config.Config
	flags          flags.Evaluator
	// routes records each registered route's auth kind so the route-coverage
	// test can detect handlers added without a requireAuth/requireAdmin wrap.
	routeSpecs []routeSpec
}

// authKind classifies a route's auth wrapping for the route-coverage test.
type authKind int

const (
	authPublic authKind = iota
	authOptional
	authRequired
	authAdmin
)

type routeSpec struct {
	pattern string
	kind    authKind
}

func New(db *sql.DB, logger *slog.Logger, cfg *config.Config) *Server {
	st := store.New(db)
	hub := realtime.NewHub(logger)
	svc := service.New(st, hub, service.AccessConfig{
		Secret: cfg.CalendarAccessSecret,
		TTL:    30 * 24 * time.Hour,
	})

	var flagEval flags.Evaluator = flags.Noop{}
	if cfg.PostHogKey != "" {
		phEval, err := flags.NewPostHog(flags.PostHogConfig{
			APIKey:         cfg.PostHogKey,
			Host:           cfg.PostHogHost,
			PersonalAPIKey: cfg.PostHogPersonalKey,
		}, logger)
		if err != nil {
			logger.Warn("feature flag evaluator unavailable, falling back to noop", "error", err)
		} else {
			flagEval = phEval
		}
	}

	h := handler.New(svc).WithFlags(flagEval)
	eh := handler.NewEvents(hub, svc, logger)

	var emailSender handler.EmailSender
	var capturedEmail *handler.CapturingEmailSender
	if cfg.E2ETestMode {
		// In E2E mode we always capture magic links to memory and expose
		// them via /test/magic-link, regardless of AuthEmailDisabled. No
		// outbound SMTP is performed.
		capturedEmail = handler.NewCapturingEmailSender()
		emailSender = capturedEmail
	} else if !cfg.AuthEmailDisabled {
		emailSender = email.NewSender(cfg.ResendAPIKey, cfg.MagicLinkFromEmail)
	}
	ah := handler.NewAuth(st, emailSender, cfg, logger)
	mh := handler.NewMetrics(st, hub)
	ch := handler.NewCalendarIntegration(st, svc, cfg, logger)

	var tlh *handler.TestLoginHandler
	if cfg.E2ETestMode {
		logger.Warn("E2E_TEST_MODE enabled: /test/login will mint sessions without auth — must never run in prod")
		tlh = handler.NewTestLogin(st, cfg.CookieDomain, cfg.SecureCookies, logger)
	}

	srv := &Server{
		handler:        h,
		events:         eh,
		auth:           ah,
		metrics:        mh,
		calIntegration: ch,
		testLogin:      tlh,
		testEmail:      capturedEmail,
		hub:            hub,
		store:          st,
		mux:            http.NewServeMux(),
		logger:         logger,
		db:             db,
		corsOrigin:     cfg.CORSOrigin,
		cfg:            cfg,
		flags:          flagEval,
	}
	srv.routes()
	return srv
}

// handlePublic registers a route that does not require authentication.
// Use sparingly — every public route is an attack surface.
func (s *Server) handlePublic(pattern string, fn http.HandlerFunc) {
	s.mux.HandleFunc(pattern, fn)
	s.routeSpecs = append(s.routeSpecs, routeSpec{pattern, authPublic})
}

func (s *Server) handleAuth(pattern string, fn http.HandlerFunc) {
	s.mux.HandleFunc(pattern, s.requireAuth(fn))
	s.routeSpecs = append(s.routeSpecs, routeSpec{pattern, authRequired})
}

func (s *Server) handleAdmin(pattern string, fn http.HandlerFunc) {
	s.mux.HandleFunc(pattern, s.requireAdmin(fn))
	s.routeSpecs = append(s.routeSpecs, routeSpec{pattern, authAdmin})
}

// handleOptional registers a route where the session is loaded if present
// but the handler must enforce its own access rules (passphrase, participant
// token, etc.) when no session is attached.
func (s *Server) handleOptional(pattern string, fn http.HandlerFunc) {
	s.mux.HandleFunc(pattern, s.optionalAuth(fn))
	s.routeSpecs = append(s.routeSpecs, routeSpec{pattern, authOptional})
}

func (s *Server) routes() {
	s.handlePublic("GET /health", s.handleHealth)

	// Auth routes
	s.handlePublic("GET /auth/google", s.auth.GoogleLogin)
	s.handlePublic("GET /auth/google/callback", s.auth.GoogleCallback)
	s.handlePublic("GET /auth/me", s.auth.Me)
	s.handleAuth("PATCH /auth/me", s.auth.UpdateMe)
	s.handlePublic("POST /auth/logout", s.auth.Logout)
	s.handlePublic("POST /auth/magic-link", s.auth.SendMagicLink)
	s.handlePublic("POST /auth/magic-link/verify", s.auth.MagicLinkVerify)

	// User routes
	s.handleAuth("GET /users/me/calendars", s.handler.ListUserCalendars)

	// Admin routes
	s.handleAdmin("GET /admin/metrics", s.metrics.GetMetrics)

	// Calendar routes — POST requires auth
	s.handleAuth("POST /calendars", s.handler.CreateCalendar)
	s.handleAuth("POST /calendars/{id}/duplicate", s.handler.DuplicateCalendar)
	s.handleOptional("GET /calendars/{id}", s.handler.GetCalendar)
	s.handleAuth("PATCH /calendars/{id}", s.handler.UpdateCalendar)
	s.handleOptional("POST /calendars/{id}/participants", s.handler.AddParticipant)
	s.handleAuth("POST /calendars/{id}/participants/{participantId}/claim", s.handler.ClaimParticipant)
	s.handleAuth("DELETE /calendars/{id}/participants/{participantId}", s.handler.RemoveParticipant)
	s.handleOptional("PATCH /calendars/{id}/participants/{participantId}", s.handler.RenameParticipant)
	s.handleOptional("PUT /calendars/{id}/availability", s.handler.SetAvailability)
	s.handleAuth("POST /calendars/{id}/finalise", s.handler.FinaliseCalendar)
	s.handleAuth("DELETE /calendars/{id}/finalise", s.handler.UnfinaliseCalendar)
	s.handleOptional("POST /calendars/{id}/unlock", s.handler.Unlock)
	s.handleAuth("POST /calendars/{id}/passphrase", s.handler.SetPassphrase)
	s.handleAuth("DELETE /calendars/{id}/passphrase", s.handler.ClearPassphrase)

	// "Who's missing?" invite list — owner-only.
	s.handleAuth("GET /calendars/{id}/missing", s.handler.ListMissing)
	s.handleAuth("POST /calendars/{id}/invites", s.handler.AddInvites)
	s.handleAuth("PUT /calendars/{id}/invites/{inviteId}", s.handler.RenameInvite)
	s.handleAuth("DELETE /calendars/{id}/invites/{inviteId}", s.handler.DeleteInvite)
	s.handleAuth("POST /calendars/{id}/invites/{inviteId}/match", s.handler.ManualMatchInvite)

	// Calendar integrations — export (Phase 1) + Google Calendar import (Phase 2).
	// Export endpoints are intentionally public — finalised calendars should be
	// addable to anyone's calendar, including anonymous joiners.
	s.handlePublic("GET /calendars/{id}/export.ics", s.handler.ExportICS)
	s.handlePublic("GET /calendars/{id}/export/google", s.handler.ExportGoogle)

	// Incremental OAuth round-trip for Google Calendar import. All three
	// endpoints require a signed-in session.
	s.handleAuth("POST /auth/google/calendar/consent-ack", s.calIntegration.ConsentAck)
	s.handleAuth("GET /auth/google/calendar/connect", s.calIntegration.Connect)
	s.handleAuth("GET /auth/google/calendar/callback", s.calIntegration.Callback)
	s.handleAuth("GET /calendars/{id}/import/suggestions", s.calIntegration.GetSuggestions)
	s.handleAuth("POST /calendars/{id}/availability/import", s.handler.ImportAvailability)

	// Server-sent events stream for live updates.
	s.handleOptional("GET /calendars/{id}/events", s.events.Stream)

	if s.testLogin != nil {
		s.handlePublic("POST /test/login", s.testLogin.Login)
	}
	if s.testEmail != nil {
		s.handlePublic("GET /test/magic-link", s.testEmail.LatestMagicLink)
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.cors(s.requestLog(s.mux)).ServeHTTP(w, r)
}
