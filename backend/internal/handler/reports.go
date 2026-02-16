package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/auth"
	"github.com/finopsmind/backend/internal/model"
	"github.com/finopsmind/backend/internal/repository"
)

// ReportHandler handles report generation requests.
type ReportHandler struct {
	costRepo    repository.CostRepository
	budgetRepo  repository.BudgetRepository
	anomalyRepo repository.AnomalyRepository
}

func NewReportHandler(costRepo repository.CostRepository, budgetRepo repository.BudgetRepository, anomalyRepo repository.AnomalyRepository) *ReportHandler {
	return &ReportHandler{
		costRepo:    costRepo,
		budgetRepo:  budgetRepo,
		anomalyRepo: anomalyRepo,
	}
}

// ExecutiveSummary generates an executive cost summary report.
func (h *ReportHandler) ExecutiveSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, _ := auth.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	// Parse period (default: last 30 days)
	period := r.URL.Query().Get("period")
	endDate := time.Now()
	var startDate time.Time
	periodLabel := "Last 30 Days"

	switch period {
	case "7d":
		startDate = endDate.AddDate(0, 0, -7)
		periodLabel = "Last 7 Days"
	case "90d":
		startDate = endDate.AddDate(0, 0, -90)
		periodLabel = "Last 90 Days"
	case "mtd":
		startDate = time.Date(endDate.Year(), endDate.Month(), 1, 0, 0, 0, 0, time.UTC)
		periodLabel = "Month to Date"
	case "qtd":
		q := (endDate.Month() - 1) / 3 * 3
		startDate = time.Date(endDate.Year(), q+1, 1, 0, 0, 0, 0, time.UTC)
		periodLabel = "Quarter to Date"
	default:
		startDate = endDate.AddDate(0, 0, -30)
	}

	dateRange := model.DateRange{Start: startDate, End: endDate}

	// Get current period summary
	summary, err := h.costRepo.GetSummary(ctx, orgID, dateRange)
	if err != nil || summary == nil || summary.TotalCost == 0 {
		h.executiveSummaryMock(w, periodLabel, startDate, endDate)
		return
	}

	// Get previous period for comparison
	prevDuration := endDate.Sub(startDate)
	prevEnd := startDate
	prevStart := prevEnd.Add(-prevDuration)
	prevRange := model.DateRange{Start: prevStart, End: prevEnd}
	prevSummary, _ := h.costRepo.GetSummary(ctx, orgID, prevRange)

	changePct := 0.0
	previousCost := 0.0
	if prevSummary != nil && prevSummary.TotalCost > 0 {
		previousCost = prevSummary.TotalCost
		changePct = ((summary.TotalCost - previousCost) / previousCost) * 100
	}

	// Budget summary
	budgets, _ := h.budgetRepo.List(ctx, orgID)
	budgetCount := len(budgets)
	exceededCount := 0
	totalBudget := 0.0
	totalSpend := 0.0
	for _, b := range budgets {
		totalBudget += b.Amount
		totalSpend += b.CurrentSpend
		if b.Status == model.BudgetStatusExceeded {
			exceededCount++
		}
	}

	// Anomaly summary
	anomalySummary, _ := h.anomalyRepo.GetSummary(ctx, orgID)
	openAnomalies := 0
	criticalAnomalies := 0
	if anomalySummary != nil {
		openAnomalies = anomalySummary.OpenCount
		criticalAnomalies = anomalySummary.BySeverity[model.SeverityCritical]
	}

	days := endDate.Sub(startDate).Hours() / 24
	if days == 0 {
		days = 1
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"report_type":  "executive_summary",
		"period":       periodLabel,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"date_range": map[string]string{
			"start": startDate.Format("2006-01-02"),
			"end":   endDate.Format("2006-01-02"),
		},
		"cost_overview": map[string]any{
			"total_cost":      summary.TotalCost,
			"previous_cost":   previousCost,
			"change_percent":  changePct,
			"daily_average":   summary.TotalCost / days,
			"currency":        string(summary.Currency),
			"top_services":    summary.ByService,
			"services_count":  len(summary.ByService),
		},
		"budget_overview": map[string]any{
			"total_budgets":    budgetCount,
			"budgets_exceeded": exceededCount,
			"total_budget":     totalBudget,
			"total_spend":      totalSpend,
			"utilization_pct":  safePercent(totalSpend, totalBudget),
		},
		"anomaly_overview": map[string]any{
			"open_anomalies":     openAnomalies,
			"critical_anomalies": criticalAnomalies,
		},
	})
}

