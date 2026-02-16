-- Migration: 004_ml_tables.sql
-- Description: Add tables for ML forecasts and anomalies

-- Cost forecasts table
CREATE TABLE IF NOT EXISTS cost_forecasts (
    id BIGSERIAL PRIMARY KEY,
    account_id VARCHAR(64) NOT NULL,
    generated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    forecast_start_date DATE NOT NULL,
    forecast_end_date DATE NOT NULL,
    predictions JSONB NOT NULL,
    summary JSONB NOT NULL,
    total_predicted_cost DECIMAL(15, 2) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Index for querying latest forecast by account
CREATE INDEX idx_cost_forecasts_account_generated 
ON cost_forecasts(account_id, generated_at DESC);

-- Cost anomalies table
CREATE TABLE IF NOT EXISTS cost_anomalies (
    id BIGSERIAL PRIMARY KEY,
    account_id VARCHAR(64) NOT NULL,
    date DATE NOT NULL,
    cost DECIMAL(15, 2) NOT NULL,
    anomaly_score DECIMAL(5, 4) NOT NULL,
    severity VARCHAR(20) NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    root_cause JSONB,
    acknowledged BOOLEAN NOT NULL DEFAULT FALSE,
    acknowledged_by VARCHAR(255),
    acknowledged_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    -- Prevent duplicate anomalies for same account/date
    UNIQUE(account_id, date)
);

-- Index for querying anomalies by account and date
CREATE INDEX idx_cost_anomalies_account_date 
ON cost_anomalies(account_id, date DESC);

-- Index for unacknowledged anomalies
CREATE INDEX idx_cost_anomalies_unacknowledged 
ON cost_anomalies(account_id, acknowledged) 
WHERE acknowledged = FALSE;

-- Daily costs table (if not exists from previous migration)
CREATE TABLE IF NOT EXISTS daily_costs (
    id BIGSERIAL PRIMARY KEY,
    account_id VARCHAR(64) NOT NULL,
    date DATE NOT NULL,
    total_cost DECIMAL(15, 2) NOT NULL,
    service_breakdown JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    UNIQUE(account_id, date)
);

CREATE INDEX IF NOT EXISTS idx_daily_costs_account_date 
ON daily_costs(account_id, date DESC);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger for daily_costs updated_at
DROP TRIGGER IF EXISTS update_daily_costs_updated_at ON daily_costs;
CREATE TRIGGER update_daily_costs_updated_at
    BEFORE UPDATE ON daily_costs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
