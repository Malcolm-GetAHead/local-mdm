-- Seed data for Local MDM dashboard development
-- Run: make seed (or psql -f migrations/seed_data.sql)
-- Idempotent: uses ON CONFLICT DO NOTHING

BEGIN;

-- Enterprise
INSERT INTO enterprises (id, name, slug, settings) VALUES
  ('00000000-0000-0000-0000-000000000001', 'Acme Corp', 'acme-corp', '{"timezone": "America/New_York", "max_devices": 100}')
ON CONFLICT (slug) DO NOTHING;

-- Admin user (password_hash is nullable since Sprint 5c — OIDC-managed users)
INSERT INTO users (id, enterprise_id, email, full_name, role, is_active) VALUES
  ('b0000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'admin@acme.test', 'Alice Admin', 'admin', true),
  ('b0000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'operator@acme.test', 'Bob Operator', 'operator', true),
  ('b0000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000001', 'viewer@acme.test', 'Carol Viewer', 'viewer', true)
ON CONFLICT (enterprise_id, email) DO NOTHING;

-- Devices: 25 across macOS, Windows, Android
INSERT INTO devices (id, enterprise_id, platform, device_id, serial_number, name, model, os_version, status, last_seen, platform_data) VALUES
  -- macOS devices (8)
  ('d0000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-001', 'C02X1234ABCD', 'Alice MacBook Pro', 'MacBook Pro 16"', '14.4.1', 'enrolled', NOW() - interval '10 minutes', '{"serial": "C02X1234ABCD", "mdm_enrolled": true}'),
  ('d0000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-002', 'C02X5678EFGH', 'Bob MacBook Air', 'MacBook Air M2', '14.3', 'enrolled', NOW() - interval '2 hours', '{"serial": "C02X5678EFGH", "mdm_enrolled": true}'),
  ('d0000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-003', 'C02X9012IJKL', 'Conf Room iMac', 'iMac 24"', '14.4.1', 'enrolled', NOW() - interval '1 day', '{"serial": "C02X9012IJKL", "mdm_enrolled": true}'),
  ('d0000000-0000-0000-0000-000000000004', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-004', 'C02X3456MNOP', 'Dev Mac Mini', 'Mac Mini M2', '14.2', 'enrolled', NOW() - interval '3 days', '{}'),
  ('d0000000-0000-0000-0000-000000000005', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-005', 'C02X7890QRST', 'Carol MacBook Pro', 'MacBook Pro 14"', '14.4', 'enrolled', NOW() - interval '30 minutes', '{"serial": "C02X7890QRST"}'),
  ('d0000000-0000-0000-0000-000000000006', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-006', 'C02XDEADBEEF', 'Old MacBook', 'MacBook Pro 13"', '13.6', 'unenrolled', NOW() - interval '30 days', '{}'),
  ('d0000000-0000-0000-0000-000000000007', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-007', 'C02XCAFEBABE', 'Lost MacBook', 'MacBook Air M1', '14.1', 'wiped', NOW() - interval '7 days', '{}'),
  ('d0000000-0000-0000-0000-000000000008', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-008', 'C02XFEEDFACE', 'Exec MacBook', 'MacBook Pro 16"', '14.4.1', 'enrolled', NOW() - interval '5 minutes', '{"serial": "C02XFEEDFACE", "mdm_enrolled": true}'),
  -- Windows devices (10)
  ('d0000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000001', 'windows', 'win-001', 'PF3K1234', 'Sales Laptop 1', 'Dell Latitude 5540', '10.0.22631', 'enrolled', NOW() - interval '15 minutes', '{"encryption": true}'),
  ('d0000000-0000-0000-0000-000000000011', '00000000-0000-0000-0000-000000000001', 'windows', 'win-002', 'PF3K5678', 'Sales Laptop 2', 'Dell Latitude 5540', '10.0.22631', 'enrolled', NOW() - interval '1 hour', '{"encryption": true}'),
  ('d0000000-0000-0000-0000-000000000012', '00000000-0000-0000-0000-000000000001', 'windows', 'win-003', 'PF3K9012', 'Finance Desktop', 'HP EliteDesk 800', '10.0.22631', 'enrolled', NOW() - interval '4 hours', '{"encryption": false}'),
  ('d0000000-0000-0000-0000-000000000013', '00000000-0000-0000-0000-000000000001', 'windows', 'win-004', 'PF3K3456', 'HR Laptop', 'Lenovo ThinkPad X1', '10.0.19045', 'enrolled', NOW() - interval '2 days', '{"encryption": true}'),
  ('d0000000-0000-0000-0000-000000000014', '00000000-0000-0000-0000-000000000001', 'windows', 'win-005', 'PF3K7890', 'Kiosk Terminal', 'Dell OptiPlex 7010', '10.0.22631', 'enrolled', NOW() - interval '6 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000015', '00000000-0000-0000-0000-000000000001', 'windows', 'win-006', 'PF3KABCD', 'Dev Workstation', 'HP ZBook Fury', '10.0.22631', 'enrolled', NOW() - interval '20 minutes', '{"encryption": true}'),
  ('d0000000-0000-0000-0000-000000000016', '00000000-0000-0000-0000-000000000001', 'windows', 'win-007', 'PF3KEFGH', 'Shared Laptop', 'Dell Latitude 3540', '10.0.19045', 'enrolled', NOW() - interval '5 days', '{}'),
  ('d0000000-0000-0000-0000-000000000017', '00000000-0000-0000-0000-000000000001', 'windows', 'win-008', 'PF3KIJKL', 'Retired PC', 'HP ProDesk 400', '10.0.19044', 'unenrolled', NOW() - interval '60 days', '{}'),
  ('d0000000-0000-0000-0000-000000000018', '00000000-0000-0000-0000-000000000001', 'windows', 'win-009', 'PF3KMNOP', 'Exec Surface', 'Surface Pro 9', '10.0.22631', 'enrolled', NOW() - interval '45 minutes', '{"encryption": true}'),
  ('d0000000-0000-0000-0000-000000000019', '00000000-0000-0000-0000-000000000001', 'windows', 'win-010', 'PF3KQRST', 'Lobby Kiosk', 'Dell OptiPlex 3000', '10.0.22631', 'enrolled', NOW() - interval '12 hours', '{}'),
  -- Android devices (7)
  ('d0000000-0000-0000-0000-000000000020', '00000000-0000-0000-0000-000000000001', 'android', 'and-001', 'R58N1234ABCD', 'Field Phone 1', 'Samsung Galaxy S24', '14', 'enrolled', NOW() - interval '25 minutes', '{"security_patch": "2024-03-01"}'),
  ('d0000000-0000-0000-0000-000000000021', '00000000-0000-0000-0000-000000000001', 'android', 'and-002', 'R58N5678EFGH', 'Field Phone 2', 'Samsung Galaxy A54', '14', 'enrolled', NOW() - interval '3 hours', '{"security_patch": "2024-02-01"}'),
  ('d0000000-0000-0000-0000-000000000022', '00000000-0000-0000-0000-000000000001', 'android', 'and-003', 'R58N9012IJKL', 'Warehouse Tablet', 'Samsung Galaxy Tab S9', '13', 'enrolled', NOW() - interval '8 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000023', '00000000-0000-0000-0000-000000000001', 'android', 'and-004', 'PIXEL1234MNOP', 'Dev Phone', 'Google Pixel 8', '14', 'enrolled', NOW() - interval '1 hour', '{"security_patch": "2024-03-05"}'),
  ('d0000000-0000-0000-0000-000000000024', '00000000-0000-0000-0000-000000000001', 'android', 'and-005', 'R58NQRST5678', 'Lost Android', 'Samsung Galaxy S23', '13', 'wiped', NOW() - interval '14 days', '{}'),
  ('d0000000-0000-0000-0000-000000000025', '00000000-0000-0000-0000-000000000001', 'android', 'and-006', 'PIXEL5678UVWX', 'Exec Phone', 'Google Pixel 8 Pro', '14', 'enrolled', NOW() - interval '10 minutes', '{"security_patch": "2024-03-05"}'),
  ('d0000000-0000-0000-0000-000000000026', '00000000-0000-0000-0000-000000000001', 'android', 'and-007', 'R58NYZAB9012', 'Shared Tablet', 'Samsung Galaxy Tab A9', '13', 'enrolled', NOW() - interval '2 days', '{}')
ON CONFLICT (enterprise_id, platform, device_id) DO UPDATE SET status = EXCLUDED.status, name = EXCLUDED.name;

-- Policies (6 enterprise policies + 2 templates)
INSERT INTO policies (id, enterprise_id, name, description, platform, policy_type, policy_config, is_active, is_template) VALUES
  ('e0000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'Corporate Security Baseline', 'Require encryption and strong passwords on all devices', 'all', 'security', '{"require_encryption": true, "min_password_length": 8, "require_firewall": true}', true, false),
  ('e0000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Corporate WiFi', 'Auto-configure corporate WiFi on enrollment', 'all', 'wifi', '{"ssid": "AcmeCorp", "security": "WPA2-Enterprise", "auto_join": true}', true, false),
  ('e0000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000001', 'macOS Restrictions', 'Disable camera and AirDrop for macOS fleet', 'macos', 'restrictions', '{"disable_camera": true, "disable_airdrop": true}', true, false),
  ('e0000000-0000-0000-0000-000000000004', '00000000-0000-0000-0000-000000000001', 'Windows BitLocker', 'Enforce BitLocker encryption on Windows devices', 'windows', 'security', '{"require_encryption": true, "encryption_method": "XTS-AES-256"}', true, false),
  ('e0000000-0000-0000-0000-000000000005', '00000000-0000-0000-0000-000000000001', 'Android Work Profile', 'Configure work profile restrictions', 'android', 'restrictions', '{"disable_camera": false, "disable_screen_capture": true}', true, false),
  ('e0000000-0000-0000-0000-000000000006', '00000000-0000-0000-0000-000000000001', 'VPN Configuration', 'Corporate VPN for remote access', 'all', 'vpn', '{"server": "vpn.acme.test", "protocol": "IKEv2", "on_demand": true}', false, false),
  -- Templates
  ('e0000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000001', 'NIST Security Template', 'NIST 800-171 baseline security controls', 'all', 'security', '{"require_encryption": true, "min_password_length": 12, "require_firewall": true, "auto_lock_minutes": 5}', true, true),
  ('e0000000-0000-0000-0000-000000000011', '00000000-0000-0000-0000-000000000001', 'BYOD Restrictions Template', 'Minimal restrictions for BYOD devices', 'all', 'restrictions', '{"disable_camera": false, "require_passcode": true}', true, true)
ON CONFLICT DO NOTHING;

-- Device groups (3)
INSERT INTO device_groups (id, enterprise_id, name, description) VALUES
  ('f0000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'Engineering', 'Engineering team devices'),
  ('f0000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Sales', 'Sales team devices'),
  ('f0000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000001', 'Executives', 'Executive team devices')
ON CONFLICT (enterprise_id, name) DO NOTHING;

-- Group memberships
INSERT INTO group_memberships (group_id, device_id) VALUES
  -- Engineering: dev machines
  ('f0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000001'),
  ('f0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000004'),
  ('f0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000015'),
  ('f0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000023'),
  -- Sales: sales laptops + field phones
  ('f0000000-0000-0000-0000-000000000002', 'd0000000-0000-0000-0000-000000000010'),
  ('f0000000-0000-0000-0000-000000000002', 'd0000000-0000-0000-0000-000000000011'),
  ('f0000000-0000-0000-0000-000000000002', 'd0000000-0000-0000-0000-000000000020'),
  ('f0000000-0000-0000-0000-000000000002', 'd0000000-0000-0000-0000-000000000021'),
  -- Executives
  ('f0000000-0000-0000-0000-000000000003', 'd0000000-0000-0000-0000-000000000008'),
  ('f0000000-0000-0000-0000-000000000003', 'd0000000-0000-0000-0000-000000000018'),
  ('f0000000-0000-0000-0000-000000000003', 'd0000000-0000-0000-0000-000000000025')
ON CONFLICT DO NOTHING;

-- Policy assignments (enterprise-wide, group, and device-level)
INSERT INTO policy_assignments (id, policy_id, target_type, target_id, priority) VALUES
  -- Security baseline → entire enterprise
  ('aa000000-0000-0000-0000-000000000001', 'e0000000-0000-0000-0000-000000000001', 'enterprise', '00000000-0000-0000-0000-000000000001', 0),
  -- WiFi → entire enterprise
  ('aa000000-0000-0000-0000-000000000002', 'e0000000-0000-0000-0000-000000000002', 'enterprise', '00000000-0000-0000-0000-000000000001', 0),
  -- macOS restrictions → Engineering group
  ('aa000000-0000-0000-0000-000000000003', 'e0000000-0000-0000-0000-000000000003', 'group', 'f0000000-0000-0000-0000-000000000001', 10),
  -- BitLocker → Sales group (Windows devices)
  ('aa000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000004', 'group', 'f0000000-0000-0000-0000-000000000002', 10),
  -- VPN → Executives group
  ('aa000000-0000-0000-0000-000000000005', 'e0000000-0000-0000-0000-000000000006', 'group', 'f0000000-0000-0000-0000-000000000003', 20),
  -- Android work profile → specific device
  ('aa000000-0000-0000-0000-000000000006', 'e0000000-0000-0000-0000-000000000005', 'device', 'd0000000-0000-0000-0000-000000000020', 30)
ON CONFLICT DO NOTHING;

-- Compliance results (mix of compliant, non_compliant, unknown)
INSERT INTO compliance_results (id, device_id, policy_id, status, details, evaluated_at) VALUES
  -- Security baseline compliance
  ('cc000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000001', 'e0000000-0000-0000-0000-000000000001', 'compliant', '{"encryption": true, "password_length": 12, "firewall": true}', NOW() - interval '10 minutes'),
  ('cc000000-0000-0000-0000-000000000002', 'd0000000-0000-0000-0000-000000000002', 'e0000000-0000-0000-0000-000000000001', 'compliant', '{"encryption": true, "password_length": 10, "firewall": true}', NOW() - interval '2 hours'),
  ('cc000000-0000-0000-0000-000000000003', 'd0000000-0000-0000-0000-000000000010', 'e0000000-0000-0000-0000-000000000001', 'compliant', '{"encryption": true, "password_length": 8, "firewall": true}', NOW() - interval '15 minutes'),
  ('cc000000-0000-0000-0000-000000000004', 'd0000000-0000-0000-0000-000000000012', 'e0000000-0000-0000-0000-000000000001', 'non_compliant', '{"encryption": false, "password_length": 6, "firewall": true, "violations": ["encryption_disabled", "password_too_short"]}', NOW() - interval '4 hours'),
  ('cc000000-0000-0000-0000-000000000005', 'd0000000-0000-0000-0000-000000000013', 'e0000000-0000-0000-0000-000000000001', 'non_compliant', '{"encryption": true, "password_length": 4, "firewall": false, "violations": ["password_too_short", "firewall_disabled"]}', NOW() - interval '2 days'),
  ('cc000000-0000-0000-0000-000000000006', 'd0000000-0000-0000-0000-000000000020', 'e0000000-0000-0000-0000-000000000001', 'compliant', '{"encryption": true, "password_length": 8}', NOW() - interval '25 minutes'),
  ('cc000000-0000-0000-0000-000000000007', 'd0000000-0000-0000-0000-000000000022', 'e0000000-0000-0000-0000-000000000001', 'unknown', '{}', NOW() - interval '8 hours'),
  ('cc000000-0000-0000-0000-000000000008', 'd0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000001', 'non_compliant', '{"encryption": true, "password_length": 6, "firewall": true, "violations": ["password_too_short"]}', NOW() - interval '3 days'),
  -- BitLocker compliance
  ('cc000000-0000-0000-0000-000000000010', 'd0000000-0000-0000-0000-000000000010', 'e0000000-0000-0000-0000-000000000004', 'compliant', '{"encryption": true}', NOW() - interval '15 minutes'),
  ('cc000000-0000-0000-0000-000000000011', 'd0000000-0000-0000-0000-000000000012', 'e0000000-0000-0000-0000-000000000004', 'non_compliant', '{"encryption": false, "violations": ["encryption_disabled"]}', NOW() - interval '4 hours'),
  -- WiFi compliance (all compliant — it's just config push)
  ('cc000000-0000-0000-0000-000000000020', 'd0000000-0000-0000-0000-000000000001', 'e0000000-0000-0000-0000-000000000002', 'compliant', '{}', NOW() - interval '10 minutes'),
  ('cc000000-0000-0000-0000-000000000021', 'd0000000-0000-0000-0000-000000000010', 'e0000000-0000-0000-0000-000000000002', 'compliant', '{}', NOW() - interval '15 minutes')
ON CONFLICT (device_id, policy_id) DO NOTHING;

-- Audit log entries (recent actions)
INSERT INTO audit_logs (enterprise_id, user_id, action, resource_type, resource_id, details, ip_address, created_at) VALUES
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', 'device.lock', 'device', 'd0000000-0000-0000-0000-000000000007', '{"reason": "reported lost"}', '10.0.1.50', NOW() - interval '7 days'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', 'device.wipe', 'device', 'd0000000-0000-0000-0000-000000000007', '{"reason": "confirmed lost, wiping"}', '10.0.1.50', NOW() - interval '7 days' + interval '5 minutes'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', 'policy.create', 'policy', 'e0000000-0000-0000-0000-000000000001', '{"name": "Corporate Security Baseline"}', '10.0.1.50', NOW() - interval '14 days'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', 'policy.assign', 'policy', 'e0000000-0000-0000-0000-000000000001', '{"target_type": "enterprise", "target_id": "00000000-0000-0000-0000-000000000001"}', '10.0.1.50', NOW() - interval '14 days' + interval '2 minutes'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000002', 'device.enroll', 'device', 'd0000000-0000-0000-0000-000000000020', '{"platform": "android", "model": "Samsung Galaxy S24"}', '10.0.1.100', NOW() - interval '5 days'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', 'group.create', 'group', 'f0000000-0000-0000-0000-000000000001', '{"name": "Engineering"}', '10.0.1.50', NOW() - interval '10 days'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', 'group.add_member', 'group', 'f0000000-0000-0000-0000-000000000001', '{"device_id": "d0000000-0000-0000-0000-000000000001"}', '10.0.1.50', NOW() - interval '10 days' + interval '1 minute'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', 'compliance.evaluate', 'device', 'd0000000-0000-0000-0000-000000000012', '{"result": "non_compliant", "violations": ["encryption_disabled"]}', '10.0.1.50', NOW() - interval '4 hours'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000002', 'device.lock', 'device', 'd0000000-0000-0000-0000-000000000024', '{"reason": "employee termination"}', '10.0.1.100', NOW() - interval '14 days'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', 'policy.update', 'policy', 'e0000000-0000-0000-0000-000000000001', '{"changes": ["min_password_length: 6 -> 8"]}', '10.0.1.50', NOW() - interval '3 days'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', 'user.login', 'user', 'b0000000-0000-0000-0000-000000000001', '{"method": "oidc"}', '10.0.1.50', NOW() - interval '1 hour'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000002', 'user.login', 'user', 'b0000000-0000-0000-0000-000000000002', '{"method": "oidc"}', '10.0.1.100', NOW() - interval '2 hours')
;

COMMIT;
