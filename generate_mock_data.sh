#!/bin/bash
#
# FinOpsMind Mock Data Generator
# Generates realistic mock data for all dashboards and features
#
# Usage: ./generate_mock_data.sh
#

set -e

echo "=========================================="
echo "FinOpsMind Mock Data Generator"
echo "=========================================="

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
API_URL="${API_URL:-http://localhost:8080/api/v1}"
ML_URL="${ML_URL:-http://localhost:8081}"
DB_URL="${DATABASE_URL:-postgres://finopsmind:finopsmind_secret@localhost:5432/finopsmind}"

echo -e "${BLUE}API URL: $API_URL${NC}"
echo -e "${BLUE}ML URL: $ML_URL${NC}"

# Check if services are running
echo -e "\n${YELLOW}Checking services...${NC}"

if curl -s "$ML_URL/" > /dev/null 2>&1; then
    echo -e "${GREEN}✓ ML Sidecar is running${NC}"
else
    echo "✗ ML Sidecar not running. Start with: docker compose up -d ml-sidecar"
    exit 1
fi

# Create Python script for generating mock data
cat > /tmp/generate_mock_data.py << 'PYTHON_SCRIPT'
import json
import random
from datetime import datetime, timedelta
import sys

def generate_ec2_instances(count=20):
    """Generate mock EC2 instances with varied characteristics."""
    instance_types = [
        ("t3.micro", 0.0104, 2, 1),
        ("t3.small", 0.0208, 2, 2),
        ("t3.medium", 0.0416, 2, 4),
        ("t3.large", 0.0832, 2, 8),
        ("t3.xlarge", 0.1664, 4, 16),
        ("m5.large", 0.096, 2, 8),
        ("m5.xlarge", 0.192, 4, 16),
        ("m5.2xlarge", 0.384, 8, 32),
        ("c5.large", 0.085, 2, 4),
        ("c5.xlarge", 0.170, 4, 8),
        ("r5.large", 0.126, 2, 16),
        ("r5.xlarge", 0.252, 4, 32),
    ]
    
    environments = ["production", "staging", "development", "test"]
    applications = ["web-app", "api-server", "worker", "batch-processor", "database", "cache", "monitoring", "ci-cd"]
    regions = ["eu-west-2"]  # London region for UK user
    azs = ["eu-west-2a", "eu-west-2b", "eu-west-2c"]
    
    instances = []
    
    # Create different workload profiles
    profiles = [
        {"name": "idle", "cpu_avg": 5, "cpu_max": 15, "mem_avg": 20, "pattern": "idle"},
        {"name": "bursty", "cpu_avg": 10, "cpu_max": 90, "mem_avg": 30, "pattern": "bursty"},
        {"name": "steady", "cpu_avg": 45, "cpu_max": 60, "mem_avg": 55, "pattern": "steady"},
        {"name": "overloaded", "cpu_avg": 85, "cpu_max": 99, "mem_avg": 80, "pattern": "high"},
        {"name": "underutilized", "cpu_avg": 8, "cpu_max": 25, "mem_avg": 15, "pattern": "low"},
        {"name": "diurnal", "cpu_avg": 35, "cpu_max": 75, "mem_avg": 40, "pattern": "diurnal"},
    ]
    
    for i in range(count):
        instance_type, hourly_cost, vcpu, memory = random.choice(instance_types)
        profile = random.choice(profiles)
        env = random.choice(environments)
        app = random.choice(applications)
        
        # Add some variance
        cpu_avg = profile["cpu_avg"] + random.uniform(-5, 5)
        cpu_max = min(100, profile["cpu_max"] + random.uniform(-10, 10))
        mem_avg = profile["mem_avg"] + random.uniform(-5, 5)
        
        instance = {
            "instance_id": f"i-{random.randint(10000000, 99999999):08x}",
            "instance_type": instance_type,
            "region": random.choice(regions),
            "availability_zone": random.choice(azs),
            "state": "running",
            "launch_time": (datetime.now() - timedelta(days=random.randint(1, 365))).isoformat(),
            "tags": {
                "Name": f"{env}-{app}-{i+1:02d}",
                "Environment": env,
                "Application": app,
                "Owner": random.choice(["platform-team", "dev-team", "ops-team", "data-team"]),
                "CostCenter": random.choice(["CC-100", "CC-200", "CC-300", "CC-400"]),
            },
            "metrics": {
                "cpu_avg": round(max(0, min(100, cpu_avg)), 1),
                "cpu_max": round(max(0, min(100, cpu_max)), 1),
                "memory_avg": round(max(0, min(100, mem_avg)), 1),
                "network_in_mbps": round(random.uniform(0.1, 50), 2),
                "network_out_mbps": round(random.uniform(0.1, 30), 2),
                "disk_read_iops": round(random.uniform(10, 500)),
                "disk_write_iops": round(random.uniform(10, 300)),
            },
            "cost": {
                "hourly": hourly_cost,
                "daily": round(hourly_cost * 24, 2),
                "monthly": round(hourly_cost * 730, 2),
            },
            "workload_profile": profile["name"],
            "vcpu": vcpu,
            "memory_gb": memory,
        }
        instances.append(instance)
    
    return instances


