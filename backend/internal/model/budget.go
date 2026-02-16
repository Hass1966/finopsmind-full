package model

import (
	"time"

	"github.com/google/uuid"
)

// BudgetPeriod represents budget time periods.
type BudgetPeriod string

const (
	BudgetPeriodMonthly   BudgetPeriod = "monthly"
	BudgetPeriodQuarterly BudgetPeriod = "quarterly"
	BudgetPeriodYearly    BudgetPeriod = "yearly"
)

// BudgetStatus represents budget status.
type BudgetStatus string

const (
	BudgetStatusActive   BudgetStatus = "active"
	BudgetStatusWarning  BudgetStatus = "warning"
	BudgetStatusExceeded BudgetStatus = "exceeded"
	BudgetStatusInactive BudgetStatus = "inactive"
)

// Budget represents a cost budget.
type Budget struct {
	BaseEntity
	OrganizationID uuid.UUID         `json:"organization_id" db:"organization_id"`
	Name           string            `json:"name" db:"name"`
	Amount         float64           `json:"amount" db:"amount"`
	Currency       Currency          `json:"currency" db:"currency"`
	Period         BudgetPeriod      `json:"period" db:"period"`
	Filters        BudgetFilters     `json:"filters" db:"filters"`
	Thresholds     []BudgetThreshold `json:"thresholds" db:"thresholds"`
	Status         BudgetStatus      `json:"status" db:"status"`
	CurrentSpend   float64           `json:"current_spend" db:"current_spend"`
	ForecastedSpend float64          `json:"forecasted_spend" db:"forecasted_spend"`
	StartDate      time.Time         `json:"start_date" db:"start_date"`
	EndDate        time.Time         `json:"end_date" db:"end_date"`
}

// BudgetFilters defines what costs are included in a budget.
type BudgetFilters struct {
	Providers  []CloudProvider `json:"providers,omitempty"`
	Services   []string        `json:"services,omitempty"`
	AccountIDs []string        `json:"account_ids,omitempty"`
	Regions    []string        `json:"regions,omitempty"`
	Tags       Tags            `json:"tags,omitempty"`
}

// BudgetThreshold defines an alert threshold.
type BudgetThreshold struct {
	Percentage           float64  `json:"percentage"`
	NotificationChannels []string `json:"notification_channels"`
	Triggered            bool     `json:"triggered"`
	TriggeredAt          *time.Time `json:"triggered_at,omitempty"`
}

// BudgetCreateRequest represents a request to create a budget.
type BudgetCreateRequest struct {
	Name       string            `json:"name" validate:"required,min=1,max=255"`
	Amount     float64           `json:"amount" validate:"required,gt=0"`
	Currency   Currency          `json:"currency"`
	Period     BudgetPeriod      `json:"period" validate:"required"`
	Filters    BudgetFilters     `json:"filters"`
	Thresholds []BudgetThreshold `json:"thresholds"`
}

// BudgetUpdateRequest represents a request to update a budget.
type BudgetUpdateRequest struct {
	Name       *string           `json:"name,omitempty"`
	Amount     *float64          `json:"amount,omitempty"`
	Thresholds []BudgetThreshold `json:"thresholds,omitempty"`
	Status     *BudgetStatus     `json:"status,omitempty"`
}

// BudgetSummary provides a summary of all budgets.
type BudgetSummary struct {
	TotalBudgets    int     `json:"total_budgets"`
	TotalBudgeted   float64 `json:"total_budgeted"`
	TotalSpent      float64 `json:"total_spent"`
	TotalForecasted float64 `json:"total_forecasted"`
	AtRiskCount     int     `json:"at_risk_count"`
	ExceededCount   int     `json:"exceeded_count"`
}
