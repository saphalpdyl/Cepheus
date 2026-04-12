package controlplane

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type ProbeConfig struct {
	Type   string         `json:"type" yaml:"type"`
	Mode   string         `json:"mode" yaml:"mode"`
	Params map[string]any `json:"params" yaml:"params"`
}

type AgentConfig struct {
	Probes []ProbeConfig `json:"probes" yaml:"probes"`
}

type Handler struct {
	agents map[string]AgentConfig // Cached
}

func (h *Handler) GetAgentConfig(w http.ResponseWriter, r *http.Request) {
	serialID := r.PathValue("serial_id")
	if serialID == "" {
		writeError(w, http.StatusBadRequest, "missing serial_id")
		return
	}

	cfg, ok := h.agents[serialID]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("agent %q not found", serialID))
		return
	}

	writeJSON(w, http.StatusOK, cfg)
}

func (h *Handler) PostAgentData(w http.ResponseWriter, r *http.Request) {
	serialID := r.PathValue("serial_id")
	if serialID == "" {
		writeError(w, http.StatusBadRequest, "missing serial_id")
		return
	}

}

func (h *Handler) GetAgentData(w http.ResponseWriter, r *http.Request) {
	serialID := r.PathValue("serial_id")
	if serialID == "" {
		writeError(w, http.StatusBadRequest, "missing serial_id")
		return
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
