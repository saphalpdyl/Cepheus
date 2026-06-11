-- name: GetBaseline :one
SELECT state, n, last_seen
FROM argus_baselines
WHERE serial_id = $1
  AND target = $2
  AND port = $3
  AND metric = $4
  AND detector = $5
  AND src_ip = $6;

-- name: UpsertBaseline :exec
INSERT INTO argus_baselines
(serial_id, src_ip, target, port, metric, detector, state, n, last_seen, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
ON CONFLICT (serial_id, src_ip, target, port, metric, detector) DO UPDATE SET state      = EXCLUDED.state,
                                                                              n          = EXCLUDED.n,
                                                                              last_seen  = EXCLUDED.last_seen,
                                                                              updated_at = now();

-- name: ListActiveStampSeries :many
SELECT DISTINCT serial_id, target, port
FROM stamp_measurements
WHERE timestamp >= $1;

-- name: ListActivePingSeries :many
SELECT DISTINCT serial_id, target
FROM ping_measurements
WHERE timestamp >= $1;

-- name: ListActiveTraceSeries :many
SELECT DISTINCT serial_id, src, dst, method
FROM trace_measurements
WHERE timestamp >= $1;

-- name: FetchStampSamples :many
SELECT timestamp, loss, rtt_p95_ns, fwd_p95_ns, bwd_p95_ns
FROM stamp_measurements
WHERE serial_id = $1
  AND target = $2
  AND port = $3
  AND timestamp > @after
  AND timestamp <= @before;

-- name: FetchPingSamples :many
SELECT timestamp, loss, rtt_p95_ns
FROM ping_measurements
WHERE serial_id = $1
  AND target = $2
  AND timestamp > @after
  AND timestamp <= @before;

-- name: FetchTraceSamples :many
SELECT timestamp, asn_path_hash, link_path_hash
FROM trace_measurements
WHERE serial_id = $1
  AND src = $2
  AND dst = $3
  AND type = $4;

-- name: InsertFinding :one
INSERT INTO argus_findings
(serial_id, target, port, metric, detector, ts, value, severity, details)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (serial_id, target, port, metric, detector, ts) DO NOTHING
RETURNING id;

-- name: GetPolicyState :one
SELECT serial_id,
       target,
       port,
       metric,
       detector,
       status,
       score,
       score_updated_at,
       open_event_id,
       pending_findings,
       entered_status_at
FROM argus_policy_state
WHERE serial_id = $1
  AND target = $2
  AND port = $3
  AND metric = $4
  AND detector = $5;

-- name: UpsertPolicyState :exec
INSERT INTO argus_policy_state
(serial_id, target, port, metric, detector,
 status, score, score_updated_at,
 open_event_id, pending_findings, entered_status_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now())
ON CONFLICT (serial_id, target, port, metric, detector) DO UPDATE SET status            = EXCLUDED.status,
                                                                      score             = EXCLUDED.score,
                                                                      score_updated_at  = EXCLUDED.score_updated_at,
                                                                      open_event_id     = EXCLUDED.open_event_id,
                                                                      pending_findings  = EXCLUDED.pending_findings,
                                                                      entered_status_at = EXCLUDED.entered_status_at,
                                                                      updated_at        = now();

-- name: ListNonCleanPolicyStates :many
SELECT serial_id,
       target,
       port,
       metric,
       detector,
       status,
       score,
       score_updated_at,
       open_event_id,
       pending_findings,
       entered_status_at
FROM argus_policy_state
WHERE status <> 'clean';

-- name: OpenEvent :one
INSERT INTO argus_events
(serial_id, target, port, metric, status, opened_at, last_seen_at,
 finding_count, peak_severity, detectors)
VALUES ($1, $2, $3, $4, 'open', $5, $5, $6, $7, $8)
RETURNING id;

-- name: AttachFindingToEvent :exec
UPDATE argus_findings
SET event_id = $1
WHERE id = $2;

-- name: UpdateEventOnFinding :exec
UPDATE argus_events
SET last_seen_at  = $2,
    finding_count = finding_count + 1,
    peak_severity = GREATEST(peak_severity, $3),
    detectors     = CASE
                        WHEN $4::TEXT = ANY (detectors) THEN detectors
                        ELSE array_append(detectors, $4::TEXT)
        END
WHERE id = $1;

-- name: CloseEvent :exec
UPDATE argus_events
SET status    = 'closed',
    closed_at = $2
WHERE id = $1;
