-- name: InsertTraceMeasurement :one
INSERT INTO trace_measurements
    (timestamp, type, src, dst, method, stop_reason, hop_count, path_hash, raw)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: InsertTraceHop :copyfrom
INSERT INTO trace_hops
    (timestamp, measurement_id, ip, ttl, rtt_ms, icmp_type, icmp_code, reply_ttl, asn, is_no_hop)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: GetMeasurementsByPath :many
SELECT * FROM trace_measurements
WHERE src = $1
  AND dst = $2
  AND timestamp > NOW() - $3::interval
ORDER BY timestamp DESC;

-- name: GetHopsByMeasurement :many
SELECT * FROM trace_hops
WHERE measurement_id = $1
ORDER BY ttl ASC;