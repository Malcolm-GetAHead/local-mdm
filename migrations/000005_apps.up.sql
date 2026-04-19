CREATE TABLE IF NOT EXISTS apps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    enterprise_id UUID NOT NULL REFERENCES enterprises(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    platform VARCHAR(20) NOT NULL,
    identifier VARCHAR(500) NOT NULL,
    version VARCHAR(100) DEFAULT '',
    install_type VARCHAR(20) NOT NULL DEFAULT 'required',
    app_config JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(enterprise_id, platform, identifier)
);

CREATE INDEX idx_apps_enterprise_id ON apps(enterprise_id);
CREATE INDEX idx_apps_platform ON apps(platform);

CREATE TRIGGER update_apps_updated_at
    BEFORE UPDATE ON apps
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
