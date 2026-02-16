-- FinOpsMind Mock Data Seeder (Fixed for actual schema)
-- Run with: docker exec -i finopsmind-postgres psql -U finopsmind -d finopsmind < seed_mock_data.sql

-- First, create an organization if none exists
INSERT INTO organizations (id, name, slug, settings, created_at, updated_at)
VALUES (
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'Demo Organization',
    'demo-org',
    '{"currency": "USD", "timezone": "Europe/London"}',
    NOW(),
    NOW()
) ON CONFLICT (id) DO NOTHING;

-- Clear existing mock data
DELETE FROM recommendations WHERE organization_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
DELETE FROM anomalies WHERE organization_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
DELETE FROM costs WHERE organization_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
DELETE FROM forecasts WHERE organization_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';

-- Insert cost history (last 90 days)
DO $$
DECLARE
    i INTEGER;
    base_cost DECIMAL := 450;
    daily_cost DECIMAL;
    variance DECIMAL;
    trend DECIMAL;
    day_date DATE;
BEGIN
    FOR i IN 0..89 LOOP
        day_date := CURRENT_DATE - i;
        trend := 1 + (90 - i) / 90.0 * 0.12;
        variance := 0.9 + random() * 0.2;
        
        IF EXTRACT(DOW FROM day_date) IN (0, 6) THEN
            variance := variance * 0.75;
        END IF;
        
        daily_cost := base_cost * trend * variance;
        
        INSERT INTO costs (id, organization_id, date, provider, account_id, service, region, amount, currency, created_at)
        VALUES 
            (gen_random_uuid(), 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', day_date, 'aws', '123456789012', 'EC2', 'eu-west-2', 
             ROUND((daily_cost * 0.52)::numeric, 2), 'USD', NOW()),
            (gen_random_uuid(), 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', day_date, 'aws', '123456789012', 'RDS', 'eu-west-2', 
             ROUND((daily_cost * 0.28)::numeric, 2), 'USD', NOW()),
            (gen_random_uuid(), 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', day_date, 'aws', '123456789012', 'S3', 'eu-west-2', 
             ROUND((daily_cost * 0.08)::numeric, 2), 'USD', NOW()),
            (gen_random_uuid(), 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', day_date, 'aws', '123456789012', 'Lambda', 'eu-west-2', 
             ROUND((daily_cost * 0.07)::numeric, 2), 'USD', NOW()),
            (gen_random_uuid(), 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', day_date, 'aws', '123456789012', 'CloudWatch', 'eu-west-2', 
             ROUND((daily_cost * 0.05)::numeric, 2), 'USD', NOW());
    END LOOP;
END $$;

-- Insert recommendations
INSERT INTO recommendations (id, organization_id, type, provider, account_id, region, resource_id, resource_type, current_config, recommended_config, estimated_savings, estimated_savings_pct, impact, effort, risk, status, details, created_at) VALUES
(gen_random_uuid(), 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'rightsizing', 'aws', '123456789012', 'eu-west-2', 
 'i-0a1b2c3d4e5f6789a', 'EC2', 'm5.xlarge', 'm5.large',
 70.08, 50.0, 'high', 'low', 'low', 'pending',
 '{"reason": "Average CPU utilization is 45% over the past 14 days. Downsizing would maintain performance while reducing costs.", "cpu_avg": 45.2, "memory_avg": 62.3}',
 NOW() - INTERVAL '2 days'),

(gen_random_uuid(), 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'idle_resource', 'aws', '123456789012', 'eu-west-2',
 'i-0b2c3d4e5f6789abc', 'EC2', 't3.large running', 'terminate or stop',
 60.74, 100.0, 'high', 'low', 'medium', 'pending',
 '{"reason": "Instance has been idle (<10% CPU) for 30+ days. Consider terminating or stopping during off-hours.", "cpu_avg": 8.5, "idle_days": 32}',
 NOW() - INTERVAL '5 days'),

(gen_random_uuid(), 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'modernization', 'aws', '123456789012', 'eu-west-2',
 'i-0c3d4e5f6789abcde', 'EC2', 't3.medium On-Demand', 'Lambda Serverless',
 22.00, 72.0, 'medium', 'medium', 'medium', 'pending',
 '{"reason": "Bursty workload pattern with 75% idle time detected. Serverless would be more cost-effective.", "pattern": "bursty", "idle_percent": 75}',
 NOW() - INTERVAL '3 days'),

