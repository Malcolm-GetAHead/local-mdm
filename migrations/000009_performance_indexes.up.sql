-- Performance indexes (S5-07)

-- Composite indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_devices_enterprise_status ON devices(enterprise_id, status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_devices_enterprise_platform ON devices(enterprise_id, platform) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_policies_enterprise_active ON policies(enterprise_id, is_active) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_compliance_device_policy ON compliance_results(device_id, policy_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_enterprise_created ON audit_logs(enterprise_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_commands_device_status ON device_commands(device_id, status);
CREATE INDEX IF NOT EXISTS idx_group_members_device ON group_memberships(device_id);
