CREATE TABLE IF NOT EXISTS agent_config (
    serial_id  TEXT        PRIMARY KEY,
    config     JSONB       NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agent_data (
    id         BIGSERIAL   PRIMARY KEY,
    serial_id  TEXT        NOT NULL,
    data       JSONB       NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_data_serial_id ON agent_data(serial_id);
