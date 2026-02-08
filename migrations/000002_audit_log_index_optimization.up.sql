-- Add optimized index for audit log queries by date range
-- Most queries fetch recent logs first (ORDER BY created_at DESC)

CREATE INDEX idx_audit_logs_created_at_desc ON audit_logs(created_at DESC);
