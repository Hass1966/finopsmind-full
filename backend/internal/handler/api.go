package handler

import (
	"net/http"
	"time"
)

func GetCostSummary(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"total_cost":        4850.00,
		"previous_cost":     4200.00,
		"change_percent":    15.5,
		"forecast_cost":     5200.00,
		"potential_savings": 675.02,
		"currency":          "USD",
		"period":            "monthly",
		"daily_average":     161.67,
		"top_service":       "EC2",
		"services_used":     6,
		"by_service": []map[string]interface{}{
			{"name": "EC2", "amount": 2522.00, "percentage": 52},
			{"name": "RDS", "amount": 1212.50, "percentage": 25},
			{"name": "S3", "amount": 388.00, "percentage": 8},
			{"name": "Lambda", "amount": 339.50, "percentage": 7},
			{"name": "CloudWatch", "amount": 242.50, "percentage": 5},
			{"name": "Other", "amount": 145.50, "percentage": 3},
		},
	})
}

func GetCostTrend(w http.ResponseWriter, r *http.Request) {
	var dataPoints []map[string]interface{}
	baseDate := time.Now().AddDate(0, 0, -30)
	var totalCost float64
	
	for i := 0; i < 30; i++ {
		date := baseDate.AddDate(0, 0, i)
		weekendMultiplier := 1.0
		if date.Weekday() == 0 || date.Weekday() == 6 {
			weekendMultiplier = 0.7
		}
		baseCost := (150.0 + float64(i)*1.5) * weekendMultiplier
		totalCost += baseCost
		
		dataPoints = append(dataPoints, map[string]interface{}{
			"date":  date.Format("2006-01-02"),
			"total": baseCost,
			"breakdown": map[string]interface{}{
				"EC2": baseCost * 0.52, "RDS": baseCost * 0.25, "S3": baseCost * 0.08,
				"Lambda": baseCost * 0.07, "CloudWatch": baseCost * 0.05, "Other": baseCost * 0.03,
			},
		})
	}
	
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"data_points":    dataPoints,
		"total_cost":     totalCost,
		"change_percent": 15.5,
		"daily_average":  totalCost / 30,
		"top_service":    "EC2",
		"services_used":  6,
		"services": []map[string]interface{}{
			{"service": "EC2", "cost": 2522.00, "percent": 52},
			{"service": "RDS", "cost": 1212.50, "percent": 25},
			{"service": "S3", "cost": 388.00, "percent": 8},
			{"service": "Lambda", "cost": 339.50, "percent": 7},
			{"service": "CloudWatch", "cost": 242.50, "percent": 5},
			{"service": "Other", "cost": 145.50, "percent": 3},
		},
	})
}

