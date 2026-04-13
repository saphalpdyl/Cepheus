-- Sender config
INSERT INTO agent_config (id, version, generation, report_endpoint, report_batch_size, report_interval_seconds)
VALUES (1, 1, 1, '/api/v1/devices/data/ap1', 10, 30)
ON CONFLICT DO NOTHING;

INSERT INTO agent_task (agent_config_id, task_id, type, enabled, params)
VALUES (1, 'stamp-to-sa', 'stamp', true, '{"schedule": {"interval_seconds": 10, "jitter_percent": 10}, "target": "10.0.0.6", "target_port": 862, "mode": "sender"}')
ON CONFLICT DO NOTHING;

-- Reflector config
INSERT INTO agent_config (id, version, generation, report_endpoint, report_batch_size, report_interval_seconds)
VALUES (2, 1, 1, '/api/v1/devices/data/security-appliance', 10, 30)
ON CONFLICT DO NOTHING;

INSERT INTO agent_task (agent_config_id, task_id, type, enabled, params)
VALUES (2, 'stamp-reflector', 'stamp', true, '{"schedule": {"interval_seconds": 0, "jitter_percent": 0}, "target": "", "target_port": 862, "mode": "reflector"}')
ON CONFLICT DO NOTHING;

-- Link devices to configs
INSERT INTO device (serial_id, agent_config_id) VALUES ('ap1', 1) ON CONFLICT DO NOTHING;
INSERT INTO device (serial_id, agent_config_id) VALUES ('security-appliance', 2) ON CONFLICT DO NOTHING;

-- Reset sequences past seeded IDs
SELECT setval('agent_config_id_seq', (SELECT COALESCE(MAX(id), 0) FROM agent_config));
SELECT setval('agent_task_id_seq', (SELECT COALESCE(MAX(id), 0) FROM agent_task));
