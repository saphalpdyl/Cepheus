package handler

import (
	"cepheus/internal/cepheus-server/logattr"
	"cepheus/internal/common/telemetry"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

func (h *Handler) PostAgentData(w http.ResponseWriter, r *http.Request) {
	var err error
	ctx, end, _ := telemetry.SpanWithErr(r.Context(), "Handler.PostAgentData", &err)
	defer end()

	serialID := r.PathValue("serial_id")
	if serialID == "" {
		writeError(w, http.StatusBadRequest, "missing serial_id")
		return
	}

	var payload json.RawMessage
	if err = json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log().Warn("invalid JSON body", "serial_id", serialID, logattr.Err(err))
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	_, err = h.Pool.Exec(ctx,
		"INSERT INTO agent_data (serial_id, data) VALUES ($1, $2)",
		serialID, payload,
	)
	if err != nil {
		log().Error("failed to store data", "serial_id", serialID, logattr.Err(err))
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to store data: %v", err))
		return
	}

	log().Info("data accepted", "serial_id", serialID)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

type AgentDataRow struct {
	ID        int64           `json:"id"`
	SerialID  string          `json:"serial_id"`
	Data      json.RawMessage `json:"data"`
	CreatedAt time.Time       `json:"created_at"`
}

func (h *Handler) GetAgentData(w http.ResponseWriter, r *http.Request) {
	var err error
	ctx, end, _ := telemetry.SpanWithErr(r.Context(), "Handler.GetAgentData", &err)
	defer end()

	serialID := r.PathValue("serial_id")
	if serialID == "" {
		writeError(w, http.StatusBadRequest, "missing serial_id")
		return
	}

	rows, err := h.Pool.Query(ctx,
		"SELECT id, serial_id, data, created_at FROM agent_data WHERE serial_id = $1 ORDER BY created_at DESC",
		serialID,
	)
	if err != nil {
		log().Error("query failed", "serial_id", serialID, logattr.Err(err))
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("query failed: %v", err))
		return
	}
	defer rows.Close()

	results, err := pgx.CollectRows(rows, pgx.RowToStructByPos[AgentDataRow])
	if err != nil {
		log().Error("scan failed", "serial_id", serialID, logattr.Err(err))
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("scan failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, results)
}
