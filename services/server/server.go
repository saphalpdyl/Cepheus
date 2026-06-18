package server

import (
	"cepheus/libs/api/gen/cepheus/agent/v1/agentv1connect"
	"cepheus/services/server/handler"
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

	// Connect AgentConfigService (control plane <-> agent). The generated
	// constructor returns the route prefix (/cepheus.agent.v1.AgentConfigService/)
	// and the handler; new RPC services register the same way.
	mux.Handle(agentv1connect.NewAgentConfigServiceHandler(h))

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
