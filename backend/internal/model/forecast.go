package model

import (
	"time"

	"github.com/google/uuid"
)

// Forecast represents a cost forecast.
type Forecast struct {
	BaseEntity
	OrganizationID   uuid.UUID         `json:"organization_id" db:"organization_id"`
	GeneratedAt      time.Time         `json:"generated_at" db:"generated_at"`
	ModelVersion     string            `json:"model_version" db:"model_version"`
	Granularity      Granularity       `json:"granularity" db:"granularity"`
	Predictions      []ForecastPoint   `json:"predictions" db:"predictions"`
	TotalForecasted  float64           `json:"total_forecasted" db:"total_forecasted"`
	ConfidenceLevel  float64           `json:"confidence_level" db:"confidence_level"`
	Currency         Currency          `json:"currency" db:"currency"`
	ServiceFilter    string            `json:"service_filter,omitempty" db:"service_filter"`
	AccountFilter    string            `json:"account_filter,omitempty" db:"account_filter"`
}

// ForecastPoint represents a single forecast data point.
type ForecastPoint struct {
	Date       time.Time `json:"date"`
	Predicted  float64   `json:"predicted"`
	LowerBound float64   `json:"lower_bound"`
	UpperBound float64   `json:"upper_bound"`
}

// ForecastRequest represents a request for forecasting.
type ForecastRequest struct {
	OrganizationID string      `json:"organization_id" validate:"required"`
	ForecastDays   int         `json:"forecast_days" validate:"min=7,max=90"`
	Granularity    Granularity `json:"granularity"`
	ServiceFilter  string      `json:"service_filter,omitempty"`
	AccountFilter  string      `json:"account_filter,omitempty"`
}

// ForecastResponse represents the response from ML sidecar.
type ForecastResponse struct {
	OrganizationID  string          `json:"organization_id"`
	GeneratedAt     time.Time       `json:"generated_at"`
	ModelVersion    string          `json:"model_version"`
	Forecasts       []ForecastPoint `json:"forecasts"`
	TotalForecasted float64         `json:"total_forecasted"`
	ConfidenceLevel float64         `json:"confidence_level"`
}

// ForecastSummary provides a summary of forecasts.
type ForecastSummary struct {
	CurrentMonthForecast float64   `json:"current_month_forecast"`
	NextMonthForecast    float64   `json:"next_month_forecast"`
	QuarterForecast      float64   `json:"quarter_forecast"`
	TrendDirection       string    `json:"trend_direction"`
	ChangePercent        float64   `json:"change_percent"`
	Currency             Currency  `json:"currency"`
	GeneratedAt          time.Time `json:"generated_at"`
}

// ForecastComparison compares forecast to budget.
type ForecastComparison struct {
	BudgetID          uuid.UUID `json:"budget_id"`
	BudgetName        string    `json:"budget_name"`
	BudgetAmount      float64   `json:"budget_amount"`
	ForecastedSpend   float64   `json:"forecasted_spend"`
	Variance          float64   `json:"variance"`
	VariancePercent   float64   `json:"variance_percent"`
	ProjectedStatus   string    `json:"projected_status"`
}
