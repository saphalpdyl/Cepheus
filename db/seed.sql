INSERT INTO agent_config (serial_id, config) VALUES
    ('ap1', '{"probes": [{"type": "stamp", "mode": "active", "params": {"local_addr": ":862", "remote_addr": "10.0.0.6:862"}}]}'),
    ('security-appliance', '{"probes": [{"type": "stamp", "mode": "reflector", "params": {"local_addr": ":862"}}]}')
ON CONFLICT (serial_id) DO NOTHING;
