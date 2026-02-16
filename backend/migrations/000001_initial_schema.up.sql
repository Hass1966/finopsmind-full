-- FinOpsMind Database Schema
-- Migration: 000001_initial_schema

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Organizations
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Costs
CREATE TABLE costs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    amount DECIMAL(15, 4) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    provider VARCHAR(20) NOT NULL,
    service VARCHAR(255) NOT NULL,
    account_id VARCHAR(100),
    region VARCHAR(50),
    resource_id VARCHAR(255),
    tags JSONB DEFAULT '{}',
    estimated BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(organization_id, date, provider, service, account_id, region, resource_id)
);

CREATE INDEX idx_costs_org_date ON costs(organization_id, date);
CREATE INDEX idx_costs_provider ON costs(provider);
CREATE INDEX idx_costs_service ON costs(service);

-- Budgets
CREATE TABLE budgets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    amount DECIMAL(15, 2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    period VARCHAR(20) NOT NULL,
    filters JSONB DEFAULT '{}',
    thresholds JSONB DEFAULT '[]',
    status VARCHAR(20) DEFAULT 'active',
    current_spend DECIMAL(15, 2) DEFAULT 0,
    forecasted_spend DECIMAL(15, 2) DEFAULT 0,
    start_date DATE,
    end_date DATE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Anomalies
CREATE TABLE anomalies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    actual_amount DECIMAL(15, 4) NOT NULL,
    expected_amount DECIMAL(15, 4) NOT NULL,
    deviation DECIMAL(15, 4) NOT NULL,
    deviation_pct DECIMAL(10, 2) NOT NULL,
    score DECIMAL(5, 4) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    status VARCHAR(20) DEFAULT 'open',
    provider VARCHAR(20),
    service VARCHAR(255),
    account_id VARCHAR(100),
    region VARCHAR(50),
    root_cause TEXT,
    notes TEXT,
    detected_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    acknowledged_at TIMESTAMP WITH TIME ZONE,
    acknowledged_by VARCHAR(255),
    resolved_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_anomalies_org ON anomalies(organization_id);
CREATE INDEX idx_anomalies_severity ON anomalies(severity);
CREATE INDEX idx_anomalies_status ON anomalies(status);

-- Recommendations
CREATE TABLE recommendations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    provider VARCHAR(20) NOT NULL,
    account_id VARCHAR(100),
    region VARCHAR(50),
    resource_id VARCHAR(255),
    resource_type VARCHAR(100),
    current_config TEXT,
    recommended_config TEXT,
    estimated_savings DECIMAL(15, 2) NOT NULL,
    estimated_savings_pct DECIMAL(5, 2),
    currency VARCHAR(3) DEFAULT 'USD',
    impact VARCHAR(20),
    effort VARCHAR(20),
    risk VARCHAR(20),
    status VARCHAR(20) DEFAULT 'pending',
    details JSONB DEFAULT '{}',
    notes TEXT,
    implemented_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_recommendations_org ON recommendations(organization_id);
CREATE INDEX idx_recommendations_status ON recommendations(status);

-- Forecasts
CREATE TABLE forecasts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    generated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    model_version VARCHAR(50),
    granularity VARCHAR(20) DEFAULT 'daily',
    predictions JSONB NOT NULL,
    total_forecasted DECIMAL(15, 2),
    confidence_level DECIMAL(5, 4),
    currency VARCHAR(3) DEFAULT 'USD',
    service_filter VARCHAR(255),
    account_filter VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Alerts
CREATE TABLE alerts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    status VARCHAR(20) DEFAULT 'open',
    title VARCHAR(255) NOT NULL,
    message TEXT,
    resource_type VARCHAR(100),
    resource_id VARCHAR(255),
    metadata JSONB DEFAULT '{}',
    triggered_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    acknowledged_at TIMESTAMP WITH TIME ZONE,
    acknowledged_by VARCHAR(255),
    resolved_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Default organization
INSERT INTO organizations (id, name, settings) VALUES 
('00000000-0000-0000-0000-000000000001', 'Default Organization', '{"default_currency": "USD", "timezone": "UTC"}');

-- Updated_at trigger
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_organizations_updated_at BEFORE UPDATE ON organizations FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_budgets_updated_at BEFORE UPDATE ON budgets FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_anomalies_updated_at BEFORE UPDATE ON anomalies FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_recommendations_updated_at BEFORE UPDATE ON recommendations FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
