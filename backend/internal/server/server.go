package server

import (
	"database/sql"
	"net/http"

	"github.com/freewhen/backend/internal/handler"
	"github.com/freewhen/backend/internal/service"
	"github.com/freewhen/backend/internal/store"
)

type Server struct {
	handler *handler.Handler
	mux     *http.ServeMux
}

func New(db *sql.DB) *Server {
	st := store.New(db)
	svc := service.New(st)
	h := handler.New(svc)

	srv := &Server{
		handler: h,
		mux:     http.NewServeMux(),
	}
	srv.routes()
	return srv
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /calendars", s.handler.CreateCalendar)
	s.mux.HandleFunc("GET /calendars/{id}", s.handler.GetCalendar)
	s.mux.HandleFunc("POST /calendars/{id}/participants", s.handler.AddParticipant)
	s.mux.HandleFunc("PUT /calendars/{id}/availability", s.handler.SetAvailability)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.cors(s.mux).ServeHTTP(w, r)
}
