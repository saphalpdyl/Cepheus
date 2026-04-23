package handler

import (
	"cepheus/api"
	logattr "cepheus/cepheus-server/log"
	"cepheus/telemetry"
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
		`SELECT c.id, c.generation,
		        c.report_endpoint, c.report_batch_size, c.report_interval_seconds, c.report_timeout_seconds,
		        EXTRACT(EPOCH FROM c.updated_at)::bigint,
		        t.task_id, t.type, t.enabled, t.generation, t.params,
		        t.schedule_interval_seconds, t.schedule_jitter_percent, schedule_enabled
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

	var cfg api.AgentConfig
	found := false
	for rows.Next() {
		var taskID *string
		var taskType *api.AgentTaskType
		var taskEnabled *bool
		var taskGeneration *int
		var taskParams *json.RawMessage
		var scheduleInterval *int
		var scheduleJitter *int
		var scheduleEnabled *bool

		if err = rows.Scan(
			&cfg.ID, &cfg.Generation,
			&cfg.ReportEndpoint, &cfg.ReportBatchSize, &cfg.ReportIntervalSeconds, &cfg.ReportTimeoutSeconds,
			&cfg.UpdatedAt,
			&taskID, &taskType, &taskEnabled, &taskGeneration, &taskParams,
			&scheduleInterval, &scheduleJitter, &scheduleEnabled,
		); err != nil {
			log().Error("scan failed", "serial_id", serialID, logattr.Err(err))
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("scan failed: %v", err))
			return
		}
		found = true
		if taskID != nil {
			var params json.RawMessage
			if taskParams != nil {
				params = *taskParams
			}
			task := api.Task{
				TaskID:     *taskID,
				Type:       *taskType,
				Enabled:    *taskEnabled,
				Generation: *taskGeneration,
				Params:     params,
			}
			if scheduleInterval != nil {
				task.Schedule.IntervalSeconds = *scheduleInterval
			}
			if scheduleJitter != nil {
				task.Schedule.JitterPercent = *scheduleJitter
			}
			if scheduleEnabled != nil {
				task.Schedule.Enabled = *scheduleEnabled
			}
			cfg.Tasks = append(cfg.Tasks, task)
		}
	}

	if !found {
		log().Warn("agent not found", "serial_id", serialID)
		writeError(w, http.StatusNotFound, fmt.Sprintf("agent %q not found", serialID))
		return
	}

	if cfg.Tasks == nil {
		cfg.Tasks = []api.Task{}
	}
	if cfg.PendingActions == nil {
		cfg.PendingActions = []api.PendingAction{}
	}

	writeJSON(w, http.StatusOK, cfg)
}
