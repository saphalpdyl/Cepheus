-- Tables for STAMP data
CREATE TABLE stamp_data
(
    timestamp       TIMESTAMPTZ      NOT NULL,
    serial_id       TEXT             NOT NULL REFERENCES device (serial_id) ON DELETE CASCADE,
    agent_config_id UUID             REFERENCES agent_config (id) ON DELETE SET NULL,
    target          TEXT             NOT NULL,
    port            INTEGER          NOT NULL,
    sent            INTEGER          NOT NULL,
    received        INTEGER          NOT NULL,
    loss            FLOAT NOT NULL,
    avg_rtt         BIGINT NOT NULL,
    min_rtt         BIGINT NOT NULL,
    max_rtt         BIGINT NOT NULL,
    p50_rtt         BIGINT NOT NULL,
    p95_rtt         BIGINT NOT NULL
);

SELECT create_hypertable('stamp_data', 'timestamp');

CREATE INDEX ON stamp_data (serial_id, target, timestamp DESC);
CREATE INDEX ON stamp_data (serial_id, timestamp DESC);
