package model

import (
	"time"

	"github.com/google/uuid"
)

// Anomaly represents a detected cost anomaly.
type Anomaly struct {
	BaseEntity
	OrganizationID uuid.UUID     `json:"organization_id" db:"organization_id"`
	Date           time.Time     `json:"date" db:"date"`
	ActualAmount   float64       `json:"actual_amount" db:"actual_amount"`
	ExpectedAmount float64       `json:"expected_amount" db:"expected_amount"`
	Deviation      float64       `json:"deviation" db:"deviation"`
	DeviationPct   float64       `json:"deviation_pct" db:"deviation_pct"`
	Score          float64       `json:"score" db:"score"`
	Severity       Severity      `json:"severity" db:"severity"`
	Status         Status        `json:"status" db:"status"`
	Provider       CloudProvider `json:"provider,omitempty" db:"provider"`
	Service        string        `json:"service,omitempty" db:"service"`
	AccountID      string        `json:"account_id,omitempty" db:"account_id"`
	Region         string        `json:"region,omitempty" db:"region"`
	RootCause      string        `json:"root_cause,omitempty" db:"root_cause"`
	Notes          string        `json:"notes,omitempty" db:"notes"`
	DetectedAt     time.Time     `json:"detected_at" db:"detected_at"`
	AcknowledgedAt *time.Time    `json:"acknowledged_at,omitempty" db:"acknowledged_at"`
	AcknowledgedBy string        `json:"acknowledged_by,omitempty" db:"acknowledged_by"`
	ResolvedAt     *time.Time    `json:"resolved_at,omitempty" db:"resolved_at"`
}

// AnomalyFilter defines filtering options for anomaly queries.
type AnomalyFilter struct {
	OrganizationID uuid.UUID
	DateRange      DateRange
	Severities     []Severity
	Statuses       []Status
	Providers      []CloudProvider
	Services       []string
}

// AnomalyDetectionRequest represents a request to detect anomalies.
type AnomalyDetectionRequest struct {
	OrganizationID string  `json:"organization_id" validate:"required"`
	LookbackDays   int     `json:"lookback_days" validate:"min=7,max=90"`
	Sensitivity    float64 `json:"sensitivity" validate:"min=0.01,max=0.5"`
	ServiceFilter  string  `json:"service_filter,omitempty"`
	AccountFilter  string  `json:"account_filter,omitempty"`
}

// AnomalyUpdateRequest represents a request to update an anomaly.
type AnomalyUpdateRequest struct {
	Status    *Status `json:"status,omitempty"`
	RootCause *string `json:"root_cause,omitempty"`
	Notes     *string `json:"notes,omitempty"`
}

// AnomalySummary provides a summary of anomalies.
type AnomalySummary struct {
	TotalCount      int            `json:"total_count"`
	OpenCount       int            `json:"open_count"`
	BySeverity      map[Severity]int `json:"by_severity"`
	TotalDeviation  float64        `json:"total_deviation"`
	AvgDeviation    float64        `json:"avg_deviation"`
}

// ClassifyAnomalySeverity determines severity based on deviation percentage.
func ClassifyAnomalySeverity(deviationPct float64) Severity {
	absDev := deviationPct
	if absDev < 0 {
		absDev = -absDev
	}
	
	switch {
	case absDev >= 100:
		return SeverityCritical
	case absDev >= 50:
		return SeverityHigh
	case absDev >= 25:
		return SeverityMedium
	default:
		return SeverityLow
	}
}
