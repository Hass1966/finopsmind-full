// Package provider defines cloud provider interfaces and types.
package provider

import (
	"context"
	"time"

	"github.com/finopsmind/backend/internal/model"
)

// Provider defines the interface for cloud providers.
type Provider interface {
	// Name returns the provider name.
	Name() string

	// Type returns the provider type (aws, azure, gcp).
	Type() model.CloudProvider

	// Health checks provider connectivity.
	Health(ctx context.Context) HealthStatus

	// GetCosts retrieves cost data for the given request.
	GetCosts(ctx context.Context, req CostRequest) (*CostResponse, error)

	// GetRecommendations retrieves optimization recommendations.
	GetRecommendations(ctx context.Context, req RecommendationRequest) (*RecommendationResponse, error)

	// Close cleans up provider resources.
	Close() error
}

// HealthStatus represents provider health.
type HealthStatus struct {
	Healthy     bool      `json:"healthy"`
	Message     string    `json:"message"`
	LastChecked time.Time `json:"last_checked"`
	Details     map[string]any `json:"details,omitempty"`
}

// CostRequest defines parameters for cost queries.
type CostRequest struct {
	StartDate   time.Time
	EndDate     time.Time
	Granularity model.Granularity
	GroupBy     []string // service, account, region
	Filters     CostFilters
}

// CostFilters defines cost query filters.
type CostFilters struct {
	Services   []string
	AccountIDs []string
	Regions    []string
	Tags       map[string]string
}

// CostResponse contains cost query results.
type CostResponse struct {
	Costs       []CostItem `json:"costs"`
	TotalAmount float64    `json:"total_amount"`
	Currency    string     `json:"currency"`
	StartDate   time.Time  `json:"start_date"`
	EndDate     time.Time  `json:"end_date"`
}

// CostItem represents a single cost data point.
type CostItem struct {
	Date      time.Time `json:"date"`
	Amount    float64   `json:"amount"`
	Service   string    `json:"service,omitempty"`
	AccountID string    `json:"account_id,omitempty"`
	Region    string    `json:"region,omitempty"`
}

// RecommendationType defines types of recommendations.
type RecommendationType string

const (
	RecommendationTypeRightsizing       RecommendationType = "rightsizing"
	RecommendationTypeReservedInstances RecommendationType = "reserved_instances"
	RecommendationTypeSavingsPlans      RecommendationType = "savings_plans"
	RecommendationTypeIdleResources     RecommendationType = "idle_resources"
)

// RecommendationRequest defines parameters for recommendation queries.
type RecommendationRequest struct {
	Types      []RecommendationType
	AccountIDs []string
	Regions    []string
}

// RecommendationResponse contains recommendation query results.
type RecommendationResponse struct {
	Recommendations []Recommendation `json:"recommendations"`
	TotalSavings    float64          `json:"total_savings"`
	Currency        string           `json:"currency"`
}

// Recommendation represents an optimization recommendation.
type Recommendation struct {
	ID                string             `json:"id"`
	Type              RecommendationType `json:"type"`
	ResourceID        string             `json:"resource_id"`
	ResourceType      string             `json:"resource_type"`
	AccountID         string             `json:"account_id"`
	Region            string             `json:"region"`
	CurrentConfig     string             `json:"current_config"`
	RecommendedConfig string             `json:"recommended_config"`
	EstimatedSavings  float64            `json:"estimated_savings"`
	Currency          string             `json:"currency"`
	Impact            string             `json:"impact"`
	Details           map[string]any     `json:"details,omitempty"`
}

// Registry manages registered providers.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry creates a new provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry.
func (r *Registry) Register(name string, provider Provider) {
	r.providers[name] = provider
}

// Get retrieves a provider by name.
func (r *Registry) Get(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// All returns all registered providers.
func (r *Registry) All() map[string]Provider {
	return r.providers
}

// Names returns all provider names.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// HealthAll checks health of all providers.
func (r *Registry) HealthAll(ctx context.Context) map[string]HealthStatus {
	health := make(map[string]HealthStatus)
	for name, provider := range r.providers {
		health[name] = provider.Health(ctx)
	}
	return health
}

// Close closes all providers.
func (r *Registry) Close() error {
	for _, provider := range r.providers {
		provider.Close()
	}
	return nil
}
