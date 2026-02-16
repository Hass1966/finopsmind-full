package model

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CostRecord represents an individual cost record.
type CostRecord struct {
	BaseEntity
	OrganizationID uuid.UUID     `json:"organization_id" db:"organization_id"`
	Date           time.Time     `json:"date" db:"date"`
	Amount         float64       `json:"amount" db:"amount"`
	Currency       Currency      `json:"currency" db:"currency"`
	Provider       CloudProvider `json:"provider" db:"provider"`
	Service        string        `json:"service" db:"service"`
	AccountID      string        `json:"account_id" db:"account_id"`
	Region         string        `json:"region" db:"region"`
	ResourceID     string        `json:"resource_id" db:"resource_id"`
	Tags           Tags          `json:"tags" db:"tags"`
	Estimated      bool          `json:"estimated" db:"estimated"`
}

// CostFilter defines filter criteria for cost queries.
type CostFilter struct {
	OrganizationID uuid.UUID       `json:"organization_id"`
	DateRange      DateRange       `json:"date_range"`
	Granularity    Granularity     `json:"granularity"`
	Providers      []CloudProvider `json:"providers"`
	Services       []string        `json:"services"`
	AccountIDs     []string        `json:"account_ids"`
	Regions        []string        `json:"regions"`
	Tags           Tags            `json:"tags"`
}

// CostAggregation represents aggregated cost data.
type CostAggregation struct {
	Date        time.Time     `json:"date"`
	Total       float64       `json:"total"`
	Currency    Currency      `json:"currency"`
	Provider    CloudProvider `json:"provider"`
	Service     string        `json:"service"`
	RecordCount int           `json:"record_count"`
}

// CostBreakdown represents a cost breakdown by dimension.
type CostBreakdown struct {
	Dimension string              `json:"dimension"`
	Items     []CostBreakdownItem `json:"items"`
	Total     float64             `json:"total"`
	Currency  Currency            `json:"currency"`
}

// CostBreakdownItem represents a single item in a cost breakdown.
type CostBreakdownItem struct {
	Name       string  `json:"name"`
	Amount     float64 `json:"amount"`
	Percentage float64 `json:"percentage"`
}

// CostTrend represents cost trend data over a time range.
type CostTrend struct {
	DateRange    DateRange         `json:"date_range"`
	Granularity  Granularity       `json:"granularity"`
	DataPoints   []CostAggregation `json:"data_points"`
	TotalCost    float64           `json:"total_cost"`
	AvgDailyCost float64           `json:"avg_daily_cost"`
}

// CostSummary provides a high-level cost summary.
type CostSummary struct {
	TotalCost float64             `json:"total_cost"`
	Currency  Currency            `json:"currency"`
	DateRange DateRange           `json:"date_range"`
	ByService []CostBreakdownItem `json:"by_service"`
}

// DailyCost represents daily cost aggregation
type DailyCost struct {
	ID               int64              `json:"id" db:"id"`
	AccountID        string             `json:"account_id" db:"account_id"`
	Date             time.Time          `json:"date" db:"date"`
	TotalCost        float64            `json:"total_cost" db:"total_cost"`
	ServiceBreakdown map[string]float64 `json:"service_breakdown" db:"service_breakdown"`
	CreatedAt        time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at" db:"updated_at"`
}

// CostForecast represents a stored forecast
type CostForecast struct {
	ID                 int64     `json:"id" db:"id"`
	AccountID          string    `json:"account_id" db:"account_id"`
	GeneratedAt        time.Time `json:"generated_at" db:"generated_at"`
	ForecastStartDate  time.Time `json:"forecast_start_date" db:"forecast_start_date"`
	ForecastEndDate    time.Time `json:"forecast_end_date" db:"forecast_end_date"`
	Predictions        []byte    `json:"-" db:"predictions"` // JSON blob
	Summary            []byte    `json:"-" db:"summary"`     // JSON blob
	TotalPredictedCost float64   `json:"total_predicted_cost" db:"total_predicted_cost"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
}

// CostAnomaly represents a detected anomaly
type CostAnomaly struct {
	ID              int64              `json:"id" db:"id"`
	AccountID       string             `json:"account_id" db:"account_id"`
	Date            time.Time          `json:"date" db:"date"`
	Cost            float64            `json:"cost" db:"cost"`
	AnomalyScore    float64            `json:"anomaly_score" db:"anomaly_score"`
	Severity        string             `json:"severity" db:"severity"`
	RootCause       map[string]any     `json:"root_cause" db:"root_cause"`
	Acknowledged    bool               `json:"acknowledged" db:"acknowledged"`
	AcknowledgedBy  string             `json:"acknowledged_by,omitempty" db:"acknowledged_by"`
	AcknowledgedAt  *time.Time         `json:"acknowledged_at,omitempty" db:"acknowledged_at"`
	CreatedAt       time.Time          `json:"created_at" db:"created_at"`
}

// CostStore defines the interface for cost data storage
type CostStore interface {
	// Daily costs
	GetDailyCosts(ctx context.Context, accountID string, startDate, endDate time.Time) ([]DailyCost, error)
	SaveDailyCost(ctx context.Context, cost *DailyCost) error
	
	// Forecasts
	GetLatestForecast(ctx context.Context, accountID string) (*CostForecast, error)
	SaveForecast(ctx context.Context, forecast *CostForecast) error
	
	// Anomalies
	GetAnomalies(ctx context.Context, accountID string, startDate, endDate time.Time) ([]CostAnomaly, error)
	GetUnacknowledgedAnomalies(ctx context.Context, accountID string) ([]CostAnomaly, error)
	SaveAnomaly(ctx context.Context, anomaly *CostAnomaly) error
	AcknowledgeAnomaly(ctx context.Context, anomalyID int64, acknowledgedBy string) error
}
