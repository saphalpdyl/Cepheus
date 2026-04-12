package controlplane

import (
	"context"
	"net/http"
)

type Server struct {
	handler    *Handler
	httpServer *http.Server
}

func NewServer(listenAddr string, agents map[string]AgentConfig) *Server {
	h := &Handler{agents: agents}

	mux := http.NewServeMux()
	v1 := http.NewServeMux()

	v1.HandleFunc("POST /devices/config/{serial_id}", h.GetAgentConfig)
	v1.HandleFunc("POST /devices/data/{serial_id}", h.PostAgentData)
	v1.HandleFunc("GET /devices/data/{serial_id}", h.GetAgentData)

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
