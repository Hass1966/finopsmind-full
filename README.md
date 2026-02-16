# FinOpsMind Phase 5: ML Classifiers

ML-powered workload classification and optimization recommendations for AWS.

## Overview

Phase 5 adds intelligent workload classification and cost optimization capabilities:

- **Workload Classifier**: Analyzes CloudWatch metrics to classify EC2 workloads
- **Pattern Detector**: Detects usage patterns (bursty, diurnal, steady-state, etc.)
- **Cost Modeler**: What-if cost calculations for Lambda, Fargate, Spot
- **Commitment Optimizer**: Optimal Reserved Instance and Savings Plan mix
- **Architecture Analyzer**: Migration recommendations for modernization

## API Endpoints

### POST /classify/workload
Classify EC2 workload for potential migration.

```bash
curl -X POST http://localhost:8081/classify/workload \
  -H "Content-Type: application/json" \
  -d '{
    "instance_id": "i-1234567890abcdef0",
    "instance_type": "t3.large",
    "cpu_utilization": [
      {"timestamp": "2024-01-01T00:00:00Z", "value": 5},
      {"timestamp": "2024-01-01T01:00:00Z", "value": 3},
      ...
    ]
  }'
```

**Response:**
```json
{
  "instance_id": "i-1234567890abcdef0",
  "classification": "lambda_candidate",
  "confidence": 0.85,
  "reasons": [
    "Low average CPU (5.2%) indicates event-driven pattern",
    "High idle time (85%) suggests sporadic usage"
  ],
  "estimated_savings_percent": 65,
  "recommended_target": "Lambda 1024MB"
}
```

### POST /model/cost
Calculate what-if costs for workload migration.

```bash
curl -X POST http://localhost:8081/model/cost \
  -H "Content-Type: application/json" \
  -d '{
    "instance_id": "i-test",
    "instance_type": "t3.large",
    "avg_cpu_percent": 15,
    "max_cpu_percent": 45,
    "avg_memory_percent": 25,
    "max_memory_percent": 60,
    "avg_requests_per_hour": 1000,
    "avg_request_duration_ms": 200,
    "active_hours_per_day": 8
  }'
```

### POST /optimize/commitment
Calculate optimal RI/Savings Plan mix.

```bash
curl -X POST http://localhost:8081/optimize/commitment \
  -H "Content-Type: application/json" \
  -d '{
    "usage_records": [...],
    "on_demand_prices": {
      "t3.large": 0.0832,
      "m5.xlarge": 0.192
    }
  }'
```

### POST /analyze/architecture
Analyze architecture for migration recommendations.

```bash
curl -X POST http://localhost:8081/analyze/architecture \
  -H "Content-Type: application/json" \
  -d '{
    "resources": [
      {
        "resource_id": "rds-main",
        "resource_type": "rds",
        "avg_cpu_percent": 15,
        "engine": "postgresql",
        "monthly_cost": 200
      }
    ]
  }'
```

## Classifications

The workload classifier outputs one of four classifications:

| Classification | Description | Ideal For |
|---------------|-------------|-----------|
| `lambda_candidate` | Bursty, short-lived, stateless | Event-driven workloads |
| `fargate_candidate` | Containerizable, predictable | Long-running services |
| `spot_candidate` | Fault tolerant, flexible timing | Batch processing |
| `keep_ec2` | None of the above | Specialized workloads |

## Pattern Types

The pattern detector identifies these usage patterns:

- **idle_dominant**: Mostly idle, occasional activity
- **steady_state**: Consistent usage
- **bursty**: Spikes and valleys
- **diurnal**: Day/night patterns
- **weekly**: Weekday/weekend patterns
- **batch**: Periodic batch processing
- **growing**: Increasing trend
- **declining**: Decreasing trend

## Running Locally

### With Docker

```bash
cd ml-sidecar
docker build -t finopsmind-ml:phase5 .
docker run -p 8081:8081 finopsmind-ml:phase5
```

### Without Docker

```bash
cd ml-sidecar
pip install -r requirements.txt
cd app
python -m uvicorn main:app --host 0.0.0.0 --port 8081 --reload
```

### Running Tests

```bash
cd ml-sidecar
pip install pytest pytest-asyncio
pytest tests/ -v
```

## Integration with Go Backend

Call the ML sidecar from the Go recommendation engine:

```go
// In recommendation engine
func classifyWorkload(metrics CloudWatchMetrics) (*ClassificationResult, error) {
    payload, _ := json.Marshal(metrics)
    
    resp, err := http.Post(
        "http://ml-sidecar:8081/classify/workload",
        "application/json",
        bytes.NewBuffer(payload),
    )
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var result ClassificationResult
    json.NewDecoder(resp.Body).Decode(&result)
    return &result, nil
}
```

## Architecture

```
ml-sidecar/
├── app/
│   ├── main.py              # FastAPI application
│   ├── classifier/
│   │   ├── workload.py      # EC2 workload classifier
│   │   └── patterns.py      # Usage pattern detection
│   ├── optimizer/
│   │   ├── cost_model.py    # What-if cost calculations
│   │   └── commitment.py    # RI/SP optimizer
│   └── architecture/
│       └── analyzer.py      # Architecture analysis
├── tests/
│   └── test_phase5.py       # Comprehensive tests
├── Dockerfile
└── requirements.txt
```

## Future Enhancements

Phase 5 uses heuristic/rule-based classification for easier testing and validation. Future enhancements could include:

1. **ML Model Training**: Train on real workload data
2. **Confidence Calibration**: Improve confidence scoring
3. **Custom Thresholds**: Per-customer threshold tuning
4. **Anomaly Detection**: Integration with Isolation Forest from Phase 4
5. **Cost Prediction**: Time-series cost forecasting

## Configuration

Thresholds can be customized via configuration:

```python
classifier = WorkloadClassifier(config={
    "lambda_max_avg_cpu": 25,
    "fargate_max_cpu_stddev": 30,
})
```

## API Documentation

Interactive API documentation available at:
- Swagger UI: http://localhost:8081/docs
- ReDoc: http://localhost:8081/redoc