def generate_rds_instances(count=5):
    """Generate mock RDS instances."""
    engines = [
        ("postgresql", "15.4"),
        ("mysql", "8.0.35"),
        ("aurora-postgresql", "15.4"),
    ]
    
    instance_classes = [
        ("db.t3.micro", 0.018, 2, 1),
        ("db.t3.small", 0.036, 2, 2),
        ("db.t3.medium", 0.072, 2, 4),
        ("db.r5.large", 0.24, 2, 16),
        ("db.r5.xlarge", 0.48, 4, 32),
    ]
    
    instances = []
    apps = ["main-db", "analytics-db", "reporting-db", "users-db", "orders-db"]
    
    for i in range(count):
        engine, version = random.choice(engines)
        instance_class, hourly_cost, vcpu, memory = random.choice(instance_classes)
        
        instance = {
            "db_instance_id": f"finopsmind-{apps[i % len(apps)]}",
            "engine": engine,
            "engine_version": version,
            "instance_class": instance_class,
            "storage_gb": random.choice([20, 50, 100, 200, 500]),
            "multi_az": random.choice([True, False]),
            "storage_type": random.choice(["gp2", "gp3", "io1"]),
            "metrics": {
                "cpu_avg": round(random.uniform(10, 60), 1),
                "connections_avg": random.randint(5, 100),
                "connections_max": random.randint(50, 200),
                "read_iops": random.randint(100, 2000),
                "write_iops": random.randint(50, 1000),
                "freeable_memory_mb": random.randint(500, 4000),
            },
            "cost": {
                "hourly": hourly_cost,
                "monthly": round(hourly_cost * 730, 2),
                "storage_monthly": round(random.uniform(10, 50), 2),
            },
        }
        instances.append(instance)
    
    return instances


def generate_s3_buckets(count=8):
    """Generate mock S3 buckets."""
    purposes = ["logs", "backups", "static-assets", "data-lake", "artifacts", "media", "exports", "archives"]
    
    buckets = []
    for i in range(count):
        size_gb = random.choice([1, 5, 10, 50, 100, 500, 1000, 5000])
        
        bucket = {
            "bucket_name": f"finopsmind-{purposes[i % len(purposes)]}-{random.randint(1000, 9999)}",
            "region": "eu-west-2",
            "creation_date": (datetime.now() - timedelta(days=random.randint(30, 500))).isoformat(),
            "size_gb": size_gb,
            "object_count": random.randint(100, 100000),
            "storage_class_breakdown": {
                "STANDARD": round(size_gb * random.uniform(0.3, 0.7), 2),
                "STANDARD_IA": round(size_gb * random.uniform(0.1, 0.3), 2),
                "GLACIER": round(size_gb * random.uniform(0, 0.2), 2),
            },
            "cost": {
                "storage_monthly": round(size_gb * 0.023, 2),
                "requests_monthly": round(random.uniform(0.5, 20), 2),
                "transfer_monthly": round(random.uniform(1, 50), 2),
            },
            "lifecycle_rules": random.choice([True, False]),
            "versioning": random.choice([True, False]),
            "encryption": True,
        }
        buckets.append(bucket)
    
    return buckets


