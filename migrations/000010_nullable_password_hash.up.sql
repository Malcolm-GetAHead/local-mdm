-- Make password_hash nullable (auth is via Keycloak OIDC, not local passwords)
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

-- Clear the misleading "oidc-managed" placeholder
UPDATE users SET password_hash = NULL WHERE password_hash = 'oidc-managed';
