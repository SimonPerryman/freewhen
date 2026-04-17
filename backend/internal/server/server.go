package server

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/freewhen/backend/internal/handler"
	"github.com/freewhen/backend/internal/service"
	"github.com/freewhen/backend/internal/store"
)

type Server struct {
	handler    *handler.Handler
	mux        *http.ServeMux
	logger     *slog.Logger
	db         *sql.DB
	corsOrigin string
}

func New(db *sql.DB, logger *slog.Logger, corsOrigin string) *Server {
	st := store.New(db)
	svc := service.New(st)
	h := handler.New(svc)

	srv := &Server{
		handler:    h,
		mux:        http.NewServeMux(),
		logger:     logger,
		db:         db,
		corsOrigin: corsOrigin,
	}
	srv.routes()
	return srv
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("POST /calendars", s.handler.CreateCalendar)
	s.mux.HandleFunc("GET /calendars/{id}", s.handler.GetCalendar)
	s.mux.HandleFunc("POST /calendars/{id}/participants", s.handler.AddParticipant)
	s.mux.HandleFunc("PUT /calendars/{id}/availability", s.handler.SetAvailability)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.cors(s.requestLog(s.mux)).ServeHTTP(w, r)
}
