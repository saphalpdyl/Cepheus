-- name: InsertPingMeasurement :one
INSERT INTO ping_measurements (timestamp, serial_id, agent_config_id, target, sent, received, loss, rtt_min_ns, rtt_avg_ns, rtt_max_ns, rtt_p50_ns, rtt_p95_ns, rtt_stddev_ns)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id;

-- name: InsertPingProbes :copyfrom
INSERT INTO ping_probes (measurement_id, tx, rx, is_lost, seq, rtt)
VALUES ($1, $2, $3, $4, $5, $6);
