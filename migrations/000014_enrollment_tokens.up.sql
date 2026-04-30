-- Enrollment tokens: short-lived, limited-use codes that authorize device enrollment
CREATE TABLE IF NOT EXISTS enrollment_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    enterprise_id UUID NOT NULL REFERENCES enterprises(id) ON DELETE CASCADE,
    token VARCHAR(64) NOT NULL UNIQUE,
    description TEXT,
    max_uses INTEGER,
    uses_remaining INTEGER,
    expires_at TIMESTAMPTZ NOT NULL,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_enrollment_tokens_enterprise ON enrollment_tokens(enterprise_id);
CREATE INDEX idx_enrollment_tokens_token ON enrollment_tokens(token) WHERE revoked_at IS NULL;
CREATE INDEX idx_enrollment_tokens_expires ON enrollment_tokens(expires_at) WHERE revoked_at IS NULL;
