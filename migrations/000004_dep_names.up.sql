-- DEP names table (based on nanoDEP schema with pgcrypto encryption for OAuth tokens)
-- OAuth token columns are encrypted at rest using pgp_sym_encrypt

CREATE TABLE IF NOT EXISTS dep_names (
    name                      VARCHAR(255) NOT NULL PRIMARY KEY,
    -- OAuth1 Tokens (encrypted with pgp_sym_encrypt)
    consumer_key              BYTEA NULL,
    consumer_secret           BYTEA NULL,
    access_token              BYTEA NULL,
    access_secret             BYTEA NULL,
    access_token_expiry       TIMESTAMPTZ NULL,
    -- Config
    config_base_url           VARCHAR(255) NULL,
    -- Token PKI (encrypted — contains private keys)
    tokenpki_cert_pem         TEXT NULL,
    tokenpki_key_pem          BYTEA NULL,
    tokenpki_staging_cert_pem TEXT NULL,
    tokenpki_staging_key_pem  BYTEA NULL,
    -- Syncer
    syncer_cursor             VARCHAR(1024) NULL,
    -- Assigner
    assigner_profile_uuid     TEXT NULL,
    assigner_profile_uuid_at  TIMESTAMPTZ NULL,
    created_at                TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at                TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER update_dep_names_updated_at
    BEFORE UPDATE ON dep_names
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- DEP devices table for tracking synced devices
CREATE TABLE IF NOT EXISTS dep_devices (
    serial_number   VARCHAR(255) NOT NULL,
    dep_name        VARCHAR(255) NOT NULL REFERENCES dep_names(name) ON DELETE CASCADE,
    profile_uuid    TEXT NULL,
    profile_status  VARCHAR(50) DEFAULT 'empty',
    device_data     JSONB DEFAULT '{}'::jsonb,
    synced_at       TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    assigned_at     TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (serial_number, dep_name)
);

CREATE INDEX idx_dep_devices_dep_name ON dep_devices(dep_name);
CREATE INDEX idx_dep_devices_profile_status ON dep_devices(profile_status);

CREATE TRIGGER update_dep_devices_updated_at
    BEFORE UPDATE ON dep_devices
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
