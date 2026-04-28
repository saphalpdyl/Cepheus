package handler

import (
	logattr "cepheus/server/log"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

func log() *slog.Logger {
	// return slog.Default().With(Domain(DomainServerHandler))
	return slog.Default().With(logattr.Domain(logattr.DomainServerHandler))
}

type Handler struct {
	Pool *pgxpool.Pool
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
