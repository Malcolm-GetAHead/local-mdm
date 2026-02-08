-- Rollback audit log index optimization

DROP INDEX IF EXISTS idx_audit_logs_created_at_desc;