def generate_cost_history(days=90):
    """Generate daily cost history."""
    base_daily_cost = 150  # Base daily cost
    history = []
    
    for i in range(days):
        date = datetime.now() - timedelta(days=days - i)
        
        # Add some patterns
        day_of_week = date.weekday()
        
        # Lower on weekends
        if day_of_week >= 5:
            multiplier = 0.7
        else:
            multiplier = 1.0
        
        # Gradual growth trend
        trend = 1 + (i / days) * 0.15  # 15% growth over period
        
        # Random variance
        variance = random.uniform(0.9, 1.1)
        
        daily_cost = base_daily_cost * multiplier * trend * variance
        
        # Breakdown by service
        ec2_pct = random.uniform(0.45, 0.55)
        rds_pct = random.uniform(0.15, 0.25)
        s3_pct = random.uniform(0.05, 0.10)
        other_pct = 1 - ec2_pct - rds_pct - s3_pct
        
        history.append({
            "date": date.strftime("%Y-%m-%d"),
            "total_cost": round(daily_cost, 2),
            "by_service": {
                "EC2": round(daily_cost * ec2_pct, 2),
                "RDS": round(daily_cost * rds_pct, 2),
                "S3": round(daily_cost * s3_pct, 2),
                "Lambda": round(daily_cost * other_pct * 0.3, 2),
                "CloudWatch": round(daily_cost * other_pct * 0.2, 2),
                "Other": round(daily_cost * other_pct * 0.5, 2),
            },
            "by_environment": {
                "production": round(daily_cost * 0.6, 2),
                "staging": round(daily_cost * 0.2, 2),
                "development": round(daily_cost * 0.15, 2),
                "test": round(daily_cost * 0.05, 2),
            },
        })
    
    return history


def generate_recommendations():
    """Generate cost optimization recommendations."""
    recommendations = [
        {
            "id": "rec-001",
            "type": "rightsizing",
            "resource_type": "EC2",
            "resource_id": "i-0a1b2c3d4e5f6789a",
            "current_config": "m5.xlarge",
            "recommended_config": "m5.large",
            "reason": "Average CPU utilization is 12% over the past 14 days",
            "estimated_monthly_savings": 96.36,
            "confidence": 0.92,
            "effort": "low",
            "risk": "low",
        },
        {
            "id": "rec-002",
            "type": "idle_resource",
            "resource_type": "EC2",
            "resource_id": "i-0b2c3d4e5f6789abc",
            "current_config": "t3.large",
            "recommended_config": "terminate",
            "reason": "Instance has been idle (<5% CPU) for 30 days",
            "estimated_monthly_savings": 60.74,
            "confidence": 0.95,
            "effort": "low",
            "risk": "medium",
        },
        {
            "id": "rec-003",
            "type": "migration",
            "resource_type": "EC2",
            "resource_id": "i-0c3d4e5f6789abcde",
            "current_config": "t3.medium (On-Demand)",
            "recommended_config": "Lambda",
            "reason": "Bursty workload pattern with 85% idle time - ideal for serverless",
            "estimated_monthly_savings": 25.50,
            "confidence": 0.78,
            "effort": "medium",
            "risk": "medium",
        },
        {
            "id": "rec-004",
            "type": "commitment",
            "resource_type": "EC2",
            "resource_id": "multiple",
            "current_config": "On-Demand",
            "recommended_config": "1-Year Reserved Instance",
            "reason": "5 production instances running 24/7 for 6+ months",
            "estimated_monthly_savings": 180.00,
            "confidence": 0.88,
            "effort": "low",
            "risk": "low",
        },
        {
            "id": "rec-005",
            "type": "storage_optimization",
            "resource_type": "S3",
            "resource_id": "finopsmind-logs-bucket",
            "current_config": "STANDARD storage class",
            "recommended_config": "Intelligent-Tiering + Lifecycle Policy",
            "reason": "70% of objects not accessed in 90 days",
            "estimated_monthly_savings": 45.00,
            "confidence": 0.90,
            "effort": "low",
            "risk": "low",
        },
        {
            "id": "rec-006",
            "type": "rightsizing",
            "resource_type": "RDS",
            "resource_id": "finopsmind-analytics-db",
            "current_config": "db.r5.xlarge",
            "recommended_config": "db.r5.large",
            "reason": "Average CPU 18%, memory usage 35% over 30 days",
            "estimated_monthly_savings": 175.20,
            "confidence": 0.85,
            "effort": "medium",
            "risk": "medium",
        },
        {
            "id": "rec-007",
            "type": "migration",
            "resource_type": "RDS",
            "resource_id": "finopsmind-main-db",
            "current_config": "RDS PostgreSQL",
            "recommended_config": "Aurora Serverless v2",
            "reason": "Variable workload with 40% idle periods - serverless would optimize costs",
            "estimated_monthly_savings": 85.00,
            "confidence": 0.72,
            "effort": "high",
            "risk": "medium",
        },
        {
            "id": "rec-008",
            "type": "spot_instance",
            "resource_type": "EC2",
            "resource_id": "i-0d4e5f6789abcdef0",
            "current_config": "On-Demand c5.xlarge",
            "recommended_config": "Spot c5.xlarge",
            "reason": "Batch processing workload tolerant to interruptions",
            "estimated_monthly_savings": 89.00,
            "confidence": 0.82,
            "effort": "medium",
            "risk": "medium",
        },
    ]
    
    return recommendations


