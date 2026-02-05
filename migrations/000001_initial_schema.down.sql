-- Drop all triggers
DROP TRIGGER IF EXISTS update_api_tokens_updated_at ON api_tokens;
DROP TRIGGER IF EXISTS update_certificates_updated_at ON certificates;
DROP TRIGGER IF EXISTS update_device_policies_updated_at ON device_policies;
DROP TRIGGER IF EXISTS update_policies_updated_at ON policies;
DROP TRIGGER IF EXISTS update_devices_updated_at ON devices;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP TRIGGER IF EXISTS update_enterprises_updated_at ON enterprises;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables in reverse order (respecting foreign keys)
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS api_tokens;
DROP TABLE IF EXISTS certificates;
DROP TABLE IF EXISTS device_policies;
DROP TABLE IF EXISTS policies;
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS enterprises;

-- Drop extension
DROP EXTENSION IF EXISTS "pgcrypto";
