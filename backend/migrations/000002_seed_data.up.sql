-- FinOpsMind Seed Data
-- Migration: 000002_seed_data.up.sql

-- Insert sample cost data (last 60 days)
DO $$
DECLARE
    org_id UUID := '00000000-0000-0000-0000-000000000001';
    day_offset INTEGER;
    base_amount DECIMAL;
    variation DECIMAL;
BEGIN
    FOR day_offset IN 0..59 LOOP
        -- EC2 costs
        base_amount := 150 + (random() * 50);
        INSERT INTO costs (organization_id, date, amount, currency, provider, service, account_id, region)
        VALUES (org_id, CURRENT_DATE - day_offset, base_amount, 'USD', 'aws', 'Amazon EC2', '123456789012', 'us-east-1');
        
        -- S3 costs
        base_amount := 45 + (random() * 15);
        INSERT INTO costs (organization_id, date, amount, currency, provider, service, account_id, region)
        VALUES (org_id, CURRENT_DATE - day_offset, base_amount, 'USD', 'aws', 'Amazon S3', '123456789012', 'us-east-1');
        
        -- RDS costs
        base_amount := 80 + (random() * 20);
        INSERT INTO costs (organization_id, date, amount, currency, provider, service, account_id, region)
        VALUES (org_id, CURRENT_DATE - day_offset, base_amount, 'USD', 'aws', 'Amazon RDS', '123456789012', 'us-east-1');
        
        -- Lambda costs
        base_amount := 25 + (random() * 10);
        INSERT INTO costs (organization_id, date, amount, currency, provider, service, account_id, region)
        VALUES (org_id, CURRENT_DATE - day_offset, base_amount, 'USD', 'aws', 'AWS Lambda', '123456789012', 'us-east-1');
        
        -- CloudFront costs
        base_amount := 35 + (random() * 12);
        INSERT INTO costs (organization_id, date, amount, currency, provider, service, account_id, region)
        VALUES (org_id, CURRENT_DATE - day_offset, base_amount, 'USD', 'aws', 'Amazon CloudFront', '123456789012', 'us-east-1');
    END LOOP;
END $$;

-- Insert sample anomalies
INSERT INTO anomalies (organization_id, date, actual_amount, expected_amount, deviation, deviation_pct, score, severity, status, service, account_id, region) VALUES
('00000000-0000-0000-0000-000000000001', CURRENT_DATE - 3, 450.00, 180.00, 270.00, 150.00, 0.95, 'critical', 'open', 'Amazon EC2', '123456789012', 'us-east-1'),
('00000000-0000-0000-0000-000000000001', CURRENT_DATE - 5, 95.00, 50.00, 45.00, 90.00, 0.85, 'high', 'open', 'Amazon S3', '123456789012', 'us-east-1'),
('00000000-0000-0000-0000-000000000001', CURRENT_DATE - 7, 120.00, 85.00, 35.00, 41.18, 0.72, 'medium', 'acknowledged', 'Amazon RDS', '123456789012', 'us-east-1'),
('00000000-0000-0000-0000-000000000001', CURRENT_DATE - 10, 55.00, 45.00, 10.00, 22.22, 0.58, 'low', 'resolved', 'AWS Lambda', '123456789012', 'us-east-1');

-- Insert sample budgets
INSERT INTO budgets (organization_id, name, amount, currency, period, status, current_spend, forecasted_spend, filters) VALUES
('00000000-0000-0000-0000-000000000001', 'Monthly AWS Budget', 15000.00, 'USD', 'monthly', 'active', 9500.00, 14200.00, '{"providers": ["aws"]}'),
('00000000-0000-0000-0000-000000000001', 'EC2 Budget', 5000.00, 'USD', 'monthly', 'warning', 4200.00, 5800.00, '{"services": ["Amazon EC2"]}'),
('00000000-0000-0000-0000-000000000001', 'Development Environment', 3000.00, 'USD', 'monthly', 'active', 1800.00, 2700.00, '{"tags": {"environment": "dev"}}');

-- Insert sample recommendations
INSERT INTO recommendations (organization_id, type, provider, account_id, region, resource_id, resource_type, current_config, recommended_config, estimated_savings, estimated_savings_pct, impact, effort, risk, status) VALUES
('00000000-0000-0000-0000-000000000001', 'rightsizing', 'aws', '123456789012', 'us-east-1', 'i-0abc123def456', 'EC2 Instance', 'm5.2xlarge', 'm5.xlarge', 1200.00, 50.00, 'high', 'low', 'low', 'pending'),
('00000000-0000-0000-0000-000000000001', 'idle_resources', 'aws', '123456789012', 'us-east-1', 'i-0def789ghi012', 'EC2 Instance', 't3.large (idle)', 'Terminate', 350.00, 100.00, 'medium', 'low', 'medium', 'pending'),
('00000000-0000-0000-0000-000000000001', 'reserved_instances', 'aws', '123456789012', 'us-east-1', NULL, 'EC2 Reserved Instance', 'On-Demand', '1-Year No Upfront RI', 3600.00, 35.00, 'high', 'medium', 'low', 'pending'),
('00000000-0000-0000-0000-000000000001', 'storage_optimization', 'aws', '123456789012', 'us-east-1', 'vol-0123456789abcdef', 'EBS Volume', 'gp2 500GB', 'gp3 500GB', 240.00, 20.00, 'low', 'low', 'low', 'pending'),
('00000000-0000-0000-0000-000000000001', 'savings_plans', 'aws', '123456789012', 'us-east-1', NULL, 'Compute Savings Plan', 'On-Demand', '$50/hr Compute SP', 5400.00, 30.00, 'high', 'medium', 'low', 'pending');

-- Insert sample forecast
INSERT INTO forecasts (organization_id, model_version, granularity, predictions, total_forecasted, confidence_level) VALUES
	('00000000-0000-0000-0000-000000000001', 'prophet-1.0', 'daily', 
	'[{"date": "2026-01-12", "predicted": 380.50, "lower_bound": 350.00, "upper_bound": 410.00}]'::jsonb, 11500.00, 0.85);
