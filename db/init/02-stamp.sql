-- Tables for STAMP data

CREATE TABLE IF NOT EXISTS stamp_measurements
(
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp       TIMESTAMPTZ NOT NULL,
    serial_id       TEXT       NOT NULL,
    agent_config_id UUID       REFERENCES agent_config (id) ON DELETE SET NULL,
    target          TEXT       NOT NULL,
    port            INTEGER    NOT NULL,
    sent            INTEGER    NOT NULL,
    received        INTEGER    NOT NULL,
    loss            FLOAT      NOT NULL

);

CREATE TABLE IF NOT EXISTS stamp_probes
(
    measurement_id UUID REFERENCES stamp_measurements(id) ON DELETE CASCADE,
    tx             TIMESTAMPTZ NOT NULL,
    is_lost        BOOLEAN    NOT NULL,
    rx             TIMESTAMPTZ,
    rtt            BIGINT,
    forward_delay  BIGINT,
    backward_delay BIGINT
);


SELECT create_hypertable('stamp_probes', 'tx');

CREATE INDEX ON stamp_probes (measurement_id) WHERE is_lost = true;
CREATE INDEX ON stamp_measurements (serial_id, timestamp DESC);
CREATE INDEX ON stamp_measurements (agent_config_id);
CREATE INDEX ON stamp_measurements (target, timestamp DESC);
CREATE INDEX ON stamp_measurements (target, port, timestamp DESC);

-- CREATE INDEX ON stamp_data (serial_id, target, timestamp DESC);
-- CREATE INDEX ON stamp_data (serial_id, timestamp DESC);
