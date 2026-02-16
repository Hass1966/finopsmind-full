package model

import (
	"time"

	"github.com/google/uuid"
)

// Organization represents a tenant organization.
type Organization struct {
	BaseEntity
	Name     string            `json:"name" db:"name"`
	Settings OrganizationSettings `json:"settings" db:"settings"`
}

// OrganizationSettings holds organization-level settings.
type OrganizationSettings struct {
	DefaultCurrency  Currency `json:"default_currency"`
	Timezone         string   `json:"timezone"`
	FiscalYearStart  int      `json:"fiscal_year_start"`
	AlertsEnabled    bool     `json:"alerts_enabled"`
	SlackWebhookURL  string   `json:"slack_webhook_url,omitempty"`
	EmailRecipients  []string `json:"email_recipients,omitempty"`
}

// OrganizationCreateRequest represents a request to create an organization.
type OrganizationCreateRequest struct {
	Name     string               `json:"name" validate:"required,min=1,max=255"`
	Settings OrganizationSettings `json:"settings"`
}

// Alert represents a system alert.
type Alert struct {
	BaseEntity
	OrganizationID uuid.UUID `json:"organization_id" db:"organization_id"`
	Type           AlertType `json:"type" db:"type"`
	Severity       Severity  `json:"severity" db:"severity"`
	Status         Status    `json:"status" db:"status"`
	Title          string    `json:"title" db:"title"`
	Message        string    `json:"message" db:"message"`
	ResourceType   string    `json:"resource_type,omitempty" db:"resource_type"`
	ResourceID     string    `json:"resource_id,omitempty" db:"resource_id"`
	Metadata       map[string]any `json:"metadata,omitempty" db:"metadata"`
	TriggeredAt    time.Time `json:"triggered_at" db:"triggered_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty" db:"acknowledged_at"`
	AcknowledgedBy string    `json:"acknowledged_by,omitempty" db:"acknowledged_by"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty" db:"resolved_at"`
}

// AlertType represents types of alerts.
type AlertType string

const (
	AlertTypeBudget    AlertType = "budget"
	AlertTypeAnomaly   AlertType = "anomaly"
	AlertTypeForecast  AlertType = "forecast"
	AlertTypeCostSpike AlertType = "cost_spike"
)

// AlertFilter defines filtering options for alerts.
type AlertFilter struct {
	OrganizationID uuid.UUID
	Types          []AlertType
	Severities     []Severity
	Statuses       []Status
	DateRange      DateRange
}

// AlertRule represents a rule that triggers alerts.
type AlertRule struct {
	BaseEntity
	OrganizationID       uuid.UUID `json:"organization_id" db:"organization_id"`
	Name                 string    `json:"name" db:"name"`
	Type                 AlertType `json:"type" db:"type"`
	Enabled              bool      `json:"enabled" db:"enabled"`
	Condition            AlertCondition `json:"condition" db:"condition"`
	NotificationChannels []string  `json:"notification_channels" db:"notification_channels"`
}

// AlertCondition defines when an alert should trigger.
type AlertCondition struct {
	Metric    string  `json:"metric"`
	Operator  string  `json:"operator"`
	Threshold float64 `json:"threshold"`
	Window    string  `json:"window"`
}
