-- Sprint 4 schema prep: groups, policy assignments, compliance, command/app enhancements

-- ============================================================
-- 1. Device groups (S4-02: static device groups)
-- ============================================================
CREATE TABLE device_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    enterprise_id UUID NOT NULL REFERENCES enterprises(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(enterprise_id, name)
);

CREATE INDEX idx_device_groups_enterprise_id ON device_groups(enterprise_id);

CREATE TRIGGER update_device_groups_updated_at
    BEFORE UPDATE ON device_groups
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================
-- 2. Group memberships (S4-02: device-to-group mapping)
-- ============================================================
CREATE TABLE group_memberships (
    group_id UUID NOT NULL REFERENCES device_groups(id) ON DELETE CASCADE,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    added_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (group_id, device_id)
);

CREATE INDEX idx_group_memberships_device_id ON group_memberships(device_id);

-- ============================================================
-- 3. Policy assignments (S4-02: replaces device_policies for
--    flexible policy targeting — device, group, or enterprise-wide)
-- ============================================================
CREATE TABLE policy_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    policy_id UUID NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    target_type VARCHAR(20) NOT NULL, -- 'device', 'group', 'enterprise'
    target_id UUID NOT NULL,          -- device_id, group_id, or enterprise_id
    priority INT NOT NULL DEFAULT 0,  -- higher = takes precedence on conflict
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(policy_id, target_type, target_id)
);

CREATE INDEX idx_policy_assignments_policy_id ON policy_assignments(policy_id);
CREATE INDEX idx_policy_assignments_target ON policy_assignments(target_type, target_id);

-- ============================================================
-- 4. Compliance results (S4-03: per-device compliance state)
-- ============================================================
CREATE TABLE compliance_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    policy_id UUID NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'unknown', -- compliant, non_compliant, unknown, error
    details JSONB DEFAULT '{}',
    evaluated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(device_id, policy_id)
);

CREATE INDEX idx_compliance_results_device_id ON compliance_results(device_id);
CREATE INDEX idx_compliance_results_status ON compliance_results(status);

-- ============================================================
-- 5. Device commands enhancements (Sprint 5 prep)
-- ============================================================
ALTER TABLE device_commands
    ADD COLUMN enterprise_id UUID REFERENCES enterprises(id) ON DELETE CASCADE,
    ADD COLUMN expires_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN batch_id UUID;

-- Backfill enterprise_id from devices table
UPDATE device_commands dc
    SET enterprise_id = d.enterprise_id
    FROM devices d
    WHERE dc.device_id = d.id AND dc.enterprise_id IS NULL;

CREATE INDEX idx_device_commands_enterprise_id ON device_commands(enterprise_id);
CREATE INDEX idx_device_commands_batch_id ON device_commands(batch_id) WHERE batch_id IS NOT NULL;

-- ============================================================
-- 6. Policies enhancement (S4-01: template support)
-- ============================================================
ALTER TABLE policies
    ADD COLUMN is_template BOOLEAN NOT NULL DEFAULT false;

-- ============================================================
-- 7. Device apps tracking (S5 reporting, S4-03 compliance)
-- ============================================================
CREATE TABLE device_apps (
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    installed_version VARCHAR(100) DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, installed, failed, removed
    installed_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (device_id, app_id)
);

CREATE INDEX idx_device_apps_app_id ON device_apps(app_id);
CREATE INDEX idx_device_apps_status ON device_apps(status);

CREATE TRIGGER update_device_apps_updated_at
    BEFORE UPDATE ON device_apps
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
