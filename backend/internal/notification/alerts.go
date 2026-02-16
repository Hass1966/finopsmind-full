package notification

import (
	"context"
	"fmt"
)

// SendAnomalyAlert sends an anomaly detection alert.
func (s *Service) SendAnomalyAlert(ctx context.Context, service, provider string, severity string, deviation float64, actualCost, expectedCost float64) error {
	return s.Send(ctx, Message{
		EventType: EventAnomalyDetected,
		Title:     fmt.Sprintf("Cost Anomaly Detected: %s", service),
		Body:      fmt.Sprintf("Unusual spending detected on %s (%s). Cost was $%.2f vs expected $%.2f (%.1f%% deviation).", service, provider, actualCost, expectedCost, deviation),
		Severity:  severity,
		Data: map[string]any{
			"Service":       service,
			"Provider":      provider,
			"Actual Cost":   fmt.Sprintf("$%.2f", actualCost),
			"Expected Cost": fmt.Sprintf("$%.2f", expectedCost),
			"Deviation":     fmt.Sprintf("%.1f%%", deviation),
		},
	})
}

// SendBudgetAlert sends a budget threshold alert.
func (s *Service) SendBudgetAlert(ctx context.Context, budgetName string, amount, spent, threshold float64, exceeded bool) error {
	eventType := EventBudgetWarning
	title := fmt.Sprintf("Budget Warning: %s", budgetName)
	severity := "medium"
	body := fmt.Sprintf("Budget '%s' has reached %.0f%% of its $%.2f limit (current spend: $%.2f).", budgetName, (spent/amount)*100, amount, spent)

	if exceeded {
		eventType = EventBudgetExceeded
		title = fmt.Sprintf("Budget Exceeded: %s", budgetName)
		severity = "high"
		body = fmt.Sprintf("Budget '%s' has been exceeded. Limit: $%.2f, Current spend: $%.2f (%.0f%% over).", budgetName, amount, spent, ((spent-amount)/amount)*100)
	}

	return s.Send(ctx, Message{
		EventType: eventType,
		Title:     title,
		Body:      body,
		Severity:  severity,
		Data: map[string]any{
			"Budget":    budgetName,
			"Limit":     fmt.Sprintf("$%.2f", amount),
			"Spent":     fmt.Sprintf("$%.2f", spent),
			"Threshold": fmt.Sprintf("%.0f%%", threshold*100),
		},
	})
}

// SendRecommendationAlert sends a new recommendation alert.
func (s *Service) SendRecommendationAlert(ctx context.Context, resourceType, recType string, savings float64) error {
	return s.Send(ctx, Message{
		EventType: EventRecommendationNew,
		Title:     fmt.Sprintf("New Recommendation: %s", recType),
		Body:      fmt.Sprintf("New optimization found for %s. Estimated savings: $%.2f/month.", resourceType, savings),
		Severity:  "low",
		Data: map[string]any{
			"Resource Type": resourceType,
			"Type":          recType,
			"Savings":       fmt.Sprintf("$%.2f/mo", savings),
		},
	})
}