def generate_anomalies():
    """Generate cost anomalies for the anomaly detection feature."""
    anomalies = [
        {
            "id": "anom-001",
            "detected_at": (datetime.now() - timedelta(days=2)).isoformat(),
            "service": "EC2",
            "resource_id": "i-0e5f6789abcdef012",
            "metric": "daily_cost",
            "expected_value": 12.50,
            "actual_value": 45.80,
            "deviation_percent": 266,
            "severity": "high",
            "status": "open",
            "possible_cause": "New instance launched or instance type changed",
        },
        {
            "id": "anom-002",
            "detected_at": (datetime.now() - timedelta(days=5)).isoformat(),
            "service": "S3",
            "resource_id": "finopsmind-data-lake",
            "metric": "data_transfer_cost",
            "expected_value": 8.00,
            "actual_value": 52.30,
            "deviation_percent": 554,
            "severity": "high",
            "status": "investigating",
            "possible_cause": "Unusual data transfer activity detected",
        },
        {
            "id": "anom-003",
            "detected_at": (datetime.now() - timedelta(days=1)).isoformat(),
            "service": "Lambda",
            "resource_id": "finopsmind-processor",
            "metric": "invocation_cost",
            "expected_value": 3.20,
            "actual_value": 8.90,
            "deviation_percent": 178,
            "severity": "medium",
            "status": "open",
            "possible_cause": "Increased function invocations",
        },
    ]
    
    return anomalies


def generate_forecast(days=30):
    """Generate cost forecast data."""
    current_daily = 165  # Current average daily cost
    forecast = []
    
    for i in range(days):
        date = datetime.now() + timedelta(days=i + 1)
        
        # Growth trend
        trend = 1 + (i / days) * 0.08  # 8% growth forecast
        
        # Confidence decreases over time
        confidence = max(0.5, 0.95 - (i / days) * 0.4)
        
        # Variance increases over time
        variance_pct = 5 + (i / days) * 15
        
        predicted = current_daily * trend
        lower_bound = predicted * (1 - variance_pct / 100)
        upper_bound = predicted * (1 + variance_pct / 100)
        
        forecast.append({
            "date": date.strftime("%Y-%m-%d"),
            "predicted_cost": round(predicted, 2),
            "lower_bound": round(lower_bound, 2),
            "upper_bound": round(upper_bound, 2),
            "confidence": round(confidence, 2),
        })
    
    return forecast