// CostComparison generates a period-over-period cost comparison report.
func (h *ReportHandler) CostComparison(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, _ := auth.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	compType := r.URL.Query().Get("type")
	endDate := time.Now()
	var currentStart, previousStart, previousEnd time.Time
	label := "Month over Month"

	switch compType {
	case "qoq":
		currentStart = time.Date(endDate.Year(), ((endDate.Month()-1)/3)*3+1, 1, 0, 0, 0, 0, time.UTC)
		previousEnd = currentStart.AddDate(0, 0, -1)
		previousStart = previousEnd.AddDate(0, -3, 1)
		label = "Quarter over Quarter"
	default: // mom
		currentStart = time.Date(endDate.Year(), endDate.Month(), 1, 0, 0, 0, 0, time.UTC)
		previousEnd = currentStart.AddDate(0, 0, -1)
		previousStart = time.Date(previousEnd.Year(), previousEnd.Month(), 1, 0, 0, 0, 0, time.UTC)
	}

	currentRange := model.DateRange{Start: currentStart, End: endDate}
	previousRange := model.DateRange{Start: previousStart, End: previousEnd}

	currentSummary, err1 := h.costRepo.GetSummary(ctx, orgID, currentRange)
	previousSummary, err2 := h.costRepo.GetSummary(ctx, orgID, previousRange)

	if err1 != nil || err2 != nil || currentSummary == nil || previousSummary == nil {
		h.costComparisonMock(w, label, currentStart, endDate, previousStart, previousEnd)
		return
	}

	changePct := 0.0
	if previousSummary.TotalCost > 0 {
		changePct = ((currentSummary.TotalCost - previousSummary.TotalCost) / previousSummary.TotalCost) * 100
	}

	// Service-level comparison
	prevByService := make(map[string]float64)
	for _, s := range previousSummary.ByService {
		prevByService[s.Name] = s.Amount
	}

	serviceComparison := make([]map[string]any, 0)
	for _, s := range currentSummary.ByService {
		prev := prevByService[s.Name]
		change := 0.0
		if prev > 0 {
			change = ((s.Amount - prev) / prev) * 100
		}
		serviceComparison = append(serviceComparison, map[string]any{
			"service":         s.Name,
			"current_cost":    s.Amount,
			"previous_cost":   prev,
			"change_percent":  change,
			"change_absolute": s.Amount - prev,
		})
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"report_type":  "cost_comparison",
		"comparison":   label,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"current_period": map[string]any{
			"start":      currentStart.Format("2006-01-02"),
			"end":        endDate.Format("2006-01-02"),
			"total_cost": currentSummary.TotalCost,
		},
		"previous_period": map[string]any{
			"start":      previousStart.Format("2006-01-02"),
			"end":        previousEnd.Format("2006-01-02"),
			"total_cost": previousSummary.TotalCost,
		},
		"change_percent":     changePct,
		"change_absolute":    currentSummary.TotalCost - previousSummary.TotalCost,
		"service_comparison": serviceComparison,
	})
}

// ExportReportCSV exports a cost report as CSV.
func (h *ReportHandler) ExportReportCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, _ := auth.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	groupBy := r.URL.Query().Get("group_by")
	if groupBy == "" {
		groupBy = "service"
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
	}

	breakdown, err := h.costRepo.GetBreakdown(ctx, filter, groupBy)

	filename := fmt.Sprintf("finopsmind-report-%s-%s.csv", groupBy, time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	// CSV header
	w.Write([]byte(fmt.Sprintf("%s,Amount,Percentage,Currency\n", groupBy)))

	if err != nil || breakdown == nil || len(breakdown.Items) == 0 {
		// Mock data
		mockItems := []struct{ name string; amount float64; pct float64 }{
			{"EC2", 2340.50, 45.2}, {"RDS", 1200.00, 23.2}, {"S3", 580.00, 11.2},
			{"Lambda", 420.00, 8.1}, {"CloudWatch", 320.00, 6.2}, {"Other", 318.30, 6.1},
		}
		for _, item := range mockItems {
			w.Write([]byte(fmt.Sprintf("%s,%.2f,%.1f,USD\n", item.name, item.amount, item.pct)))
		}
		return
	}

	for _, item := range breakdown.Items {
		w.Write([]byte(fmt.Sprintf("%s,%.4f,%.2f,%s\n", item.Name, item.Amount, item.Percentage, string(breakdown.Currency))))
	}
}

