-- Restore placeholder for NULL password hashes
UPDATE users SET password_hash = 'oidc-managed' WHERE password_hash IS NULL;

ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
