-- Sender config
INSERT INTO agent_config (id, generation, report_endpoint, report_batch_size, report_interval_seconds, report_timeout_seconds, scamper_pps)
VALUES ('cfg-ap1', 1, 'nats://localhost:4222', 10, 30, 10, 100)
ON CONFLICT DO NOTHING;

INSERT INTO agent_task (agent_config_id, task_id, type, enabled, generation, schedule_interval_seconds, schedule_jitter_percent, schedule_enabled, params)
VALUES ('cfg-ap1', 'stamp-to-sa', 'stamp-sender', true, 1, 10, 10, true, '{"target": "10.0.0.6", "target_port": 862, "dscp": 0, "require_clock_sync": false, "packet_count": 20, "packet_interval": 100000000}')
ON CONFLICT DO NOTHING;

INSERT INTO agent_task (agent_config_id, task_id, type, enabled, generation, schedule_interval_seconds, schedule_jitter_percent, schedule_enabled, params)
VALUES ('cfg-ap1', 'trace-to-sa', 'trace', true, 1, 40, 10, true, '{"target": "1.1.1.1", "method": "icmp-paris"}')
ON CONFLICT DO NOTHING;

-- Reflector config
INSERT INTO agent_config (id, generation, report_endpoint, report_batch_size, report_interval_seconds, report_timeout_seconds, scamper_pps)
VALUES ('cfg-sa', 1, 'nats://localhost:4222', 10, 30, 10, 100)
ON CONFLICT DO NOTHING;

INSERT INTO agent_task (agent_config_id, task_id, type, enabled, generation, schedule_interval_seconds, schedule_jitter_percent, schedule_enabled, params)
VALUES ('cfg-sa', 'stamp-reflector', 'stamp-reflector', true, 1, 0, 0, false, '{"listen_port": 862, "dscp": 0, "require_clock_sync": false, "source_ip": "10.0.0.6"}')
ON CONFLICT DO NOTHING;

-- Link devices to configs
INSERT INTO device (serial_id, agent_config_id) VALUES ('ap1', 'cfg-ap1') ON CONFLICT DO NOTHING;
INSERT INTO device (serial_id, agent_config_id) VALUES ('security-appliance', 'cfg-sa') ON CONFLICT DO NOTHING;