func GetRecommendations(w http.ResponseWriter, r *http.Request) {
	recommendations := []map[string]interface{}{
		{"id": "rec-001", "resource_id": "i-0a1b2c3d4e5f6789a", "resource_type": "EC2", "type": "rightsizing", "current_config": "m5.xlarge", "recommended_config": "m5.large", "estimated_savings": 70.08, "impact": "high", "effort": "low", "risk": "low", "status": "pending"},
		{"id": "rec-002", "resource_id": "i-0b2c3d4e5f6789abc", "resource_type": "EC2", "type": "idle_resource", "current_config": "t3.large", "recommended_config": "terminate", "estimated_savings": 60.74, "impact": "high", "effort": "low", "risk": "medium", "status": "pending"},
		{"id": "rec-003", "resource_id": "i-0c3d4e5f6789abcde", "resource_type": "EC2", "type": "modernization", "current_config": "t3.medium", "recommended_config": "Lambda", "estimated_savings": 22.00, "impact": "medium", "effort": "medium", "risk": "medium", "status": "pending"},
		{"id": "rec-004", "resource_id": "i-0d4e5f6789abcdef0", "resource_type": "EC2", "type": "spot_instance", "current_config": "c5.xlarge", "recommended_config": "Spot", "estimated_savings": 85.00, "impact": "high", "effort": "medium", "risk": "medium", "status": "pending"},
		{"id": "rec-005", "resource_id": "multiple", "resource_type": "EC2", "type": "commitment", "current_config": "On-Demand", "recommended_config": "Savings Plan", "estimated_savings": 165.00, "impact": "high", "effort": "low", "risk": "low", "status": "pending"},
		{"id": "rec-006", "resource_id": "analytics-db", "resource_type": "RDS", "type": "rightsizing", "current_config": "db.r5.xlarge", "recommended_config": "db.r5.large", "estimated_savings": 175.20, "impact": "high", "effort": "medium", "risk": "medium", "status": "pending"},
		{"id": "rec-007", "resource_id": "main-db", "resource_type": "RDS", "type": "modernization", "current_config": "PostgreSQL", "recommended_config": "Aurora Serverless", "estimated_savings": 52.00, "impact": "medium", "effort": "high", "risk": "medium", "status": "pending"},
		{"id": "rec-008", "resource_id": "logs-bucket", "resource_type": "S3", "type": "storage", "current_config": "STANDARD", "recommended_config": "Intelligent-Tiering", "estimated_savings": 45.00, "impact": "medium", "effort": "low", "risk": "low", "status": "pending"},
		{"id": "rec-009", "resource_id": "i-0f1a2b3c4d5e6f789", "resource_type": "EC2", "type": "rightsizing", "current_config": "r5.2xlarge", "recommended_config": "r5.xlarge", "estimated_savings": 245.00, "impact": "high", "effort": "low", "risk": "low", "status": "pending"},
		{"id": "rec-010", "resource_id": "i-0a9b8c7d6e5f4321", "resource_type": "EC2", "type": "rightsizing", "current_config": "m5.4xlarge", "recommended_config": "m5.2xlarge", "estimated_savings": 380.00, "impact": "high", "effort": "low", "risk": "low", "status": "pending"},
		{"id": "rec-011", "resource_id": "i-idle001122334455", "resource_type": "EC2", "type": "idle_resource", "current_config": "t3.xlarge", "recommended_config": "terminate", "estimated_savings": 122.00, "impact": "high", "effort": "low", "risk": "medium", "status": "pending"},
		{"id": "rec-012", "resource_id": "vol-0aabbccdd1122334", "resource_type": "EBS", "type": "idle_resource", "current_config": "500GB unattached", "recommended_config": "delete", "estimated_savings": 45.00, "impact": "medium", "effort": "low", "risk": "low", "status": "pending"},
		{"id": "rec-013", "resource_id": "i-batch-processor", "resource_type": "EC2", "type": "modernization", "current_config": "c5.2xlarge", "recommended_config": "AWS Batch Spot", "estimated_savings": 320.00, "impact": "high", "effort": "high", "risk": "medium", "status": "pending"},
		{"id": "rec-014", "resource_id": "rds-prod-cluster", "resource_type": "RDS", "type": "commitment", "current_config": "On-Demand", "recommended_config": "1-Year Reserved", "estimated_savings": 420.00, "impact": "high", "effort": "low", "risk": "low", "status": "pending"},
		{"id": "rec-015", "resource_id": "savings-plan", "resource_type": "SavingsPlan", "type": "commitment", "current_config": "On-Demand", "recommended_config": "3-Year Compute SP", "estimated_savings": 890.00, "impact": "high", "effort": "low", "risk": "medium", "status": "pending"},
		{"id": "rec-016", "resource_id": "backup-bucket", "resource_type": "S3", "type": "storage", "current_config": "Standard", "recommended_config": "Glacier Deep Archive", "estimated_savings": 125.00, "impact": "medium", "effort": "low", "risk": "low", "status": "pending"},
		{"id": "rec-017", "resource_id": "logs-archive", "resource_type": "S3", "type": "storage", "current_config": "Standard-IA", "recommended_config": "Glacier", "estimated_savings": 78.00, "impact": "medium", "effort": "low", "risk": "low", "status": "implemented"},
		{"id": "rec-018", "resource_id": "asg-workers", "resource_type": "EC2", "type": "spot_instance", "current_config": "On-Demand ASG", "recommended_config": "Spot ASG", "estimated_savings": 540.00, "impact": "high", "effort": "medium", "risk": "medium", "status": "pending"},
		{"id": "rec-019", "resource_id": "nat-gateway", "resource_type": "NAT", "type": "networking", "current_config": "NAT Gateway", "recommended_config": "NAT Instance", "estimated_savings": 85.00, "impact": "medium", "effort": "medium", "risk": "medium", "status": "dismissed"},
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"data": recommendations, "total": len(recommendations)})
}

