-- Reverse Sprint 4 features migration

-- Drop triggers
DROP TRIGGER IF EXISTS compliance_evaluated_event ON compliance_results;
DROP TRIGGER IF EXISTS policy_assigned_event ON policy_assignments;
DROP TRIGGER IF EXISTS command_created_event ON device_commands;
DROP TRIGGER IF EXISTS policy_updated_event ON policies;
DROP TRIGGER IF EXISTS device_updated_event ON devices;
DROP TRIGGER IF EXISTS device_enrolled_event ON devices;

-- Drop trigger function
DROP FUNCTION IF EXISTS notify_mdm_event();

-- Drop tables
DROP TABLE IF EXISTS policy_versions;
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS token_cache;
