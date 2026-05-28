-- Tables for ping data

CREATE TABLE IF NOT EXISTS ping_measurements
(
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp       TIMESTAMPTZ NOT NULL,
    serial_id       TEXT        NOT NULL,
    agent_config_id UUID        REFERENCES agent_config (id) ON DELETE SET NULL,
    target          TEXT        NOT NULL,
    sent            INTEGER     NOT NULL,
    received        INTEGER     NOT NULL,
    loss            FLOAT       NOT NULL,
    rtt_min_ns      BIGINT      NOT NULL,
    rtt_avg_ns      BIGINT      NOT NULL,
    rtt_max_ns      BIGINT      NOT NULL,
    rtt_p50_ns      BIGINT      NOT NULL,
    rtt_p95_ns      BIGINT      NOT NULL,
    rtt_stddev_ns   BIGINT      NOT NULL
);

CREATE TABLE IF NOT EXISTS ping_probes
(
    measurement_id UUID        REFERENCES ping_measurements (id) ON DELETE CASCADE,
    tx             TIMESTAMPTZ NOT NULL,
    rx             TIMESTAMPTZ,
    is_lost        BOOLEAN     NOT NULL,
    seq            INTEGER,
    rtt            BIGINT
);

SELECT create_hypertable('ping_probes', 'tx');

CREATE INDEX ON ping_probes (measurement_id) WHERE is_lost = true;
CREATE INDEX ON ping_measurements (serial_id, timestamp DESC);
CREATE INDEX ON ping_measurements (agent_config_id);
CREATE INDEX ON ping_measurements (target, timestamp DESC);
