package handler

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/auth"
)

type UnitEconomicsHandler struct{}

func NewUnitEconomicsHandler() *UnitEconomicsHandler {
	return &UnitEconomicsHandler{}
}

// GetMetrics returns unit economics metrics.
func (h *UnitEconomicsHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	orgID, _ := auth.GetOrgIDFromContext(r.Context())
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}
	_ = orgID

	WriteJSON(w, http.StatusOK, map[string]any{
		"metrics": []map[string]any{
			{
				"name":           "Cost per Active User",
				"current_value":  0.42,
				"previous_value": 0.48,
				"change_pct":     -12.5,
				"unit":           "USD",
				"trend":          "improving",
				"history": []map[string]any{
					{"month": "Sep 2025", "value": 0.55},
					{"month": "Oct 2025", "value": 0.52},
					{"month": "Nov 2025", "value": 0.48},
					{"month": "Dec 2025", "value": 0.45},
					{"month": "Jan 2026", "value": 0.42},
				},
			},
			{
				"name":           "Cost per API Call",
				"current_value":  0.000082,
				"previous_value": 0.000089,
				"change_pct":     -7.9,
				"unit":           "USD",
				"trend":          "improving",
				"history": []map[string]any{
					{"month": "Sep 2025", "value": 0.000095},
					{"month": "Oct 2025", "value": 0.000092},
					{"month": "Nov 2025", "value": 0.000089},
					{"month": "Dec 2025", "value": 0.000085},
					{"month": "Jan 2026", "value": 0.000082},
				},
			},
			{
				"name":           "Cost per Transaction",
				"current_value":  0.0035,
				"previous_value": 0.0031,
				"change_pct":     12.9,
				"unit":           "USD",
				"trend":          "degrading",
				"history": []map[string]any{
					{"month": "Sep 2025", "value": 0.0028},
					{"month": "Oct 2025", "value": 0.0029},
					{"month": "Nov 2025", "value": 0.0031},
					{"month": "Dec 2025", "value": 0.0033},
					{"month": "Jan 2026", "value": 0.0035},
				},
			},
			{
				"name":           "Cost per GB Stored",
				"current_value":  0.023,
				"previous_value": 0.023,
				"change_pct":     0.0,
				"unit":           "USD",
				"trend":          "stable",
				"history": []map[string]any{
					{"month": "Sep 2025", "value": 0.024},
					{"month": "Oct 2025", "value": 0.023},
					{"month": "Nov 2025", "value": 0.023},
					{"month": "Dec 2025", "value": 0.023},
					{"month": "Jan 2026", "value": 0.023},
				},
			},
		},
		"summary": map[string]any{
			"total_active_users":  12450,
			"total_api_calls":     "63.2M",
			"total_transactions":  "1.48M",
			"total_storage_gb":    2840,
			"infrastructure_cost": 5178.80,
		},
	})
}
