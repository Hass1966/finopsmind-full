package handler

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/auth"
)

type CommitmentHandler struct{}

func NewCommitmentHandler() *CommitmentHandler {
	return &CommitmentHandler{}
}

// GetPortfolio returns the commitment portfolio overview.
func (h *CommitmentHandler) GetPortfolio(w http.ResponseWriter, r *http.Request) {
	orgID, _ := auth.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}
	_ = orgID

	WriteJSON(w, http.StatusOK, map[string]any{
		"summary": map[string]any{
			"total_commitments":     8,
			"total_monthly_commit":  3200.00,
			"total_monthly_savings": 960.00,
			"avg_utilization":       82.5,
			"expiring_soon":         2,
			"underutilized":         1,
		},
		"commitments": []map[string]any{
			{
				"id": "ri-001", "type": "Reserved Instance", "service": "EC2",
				"instance_type": "m5.xlarge", "quantity": 3,
				"term": "1 year", "payment": "All Upfront",
				"monthly_cost": 450.00, "on_demand_cost": 720.00, "savings": 270.00,
				"utilization": 95.2, "start_date": "2025-03-01", "end_date": "2026-03-01",
				"status": "active", "days_remaining": 13,
			},
			{
				"id": "ri-002", "type": "Reserved Instance", "service": "RDS",
				"instance_type": "db.r5.large", "quantity": 2,
				"term": "1 year", "payment": "Partial Upfront",
				"monthly_cost": 380.00, "on_demand_cost": 580.00, "savings": 200.00,
				"utilization": 88.0, "start_date": "2025-06-01", "end_date": "2026-06-01",
				"status": "active", "days_remaining": 105,
			},
			{
				"id": "sp-001", "type": "Savings Plan", "service": "Compute",
				"instance_type": "Flexible", "quantity": 1,
				"term": "1 year", "payment": "No Upfront",
				"monthly_cost": 800.00, "on_demand_cost": 1100.00, "savings": 300.00,
				"utilization": 91.5, "start_date": "2025-09-01", "end_date": "2026-09-01",
				"status": "active", "days_remaining": 197,
			},
			{
				"id": "sp-002", "type": "Savings Plan", "service": "EC2 Instance",
				"instance_type": "m5 family", "quantity": 1,
				"term": "3 year", "payment": "All Upfront",
				"monthly_cost": 520.00, "on_demand_cost": 850.00, "savings": 330.00,
				"utilization": 72.0, "start_date": "2024-01-01", "end_date": "2027-01-01",
				"status": "active", "days_remaining": 319,
			},
			{
				"id": "ri-003", "type": "Reserved Instance", "service": "ElastiCache",
				"instance_type": "cache.r5.large", "quantity": 1,
				"term": "1 year", "payment": "All Upfront",
				"monthly_cost": 180.00, "on_demand_cost": 260.00, "savings": 80.00,
				"utilization": 45.0, "start_date": "2025-04-01", "end_date": "2026-04-01",
				"status": "underutilized", "days_remaining": 44,
			},
		},
		"recommendations": []map[string]any{
			{
				"action":         "Renew RI ri-001 (EC2 m5.xlarge) expiring in 13 days",
				"type":           "renewal",
				"savings_impact": 270.00,
				"priority":       "high",
			},
			{
				"action":         "Consider converting ri-003 (ElastiCache) to on-demand — only 45% utilized",
				"type":           "convert",
				"savings_impact": -80.00,
				"priority":       "medium",
			},
			{
				"action":         "Purchase additional Compute Savings Plan — $400/month of on-demand eligible",
				"type":           "purchase",
				"savings_impact": 120.00,
				"priority":       "medium",
			},
		},
	})
}
