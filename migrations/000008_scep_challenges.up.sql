-- SCEP challenge storage (moved from in-memory to PostgreSQL for multi-instance safety)
CREATE TABLE scep_challenges (
    password VARCHAR(64) PRIMARY KEY,
    device_id VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_scep_challenges_expires ON scep_challenges(expires_at);