func GetBudgets(w http.ResponseWriter, r *http.Request) {
	budgets := []map[string]interface{}{
		{"id": "budget-001", "name": "Production", "amount": 5000.00, "spent": 4250.00, "period": "monthly", "status": "active"},
		{"id": "budget-002", "name": "Development", "amount": 1500.00, "spent": 980.00, "period": "monthly", "status": "active"},
		{"id": "budget-003", "name": "Engineering", "amount": 8500.00, "spent": 7225.00, "period": "monthly", "status": "active"},
		{"id": "budget-004", "name": "Data Science", "amount": 3200.00, "spent": 2880.00, "period": "monthly", "status": "active"},
		{"id": "budget-005", "name": "DevOps", "amount": 2100.00, "spent": 1890.00, "period": "monthly", "status": "active"},
		{"id": "budget-006", "name": "QA Environment", "amount": 800.00, "spent": 920.00, "period": "monthly", "status": "exceeded"},
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{"data": budgets})
}

func GetProviders(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, []map[string]interface{}{
		{"id": "aws", "name": "AWS", "status": "connected", "accounts": 2, "monthly_cost": 4850.00},
	})
}

func GetForecasts(w http.ResponseWriter, r *http.Request) {
	var predictions []map[string]interface{}
	baseDate := time.Now()
	for i := 1; i <= 30; i++ {
		date := baseDate.AddDate(0, 0, i)
		predicted := 165.0 + float64(i)*0.8
		predictions = append(predictions, map[string]interface{}{
			"date":        date.Format("2006-01-02"),
			"predicted":   predicted,
			"lower_bound": predicted * 0.85,
			"upper_bound": predicted * 1.15,
		})
	}

	var totalForecast float64
	for _, p := range predictions {
		totalForecast += p["predicted"].(float64)
	}

	forecasts := []map[string]interface{}{
		{
			"id":               "forecast-001",
			"generated_at":     time.Now().Format(time.RFC3339),
			"model_version":    "prophet-1.2",
			"confidence_level": 0.92,
			"total_forecasted": totalForecast,
			"predictions":      predictions,
		},
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{"data": forecasts})
}

