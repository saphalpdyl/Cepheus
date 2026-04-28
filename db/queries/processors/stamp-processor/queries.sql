-- name: InsertStampMeasurement :one
INSERT INTO stamp_measurements (timestamp, serial_id, agent_config_id, target, port, sent, received, loss)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id;

-- name: InsertStampProbes :copyfrom
INSERT INTO stamp_probes (measurement_id, tx, is_lost, rx, rtt, forward_delay, backward_delay)
VALUES ($1, $2, $3, $4, $5, $6, $7);