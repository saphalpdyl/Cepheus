-- Tables for trace data
CREATE TABLE trace_measurements
(
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    serial_id       TEXT        NOT NULL REFERENCES device (serial_id) ON DELETE CASCADE,
    agent_config_id UUID        REFERENCES agent_config (id) ON DELETE SET NULL,
    timestamp       TIMESTAMPTZ NOT NULL,
    type            TEXT        NOT NULL,
    src             INET        NOT NULL,
    dst             INET        NOT NULL,
    method          TEXT        NOT NULL,
    stop_reason     TEXT        NOT NULL,
    hop_count       INT         NOT NULL,
    path_hash       TEXT        NOT NULL,
    raw             JSONB       NOT NULL
);

CREATE INDEX idx_trace_dst ON trace_measurements (dst);
CREATE INDEX idx_trace_method ON trace_measurements (method);
CREATE INDEX idx_trace_measurements_timestamp ON trace_measurements (timestamp DESC);
CREATE INDEX idx_trace_measurements_serial_id ON trace_measurements (serial_id);

CREATE TABLE trace_hops
(
    timestamp      TIMESTAMPTZ NOT NULL,
    measurement_id UUID        NOT NULL REFERENCES trace_measurements (id) ON DELETE CASCADE,
    ip             INET,
    ttl            INT         NOT NULL,
    rtt            BIGINT,
    icmp_type      INT,
    icmp_code      INT,
    reply_ttl      INT,
    asn            INT,
    is_no_hop      BOOLEAN     NOT NULL DEFAULT FALSE
);

SELECT create_hypertable('trace_hops', 'timestamp');
CREATE INDEX ON trace_hops (ip, timestamp DESC);