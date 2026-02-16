// Package allocation provides cost allocation and showback functionality.
package allocation

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// AllocationRule defines how costs are allocated to a team/project.
type AllocationRule struct {
	ID             uuid.UUID         `json:"id"`
	OrganizationID uuid.UUID         `json:"organization_id"`
	Name           string            `json:"name"`
	Description    string            `json:"description,omitempty"`
	Target         AllocationTarget  `json:"target"`
	Filters        AllocationFilters `json:"filters"`
	SplitType      SplitType         `json:"split_type"`
	SplitPercent   float64           `json:"split_percent,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// AllocationTarget defines where costs are allocated to.
type AllocationTarget struct {
	Type string `json:"type"` // team, project, cost_center, environment
	Name string `json:"name"`
	ID   string `json:"id,omitempty"`
}

// AllocationFilters defines which costs to include.
type AllocationFilters struct {
	Tags       map[string]string `json:"tags,omitempty"`
	Services   []string          `json:"services,omitempty"`
	AccountIDs []string          `json:"account_ids,omitempty"`
	Regions    []string          `json:"regions,omitempty"`
}

// SplitType defines how costs are split.
type SplitType string

const (
	SplitTypeTagBased   SplitType = "tag_based"
	SplitTypePercentage SplitType = "percentage"
	SplitTypeEven       SplitType = "even"
)

// AllocationResult represents allocated cost for a target.
type AllocationResult struct {
	Target        AllocationTarget `json:"target"`
	TotalCost     float64          `json:"total_cost"`
	Currency      string           `json:"currency"`
	CostByService []ServiceCost    `json:"cost_by_service,omitempty"`
	Period        string           `json:"period"`
	StartDate     time.Time        `json:"start_date"`
	EndDate       time.Time        `json:"end_date"`
}

// ServiceCost represents cost for a specific service.
type ServiceCost struct {
	Service    string  `json:"service"`
	Amount     float64 `json:"amount"`
	Percentage float64 `json:"percentage"`
}

// UntaggedResource represents a resource without proper tags.
type UntaggedResource struct {
	ResourceID   string    `json:"resource_id"`
	ResourceType string    `json:"resource_type"`
	Provider     string    `json:"provider"`
	Region       string    `json:"region"`
	MonthlyCost  float64   `json:"monthly_cost"`
	MissingTags  []string  `json:"missing_tags"`
	DetectedAt   time.Time `json:"detected_at"`
}

// Service provides cost allocation operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new allocation service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// GetAllocationsByOrg returns cost allocations for an organization.
func (s *Service) GetAllocationsByOrg(ctx context.Context, orgID uuid.UUID, startDate, endDate time.Time) ([]AllocationResult, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			COALESCE(c.tags->>'team', COALESCE(c.tags->>'project', 'unallocated')) as target_name,
			SUM(c.amount) as total_cost,
			c.currency,
			c.service
		FROM cost_records c
		WHERE c.organization_id = $1
			AND c.date >= $2
			AND c.date <= $3
		GROUP BY target_name, c.currency, c.service
		ORDER BY total_cost DESC
	`, orgID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Aggregate by target
	targetMap := make(map[string]*AllocationResult)
	for rows.Next() {
		var targetName, currency, service string
		var amount float64
		if err := rows.Scan(&targetName, &amount, &currency, &service); err != nil {
			return nil, err
		}

		result, ok := targetMap[targetName]
		if !ok {
			result = &AllocationResult{
				Target:    AllocationTarget{Type: "team", Name: targetName},
				Currency:  currency,
				StartDate: startDate,
				EndDate:   endDate,
				Period:    "custom",
			}
			targetMap[targetName] = result
		}
		result.TotalCost += amount
		result.CostByService = append(result.CostByService, ServiceCost{
			Service: service,
			Amount:  amount,
		})
	}

	// Calculate percentages
	results := make([]AllocationResult, 0, len(targetMap))
	for _, r := range targetMap {
		for i := range r.CostByService {
			if r.TotalCost > 0 {
				r.CostByService[i].Percentage = (r.CostByService[i].Amount / r.TotalCost) * 100
			}
		}
		results = append(results, *r)
	}

	return results, nil
}

// GetUntaggedResources returns resources missing required tags.
func (s *Service) GetUntaggedResources(ctx context.Context, orgID uuid.UUID, requiredTags []string) ([]UntaggedResource, error) {
	// Query cost records where required tags are missing
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT
			c.resource_id,
			c.service as resource_type,
			c.provider,
			c.region,
			SUM(c.amount) as monthly_cost
		FROM cost_records c
		WHERE c.organization_id = $1
			AND c.date >= NOW() - INTERVAL '30 days'
			AND c.resource_id != ''
		GROUP BY c.resource_id, c.service, c.provider, c.region
		ORDER BY monthly_cost DESC
		LIMIT 100
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var resources []UntaggedResource
	for rows.Next() {
		var r UntaggedResource
		if err := rows.Scan(&r.ResourceID, &r.ResourceType, &r.Provider, &r.Region, &r.MonthlyCost); err != nil {
			return nil, err
		}
		r.MissingTags = requiredTags
		r.DetectedAt = time.Now().UTC()
		resources = append(resources, r)
	}

	return resources, nil
}
