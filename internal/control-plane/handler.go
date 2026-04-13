package controlplane

import (
	"cepheus/internal/common"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	pool *pgxpool.Pool
	conn []*pgx.Conn
}

func (h *Handler) GetAgentConfig(w http.ResponseWriter, r *http.Request) {
	serialID := r.PathValue("serial_id")
	if serialID == "" {
		writeError(w, http.StatusBadRequest, "missing serial_id")
		return
	}

	var config common.AgentConfig
	err := h.pool.QueryRow(r.Context(),
		"SELECT config FROM agent_config WHERE serial_id = $1",
		serialID,
	).Scan(&config)
	if err != nil {
		if err.Error() == "no rows in result set" {
			writeError(w, http.StatusNotFound, fmt.Sprintf("agent %q not found", serialID))
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("query failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, config)
}

func (h *Handler) PostAgentData(w http.ResponseWriter, r *http.Request) {
	serialID := r.PathValue("serial_id")
	if serialID == "" {
		writeError(w, http.StatusBadRequest, "missing serial_id")
		return
	}

	var payload json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	_, err := h.pool.Exec(r.Context(),
		"INSERT INTO agent_data (serial_id, data) VALUES ($1, $2)",
		serialID, payload,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to store data: %v", err))
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

type AgentDataRow struct {
	ID        int64           `json:"id"`
	SerialID  string          `json:"serial_id"`
	Data      json.RawMessage `json:"data"`
	CreatedAt time.Time       `json:"created_at"`
}

func (h *Handler) GetAgentData(w http.ResponseWriter, r *http.Request) {
	serialID := r.PathValue("serial_id")
	if serialID == "" {
		writeError(w, http.StatusBadRequest, "missing serial_id")
		return
	}

	rows, err := h.pool.Query(r.Context(),
		"SELECT id, serial_id, data, created_at FROM agent_data WHERE serial_id = $1 ORDER BY created_at DESC",
		serialID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("query failed: %v", err))
		return
	}
	defer rows.Close()

	results, err := pgx.CollectRows(rows, pgx.RowToStructByPos[AgentDataRow])
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("scan failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, results)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
