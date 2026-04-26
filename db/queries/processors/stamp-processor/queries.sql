-- name: InsertStampData :one
INSERT INTO stamp_data
(timestamp, serial_id, target, port, sent, received, loss, avg_rtt, min_rtt, max_rtt, p50_rtt, p95_rtt, agent_config_id)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12, $13)
RETURNING *;