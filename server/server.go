package server

import (
	"cepheus/server/handler"
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	handler    *handler.Handler
	httpServer *http.Server
}

func NewServer(listenAddr string, dbPool *pgxpool.Pool) *Server {
	h := &handler.Handler{
		Pool: dbPool,
	}

	mux := http.NewServeMux()
	v1 := http.NewServeMux()

	v1.HandleFunc("POST /devices/config/{serial_id}", h.GetAgentConfig)

	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", v1))

	return &Server{
		handler: h,
		httpServer: &http.Server{
			Addr:    listenAddr,
			Handler: mux,
		},
	}
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