func GetAnomaliesMock(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	anomalies := []map[string]interface{}{
		{"id": "anom-001", "date": now.AddDate(0, 0, -1).Format("2006-01-02"), "service": "EC2", "resource_id": "i-0e5f6789abcdef012", "actual_amount": 892.50, "expected_amount": 245.00, "deviation": 647.50, "deviation_pct": 264.3, "severity": "critical", "status": "open", "root_cause": "Auto-scaling triggered unexpectedly"},
		{"id": "anom-002", "date": now.AddDate(0, 0, -1).Format("2006-01-02"), "service": "Data Transfer", "resource_id": "vpc-transfer", "actual_amount": 156.80, "expected_amount": 42.00, "deviation": 114.80, "deviation_pct": 273.3, "severity": "critical", "status": "investigating", "root_cause": "Cross-region data transfer spike"},
		{"id": "anom-003", "date": now.AddDate(0, 0, -2).Format("2006-01-02"), "service": "RDS", "resource_id": "finopsmind-main-db", "actual_amount": 345.00, "expected_amount": 180.00, "deviation": 165.00, "deviation_pct": 91.7, "severity": "high", "status": "open", "root_cause": "IOPS burst from inefficient queries"},
		{"id": "anom-004", "date": now.AddDate(0, 0, -2).Format("2006-01-02"), "service": "Lambda", "resource_id": "finopsmind-processor", "actual_amount": 78.50, "expected_amount": 32.00, "deviation": 46.50, "deviation_pct": 145.3, "severity": "high", "status": "open", "root_cause": "Timeout retries causing spike"},
		{"id": "anom-005", "date": now.AddDate(0, 0, -3).Format("2006-01-02"), "service": "EBS", "resource_id": "vol-snapshots", "actual_amount": 234.00, "expected_amount": 120.00, "deviation": 114.00, "deviation_pct": 95.0, "severity": "high", "status": "acknowledged", "root_cause": "Snapshots running twice daily"},
		{"id": "anom-006", "date": now.AddDate(0, 0, -4).Format("2006-01-02"), "service": "CloudWatch", "resource_id": "log-group-api", "actual_amount": 45.60, "expected_amount": 22.00, "deviation": 23.60, "deviation_pct": 107.3, "severity": "medium", "status": "open", "root_cause": "Log ingestion doubled after deployment"},
		{"id": "anom-007", "date": now.AddDate(0, 0, -4).Format("2006-01-02"), "service": "S3", "resource_id": "finopsmind-uploads", "actual_amount": 89.00, "expected_amount": 55.00, "deviation": 34.00, "deviation_pct": 61.8, "severity": "medium", "status": "open", "root_cause": "PUT request surge from batch job"},
		{"id": "anom-008", "date": now.AddDate(0, 0, -5).Format("2006-01-02"), "service": "ElastiCache", "resource_id": "redis-cluster", "actual_amount": 67.80, "expected_amount": 38.00, "deviation": 29.80, "deviation_pct": 78.4, "severity": "medium", "status": "resolved", "root_cause": "Cache node auto-scaled - resolved"},
		{"id": "anom-009", "date": now.AddDate(0, 0, -6).Format("2006-01-02"), "service": "SNS", "resource_id": "notifications", "actual_amount": 23.40, "expected_amount": 12.00, "deviation": 11.40, "deviation_pct": 95.0, "severity": "low", "status": "resolved", "root_cause": "Notification spike during incident"},
		{"id": "anom-010", "date": now.AddDate(0, 0, -7).Format("2006-01-02"), "service": "SQS", "resource_id": "job-queue", "actual_amount": 34.20, "expected_amount": 20.00, "deviation": 14.20, "deviation_pct": 71.0, "severity": "low", "status": "resolved", "root_cause": "Queue backlog cleared"},
		{"id": "anom-011", "date": now.AddDate(0, 0, -8).Format("2006-01-02"), "service": "EC2", "resource_id": "i-loadtest-01", "actual_amount": 156.00, "expected_amount": 95.00, "deviation": 61.00, "deviation_pct": 64.2, "severity": "medium", "status": "resolved", "root_cause": "Temporary capacity for load test"},
		{"id": "anom-012", "date": now.AddDate(0, 0, -10).Format("2006-01-02"), "service": "RDS", "resource_id": "analytics-db", "actual_amount": 445.00, "expected_amount": 280.00, "deviation": 165.00, "deviation_pct": 58.9, "severity": "medium", "status": "resolved", "root_cause": "End of month batch processing"},
		{"id": "anom-013", "date": now.AddDate(0, 0, -12).Format("2006-01-02"), "service": "NAT Gateway", "resource_id": "nat-prod", "actual_amount": 89.00, "expected_amount": 45.00, "deviation": 44.00, "deviation_pct": 97.8, "severity": "medium", "status": "resolved", "root_cause": "Increased outbound traffic"},
	}

	openCount := 0
	criticalCount := 0
	resolvedCount := 0
	for _, a := range anomalies {
		if a["status"] == "open" || a["status"] == "investigating" {
			openCount++
		}
		if a["status"] == "resolved" {
			resolvedCount++
		}
		if a["severity"] == "critical" {
			criticalCount++
		}
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"data": anomalies, "total": len(anomalies),
		"open": openCount, "critical": criticalCount, "resolved": resolvedCount,
	})
}

func UpdateRecommendation(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{"status": "updated"})
}

func UpdateAnomaly(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{"status": "updated"})
}
