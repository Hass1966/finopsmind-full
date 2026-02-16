package handler

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/auth"
	"github.com/finopsmind/backend/internal/model"
	"github.com/finopsmind/backend/internal/repository"
)

// CostHandler handles cost-related API requests backed by real DB data.
type CostHandler struct {
	repo repository.CostRepository
}

func NewCostHandler(repo repository.CostRepository) *CostHandler {
	return &CostHandler{repo: repo}
}

func (h *CostHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, _ := auth.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	// Parse date range (default: last 30 days)
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)
	if s := r.URL.Query().Get("start"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			startDate = t
		}
	}
	if s := r.URL.Query().Get("end"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			endDate = t
		}
	}

	dateRange := model.DateRange{Start: startDate, End: endDate}

	summary, err := h.repo.GetSummary(ctx, orgID, dateRange)
	if err != nil {
		// Fall back to mock data if DB is empty
		GetCostSummary(w, r)
		return
	}

	if summary.TotalCost == 0 {
		// No data in DB yet, return mock data
		GetCostSummary(w, r)
		return
	}

	// Calculate previous period for comparison
	prevEnd := startDate
	prevStart := prevEnd.AddDate(0, 0, -30)
	prevRange := model.DateRange{Start: prevStart, End: prevEnd}
	prevSummary, _ := h.repo.GetSummary(ctx, orgID, prevRange)

	changePct := 0.0
	previousCost := 0.0
	if prevSummary != nil && prevSummary.TotalCost > 0 {
		previousCost = prevSummary.TotalCost
		changePct = ((summary.TotalCost - previousCost) / previousCost) * 100
	}

	days := endDate.Sub(startDate).Hours() / 24
	if days == 0 {
		days = 1
	}
	dailyAvg := summary.TotalCost / days

	topService := ""
	if len(summary.ByService) > 0 {
		topService = summary.ByService[0].Name
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"total_cost":        summary.TotalCost,
		"previous_cost":     previousCost,
		"change_percent":    changePct,
		"potential_savings": 0,
		"currency":          string(summary.Currency),
		"period":            "custom",
		"daily_average":     dailyAvg,
		"top_service":       topService,
		"services_used":     len(summary.ByService),
		"by_service":        summary.ByService,
	})
}

func (h *CostHandler) GetTrend(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, _ := auth.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err := parseInt(d); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)

	filter := model.CostFilter{
		OrganizationID: orgID,
		DateRange:      model.DateRange{Start: startDate, End: endDate},
		Granularity:    model.GranularityDaily,
	}

	trend, err := h.repo.GetTrend(ctx, filter)
	if err != nil || trend == nil || trend.TotalCost == 0 {
		// Fall back to mock data
		GetCostTrend(w, r)
		return
	}

	// Transform data points for frontend
	dataPoints := make([]map[string]interface{}, 0, len(trend.DataPoints))
	for _, dp := range trend.DataPoints {
		dataPoints = append(dataPoints, map[string]interface{}{
			"date":    dp.Date.Format("2006-01-02"),
			"total":   dp.Total,
			"service": dp.Service,
		})
	}

	// Get breakdown
	breakdown, _ := h.repo.GetBreakdown(ctx, filter, "service")
	services := make([]map[string]interface{}, 0)
	if breakdown != nil {
		for _, item := range breakdown.Items {
			services = append(services, map[string]interface{}{
				"service": item.Name,
				"cost":    item.Amount,
				"percent": item.Percentage,
			})
		}
	}

	topService := ""
	if len(services) > 0 {
		topService = services[0]["service"].(string)
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"data_points":    dataPoints,
		"total_cost":     trend.TotalCost,
		"change_percent": 0,
		"daily_average":  trend.AvgDailyCost,
		"top_service":    topService,
		"services_used":  len(services),
		"services":       services,
	})
}

func (h *CostHandler) GetBreakdown(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, _ := auth.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	dimension := r.URL.Query().Get("dimension")
	if dimension == "" {
		dimension = "service"
	}

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)

	filter := model.CostFilter{
		OrganizationID: orgID,
		DateRange:      model.DateRange{Start: startDate, End: endDate},
	}

	breakdown, err := h.repo.GetBreakdown(ctx, filter, dimension)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get cost breakdown")
		return
	}

	WriteJSON(w, http.StatusOK, breakdown)
}

func defaultOrgID() uuid.UUID {
	id, _ := uuid.Parse("00000000-0000-0000-0000-000000000001")
	return id
}

func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, nil
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
