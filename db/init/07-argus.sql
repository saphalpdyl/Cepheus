CREATE TABLE IF NOT EXISTS argus_baselines
(
    serial_id  TEXT        NOT NULL,
    target     TEXT        NOT NULL,
    port       INTEGER     NOT NULL,
    metric     TEXT        NOT NULL,
    detector   TEXT        NOT NULL,
    state      JSONB       NOT NULL,
    n          BIGINT      NOT NULL DEFAULT 0,
    last_seen  TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (serial_id, target, port, metric, detector)
);

CREATE TABLE IF NOT EXISTS argus_events
(
    id            UUID PRIMARY KEY          DEFAULT gen_random_uuid(),
    serial_id     TEXT             NOT NULL,
    target        TEXT             NOT NULL,
    port          INTEGER          NOT NULL,
    metric        TEXT             NOT NULL,
    status        TEXT             NOT NULL,
    opened_at     TIMESTAMPTZ      NOT NULL,
    last_seen_at  TIMESTAMPTZ      NOT NULL,
    closed_at     TIMESTAMPTZ,
    finding_count INTEGER          NOT NULL DEFAULT 0,
    peak_severity DOUBLE PRECISION NOT NULL,
    detectors     TEXT[]           NOT NULL DEFAULT '{}'
);

CREATE INDEX ON argus_events (status, last_seen_at) WHERE status = 'open';
CREATE INDEX ON argus_events (serial_id, target, opened_at DESC);

CREATE TABLE IF NOT EXISTS argus_findings
(
    id         UUID PRIMARY KEY          DEFAULT gen_random_uuid(),
    serial_id  TEXT             NOT NULL,
    target     TEXT             NOT NULL,
    port       INTEGER          NOT NULL,
    metric     TEXT             NOT NULL,
    detector   TEXT             NOT NULL,
    ts         TIMESTAMPTZ      NOT NULL,
    value      DOUBLE PRECISION NOT NULL,
    severity   DOUBLE PRECISION NOT NULL,
    details    JSONB            NOT NULL DEFAULT '{}',
    event_id   UUID             REFERENCES argus_events (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ      NOT NULL DEFAULT now()
);

CREATE INDEX ON argus_findings (serial_id, target, ts DESC);
CREATE INDEX ON argus_findings (event_id) WHERE event_id IS NOT NULL;
CREATE UNIQUE INDEX ON argus_findings (serial_id, target, port, metric, detector, ts);

CREATE TABLE IF NOT EXISTS argus_policy_state
(
    serial_id         TEXT             NOT NULL,
    target            TEXT             NOT NULL,
    port              INTEGER          NOT NULL,
    metric            TEXT             NOT NULL,
    detector          TEXT             NOT NULL, 
    status            TEXT             NOT NULL,
    score             DOUBLE PRECISION NOT NULL,
    score_updated_at  TIMESTAMPTZ      NOT NULL,
    open_event_id     UUID             REFERENCES argus_events (id) ON DELETE SET NULL,
    pending_findings  UUID[]           NOT NULL DEFAULT '{}',
    entered_status_at TIMESTAMPTZ      NOT NULL,
    updated_at        TIMESTAMPTZ      NOT NULL DEFAULT now(),
    PRIMARY KEY (serial_id, target, port, metric, detector)
);