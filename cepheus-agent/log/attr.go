// defines typed structured log attributes for the cepheus services.
//
// All loggable fields must be defined here to ensure consistent key names across the codebase.
//
// Usage:
//
//	slog.LogAttrs(ctx, slog.LevelInfo, "configuration applied",
//	    logattr.DurationMs(elapsed.Milliseconds()),
//	)
package log

import (
	"cepheus/api"
	"log/slog"
)

type LogDomain string

// This will be used as a top-level classifier when categorizing log domains
const (
	DomainAgentLifecycle  LogDomain = "AGENT_LIFECYCLE"
	DomainAgentSupervisor LogDomain = "AGENT_SUPERVISOR"
	DomainProbeExecutor   LogDomain = "PROBE_EXECUTOR"
	DomainScamper         LogDomain = "SCAMPER"
)

// ── Domain/Context ────────────────────────────────────────────────────────────
func Domain(v LogDomain) slog.Attr { return slog.String("domain", string(v)) }

// ── Identity ────────────────────────────────────────────────────────────

func InstanceID(v string) slog.Attr          { return slog.String("service.instance.id", v) }
func SerialID(v string) slog.Attr            { return slog.String("serial_id", v) }
func Executor(v api.AgentTaskType) slog.Attr { return slog.String("executor", string(v)) }

// ── Operational ─────────────────────────────────────────────────────────

func Operation(v string) slog.Attr  { return slog.String("operation", v) }
func DurationMs(ms int64) slog.Attr { return slog.Int64("duration_ms", ms) }
func Attempt(n int) slog.Attr       { return slog.Int("attempt", n) }

// ── Errors ──────────────────────────────────────────────────────────────

// Err logs an error. Safe to call with nil.
func Err(err error) slog.Attr {
	if err == nil {
		return slog.String("error", "<nil>")
	}
	return slog.String("error", err.Error())
}
