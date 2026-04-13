package handler

import (
	"cepheus/internal/common"
	"cepheus/internal/common/telemetry"
	"cepheus/internal/control-plane/logattr"
	"encoding/json"
	"fmt"
	"net/http"
)

func (h *Handler) GetAgentConfig(w http.ResponseWriter, r *http.Request) {
	var err error
	ctx, end, _ := telemetry.SpanWithErr(r.Context(), "Handler.GetAgentConfig", &err)
	defer end()

	serialID := r.PathValue("serial_id")
	if serialID == "" {
		writeError(w, http.StatusBadRequest, "missing serial_id")
		return
	}

	log().Info("fetching agent config", "serial_id", serialID)

	rows, err := h.Pool.Query(ctx,
		`SELECT c.version, c.generation,
		        c.report_endpoint, c.report_batch_size, c.report_interval_seconds,
		        EXTRACT(EPOCH FROM c.created_at)::bigint,
		        EXTRACT(EPOCH FROM c.updated_at)::bigint,
		        t.task_id, t.type, t.enabled, t.params
		 FROM device d
		 JOIN agent_config c ON c.id = d.agent_config_id
		 LEFT JOIN agent_task t ON t.agent_config_id = c.id
		 WHERE d.serial_id = $1`,
		serialID,
	)
	if err != nil {
		log().Error("query failed", "serial_id", serialID, logattr.Err(err))
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("query failed: %v", err))
		return
	}
	defer rows.Close()

	var cfg common.AgentConfig
	found := false
	for rows.Next() {
		var taskID *string
		var taskType *common.AgentTaskType
		var taskEnabled *bool
		var taskParams *json.RawMessage

		if err = rows.Scan(
			&cfg.Version, &cfg.Generation,
			&cfg.ReportEndpoint, &cfg.ReportBatchSize, &cfg.ReportIntervalSeconds,
			&cfg.CreatedAt, &cfg.UpdatedAt,
			&taskID, &taskType, &taskEnabled, &taskParams,
		); err != nil {
			log().Error("scan failed", "serial_id", serialID, logattr.Err(err))
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("scan failed: %v", err))
			return
		}
		found = true
		if taskID != nil {
			cfg.Tasks = append(cfg.Tasks, common.AgentTask{
				TaskID:  *taskID,
				Type:    *taskType,
				Enabled: *taskEnabled,
				Params:  *taskParams,
			})
		}
	}

	if !found {
		log().Warn("agent not found", "serial_id", serialID)
		writeError(w, http.StatusNotFound, fmt.Sprintf("agent %q not found", serialID))
		return
	}

	if cfg.Tasks == nil {
		cfg.Tasks = []common.AgentTask{}
	}

	writeJSON(w, http.StatusOK, cfg)
}
