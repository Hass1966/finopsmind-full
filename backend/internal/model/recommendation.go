package model

import (
	"github.com/google/uuid"
)

// RecommendationType represents types of recommendations.
type RecommendationType string

const (
	RecommendationTypeRightsizing       RecommendationType = "rightsizing"
	RecommendationTypeReservedInstances RecommendationType = "reserved_instances"
	RecommendationTypeSavingsPlans      RecommendationType = "savings_plans"
	RecommendationTypeIdleResources     RecommendationType = "idle_resources"
	RecommendationTypeStorageOptimization RecommendationType = "storage_optimization"
	RecommendationTypeNetworkOptimization RecommendationType = "network_optimization"
)

// Impact represents the impact level of a recommendation.
type Impact string

const (
	ImpactHigh   Impact = "high"
	ImpactMedium Impact = "medium"
	ImpactLow    Impact = "low"
)

// Recommendation represents a cost optimization recommendation.
type Recommendation struct {
	BaseEntity
	OrganizationID      uuid.UUID          `json:"organization_id" db:"organization_id"`
	Type                RecommendationType `json:"type" db:"type"`
	Provider            CloudProvider      `json:"provider" db:"provider"`
	AccountID           string             `json:"account_id,omitempty" db:"account_id"`
	Region              string             `json:"region,omitempty" db:"region"`
	ResourceID          string             `json:"resource_id,omitempty" db:"resource_id"`
	ResourceType        string             `json:"resource_type,omitempty" db:"resource_type"`
	CurrentConfig       string             `json:"current_config,omitempty" db:"current_config"`
	RecommendedConfig   string             `json:"recommended_config,omitempty" db:"recommended_config"`
	EstimatedSavings    float64            `json:"estimated_savings" db:"estimated_savings"`
	EstimatedSavingsPct float64            `json:"estimated_savings_pct" db:"estimated_savings_pct"`
	Currency            Currency           `json:"currency" db:"currency"`
	Impact              Impact             `json:"impact" db:"impact"`
	Effort              string             `json:"effort,omitempty" db:"effort"`
	Risk                string             `json:"risk,omitempty" db:"risk"`
	Status              Status             `json:"status" db:"status"`
	Details             map[string]any     `json:"details,omitempty" db:"details"`
	Notes               string             `json:"notes,omitempty" db:"notes"`
	ImplementedBy       string             `json:"implemented_by,omitempty" db:"implemented_by"`
}

// RecommendationFilter defines filtering options.
type RecommendationFilter struct {
	OrganizationID uuid.UUID
	Types          []RecommendationType
	Providers      []CloudProvider
	Impacts        []Impact
	Statuses       []Status
	MinSavings     float64
}

// RecommendationUpdateRequest represents a request to update a recommendation.
type RecommendationUpdateRequest struct {
	Status *Status `json:"status,omitempty"`
	Notes  *string `json:"notes,omitempty"`
}

// RecommendationSummary provides a summary of recommendations.
type RecommendationSummary struct {
	TotalCount       int                    `json:"total_count"`
	PendingCount     int                    `json:"pending_count"`
	TotalSavings     float64                `json:"total_savings"`
	ImplementedSavings float64              `json:"implemented_savings"`
	ByType           map[RecommendationType]int `json:"by_type"`
	ByImpact         map[Impact]int         `json:"by_impact"`
	Currency         Currency               `json:"currency"`
}
