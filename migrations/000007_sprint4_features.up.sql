-- Sprint 4: token cache, idempotency keys, policy versions, event triggers

-- 1. Token cache (replaces Redis token cache)
CREATE TABLE token_cache (
    token_hash VARCHAR(64) PRIMARY KEY,  -- SHA-256 hex of the token
    user_data JSONB NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);
CREATE INDEX idx_token_cache_expires_at ON token_cache(expires_at);

-- 2. Idempotency keys (Idempotency-Key header support)
CREATE TABLE idempotency_keys (
    key VARCHAR(255) NOT NULL,
    method VARCHAR(10) NOT NULL,
    path VARCHAR(500) NOT NULL,
    status_code INT NOT NULL,
    response_headers JSONB DEFAULT '{}',
    response_body BYTEA,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    PRIMARY KEY (key)
);
CREATE INDEX idx_idempotency_keys_expires_at ON idempotency_keys(expires_at);

-- 3. Policy versions (full snapshots for rollback)
CREATE TABLE policy_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    policy_id UUID NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    version INT NOT NULL,
    policy_config JSONB NOT NULL,
    name VARCHAR(255) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    platform VARCHAR(20) NOT NULL DEFAULT '',
    policy_type VARCHAR(50) NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_by VARCHAR(255) DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(policy_id, version)
);
CREATE INDEX idx_policy_versions_policy_id ON policy_versions(policy_id);

-- 4. Event notification trigger function (reusable)
CREATE OR REPLACE FUNCTION notify_mdm_event() RETURNS TRIGGER AS $$
DECLARE
    payload TEXT;
    event_type TEXT;
    entity_id UUID;
    device_id UUID;
BEGIN
    event_type := TG_ARGV[0];

    -- Determine IDs based on table
    CASE TG_TABLE_NAME
        WHEN 'devices' THEN
            entity_id := NEW.id;
            device_id := NEW.id;
        WHEN 'policies' THEN
            entity_id := NEW.id;
            device_id := NULL;
        WHEN 'device_commands' THEN
            entity_id := NEW.id;
            device_id := NEW.device_id;
        WHEN 'policy_assignments' THEN
            entity_id := NEW.id;
            device_id := NULL;
        WHEN 'compliance_results' THEN
            entity_id := NEW.id;
            device_id := NEW.device_id;
        ELSE
            entity_id := NEW.id;
            device_id := NULL;
    END CASE;

    payload := json_build_object(
        'type', event_type,
        'id', entity_id,
        'device_id', device_id,
        'table', TG_TABLE_NAME,
        'op', TG_OP
    )::text;

    PERFORM pg_notify('mdm_events', payload);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 5. Attach event triggers

-- Device events
CREATE TRIGGER device_enrolled_event
    AFTER INSERT ON devices
    FOR EACH ROW EXECUTE FUNCTION notify_mdm_event('device.enrolled');

CREATE TRIGGER device_updated_event
    AFTER UPDATE OF status ON devices
    FOR EACH ROW
    WHEN (OLD.status IS DISTINCT FROM NEW.status)
    EXECUTE FUNCTION notify_mdm_event('device.status_changed');

-- Policy events
CREATE TRIGGER policy_updated_event
    AFTER UPDATE ON policies
    FOR EACH ROW EXECUTE FUNCTION notify_mdm_event('policy.updated');

-- Command events
CREATE TRIGGER command_created_event
    AFTER INSERT ON device_commands
    FOR EACH ROW EXECUTE FUNCTION notify_mdm_event('command.created');

-- Policy assignment events
CREATE TRIGGER policy_assigned_event
    AFTER INSERT ON policy_assignments
    FOR EACH ROW EXECUTE FUNCTION notify_mdm_event('policy.assigned');

-- Compliance result events
CREATE TRIGGER compliance_evaluated_event
    AFTER INSERT OR UPDATE ON compliance_results
    FOR EACH ROW EXECUTE FUNCTION notify_mdm_event('compliance.evaluated');
