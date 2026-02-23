CREATE TABLE cloud_providers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    provider_type VARCHAR(20) NOT NULL,
    name VARCHAR(255) NOT NULL DEFAULT '',
    credentials BYTEA NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    status VARCHAR(20) DEFAULT 'pending',
    status_message TEXT DEFAULT '',
    last_sync_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(organization_id, provider_type)
);

CREATE INDEX idx_cloud_providers_org ON cloud_providers(organization_id);

CREATE TRIGGER update_cloud_providers_updated_at BEFORE UPDATE ON cloud_providers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