// ExportReportJSON exports a cost report as JSON.
func (h *ReportHandler) ExportReportJSON(w http.ResponseWriter, r *http.Request) {
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
	dateRange := model.DateRange{Start: startDate, End: endDate}

	summary, _ := h.costRepo.GetSummary(ctx, orgID, dateRange)

	filter := model.CostFilter{
		OrganizationID: orgID,
		DateRange:      dateRange,
		Granularity:    model.GranularityDaily,
	}
	trend, _ := h.costRepo.GetTrend(ctx, filter)

	report := map[string]any{
		"report_type":  "cost_report",
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"period": map[string]string{
			"start": startDate.Format("2006-01-02"),
			"end":   endDate.Format("2006-01-02"),
		},
	}

	if summary != nil {
		report["summary"] = summary
	}
	if trend != nil {
		report["trend"] = trend
	}

	filename := fmt.Sprintf("finopsmind-report-%s.json", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	json.NewEncoder(w).Encode(report)
}

// Mock data helpers

func (h *ReportHandler) executiveSummaryMock(w http.ResponseWriter, period string, start, end time.Time) {
	WriteJSON(w, http.StatusOK, map[string]any{
		"report_type":  "executive_summary",
		"period":       period,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"date_range": map[string]string{
			"start": start.Format("2006-01-02"),
			"end":   end.Format("2006-01-02"),
		},
		"cost_overview": map[string]any{
			"total_cost":     5178.80,
			"previous_cost":  4820.50,
			"change_percent": 7.4,
			"daily_average":  172.63,
			"currency":       "USD",
			"top_services": []map[string]any{
				{"name": "EC2", "amount": 2680.50, "percentage": 51.8},
				{"name": "RDS", "amount": 1295.00, "percentage": 25.0},
				{"name": "S3", "amount": 415.30, "percentage": 8.0},
				{"name": "Lambda", "amount": 362.00, "percentage": 7.0},
				{"name": "CloudWatch", "amount": 259.00, "percentage": 5.0},
				{"name": "Other", "amount": 167.00, "percentage": 3.2},
			},
			"services_count": 6,
		},
		"budget_overview": map[string]any{
			"total_budgets":    3,
			"budgets_exceeded": 0,
			"total_budget":     6000.00,
			"total_spend":      5178.80,
			"utilization_pct":  86.3,
		},
		"anomaly_overview": map[string]any{
			"open_anomalies":     3,
			"critical_anomalies": 1,
		},
	})
}

func (h *ReportHandler) costComparisonMock(w http.ResponseWriter, label string, cs, ce, ps, pe time.Time) {
	WriteJSON(w, http.StatusOK, map[string]any{
		"report_type":  "cost_comparison",
		"comparison":   label,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"current_period": map[string]any{
			"start": cs.Format("2006-01-02"), "end": ce.Format("2006-01-02"),
			"total_cost": 5178.80,
		},
		"previous_period": map[string]any{
			"start": ps.Format("2006-01-02"), "end": pe.Format("2006-01-02"),
			"total_cost": 4820.50,
		},
		"change_percent":  7.4,
		"change_absolute": 358.30,
		"service_comparison": []map[string]any{
			{"service": "EC2", "current_cost": 2680.50, "previous_cost": 2450.00, "change_percent": 9.4},
			{"service": "RDS", "current_cost": 1295.00, "previous_cost": 1200.00, "change_percent": 7.9},
			{"service": "S3", "current_cost": 415.30, "previous_cost": 400.00, "change_percent": 3.8},
			{"service": "Lambda", "current_cost": 362.00, "previous_cost": 380.50, "change_percent": -4.9},
		},
	})
}

func safePercent(value, total float64) float64 {
	if total == 0 {
		return 0
	}
	return (value / total) * 100
}
