-- Revert Sprint 5b EventBus triggers
DROP TRIGGER IF EXISTS device_info_updated_event ON devices;
DROP TRIGGER IF EXISTS policy_unassigned_event ON policy_assignments;
DROP TRIGGER IF EXISTS group_member_added_event ON group_memberships;
DROP TRIGGER IF EXISTS group_member_removed_event ON group_memberships;

-- Restore original notify_mdm_event (without DELETE support)
CREATE OR REPLACE FUNCTION notify_mdm_event() RETURNS TRIGGER AS $$
DECLARE
    payload TEXT;
    event_type TEXT;
    entity_id UUID;
    device_id UUID;
BEGIN
    event_type := TG_ARGV[0];

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