(gen_random_uuid(), 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'spot_instance', 'aws', '123456789012', 'eu-west-2',
 'i-0d4e5f6789abcdef0', 'EC2', 'c5.xlarge On-Demand', 'c5.xlarge Spot',
 85.00, 68.0, 'high', 'medium', 'medium', 'pending',
 '{"reason": "Batch processing workload identified. Spot instances offer 60-70% savings for fault-tolerant workloads.", "workload_type": "batch"}',
 NOW() - INTERVAL '1 day'),

(gen_random_uuid(), 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'commitment', 'aws', '123456789012', 'eu-west-2',
 'multiple-instances', 'EC2', 'On-Demand', '1-Year Savings Plan',
 165.00, 35.0, 'high', 'low', 'low', 'pending',
 '{"reason": "5 production instances running consistently 24/7. Savings Plan would provide 30-40% discount.", "instance_count": 5}',
 NOW() - INTERVAL '7 days'),

(gen_random_uuid(), 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'rightsizing', 'aws', '123456789012', 'eu-west-2',
 'finopsmind-analytics-db', 'RDS', 'db.r5.xlarge', 'db.r5.large',
 175.20, 50.0, 'high', 'medium', 'medium', 'pending',
 '{"reason": "Database CPU averaging 18% with max 42%. Downsizing would save costs without impacting performance.", "cpu_avg": 18, "cpu_max": 42}',
 NOW() - INTERVAL '4 days'),

(gen_random_uuid(), 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'modernization', 'aws', '123456789012', 'eu-west-2',
 'finopsmind-main-db', 'RDS', 'RDS PostgreSQL', 'Aurora Serverless v2',
 52.00, 30.0, 'medium', 'high', 'medium', 'pending',
 '{"reason": "Variable workload pattern detected. Aurora Serverless would auto-scale and reduce costs during low-usage periods.", "pattern": "variable"}',
 NOW() - INTERVAL '6 days'),

(gen_random_uuid(), 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'storage_optimization', 'aws', '123456789012', 'eu-west-2',
 'finopsmind-logs-prod', 'S3', 'STANDARD storage', 'Intelligent-Tiering',
 45.00, 40.0, 'medium', 'low', 'low', 'pending',
 '{"reason": "65% of objects not accessed in 90 days. Intelligent-Tiering with lifecycle rules would reduce storage costs.", "cold_data_percent": 65}',
 NOW() - INTERVAL '2 days');

-- Insert anomalies
INSERT INTO anomalies (id, organization_id, date, actual_amount, expected_amount, deviation, deviation_pct, score, severity, status, provider, service, account_id, region, root_cause, detected_at, created_at) VALUES
(gen_random_uuid(), 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 
 CURRENT_DATE - 1, 48.50, 15.18, 33.32, 219.5, 0.92, 'high', 'open', 
 'aws', 'EC2', '123456789012', 'eu-west-2',
 'Unexpected instance type change or new EBS volumes attached',
 NOW() - INTERVAL '1 day', NOW()),

(gen_random_uuid(), 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 CURRENT_DATE - 3, 18.75, 2.50, 16.25, 650.0, 0.95, 'high', 'investigating',
 'aws', 'S3', '123456789012', 'eu-west-2',
 'Unusual data transfer activity - possible misconfigured application',
 NOW() - INTERVAL '3 days', NOW()),

(gen_random_uuid(), 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 CURRENT_DATE - 2, 28.50, 12.00, 16.50, 137.5, 0.78, 'medium', 'open',
 'aws', 'RDS', '123456789012', 'eu-west-2',
 'Increased database I/O - check for inefficient queries',
 NOW() - INTERVAL '2 days', NOW());

-- Insert forecasts
INSERT INTO forecasts (id, organization_id, forecast_date, predicted_amount, lower_bound, upper_bound, confidence, model_version, created_at) 
SELECT 
    gen_random_uuid(),
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    CURRENT_DATE + i,
    ROUND((450 * (1 + i * 0.003) + (random() * 20 - 10))::numeric, 2),
    ROUND((450 * (1 + i * 0.003) * 0.9)::numeric, 2),
    ROUND((450 * (1 + i * 0.003) * 1.1)::numeric, 2),
    ROUND((0.95 - i * 0.01)::numeric, 2),
    'prophet-1.0',
    NOW()
FROM generate_series(1, 30) AS i;

-- Summary output
SELECT 'Mock data seeded successfully!' as status;

SELECT 'Costs' as table_name, COUNT(*) as record_count FROM costs WHERE organization_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'
UNION ALL
SELECT 'Recommendations', COUNT(*) FROM recommendations WHERE organization_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'
UNION ALL
SELECT 'Anomalies', COUNT(*) FROM anomalies WHERE organization_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'
UNION ALL
SELECT 'Forecasts', COUNT(*) FROM forecasts WHERE organization_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
