package handler

import (
	logattr "cepheus/services/server/log"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

func log() *slog.Logger {
	return slog.Default().With(logattr.Domain(logattr.DomainServerHandler))
}

// Handler implements the cepheus.agent.v1 Connect services, backed by Postgres.
type Handler struct {
	Pool *pgxpool.Pool
}
