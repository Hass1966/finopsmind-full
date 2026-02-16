package handler

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/auth"
)

type DriftHandler struct{}

func NewDriftHandler() *DriftHandler {
	return &DriftHandler{}
}

// GetSummary returns drift detection summary.
func (h *DriftHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	orgID, _ := auth.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}
	_ = orgID

	WriteJSON(w, http.StatusOK, map[string]any{
		"summary": map[string]any{
			"total_resources":    156,
			"managed_by_iac":     128,
			"shadow_resources":   14,
			"drifted_resources":  8,
			"compliant":         134,
			"iac_coverage_pct":  82.1,
			"drift_cost_impact": 2340.00,
			"shadow_cost":       1280.00,
		},
		"drifted_resources": []map[string]any{
			{
				"resource_id": "i-0abc123def456", "resource_type": "EC2 Instance",
				"provider": "aws", "region": "us-east-1",
				"iac_state": map[string]any{"instance_type": "t3.medium", "tags": map[string]string{"Environment": "prod"}},
				"actual_state": map[string]any{"instance_type": "t3.xlarge", "tags": map[string]string{"Environment": "prod", "manual": "true"}},
				"drift_type": "modified", "severity": "high",
				"detected_at": "2026-02-14T10:30:00Z",
				"monthly_cost_impact": 85.00,
				"description": "Instance type changed from t3.medium to t3.xlarge outside of IaC",
			},
			{
				"resource_id": "vol-0xyz789", "resource_type": "EBS Volume",
				"provider": "aws", "region": "us-east-1",
				"iac_state": map[string]any{"size": 100, "type": "gp3"},
				"actual_state": map[string]any{"size": 500, "type": "gp2"},
				"drift_type": "modified", "severity": "medium",
				"detected_at": "2026-02-13T15:20:00Z",
				"monthly_cost_impact": 32.00,
				"description": "Volume resized from 100GB gp3 to 500GB gp2 outside of IaC",
			},
			{
				"resource_id": "sg-0manual123", "resource_type": "Security Group",
				"provider": "aws", "region": "us-east-1",
				"iac_state": nil,
				"actual_state": map[string]any{"ingress_rules": 5, "egress_rules": 2},
				"drift_type": "unmanaged", "severity": "critical",
				"detected_at": "2026-02-12T08:00:00Z",
				"monthly_cost_impact": 0,
				"description": "Security group created manually - not managed by IaC",
			},
			{
				"resource_id": "i-0shadow456", "resource_type": "EC2 Instance",
				"provider": "aws", "region": "us-west-2",
				"iac_state": nil,
				"actual_state": map[string]any{"instance_type": "m5.large", "state": "running"},
				"drift_type": "shadow", "severity": "high",
				"detected_at": "2026-02-11T12:00:00Z",
				"monthly_cost_impact": 156.00,
				"description": "Shadow infrastructure - EC2 instance not managed by any IaC",
			},
			{
				"resource_id": "rds-manual-01", "resource_type": "RDS Instance",
				"provider": "aws", "region": "eu-west-1",
				"iac_state": nil,
				"actual_state": map[string]any{"instance_class": "db.r5.xlarge", "engine": "postgres"},
				"drift_type": "shadow", "severity": "critical",
				"detected_at": "2026-02-10T09:30:00Z",
				"monthly_cost_impact": 820.00,
				"description": "Shadow RDS instance costing $820/month - not in Terraform state",
			},
		},
	})
}
