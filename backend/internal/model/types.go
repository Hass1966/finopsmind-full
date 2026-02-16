// Package model contains the core domain entities for FinOpsMind.
package model

import (
	"time"

	"github.com/google/uuid"
)

// CloudProvider represents supported cloud providers.
type CloudProvider string

const (
	CloudProviderAWS   CloudProvider = "aws"
	CloudProviderAzure CloudProvider = "azure"
	CloudProviderGCP   CloudProvider = "gcp"
)

// Currency represents monetary currency codes.
type Currency string

const (
	CurrencyUSD Currency = "USD"
	CurrencyEUR Currency = "EUR"
	CurrencyGBP Currency = "GBP"
)

// Granularity represents time granularity for cost data.
type Granularity string

const (
	GranularityHourly  Granularity = "hourly"
	GranularityDaily   Granularity = "daily"
	GranularityWeekly  Granularity = "weekly"
	GranularityMonthly Granularity = "monthly"
)

// Severity represents alert/anomaly severity levels.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Status represents entity lifecycle status.
type Status string

const (
	StatusOpen         Status = "open"
	StatusAcknowledged Status = "acknowledged"
	StatusResolved     Status = "resolved"
	StatusDismissed    Status = "dismissed"
	StatusPending      Status = "pending"
	StatusAccepted     Status = "accepted"
	StatusRejected     Status = "rejected"
	StatusImplemented  Status = "implemented"
)

// Money represents a monetary value with currency.
type Money struct {
	Amount   float64  `json:"amount"`
	Currency Currency `json:"currency"`
}

// DateRange represents a time period.
type DateRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Tags represents key-value metadata.
type Tags map[string]string

// Pagination holds pagination parameters.
type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

// BaseEntity contains common fields for all entities.
type BaseEntity struct {
	ID        uuid.UUID `json:"id" db:"id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// NewBaseEntity creates a new BaseEntity with generated ID and timestamps.
func NewBaseEntity() BaseEntity {
	now := time.Now().UTC()
	return BaseEntity{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}
