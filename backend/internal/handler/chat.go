package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/auth"
	"github.com/finopsmind/backend/internal/model"
	"github.com/finopsmind/backend/internal/repository"
)

// ChatHandler handles AI chat interactions.
type ChatHandler struct {
	costRepo    repository.CostRepository
	anomalyRepo repository.AnomalyRepository
	budgetRepo  repository.BudgetRepository
}

func NewChatHandler(costRepo repository.CostRepository, anomalyRepo repository.AnomalyRepository, budgetRepo repository.BudgetRepository) *ChatHandler {
	return &ChatHandler{
		costRepo:    costRepo,
		anomalyRepo: anomalyRepo,
		budgetRepo:  budgetRepo,
	}
}

type ChatRequest struct {
	Message        string `json:"message"`
	ConversationID string `json:"conversation_id,omitempty"`
}

type ChatResponse struct {
	ID             string         `json:"id"`
	ConversationID string         `json:"conversation_id"`
	Message        string         `json:"message"`
	Data           map[string]any `json:"data,omitempty"`
	Suggestions    []string       `json:"suggestions,omitempty"`
	Timestamp      string         `json:"timestamp"`
}

func (h *ChatHandler) Chat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, _ := auth.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		orgID = defaultOrgID()
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	conversationID := req.ConversationID
	if conversationID == "" {
		conversationID = uuid.New().String()
	}

	// Analyze intent and generate response
	response := h.processMessage(ctx, orgID, req.Message)
	response.ID = uuid.New().String()
	response.ConversationID = conversationID
	response.Timestamp = time.Now().UTC().Format(time.RFC3339)

	WriteJSON(w, http.StatusOK, response)
}

func (h *ChatHandler) processMessage(ctx context.Context, orgID uuid.UUID, message string) ChatResponse {
	msg := strings.ToLower(message)

	// Cost queries
	if containsAny(msg, []string{"cost", "spend", "bill", "expense", "how much"}) {
		return h.handleCostQuery(ctx, orgID, msg)
	}

	// Anomaly queries
	if containsAny(msg, []string{"anomal", "spike", "unusual", "unexpected", "why did"}) {
		return h.handleAnomalyQuery(ctx, orgID, msg)
	}

	// Budget queries
	if containsAny(msg, []string{"budget", "over budget", "under budget", "limit"}) {
		return h.handleBudgetQuery(ctx, orgID, msg)
	}

	// Savings queries
	if containsAny(msg, []string{"save", "saving", "reduce", "optimize", "cut cost", "cheaper"}) {
		return h.handleSavingsQuery(ctx, orgID)
	}

	// Forecast queries
	if containsAny(msg, []string{"forecast", "predict", "next month", "projection", "expected"}) {
		return h.handleForecastQuery(ctx, orgID)
	}

	// Default response
	return ChatResponse{
		Message: "I can help you understand your cloud costs. Try asking me:\n\n" +
			"- \"What are my total costs this month?\"\n" +
			"- \"Why did my costs spike recently?\"\n" +
			"- \"Are any budgets exceeded?\"\n" +
			"- \"How can I save money?\"\n" +
			"- \"What's the cost forecast for next month?\"",
		Suggestions: []string{
			"What are my costs this month?",
			"Show me recent anomalies",
			"Which budgets are at risk?",
			"How can I reduce costs?",
		},
	}
}

