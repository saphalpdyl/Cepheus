-- Tables for trace data
CREATE TABLE trace_measurements (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp       TIMESTAMPTZ NOT NULL,
    type            TEXT NOT NULL,
    src             INET NOT NULL,
    dst             INET NOT NULL,
    method          TEXT NOT NULL,
    stop_reason     TEXT NOT NULL,
    hop_count       INT NOT NULL,
    path_hash       TEXT NOT NULL,
    raw             JSONB NOT NULL
);

CREATE TABLE trace_hops (
    timestamp       TIMESTAMPTZ NOT NULL,
    measurement_id  UUID NOT NULL,
    ip              INET,
    ttl             INT NOT NULL,
    rtt_ms          FLOAT,
    icmp_type       INT,
    icmp_code       INT,
    reply_ttl       INT,
    asn             INT,
    is_no_hop       BOOLEAN NOT NULL DEFAULT FALSE
);

SELECT create_hypertable('trace_hops', 'timestamp');
CREATE INDEX ON trace_hops (ip, timestamp DESC);