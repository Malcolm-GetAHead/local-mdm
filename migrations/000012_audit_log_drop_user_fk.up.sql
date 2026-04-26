-- Drop FK on audit_logs.user_id to allow logging actions from
-- OIDC users who aren't in the users table (e.g. Keycloak admin).
ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS audit_logs_user_id_fkey;
