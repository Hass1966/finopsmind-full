package handler

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/auth"
)

type KubernetesHandler struct{}

func NewKubernetesHandler() *KubernetesHandler {
	return &KubernetesHandler{}
}

// GetClusters returns K8s cluster cost overview.
func (h *KubernetesHandler) GetClusters(w http.ResponseWriter, r *http.Request) {
	orgID, _ := auth.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}
	_ = orgID

	WriteJSON(w, http.StatusOK, map[string]any{
		"data": []map[string]any{
			{
				"cluster_name": "prod-eks-us-east-1",
				"provider":     "aws",
				"region":       "us-east-1",
				"node_count":   12,
				"pod_count":    87,
				"total_cost":   3420.50,
				"cpu_cost":     1850.00,
				"memory_cost":  1120.50,
				"storage_cost": 450.00,
				"efficiency":   72.5,
				"namespaces":   8,
			},
			{
				"cluster_name": "staging-eks-eu-west-1",
				"provider":     "aws",
				"region":       "eu-west-1",
				"node_count":   4,
				"pod_count":    32,
				"total_cost":   980.00,
				"cpu_cost":     520.00,
				"memory_cost":  310.00,
				"storage_cost": 150.00,
				"efficiency":   58.3,
				"namespaces":   5,
			},
		},
		"total_cost": 4400.50,
	})
}

// GetNamespaces returns namespace-level cost breakdown.
func (h *KubernetesHandler) GetNamespaces(w http.ResponseWriter, r *http.Request) {
	orgID, _ := auth.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}
	_ = orgID

	WriteJSON(w, http.StatusOK, map[string]any{
		"data": []map[string]any{
			{"namespace": "production", "pods": 24, "cpu_requests": "12.5 cores", "memory_requests": "48Gi", "cost": 1280.00, "efficiency": 78.2},
			{"namespace": "api-gateway", "pods": 8, "cpu_requests": "4 cores", "memory_requests": "16Gi", "cost": 620.00, "efficiency": 82.1},
			{"namespace": "data-pipeline", "pods": 15, "cpu_requests": "8 cores", "memory_requests": "32Gi", "cost": 540.00, "efficiency": 65.4},
			{"namespace": "monitoring", "pods": 12, "cpu_requests": "3 cores", "memory_requests": "12Gi", "cost": 380.00, "efficiency": 71.0},
			{"namespace": "ml-training", "pods": 6, "cpu_requests": "16 cores", "memory_requests": "64Gi", "cost": 320.00, "efficiency": 45.8},
			{"namespace": "frontend", "pods": 4, "cpu_requests": "2 cores", "memory_requests": "8Gi", "cost": 180.00, "efficiency": 88.5},
			{"namespace": "kube-system", "pods": 10, "cpu_requests": "2 cores", "memory_requests": "4Gi", "cost": 65.00, "efficiency": 92.3},
			{"namespace": "cert-manager", "pods": 3, "cpu_requests": "0.5 cores", "memory_requests": "1Gi", "cost": 15.50, "efficiency": 95.0},
		},
	})
}

// GetRightsizing returns pod rightsizing recommendations.
func (h *KubernetesHandler) GetRightsizing(w http.ResponseWriter, r *http.Request) {
	orgID, _ := auth.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}
	_ = orgID

	WriteJSON(w, http.StatusOK, map[string]any{
		"data": []map[string]any{
			{
				"namespace": "ml-training", "deployment": "model-trainer",
				"current_cpu": "4 cores", "recommended_cpu": "2 cores",
				"current_memory": "16Gi", "recommended_memory": "8Gi",
				"avg_cpu_usage": 22.5, "avg_memory_usage": 38.0,
				"monthly_savings": 85.00,
			},
			{
				"namespace": "data-pipeline", "deployment": "etl-worker",
				"current_cpu": "2 cores", "recommended_cpu": "1 core",
				"current_memory": "8Gi", "recommended_memory": "4Gi",
				"avg_cpu_usage": 15.3, "avg_memory_usage": 28.7,
				"monthly_savings": 42.00,
			},
			{
				"namespace": "monitoring", "deployment": "prometheus",
				"current_cpu": "1 core", "recommended_cpu": "0.5 cores",
				"current_memory": "4Gi", "recommended_memory": "2Gi",
				"avg_cpu_usage": 30.1, "avg_memory_usage": 45.0,
				"monthly_savings": 28.00,
			},
			{
				"namespace": "api-gateway", "deployment": "nginx-ingress",
				"current_cpu": "2 cores", "recommended_cpu": "1.5 cores",
				"current_memory": "4Gi", "recommended_memory": "2Gi",
				"avg_cpu_usage": 55.0, "avg_memory_usage": 40.0,
				"monthly_savings": 18.00,
			},
		},
		"total_monthly_savings": 173.00,
	})
}
