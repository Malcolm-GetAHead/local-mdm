DROP INDEX IF EXISTS idx_enrollment_tokens_status;
ALTER TABLE enrollment_tokens DROP COLUMN IF EXISTS status;
