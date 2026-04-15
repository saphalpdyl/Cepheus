CREATE TABLE IF NOT EXISTS agent_config (
    id                      TEXT        PRIMARY KEY,
    generation              INT         NOT NULL DEFAULT 1,
    report_endpoint         TEXT        NOT NULL DEFAULT '',
    report_batch_size       INT         NOT NULL DEFAULT 1,
    report_interval_seconds INT         NOT NULL DEFAULT 60,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS device (
    serial_id       TEXT    PRIMARY KEY,
    agent_config_id TEXT    NOT NULL REFERENCES agent_config(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_device_config_id ON device(agent_config_id);

CREATE TABLE IF NOT EXISTS agent_task (
    id              BIGSERIAL   PRIMARY KEY,
    agent_config_id TEXT        NOT NULL REFERENCES agent_config(id) ON DELETE CASCADE,
    task_id         TEXT        NOT NULL,
    type            TEXT        NOT NULL,
    enabled         BOOLEAN     NOT NULL DEFAULT true,
    generation      INT         NOT NULL DEFAULT 1,
    params          JSONB       NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_task_config_id ON agent_task(agent_config_id);

CREATE TABLE IF NOT EXISTS agent_data (
    id         BIGSERIAL   PRIMARY KEY,
    serial_id  TEXT        NOT NULL,
    data       JSONB       NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_data_serial_id ON agent_data(serial_id);
