CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE
    IF NOT EXISTS agent_config (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        generation INT NOT NULL DEFAULT 1,
        report_endpoint TEXT NOT NULL DEFAULT '',
        report_batch_size INT NOT NULL DEFAULT 1,
        report_interval_seconds INT NOT NULL DEFAULT 60,
        report_timeout_seconds INT NOT NULL DEFAULT 10,
        scamper_pps INT NOT NULL DEFAULT 100,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW (),
        updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW ()
    );

CREATE TABLE
    IF NOT EXISTS device (
        serial_id TEXT PRIMARY KEY,
        agent_config_id UUID NOT NULL REFERENCES agent_config (id) ON DELETE RESTRICT
    );

CREATE INDEX IF NOT EXISTS idx_device_config_id ON device (agent_config_id);

CREATE TABLE
    IF NOT EXISTS agent_task (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        agent_config_id UUID NOT NULL REFERENCES agent_config (id) ON DELETE CASCADE,
        task_id TEXT NOT NULL,
        type TEXT NOT NULL,
        enabled BOOLEAN NOT NULL DEFAULT true,
        generation INT NOT NULL DEFAULT 1,
        schedule_interval_seconds INT NOT NULL DEFAULT 0,
        schedule_jitter_percent INT NOT NULL DEFAULT 0,
        schedule_enabled BOOLEAN NOT NULL,
        params JSONB NOT NULL DEFAULT '{}',
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW ()
    );

CREATE INDEX IF NOT EXISTS idx_agent_task_config_id ON agent_task (agent_config_id);

CREATE TABLE
    IF NOT EXISTS pending_action (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        agent_config_id UUID NOT NULL REFERENCES agent_config (id) ON DELETE CASCADE,
        type TEXT NOT NULL,
        params JSONB NOT NULL DEFAULT '{}',
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW ()
    );

CREATE INDEX IF NOT EXISTS idx_pending_action_config_id ON pending_action (agent_config_id);