def generate_cloudwatch_metrics(instance_id, hours=336):
    """Generate CloudWatch-style metrics for ML classification."""
    metrics = []
    base_time = datetime.now() - timedelta(hours=hours)
    
    # Random profile
    profile = random.choice(["bursty", "steady", "idle", "diurnal"])
    
    for h in range(hours):
        ts = base_time + timedelta(hours=h)
        
        if profile == "bursty":
            if random.random() < 0.1:
                cpu = random.uniform(70, 95)
            else:
                cpu = random.uniform(2, 8)
        elif profile == "steady":
            cpu = random.gauss(50, 8)
        elif profile == "idle":
            cpu = random.gauss(5, 2)
        elif profile == "diurnal":
            hour = ts.hour
            if 9 <= hour <= 17:
                cpu = random.gauss(65, 10)
            else:
                cpu = random.gauss(15, 5)
        
        cpu = max(0, min(100, cpu))
        
        metrics.append({
            "timestamp": ts.isoformat(),
            "value": round(cpu, 1)
        })
    
    return metrics, profile


# Main execution
if __name__ == "__main__":
    output = {
        "generated_at": datetime.now().isoformat(),
        "ec2_instances": generate_ec2_instances(20),
        "rds_instances": generate_rds_instances(5),
        "s3_buckets": generate_s3_buckets(8),
        "cost_history": generate_cost_history(90),
        "recommendations": generate_recommendations(),
        "anomalies": generate_anomalies(),
        "forecast": generate_forecast(30),
    }
    
    # Calculate summary stats
    total_ec2_monthly = sum(i["cost"]["monthly"] for i in output["ec2_instances"])
    total_rds_monthly = sum(i["cost"]["monthly"] for i in output["rds_instances"])
    total_s3_monthly = sum(i["cost"]["storage_monthly"] + i["cost"]["requests_monthly"] + i["cost"]["transfer_monthly"] for i in output["s3_buckets"])
    total_savings = sum(r["estimated_monthly_savings"] for r in output["recommendations"])
    
    output["summary"] = {
        "total_ec2_instances": len(output["ec2_instances"]),
        "total_rds_instances": len(output["rds_instances"]),
        "total_s3_buckets": len(output["s3_buckets"]),
        "total_monthly_cost": round(total_ec2_monthly + total_rds_monthly + total_s3_monthly, 2),
        "ec2_monthly_cost": round(total_ec2_monthly, 2),
        "rds_monthly_cost": round(total_rds_monthly, 2),
        "s3_monthly_cost": round(total_s3_monthly, 2),
        "total_recommendations": len(output["recommendations"]),
        "total_potential_savings": round(total_savings, 2),
        "savings_percentage": round((total_savings / (total_ec2_monthly + total_rds_monthly + total_s3_monthly)) * 100, 1),
        "open_anomalies": len([a for a in output["anomalies"] if a["status"] == "open"]),
    }
    
    print(json.dumps(output, indent=2))
PYTHON_SCRIPT

echo -e "\n${YELLOW}Generating mock data...${NC}"
python3 /tmp/generate_mock_data.py > /tmp/mock_data.json

echo -e "${GREEN}✓ Mock data generated${NC}"

# Display summary
echo -e "\n${BLUE}=========================================="
echo "Mock Data Summary"
echo "==========================================${NC}"
python3 -c "
import json
with open('/tmp/mock_data.json') as f:
    data = json.load(f)
    s = data['summary']
    print(f'''
EC2 Instances:        {s['total_ec2_instances']}
RDS Instances:        {s['total_rds_instances']}
S3 Buckets:           {s['total_s3_buckets']}

Monthly Costs:
  EC2:                \${s['ec2_monthly_cost']:,.2f}
  RDS:                \${s['rds_monthly_cost']:,.2f}
  S3:                 \${s['s3_monthly_cost']:,.2f}
  Total:              \${s['total_monthly_cost']:,.2f}

Recommendations:      {s['total_recommendations']}
Potential Savings:    \${s['total_potential_savings']:,.2f}/month ({s['savings_percentage']}%)
Open Anomalies:       {s['open_anomalies']}
''')
"

# Test ML Sidecar with mock data
echo -e "\n${YELLOW}Testing ML Sidecar endpoints...${NC}"