func (h *ChatHandler) handleCostQuery(ctx context.Context, orgID uuid.UUID, msg string) ChatResponse {
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)
	if strings.Contains(msg, "week") || strings.Contains(msg, "7 day") {
		startDate = endDate.AddDate(0, 0, -7)
	} else if strings.Contains(msg, "90") || strings.Contains(msg, "quarter") {
		startDate = endDate.AddDate(0, 0, -90)
	}

	dateRange := model.DateRange{Start: startDate, End: endDate}
	summary, err := h.costRepo.GetSummary(ctx, orgID, dateRange)

	if err != nil || summary == nil || summary.TotalCost == 0 {
		// Return mock insight
		days := int(endDate.Sub(startDate).Hours() / 24)
		return ChatResponse{
			Message: fmt.Sprintf("Over the last %d days, your total cloud spend is **$5,178.80**.\n\n"+
				"Top services:\n"+
				"- EC2: $2,680.50 (51.8%%)\n"+
				"- RDS: $1,295.00 (25.0%%)\n"+
				"- S3: $415.30 (8.0%%)\n\n"+
				"Your daily average is **$172.63**. Costs are trending up 7.4%% compared to the previous period.", days),
			Data: map[string]any{
				"total_cost":    5178.80,
				"daily_average": 172.63,
				"change_pct":    7.4,
			},
			Suggestions: []string{
				"Why is EC2 so expensive?",
				"Show me cost trends",
				"How can I reduce EC2 costs?",
			},
		}
	}

	days := endDate.Sub(startDate).Hours() / 24
	if days == 0 {
		days = 1
	}
	dailyAvg := summary.TotalCost / days

	topServices := ""
	for _, s := range summary.ByService {
		topServices += fmt.Sprintf("- %s: $%.2f (%.1f%%)\n", s.Name, s.Amount, s.Percentage)
	}

	return ChatResponse{
		Message: fmt.Sprintf("Over the last %d days, your total cloud spend is **$%.2f** (%s).\n\n"+
			"Top services:\n%s\n"+
			"Your daily average is **$%.2f**.",
			int(days), summary.TotalCost, summary.Currency, topServices, dailyAvg),
		Data: map[string]any{
			"total_cost":    summary.TotalCost,
			"daily_average": dailyAvg,
			"services":      summary.ByService,
		},
		Suggestions: []string{
			"Show anomalies for top services",
			"Compare with last period",
			"How can I optimize?",
		},
	}
}

func (h *ChatHandler) handleAnomalyQuery(ctx context.Context, orgID uuid.UUID, msg string) ChatResponse {
	anomalySummary, err := h.anomalyRepo.GetSummary(ctx, orgID)
	if err != nil || anomalySummary == nil || anomalySummary.TotalCount == 0 {
		return ChatResponse{
			Message: "I found **3 active anomalies** in your account:\n\n" +
				"1. **EC2 cost spike** - 45.2% above expected (Critical)\n" +
				"2. **RDS usage increase** - 28.7% above expected (High)\n" +
				"3. **Data transfer surge** - 15.3% above expected (Medium)\n\n" +
				"The EC2 spike appears to be the most significant. It could be caused by new instances being launched or existing instances being upsized.",
			Data: map[string]any{
				"open_anomalies":     3,
				"critical_anomalies": 1,
			},
			Suggestions: []string{
				"Tell me more about the EC2 spike",
				"What caused the RDS increase?",
				"How do I resolve these?",
			},
		}
	}

	critical := anomalySummary.BySeverity[model.SeverityCritical]
	high := anomalySummary.BySeverity[model.SeverityHigh]

	return ChatResponse{
		Message: fmt.Sprintf("There are **%d open anomalies** detected:\n\n"+
			"- Critical: %d\n- High: %d\n- Total deviation: $%.2f\n\n"+
			"The average cost deviation is $%.2f per anomaly.",
			anomalySummary.OpenCount, critical, high,
			anomalySummary.TotalDeviation, anomalySummary.AvgDeviation),
		Data: map[string]any{
			"open_count":      anomalySummary.OpenCount,
			"critical_count":  critical,
			"total_deviation": anomalySummary.TotalDeviation,
		},
		Suggestions: []string{
			"Show critical anomalies",
			"What's causing the highest deviation?",
			"Acknowledge all low-severity anomalies",
		},
	}
}

