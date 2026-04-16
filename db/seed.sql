-- Sender config
INSERT INTO agent_config (id, generation, report_endpoint, report_batch_size, report_interval_seconds, scamper_pps)
VALUES ('cfg-ap1', 1, '/api/v1/devices/data/ap1', 10, 30, 100)
ON CONFLICT DO NOTHING;

INSERT INTO agent_task (agent_config_id, task_id, type, enabled, generation, schedule_interval_seconds, schedule_jitter_percent, params)
VALUES ('cfg-ap1', 'stamp-to-sa', 'stamp', true, 1, 10, 10, '{"target": "10.0.0.6", "target_port": 862, "mode": "sender", "dscp": 0, "require_clock_sync": false}')
ON CONFLICT DO NOTHING;

INSERT INTO agent_task (agent_config_id, task_id, type, enabled, generation, schedule_interval_seconds, schedule_jitter_percent, params)
VALUES ('cfg-ap1', 'trace-to-sa', 'trace', true, 1, 60, 20, '{"target": "10.0.0.6", "method": "icmp-paris", "max_ttl": 30, "first_ttl": 1, "dscp": 0, "wait_seconds": 5}')
ON CONFLICT DO NOTHING;

INSERT INTO agent_task (agent_config_id, task_id, type, enabled, generation, schedule_interval_seconds, schedule_jitter_percent, params)
VALUES ('cfg-ap1', 'tracelb-to-sa', 'tracelb', true, 1, 120, 20, '{"target": "10.0.0.6", "max_ttl": 30, "first_ttl": 1, "dscp": 0, "confidence": 0.95}')
ON CONFLICT DO NOTHING;

INSERT INTO agent_task (agent_config_id, task_id, type, enabled, generation, schedule_interval_seconds, schedule_jitter_percent, params)
VALUES ('cfg-ap1', 'ping-to-sa', 'ping', true, 1, 10, 10, '{"target": "10.0.0.6", "count": 5, "size": 64, "dscp": 0, "timeout_seconds": 3}')
ON CONFLICT DO NOTHING;

-- Reflector config
INSERT INTO agent_config (id, generation, report_endpoint, report_batch_size, report_interval_seconds, scamper_pps)
VALUES ('cfg-sa', 1, '/api/v1/devices/data/security-appliance', 10, 30, 100)
ON CONFLICT DO NOTHING;

INSERT INTO agent_task (agent_config_id, task_id, type, enabled, generation, schedule_interval_seconds, schedule_jitter_percent, params)
VALUES ('cfg-sa', 'stamp-reflector', 'stamp', true, 1, 0, 0, '{"target": "", "target_port": 862, "mode": "reflector", "dscp": 0, "require_clock_sync": false}')
ON CONFLICT DO NOTHING;

-- Link devices to configs
INSERT INTO device (serial_id, agent_config_id) VALUES ('ap1', 'cfg-ap1') ON CONFLICT DO NOTHING;
INSERT INTO device (serial_id, agent_config_id) VALUES ('security-appliance', 'cfg-sa') ON CONFLICT DO NOTHING;
