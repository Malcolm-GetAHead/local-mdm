-- Sprint 5b: EventBus triggers — new triggers + fix notify_mdm_event for DELETE
-- Includes extra context fields for policy_assignments and group_memberships

-- 1. Replace notify_mdm_event to handle DELETE (use OLD instead of NEW)
--    and support tables without an id column (group_memberships uses composite PK).
--    Includes table-specific extra fields so subscribers can resolve affected devices.
CREATE OR REPLACE FUNCTION notify_mdm_event() RETURNS TRIGGER AS $$
DECLARE
    payload TEXT;
    event_type TEXT;
    entity_id UUID;
    device_id UUID;
    extra JSONB;
    rec RECORD;
BEGIN
    event_type := TG_ARGV[0];

    -- Use OLD for DELETE, NEW for INSERT/UPDATE
    IF TG_OP = 'DELETE' THEN
        rec := OLD;
    ELSE
        rec := NEW;
    END IF;

    extra := '{}'::jsonb;

    -- Determine IDs and extra context based on table
    CASE TG_TABLE_NAME
        WHEN 'devices' THEN
            entity_id := rec.id;
            device_id := rec.id;
        WHEN 'policies' THEN
            entity_id := rec.id;
            device_id := NULL;
        WHEN 'device_commands' THEN
            entity_id := rec.id;
            device_id := rec.device_id;
        WHEN 'policy_assignments' THEN
            entity_id := rec.id;
            device_id := NULL;
            extra := json_build_object(
                'policy_id', rec.policy_id,
                'target_type', rec.target_type,
                'target_id', rec.target_id
            )::jsonb;
        WHEN 'compliance_results' THEN
            entity_id := rec.id;
            device_id := rec.device_id;
        WHEN 'group_memberships' THEN
            entity_id := rec.device_id;
            device_id := rec.device_id;
            extra := json_build_object(
                'group_id', rec.group_id
            )::jsonb;
        ELSE
            entity_id := rec.id;
            device_id := NULL;
    END CASE;

    payload := json_build_object(
        'type', event_type,
        'id', entity_id,
        'device_id', device_id,
        'table', TG_TABLE_NAME,
        'op', TG_OP,
        'extra', extra
    )::text;

    PERFORM pg_notify('mdm_events', payload);

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 2. New triggers

-- platform_data changes (device reports new state during check-in)
CREATE TRIGGER device_info_updated_event
    AFTER UPDATE OF platform_data ON devices
    FOR EACH ROW
    WHEN (OLD.platform_data IS DISTINCT FROM NEW.platform_data)
    EXECUTE FUNCTION notify_mdm_event('device.info_updated');

-- Policy unassignment
CREATE TRIGGER policy_unassigned_event
    AFTER DELETE ON policy_assignments
    FOR EACH ROW
    EXECUTE FUNCTION notify_mdm_event('policy.unassigned');

-- Group membership changes
CREATE TRIGGER group_member_added_event
    AFTER INSERT ON group_memberships
    FOR EACH ROW
    EXECUTE FUNCTION notify_mdm_event('group.member_added');

CREATE TRIGGER group_member_removed_event
    AFTER DELETE ON group_memberships
    FOR EACH ROW
    EXECUTE FUNCTION notify_mdm_event('group.member_removed');