# Test workload classification
echo -e "\n${BLUE}Testing /classify/workload...${NC}"
CLASSIFY_RESPONSE=$(curl -s -X POST "$ML_URL/classify/workload" \
  -H "Content-Type: application/json" \
  -d '{
    "instance_id": "i-mock12345678",
    "instance_type": "t3.large",
    "cpu_utilization": [
      {"timestamp": "2025-01-01T00:00:00Z", "value": 5},
      {"timestamp": "2025-01-01T01:00:00Z", "value": 3},
      {"timestamp": "2025-01-01T02:00:00Z", "value": 85},
      {"timestamp": "2025-01-01T03:00:00Z", "value": 4},
      {"timestamp": "2025-01-01T04:00:00Z", "value": 2},
      {"timestamp": "2025-01-01T05:00:00Z", "value": 6},
      {"timestamp": "2025-01-01T06:00:00Z", "value": 90},
      {"timestamp": "2025-01-01T07:00:00Z", "value": 3}
    ]
  }')

echo "$CLASSIFY_RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(f\"  Classification: {data['classification']}\")
print(f\"  Confidence: {data['confidence']:.0%}\")
print(f\"  Recommended: {data.get('recommended_target', 'N/A')}\")
print(f\"  Est. Savings: {data.get('estimated_savings_percent', 0):.0f}%\")
"

# Test cost model
echo -e "\n${BLUE}Testing /model/cost...${NC}"
COST_RESPONSE=$(curl -s -X POST "$ML_URL/model/cost" \
  -H "Content-Type: application/json" \
  -d '{
    "instance_id": "i-mock12345678",
    "instance_type": "t3.large",
    "avg_cpu_percent": 15,
    "max_cpu_percent": 45,
    "avg_memory_percent": 25,
    "max_memory_percent": 60,
    "avg_requests_per_hour": 1000,
    "avg_request_duration_ms": 200,
    "active_hours_per_day": 8
  }')

echo "$COST_RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(f\"  Current EC2 Cost: \${data['ec2_on_demand']:.2f}/month\")
print(f\"  Best Option: {data['best_option']}\")
print(f\"  Best Option Cost: \${data['best_option_cost']:.2f}/month\")
print(f\"  Potential Savings: \${data['potential_savings_dollars']:.2f}/month ({data['potential_savings_percent']:.0f}%)\")
"

# Test architecture analysis
echo -e "\n${BLUE}Testing /analyze/architecture...${NC}"
ARCH_RESPONSE=$(curl -s -X POST "$ML_URL/analyze/architecture" \
  -H "Content-Type: application/json" \
  -d '{
    "resources": [
      {
        "resource_id": "rds-main-db",
        "resource_type": "rds",
        "avg_cpu_percent": 15,
        "max_cpu_percent": 40,
        "avg_connections": 25,
        "max_connections": 100,
        "monthly_cost": 200,
        "engine": "postgresql"
      },
      {
        "resource_id": "ec2-web-server",
        "resource_type": "ec2",
        "avg_cpu_percent": 20,
        "monthly_cost": 150
      },
      {
        "resource_id": "alb-main",
        "resource_type": "alb",
        "monthly_cost": 30
      }
    ]
  }')

echo "$ARCH_RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(f\"  Modernization Score: {data['modernization_score']}/100\")
print(f\"  Patterns Detected: {len(data['patterns_detected'])}\")
print(f\"  Migration Candidates: {len(data['migration_candidates'])}\")
print(f\"  Potential Savings: \${data['total_potential_savings']:.2f}/month\")
if data['recommendations']:
    print(f\"  Top Recommendation: {data['recommendations'][0][:60]}...\")
"

echo -e "\n${GREEN}=========================================="
echo "Mock Data Generation Complete!"
echo "==========================================${NC}"
echo ""
echo "Mock data saved to: /tmp/mock_data.json"
echo ""
echo "Next steps:"
echo "1. Start all services: docker compose up -d"
echo "2. View dashboard: http://localhost:3000"
echo "3. View API docs: http://localhost:8081/docs"
echo ""
echo "To use mock data in the frontend, the backend needs"
echo "to serve this data. See README for integration details."
