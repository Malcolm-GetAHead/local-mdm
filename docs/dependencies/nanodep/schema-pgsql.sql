-- NanoDEP PostgreSQL Schema
-- Source: https://github.com/micromdm/nanodep/blob/main/storage/pgsql/schema.sql

CREATE TABLE dep_names (
    name                     VARCHAR(255) NOT NULL,
    -- OAuth1 Tokens
    consumer_key             TEXT NULL,
    consumer_secret          TEXT NULL,
    access_token             TEXT NULL,
    access_secret            TEXT NULL,
    access_token_expiry      TIMESTAMPTZ NULL,
    -- Config
    config_base_url          VARCHAR(255) NULL,
    -- Token PKI
    tokenpki_cert_pem        TEXT NULL,
    tokenpki_key_pem         TEXT NULL,
    tokenpki_staging_cert_pem TEXT NULL,
    tokenpki_staging_key_pem  TEXT NULL,
    -- Syncer
    syncer_cursor            VARCHAR(1024) NULL,
    -- Assigner
    assigner_profile_uuid    TEXT NULL,
    assigner_profile_uuid_at TIMESTAMPTZ NULL,
    created_at               TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at               TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (name)
);

CREATE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_updated_at_on_change
    BEFORE UPDATE ON dep_names
    FOR EACH ROW EXECUTE PROCEDURE update_updated_at();
