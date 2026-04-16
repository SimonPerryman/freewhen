package server

import (
	"database/sql"
	"net/http"

	"github.com/freewhen/backend/internal/handler"
	"github.com/freewhen/backend/internal/store"
)

type Server struct {
	handler *handler.Handler
	mux     *http.ServeMux
}

func New(db *sql.DB) *Server {
	s := store.New(db)
	h := handler.New(s)

	srv := &Server{
		handler: h,
		mux:     http.NewServeMux(),
	}
	srv.routes()
	return srv
}

func (s *Server) routes() {
	// Routes will be registered here as handlers are implemented.
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
