-- Seed data for Local MDM dashboard development
-- Run: make seed (or psql -f migrations/seed_data.sql)
-- Idempotent: uses ON CONFLICT DO NOTHING

BEGIN;

-- Reset test mutations: restore seed devices that were soft-deleted
UPDATE devices SET deleted_at = NULL WHERE id::text LIKE 'd0000000-0000-0000-0000-%' AND enterprise_id = '00000000-0000-0000-0000-000000000001';
UPDATE policies SET deleted_at = NULL WHERE id::text LIKE 'e0000000-0000-0000-0000-%' AND enterprise_id = '00000000-0000-0000-0000-000000000001';
UPDATE device_groups SET deleted_at = NULL WHERE id::text LIKE 'f0000000-0000-0000-0000-%' AND enterprise_id = '00000000-0000-0000-0000-000000000001';

-- Enterprise
INSERT INTO enterprises (id, name, slug, settings) VALUES
  ('00000000-0000-0000-0000-000000000001', 'Acme Corp', 'acme-corp', '{"timezone": "America/New_York", "max_devices": 100}')
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, settings = EXCLUDED.settings;

-- Test enterprise — used by Go integration tests. DO NOT DELETE.
INSERT INTO enterprises (id, name, slug, settings) VALUES
  ('99999999-9999-9999-9999-999999999999', 'Test Enterprise (DO NOT DELETE)', 'test-enterprise', '{"test": true}')
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
  ('d0000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-001', 'C02X1234ABCD', '[Seed] Alice MacBook Pro', 'MacBook Pro 16"', '14.4.1', 'enrolled', NOW() - interval '10 minutes', '{"serial": "C02X1234ABCD", "mdm_enrolled": true, "build_version": "23E224", "FileVaultEnabled": true, "firewall_enabled": true, "password_present": true, "password_length": 12, "hostname": "Alices-MBP.local", "ip_address": "10.0.1.42", "mac_address": "A4:83:E7:2B:1F:90", "storage_total_gb": 512, "storage_free_gb": 287, "supervised": false, "architecture": "Apple Silicon"}'),
  ('d0000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-002', 'C02X5678EFGH', '[Seed] Bob MacBook Air', 'MacBook Air M2', '14.3', 'enrolled', NOW() - interval '2 hours', '{"serial": "C02X5678EFGH", "mdm_enrolled": true}'),
  ('d0000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-003', 'C02X9012IJKL', '[Seed] Conf Room iMac', 'iMac 24"', '14.4.1', 'enrolled', NOW() - interval '1 day', '{"serial": "C02X9012IJKL", "mdm_enrolled": true}'),
  ('d0000000-0000-0000-0000-000000000004', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-004', 'C02X3456MNOP', '[Seed] Dev Mac Mini', 'Mac Mini M2', '14.2', 'enrolled', NOW() - interval '3 days', '{}'),
  ('d0000000-0000-0000-0000-000000000005', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-005', 'C02X7890QRST', '[Seed] Carol MacBook Pro', 'MacBook Pro 14"', '14.4', 'enrolled', NOW() - interval '30 minutes', '{"serial": "C02X7890QRST"}'),
  ('d0000000-0000-0000-0000-000000000006', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-006', 'C02XDEADBEEF', '[Seed] Old MacBook', 'MacBook Pro 13"', '13.6', 'unenrolled', NOW() - interval '30 days', '{}'),
  ('d0000000-0000-0000-0000-000000000007', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-007', 'C02XCAFEBABE', '[Seed] Lost MacBook', 'MacBook Air M1', '14.1', 'wiped', NOW() - interval '7 days', '{}'),
  ('d0000000-0000-0000-0000-000000000008', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-008', 'C02XFEEDFACE', '[Seed] Exec MacBook', 'MacBook Pro 16"', '14.4.1', 'enrolled', NOW() - interval '5 minutes', '{"serial": "C02XFEEDFACE", "mdm_enrolled": true}'),
  -- Windows devices (10)
  ('d0000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000001', 'windows', 'win-001', 'PF3K1234', '[Seed] Sales Laptop 1', 'Dell Latitude 5540', '10.0.22631', 'enrolled', NOW() - interval '15 minutes', '{"encryption": true}'),
  ('d0000000-0000-0000-0000-000000000011', '00000000-0000-0000-0000-000000000001', 'windows', 'win-002', 'PF3K5678', '[Seed] Sales Laptop 2', 'Dell Latitude 5540', '10.0.22631', 'enrolled', NOW() - interval '1 hour', '{"encryption": true}'),
  ('d0000000-0000-0000-0000-000000000012', '00000000-0000-0000-0000-000000000001', 'windows', 'win-003', 'PF3K9012', '[Seed] Finance Desktop', 'HP EliteDesk 800', '10.0.22631', 'enrolled', NOW() - interval '4 hours', '{"encryption": false}'),
  ('d0000000-0000-0000-0000-000000000013', '00000000-0000-0000-0000-000000000001', 'windows', 'win-004', 'PF3K3456', '[Seed] HR Laptop', 'Lenovo ThinkPad X1', '10.0.19045', 'enrolled', NOW() - interval '2 days', '{"encryption": true}'),
  ('d0000000-0000-0000-0000-000000000014', '00000000-0000-0000-0000-000000000001', 'windows', 'win-005', 'PF3K7890', '[Seed] Kiosk Terminal', 'Dell OptiPlex 7010', '10.0.22631', 'enrolled', NOW() - interval '6 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000015', '00000000-0000-0000-0000-000000000001', 'windows', 'win-006', 'PF3KABCD', '[Seed] Dev Workstation', 'HP ZBook Fury', '10.0.22631', 'enrolled', NOW() - interval '20 minutes', '{"encryption": true}'),
  ('d0000000-0000-0000-0000-000000000016', '00000000-0000-0000-0000-000000000001', 'windows', 'win-007', 'PF3KEFGH', '[Seed] Shared Laptop', 'Dell Latitude 3540', '10.0.19045', 'enrolled', NOW() - interval '5 days', '{}'),
  ('d0000000-0000-0000-0000-000000000017', '00000000-0000-0000-0000-000000000001', 'windows', 'win-008', 'PF3KIJKL', '[Seed] Retired PC', 'HP ProDesk 400', '10.0.19044', 'unenrolled', NOW() - interval '60 days', '{}'),
  ('d0000000-0000-0000-0000-000000000018', '00000000-0000-0000-0000-000000000001', 'windows', 'win-009', 'PF3KMNOP', '[Seed] Exec Surface', 'Surface Pro 9', '10.0.22631', 'enrolled', NOW() - interval '45 minutes', '{"encryption": true}'),
  ('d0000000-0000-0000-0000-000000000019', '00000000-0000-0000-0000-000000000001', 'windows', 'win-010', 'PF3KQRST', '[Seed] Lobby Kiosk', 'Dell OptiPlex 3000', '10.0.22631', 'enrolled', NOW() - interval '12 hours', '{}'),
  -- Android devices (7)
  ('d0000000-0000-0000-0000-000000000020', '00000000-0000-0000-0000-000000000001', 'android', 'and-001', 'R58N1234ABCD', '[Seed] Field Phone 1', 'Samsung Galaxy S24', '14', 'enrolled', NOW() - interval '25 minutes', '{"security_patch": "2024-03-01"}'),
  ('d0000000-0000-0000-0000-000000000021', '00000000-0000-0000-0000-000000000001', 'android', 'and-002', 'R58N5678EFGH', '[Seed] Field Phone 2', 'Samsung Galaxy A54', '14', 'enrolled', NOW() - interval '3 hours', '{"security_patch": "2024-02-01"}'),
  ('d0000000-0000-0000-0000-000000000022', '00000000-0000-0000-0000-000000000001', 'android', 'and-003', 'R58N9012IJKL', '[Seed] Warehouse Tablet', 'Samsung Galaxy Tab S9', '13', 'enrolled', NOW() - interval '8 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000023', '00000000-0000-0000-0000-000000000001', 'android', 'and-004', 'PIXEL1234MNOP', '[Seed] Dev Phone', 'Google Pixel 8', '14', 'enrolled', NOW() - interval '1 hour', '{"security_patch": "2024-03-05"}'),
  ('d0000000-0000-0000-0000-000000000024', '00000000-0000-0000-0000-000000000001', 'android', 'and-005', 'R58NQRST5678', '[Seed] Lost Android', 'Samsung Galaxy S23', '13', 'wiped', NOW() - interval '14 days', '{}'),
  ('d0000000-0000-0000-0000-000000000025', '00000000-0000-0000-0000-000000000001', 'android', 'and-006', 'PIXEL5678UVWX', '[Seed] Exec Phone', 'Google Pixel 8 Pro', '14', 'enrolled', NOW() - interval '10 minutes', '{"security_patch": "2024-03-05"}'),
  ('d0000000-0000-0000-0000-000000000026', '00000000-0000-0000-0000-000000000001', 'android', 'and-007', 'R58NYZAB9012', '[Seed] Shared Tablet', 'Samsung Galaxy Tab A9', '13', 'enrolled', NOW() - interval '2 days', '{}'),
  -- Additional devices for pagination testing (30)
  ('d0000000-0000-0000-0000-000000000030', '00000000-0000-0000-0000-000000000001', 'windows', 'win-030', 'BULK001', '[Seed] Accounting PC 1', 'Dell OptiPlex 7010', '10.0.22631', 'enrolled', NOW() - interval '1 hour', '{}'),
  ('d0000000-0000-0000-0000-000000000031', '00000000-0000-0000-0000-000000000001', 'windows', 'win-031', 'BULK002', '[Seed] Accounting PC 2', 'Dell OptiPlex 7010', '10.0.22631', 'enrolled', NOW() - interval '2 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000032', '00000000-0000-0000-0000-000000000001', 'windows', 'win-032', 'BULK003', '[Seed] Accounting PC 3', 'Dell OptiPlex 7010', '10.0.22631', 'enrolled', NOW() - interval '3 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000033', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-033', 'BULK004', '[Seed] Design Mac 1', 'iMac 24"', '14.4.1', 'enrolled', NOW() - interval '4 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000034', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-034', 'BULK005', '[Seed] Design Mac 2', 'iMac 24"', '14.4.1', 'enrolled', NOW() - interval '5 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000035', '00000000-0000-0000-0000-000000000001', 'windows', 'win-035', 'BULK006', '[Seed] Support PC 1', 'HP ProDesk 400', '10.0.22631', 'enrolled', NOW() - interval '6 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000036', '00000000-0000-0000-0000-000000000001', 'windows', 'win-036', 'BULK007', '[Seed] Support PC 2', 'HP ProDesk 400', '10.0.22631', 'enrolled', NOW() - interval '7 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000037', '00000000-0000-0000-0000-000000000001', 'windows', 'win-037', 'BULK008', '[Seed] Support PC 3', 'HP ProDesk 400', '10.0.22631', 'enrolled', NOW() - interval '8 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000038', '00000000-0000-0000-0000-000000000001', 'android', 'and-038', 'BULK009', '[Seed] Warehouse Phone 1', 'Samsung Galaxy A14', '13', 'enrolled', NOW() - interval '9 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000039', '00000000-0000-0000-0000-000000000001', 'android', 'and-039', 'BULK010', '[Seed] Warehouse Phone 2', 'Samsung Galaxy A14', '13', 'enrolled', NOW() - interval '10 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000040', '00000000-0000-0000-0000-000000000001', 'windows', 'win-040', 'BULK011', '[Seed] Training Room PC 1', 'Dell Latitude 3540', '10.0.22631', 'enrolled', NOW() - interval '11 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000041', '00000000-0000-0000-0000-000000000001', 'windows', 'win-041', 'BULK012', '[Seed] Training Room PC 2', 'Dell Latitude 3540', '10.0.22631', 'enrolled', NOW() - interval '12 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000042', '00000000-0000-0000-0000-000000000001', 'windows', 'win-042', 'BULK013', '[Seed] Training Room PC 3', 'Dell Latitude 3540', '10.0.22631', 'enrolled', NOW() - interval '13 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000043', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-043', 'BULK014', '[Seed] Marketing Mac 1', 'MacBook Air M2', '14.3', 'enrolled', NOW() - interval '14 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000044', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-044', 'BULK015', '[Seed] Marketing Mac 2', 'MacBook Air M2', '14.3', 'enrolled', NOW() - interval '15 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000045', '00000000-0000-0000-0000-000000000001', 'windows', 'win-045', 'BULK016', '[Seed] Reception PC', 'HP EliteDesk 800', '10.0.22631', 'enrolled', NOW() - interval '16 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000046', '00000000-0000-0000-0000-000000000001', 'windows', 'win-046', 'BULK017', '[Seed] Server Room KVM', 'Dell OptiPlex 3000', '10.0.22631', 'enrolled', NOW() - interval '17 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000047', '00000000-0000-0000-0000-000000000001', 'android', 'and-047', 'BULK018', '[Seed] Delivery Phone 1', 'Google Pixel 7a', '14', 'enrolled', NOW() - interval '18 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000048', '00000000-0000-0000-0000-000000000001', 'android', 'and-048', 'BULK019', '[Seed] Delivery Phone 2', 'Google Pixel 7a', '14', 'enrolled', NOW() - interval '19 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000049', '00000000-0000-0000-0000-000000000001', 'android', 'and-049', 'BULK020', '[Seed] Delivery Phone 3', 'Google Pixel 7a', '14', 'enrolled', NOW() - interval '20 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000050', '00000000-0000-0000-0000-000000000001', 'windows', 'win-050', 'BULK021', '[Seed] Lab PC 1', 'HP ZBook Fury', '10.0.22631', 'enrolled', NOW() - interval '21 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000051', '00000000-0000-0000-0000-000000000001', 'windows', 'win-051', 'BULK022', '[Seed] Lab PC 2', 'HP ZBook Fury', '10.0.22631', 'enrolled', NOW() - interval '22 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000052', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-052', 'BULK023', '[Seed] QA Mac 1', 'Mac Mini M2', '14.4', 'enrolled', NOW() - interval '23 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000053', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-053', 'BULK024', '[Seed] QA Mac 2', 'Mac Mini M2', '14.4', 'enrolled', NOW() - interval '24 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000054', '00000000-0000-0000-0000-000000000001', 'windows', 'win-054', 'BULK025', '[Seed] Intern Laptop 1', 'Dell Latitude 3540', '10.0.22631', 'enrolled', NOW() - interval '25 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000055', '00000000-0000-0000-0000-000000000001', 'windows', 'win-055', 'BULK026', '[Seed] Intern Laptop 2', 'Dell Latitude 3540', '10.0.22631', 'enrolled', NOW() - interval '26 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000056', '00000000-0000-0000-0000-000000000001', 'android', 'and-056', 'BULK027', '[Seed] Security Phone 1', 'Samsung Galaxy XCover6', '13', 'enrolled', NOW() - interval '27 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000057', '00000000-0000-0000-0000-000000000001', 'android', 'and-057', 'BULK028', '[Seed] Security Phone 2', 'Samsung Galaxy XCover6', '13', 'enrolled', NOW() - interval '28 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000058', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-058', 'BULK029', '[Seed] Remote Mac 1', 'MacBook Pro 14"', '14.4', 'enrolled', NOW() - interval '29 hours', '{}'),
  ('d0000000-0000-0000-0000-000000000059', '00000000-0000-0000-0000-000000000001', 'macos', 'mac-059', 'BULK030', '[Seed] Remote Mac 2', 'MacBook Pro 14"', '14.4', 'enrolled', NOW() - interval '30 hours', '{}')
ON CONFLICT (enterprise_id, platform, device_id) DO UPDATE SET status = EXCLUDED.status, name = EXCLUDED.name, platform_data = EXCLUDED.platform_data;

-- Policies (6 enterprise policies + 2 templates)
INSERT INTO policies (id, enterprise_id, name, description, platform, policy_type, policy_config, is_active, is_template) VALUES
  ('e0000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', '[Seed] Corporate Security Baseline', 'Require encryption and strong passwords on all devices', 'all', 'security', '{"require_encryption": true, "min_password_length": 8, "require_firewall": true}', true, false),
  ('e0000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', '[Seed] Corporate WiFi', 'Auto-configure corporate WiFi on enrollment', 'all', 'wifi', '{"ssid": "AcmeCorp", "security": "WPA2-Enterprise", "auto_join": true}', true, false),
  ('e0000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000001', '[Seed] macOS Restrictions', 'Disable camera and AirDrop for macOS fleet', 'macos', 'restrictions', '{"disable_camera": true, "disable_airdrop": true}', true, false),
  ('e0000000-0000-0000-0000-000000000004', '00000000-0000-0000-0000-000000000001', '[Seed] Windows BitLocker', 'Enforce BitLocker encryption on Windows devices', 'windows', 'security', '{"require_encryption": true, "encryption_method": "XTS-AES-256"}', true, false),
  ('e0000000-0000-0000-0000-000000000005', '00000000-0000-0000-0000-000000000001', '[Seed] Android Work Profile', 'Configure work profile restrictions', 'android', 'restrictions', '{"disable_camera": false, "disable_screen_capture": true}', true, false),
  ('e0000000-0000-0000-0000-000000000006', '00000000-0000-0000-0000-000000000001', '[Seed] VPN Configuration', 'Corporate VPN for remote access', 'all', 'vpn', '{"server": "vpn.acme.test", "protocol": "IKEv2", "on_demand": true}', false, false),
  -- Templates
  ('e0000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000001', '[Seed] NIST Security Template', 'NIST 800-171 baseline security controls', 'all', 'security', '{"require_encryption": true, "min_password_length": 12, "require_firewall": true, "auto_lock_minutes": 5}', true, true),
  ('e0000000-0000-0000-0000-000000000011', '00000000-0000-0000-0000-000000000001', '[Seed] BYOD Restrictions Template', 'Minimal restrictions for BYOD devices', 'all', 'restrictions', '{"disable_camera": false, "require_passcode": true}', true, true)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description, policy_config = EXCLUDED.policy_config, is_active = EXCLUDED.is_active;

-- Device groups (3)
INSERT INTO device_groups (id, enterprise_id, name, description) VALUES
  ('f0000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', '[Seed] Engineering', 'Engineering team devices'),
  ('f0000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', '[Seed] Sales', 'Sales team devices'),
  ('f0000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000001', '[Seed] Executives', 'Executive team devices')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description;

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
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', 'policy.create', 'policy', 'e0000000-0000-0000-0000-000000000001', '{"name": "[Seed] Corporate Security Baseline"}', '10.0.1.50', NOW() - interval '14 days'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', 'policy.assign', 'policy', 'e0000000-0000-0000-0000-000000000001', '{"target_type": "enterprise", "target_id": "00000000-0000-0000-0000-000000000001"}', '10.0.1.50', NOW() - interval '14 days' + interval '2 minutes'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000002', 'device.enroll', 'device', 'd0000000-0000-0000-0000-000000000020', '{"platform": "android", "model": "Samsung Galaxy S24"}', '10.0.1.100', NOW() - interval '5 days'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', 'group.create', 'group', 'f0000000-0000-0000-0000-000000000001', '{"name": "[Seed] Engineering"}', '10.0.1.50', NOW() - interval '10 days'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', 'group.add_member', 'group', 'f0000000-0000-0000-0000-000000000001', '{"device_id": "d0000000-0000-0000-0000-000000000001"}', '10.0.1.50', NOW() - interval '10 days' + interval '1 minute'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', 'compliance.evaluate', 'device', 'd0000000-0000-0000-0000-000000000012', '{"result": "non_compliant", "violations": ["encryption_disabled"]}', '10.0.1.50', NOW() - interval '4 hours'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000002', 'device.lock', 'device', 'd0000000-0000-0000-0000-000000000024', '{"reason": "employee termination"}', '10.0.1.100', NOW() - interval '14 days'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', 'policy.update', 'policy', 'e0000000-0000-0000-0000-000000000001', '{"changes": ["min_password_length: 6 -> 8"]}', '10.0.1.50', NOW() - interval '3 days'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', 'user.login', 'user', 'b0000000-0000-0000-0000-000000000001', '{"method": "oidc"}', '10.0.1.50', NOW() - interval '1 hour'),
  ('00000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000002', 'user.login', 'user', 'b0000000-0000-0000-0000-000000000002', '{"method": "oidc"}', '10.0.1.100', NOW() - interval '2 hours')
;

-- Seed enterprise_id: 00000000-0000-0000-0000-000000000001
-- This MUST match the Keycloak admin user's enterprise_id attribute
-- (set in docker/keycloak/realm-export.json → users → attributes → enterprise_id).
-- If the dashboard shows no seed data after login, verify the JWT claim matches:
--   curl -s -X POST "http://localhost:8180/realms/localmdm/protocol/openid-connect/token" \
--     -d "grant_type=password&client_id=localmdm-api&client_secret=localmdm-dev-dashboard-secret-2026&username=admin&password=admin123" \
--     | python3 -c "import sys,json,base64; t=json.load(sys.stdin)['access_token'].split('.')[1]; t+='='*(4-len(t)%4); print(json.loads(base64.urlsafe_b64decode(t)).get('enterprise_id'))"
-- Expected output: 00000000-0000-0000-0000-000000000001

-- Verify seed enterprise exists and has expected data
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM enterprises WHERE id = '00000000-0000-0000-0000-000000000001') THEN
    RAISE WARNING 'SEED ERROR: Enterprise 00000000-0000-0000-0000-000000000001 does not exist!';
  END IF;
  IF (SELECT COUNT(*) FROM devices WHERE enterprise_id = '00000000-0000-0000-0000-000000000001' AND deleted_at IS NULL) = 0 THEN
    RAISE WARNING 'SEED ERROR: No active devices found for seed enterprise!';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM enterprises WHERE id = '99999999-9999-9999-9999-999999999999') THEN
    RAISE WARNING 'SEED ERROR: Test enterprise 99999999-9999-9999-9999-999999999999 does not exist!';
  END IF;
END $$;

COMMIT;
