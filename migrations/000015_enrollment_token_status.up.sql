-- Add status column to enrollment_tokens for explicit state tracking
ALTER TABLE enrollment_tokens ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'active';

-- Set status for existing rows based on current state
UPDATE enrollment_tokens SET status = 'revoked' WHERE revoked_at IS NOT NULL;
UPDATE enrollment_tokens SET status = 'expired' WHERE revoked_at IS NULL AND expires_at < NOW();

-- Index for the periodic cleanup query
CREATE INDEX idx_enrollment_tokens_status ON enrollment_tokens(status) WHERE status = 'active';
