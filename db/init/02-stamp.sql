-- Tables for STAMP data
CREATE TABLE stamp_data (
    timestamp        TIMESTAMPTZ     NOT NULL,
    serial_id   TEXT            NOT NULL,
    target      TEXT            NOT NULL,
    port        INTEGER         NOT NULL,
    sent        INTEGER         NOT NULL,
    received    INTEGER         NOT NULL,
    loss        DOUBLE PRECISION NOT NULL,
    avg_rtt     DOUBLE PRECISION NOT NULL,
    min_rtt     DOUBLE PRECISION NOT NULL,
    max_rtt     DOUBLE PRECISION NOT NULL,
    p50_rtt     DOUBLE PRECISION NOT NULL,
    p95_rtt     DOUBLE PRECISION NOT NULL
);

SELECT create_hypertable('stamp_data', by_range('timestamp'));

CREATE INDEX ON stamp_data (serial_id, target, timestamp DESC);
CREATE INDEX ON stamp_data (serial_id, timestamp DESC);
