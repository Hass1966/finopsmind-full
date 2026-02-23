// Package repository defines data access interfaces.
package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/model"
)

// CostRepository defines cost data access methods.
type CostRepository interface {
	Create(ctx context.Context, cost *model.CostRecord) error
	CreateBatch(ctx context.Context, costs []*model.CostRecord) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.CostRecord, error)
	List(ctx context.Context, filter model.CostFilter, pagination model.Pagination) ([]*model.CostRecord, int, error)
	GetAggregated(ctx context.Context, filter model.CostFilter) ([]model.CostAggregation, error)
	GetBreakdown(ctx context.Context, filter model.CostFilter, dimension string) (*model.CostBreakdown, error)
	GetTrend(ctx context.Context, filter model.CostFilter) (*model.CostTrend, error)
	GetSummary(ctx context.Context, orgID uuid.UUID, dateRange model.DateRange) (*model.CostSummary, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// BudgetRepository defines budget data access methods.
type BudgetRepository interface {
	Create(ctx context.Context, budget *model.Budget) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Budget, error)
	List(ctx context.Context, orgID uuid.UUID) ([]*model.Budget, error)
	Update(ctx context.Context, budget *model.Budget) error
	Delete(ctx context.Context, id uuid.UUID) error
	UpdateSpend(ctx context.Context, id uuid.UUID, currentSpend, forecastedSpend float64) error
	GetSummary(ctx context.Context, orgID uuid.UUID) (*model.BudgetSummary, error)
}

// AnomalyRepository defines anomaly data access methods.
type AnomalyRepository interface {
	Create(ctx context.Context, anomaly *model.Anomaly) error
	CreateBatch(ctx context.Context, anomalies []*model.Anomaly) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Anomaly, error)
	List(ctx context.Context, filter model.AnomalyFilter, pagination model.Pagination) ([]*model.Anomaly, int, error)
	Update(ctx context.Context, anomaly *model.Anomaly) error
	Acknowledge(ctx context.Context, id uuid.UUID, acknowledgedBy string) error
	Resolve(ctx context.Context, id uuid.UUID) error
	GetSummary(ctx context.Context, orgID uuid.UUID) (*model.AnomalySummary, error)
}

// RecommendationRepository defines recommendation data access methods.
type RecommendationRepository interface {
	Create(ctx context.Context, rec *model.Recommendation) error
	CreateBatch(ctx context.Context, recs []*model.Recommendation) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Recommendation, error)
	List(ctx context.Context, filter model.RecommendationFilter, pagination model.Pagination) ([]*model.Recommendation, int, error)
	Update(ctx context.Context, rec *model.Recommendation) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.Status) error
	GetSummary(ctx context.Context, orgID uuid.UUID) (*model.RecommendationSummary, error)
}

// ForecastRepository defines forecast data access methods.
type ForecastRepository interface {
	Create(ctx context.Context, forecast *model.Forecast) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Forecast, error)
	GetLatest(ctx context.Context, orgID uuid.UUID) (*model.Forecast, error)
	List(ctx context.Context, orgID uuid.UUID, pagination model.Pagination) ([]*model.Forecast, int, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// AlertRepository defines alert data access methods.
type AlertRepository interface {
	Create(ctx context.Context, alert *model.Alert) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Alert, error)
	List(ctx context.Context, filter model.AlertFilter, pagination model.Pagination) ([]*model.Alert, int, error)
	Update(ctx context.Context, alert *model.Alert) error
	Acknowledge(ctx context.Context, id uuid.UUID, acknowledgedBy string) error
	Resolve(ctx context.Context, id uuid.UUID) error
}

// OrganizationRepository defines organization data access methods.
type OrganizationRepository interface {
	Create(ctx context.Context, org *model.Organization) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Organization, error)
	List(ctx context.Context) ([]*model.Organization, error)
	Update(ctx context.Context, org *model.Organization) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// CloudProviderRepository manages stored cloud provider configurations.
type CloudProviderRepository interface {
	Create(ctx context.Context, provider *model.CloudProviderConfig) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.CloudProviderConfig, error)
	GetByOrgID(ctx context.Context, orgID uuid.UUID) ([]*model.CloudProviderConfig, error)
	GetByOrgAndType(ctx context.Context, orgID uuid.UUID, providerType model.CloudProvider) (*model.CloudProviderConfig, error)
	GetAllEnabled(ctx context.Context) ([]*model.CloudProviderConfig, error)
	Update(ctx context.Context, provider *model.CloudProviderConfig) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status, message string) error
	UpdateLastSync(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// RemediationRepository manages remediation actions and auto-approval rules.
type RemediationRepository interface {
	// Actions
	Create(ctx context.Context, action *model.RemediationAction) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.RemediationAction, error)
	List(ctx context.Context, filter model.RemediationFilter, pagination model.Pagination) ([]*model.RemediationAction, int, error)
	Update(ctx context.Context, action *model.RemediationAction) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.RemediationStatus) error
	GetSummary(ctx context.Context, orgID uuid.UUID) (*model.RemediationSummary, error)

	// Auto-approval rules
	CreateRule(ctx context.Context, rule *model.AutoApprovalRule) error
	ListRules(ctx context.Context, orgID uuid.UUID) ([]*model.AutoApprovalRule, error)
	GetRuleByID(ctx context.Context, id uuid.UUID) (*model.AutoApprovalRule, error)
	UpdateRule(ctx context.Context, rule *model.AutoApprovalRule) error
	DeleteRule(ctx context.Context, id uuid.UUID) error
	GetActiveRules(ctx context.Context, orgID uuid.UUID) ([]*model.AutoApprovalRule, error)
}