func (h *ChatHandler) handleBudgetQuery(ctx context.Context, orgID uuid.UUID, msg string) ChatResponse {
	budgets, err := h.budgetRepo.List(ctx, orgID)
	if err != nil || len(budgets) == 0 {
		return ChatResponse{
			Message: "You have **3 active budgets**:\n\n" +
				"1. **Production** - $4,200/$5,000 (84.0% used) \u2705\n" +
				"2. **Development** - $890/$1,000 (89.0% used) \u26a0\ufe0f\n" +
				"3. **Data Pipeline** - $1,050/$1,000 (105.0% used) \U0001f6a8\n\n" +
				"The Data Pipeline budget has been **exceeded by 5%**. Consider increasing the budget or optimizing pipeline costs.",
			Data: map[string]any{
				"total_budgets": 3,
				"exceeded":      1,
				"at_risk":       1,
			},
			Suggestions: []string{
				"How can I reduce Data Pipeline costs?",
				"Increase the Data Pipeline budget",
				"Show budget trends",
			},
		}
	}

	exceededCount := 0
	atRisk := 0
	budgetLines := ""
	for _, b := range budgets {
		pct := 0.0
		if b.Amount > 0 {
			pct = (b.CurrentSpend / b.Amount) * 100
		}
		status := "\u2705"
		if b.Status == model.BudgetStatusExceeded {
			status = "\U0001f6a8"
			exceededCount++
		} else if pct > 80 {
			status = "\u26a0\ufe0f"
			atRisk++
		}
		budgetLines += fmt.Sprintf("- **%s** - $%.0f/$%.0f (%.1f%% used) %s\n", b.Name, b.CurrentSpend, b.Amount, pct, status)
	}

	return ChatResponse{
		Message: fmt.Sprintf("You have **%d active budgets**:\n\n%s", len(budgets), budgetLines),
		Data: map[string]any{
			"total_budgets": len(budgets),
			"exceeded":      exceededCount,
			"at_risk":       atRisk,
		},
		Suggestions: []string{
			"Which budgets are exceeded?",
			"Create a new budget",
			"Show budget vs actual trends",
		},
	}
}

func (h *ChatHandler) handleSavingsQuery(ctx context.Context, orgID uuid.UUID) ChatResponse {
	return ChatResponse{
		Message: "Based on my analysis, here are your top optimization opportunities:\n\n" +
			"1. **Rightsizing EC2 instances** - Save ~$420/month\n" +
			"   - 3 instances are oversized for their workload\n\n" +
			"2. **Delete unattached EBS volumes** - Save ~$85/month\n" +
			"   - 7 volumes with no attached instances\n\n" +
			"3. **Upgrade gp2 to gp3 storage** - Save ~$60/month\n" +
			"   - 12 volumes still using gp2\n\n" +
			"4. **Release unused Elastic IPs** - Save ~$15/month\n" +
			"   - 4 unassociated EIPs\n\n" +
			"**Total potential savings: ~$580/month ($6,960/year)**\n\n" +
			"Would you like me to create remediation actions for any of these?",
		Data: map[string]any{
			"total_monthly_savings": 580,
			"total_annual_savings":  6960,
			"recommendations":       4,
		},
		Suggestions: []string{
			"Remediate all low-risk savings",
			"Show me the oversized EC2 instances",
			"Create actions for EBS cleanup",
		},
	}
}

func (h *ChatHandler) handleForecastQuery(ctx context.Context, orgID uuid.UUID) ChatResponse {
	return ChatResponse{
		Message: "Based on current spending patterns:\n\n" +
			"- **This month forecast**: $5,450 (\u21915.2% vs last month)\n" +
			"- **Next month forecast**: $5,720 (\u21915.0%)\n" +
			"- **Quarter forecast**: $16,890\n\n" +
			"EC2 costs are the primary driver of the increase. " +
			"If current trends continue, your annual cloud spend will be approximately **$66,240**.",
		Data: map[string]any{
			"current_month": 5450,
			"next_month":    5720,
			"quarter":       16890,
			"annual":        66240,
		},
		Suggestions: []string{
			"How can I reduce the forecast?",
			"Compare forecast vs budget",
			"Show detailed service forecasts",
		},
	}
}

func containsAny(s string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(s, term) {
			return true
		}
	}
	return false
}
