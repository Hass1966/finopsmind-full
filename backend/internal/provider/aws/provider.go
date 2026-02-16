// Package aws provides AWS cloud provider implementation.

package aws

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/finopsmind/backend/internal/config"
	"github.com/finopsmind/backend/internal/model"
	"github.com/finopsmind/backend/internal/provider"
)

// Provider implements the AWS cloud provider.
type Provider struct {
	name         string
	cfg          aws.Config
	costExplorer *costexplorer.Client
	logger       *slog.Logger
	retryConfig  RetryConfig
}

// RetryConfig holds retry settings.
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// NewProvider creates a new AWS provider.
func NewProvider(cfg config.AWSConfig, logger *slog.Logger) (*Provider, error) {
	ctx := context.Background()

	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}

	// Use explicit credentials if provided
	if cfg.AccessKeyID != "" && cfg.SecretKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Handle role assumption if configured
	if cfg.AssumeRoleARN != "" {
		stsClient := sts.NewFromConfig(awsCfg)
		creds := stscreds.NewAssumeRoleProvider(stsClient, cfg.AssumeRoleARN, func(o *stscreds.AssumeRoleOptions) {
			if cfg.ExternalID != "" {
				o.ExternalID = aws.String(cfg.ExternalID)
			}
		})
		awsCfg.Credentials = aws.NewCredentialsCache(creds)
	}

	return &Provider{
		name:         "aws",
		cfg:          awsCfg,
		costExplorer: costexplorer.NewFromConfig(awsCfg),
		logger:       logger,
		retryConfig: RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   1 * time.Second,
			MaxDelay:    30 * time.Second,
		},
	}, nil
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return p.name
}

// Type returns the provider type.
func (p *Provider) Type() model.CloudProvider {
	return model.CloudProviderAWS
}

// Health checks AWS connectivity.
func (p *Provider) Health(ctx context.Context) provider.HealthStatus {
	// Simple health check - try to get cost data for today
	_, err := p.costExplorer.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(time.Now().AddDate(0, 0, -1).Format("2006-01-02")),
			End:   aws.String(time.Now().Format("2006-01-02")),
		},
		Granularity: types.GranularityDaily,
		Metrics:     []string{"UnblendedCost"},
	})

	status := provider.HealthStatus{
		LastChecked: time.Now(),
		Details:     map[string]any{"region": p.cfg.Region},
	}

	if err != nil {
		status.Healthy = false
		status.Message = fmt.Sprintf("AWS health check failed: %v", err)
	} else {
		status.Healthy = true
		status.Message = "AWS provider healthy"
	}

	return status
}

// GetCosts retrieves cost data from AWS Cost Explorer.
func (p *Provider) GetCosts(ctx context.Context, req provider.CostRequest) (*provider.CostResponse, error) {
	p.logger.Info("fetching AWS costs",
		"start", req.StartDate.Format("2006-01-02"),
		"end", req.EndDate.Format("2006-01-02"),
		"granularity", req.Granularity,
	)

	granularity := types.GranularityDaily
	switch req.Granularity {
	case model.GranularityHourly:
		granularity = types.GranularityHourly
	case model.GranularityMonthly:
		granularity = types.GranularityMonthly
	}

	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(req.StartDate.Format("2006-01-02")),
			End:   aws.String(req.EndDate.Format("2006-01-02")),
		},
		Granularity: granularity,
		Metrics:     []string{"UnblendedCost", "UsageQuantity"},
	}

	// Add grouping if requested
	if len(req.GroupBy) > 0 {
		var groupDefs []types.GroupDefinition
		for _, g := range req.GroupBy {
			switch g {
			case "service":
				groupDefs = append(groupDefs, types.GroupDefinition{
					Type: types.GroupDefinitionTypeDimension,
					Key:  aws.String("SERVICE"),
				})
			case "account":
				groupDefs = append(groupDefs, types.GroupDefinition{
					Type: types.GroupDefinitionTypeDimension,
					Key:  aws.String("LINKED_ACCOUNT"),
				})
			case "region":
				groupDefs = append(groupDefs, types.GroupDefinition{
					Type: types.GroupDefinitionTypeDimension,
					Key:  aws.String("REGION"),
				})
			}
		}
		input.GroupBy = groupDefs
	}

	// Add filters if specified
	if len(req.Filters.Services) > 0 || len(req.Filters.AccountIDs) > 0 || len(req.Filters.Regions) > 0 {
		filter := p.buildFilter(req.Filters)
		input.Filter = filter
	}

	var costs []provider.CostItem
	var totalAmount float64

	// Get cost data (single call, no pagination for simplicity)
	output, err := p.costExplorer.GetCostAndUsage(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get cost data: %w", err)
	}

	for _, result := range output.ResultsByTime {
		date, _ := time.Parse("2006-01-02", *result.TimePeriod.Start)

		if len(result.Groups) > 0 {
			// Grouped results
			for _, group := range result.Groups {
				amount := 0.0
				if cost, ok := result.Total["UnblendedCost"]; ok && cost.Amount != nil {
					fmt.Sscanf(*cost.Amount, "%f", &amount)
				}
				if len(group.Metrics) > 0 {
					if cost, ok := group.Metrics["UnblendedCost"]; ok && cost.Amount != nil {
						fmt.Sscanf(*cost.Amount, "%f", &amount)
					}
				}

				item := provider.CostItem{
					Date:   date,
					Amount: amount,
				}

				// Extract group keys
				for i, key := range group.Keys {
					if i < len(req.GroupBy) {
						switch req.GroupBy[i] {
						case "service":
							item.Service = key
						case "account":
							item.AccountID = key
						case "region":
							item.Region = key
						}
					}
				}

				costs = append(costs, item)
				totalAmount += amount
			}
		} else {
			// Ungrouped results
			amount := 0.0
			if cost, ok := result.Total["UnblendedCost"]; ok && cost.Amount != nil {
				fmt.Sscanf(*cost.Amount, "%f", &amount)
			}

			costs = append(costs, provider.CostItem{
				Date:   date,
				Amount: amount,
			})
			totalAmount += amount
		}
	}

	return &provider.CostResponse{
		Costs:       costs,
		TotalAmount: totalAmount,
		Currency:    "USD",
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
	}, nil
}

// GetRecommendations retrieves optimization recommendations.
func (p *Provider) GetRecommendations(ctx context.Context, req provider.RecommendationRequest) (*provider.RecommendationResponse, error) {
	p.logger.Info("fetching AWS recommendations", "types", req.Types)

	var recommendations []provider.Recommendation
	var totalSavings float64

	for _, recType := range req.Types {
		switch recType {
		case provider.RecommendationTypeRightsizing:
			recs, err := p.getRightsizingRecommendations(ctx)
			if err != nil {
				p.logger.Warn("failed to get rightsizing recommendations", "error", err)
				continue
			}
			for _, rec := range recs {
				recommendations = append(recommendations, rec)
				totalSavings += rec.EstimatedSavings
			}

		case provider.RecommendationTypeReservedInstances:
			recs, err := p.getReservedInstanceRecommendations(ctx)
			if err != nil {
				p.logger.Warn("failed to get RI recommendations", "error", err)
				continue
			}
			for _, rec := range recs {
				recommendations = append(recommendations, rec)
				totalSavings += rec.EstimatedSavings
			}

		case provider.RecommendationTypeSavingsPlans:
			recs, err := p.getSavingsPlansRecommendations(ctx)
			if err != nil {
				p.logger.Warn("failed to get Savings Plans recommendations", "error", err)
				continue
			}
			for _, rec := range recs {
				recommendations = append(recommendations, rec)
				totalSavings += rec.EstimatedSavings
			}
		}
	}

	return &provider.RecommendationResponse{
		Recommendations: recommendations,
		TotalSavings:    totalSavings,
		Currency:        "USD",
	}, nil
}

func (p *Provider) getRightsizingRecommendations(ctx context.Context) ([]provider.Recommendation, error) {
	input := &costexplorer.GetRightsizingRecommendationInput{
		Service: aws.String("AmazonEC2"),
		Configuration: &types.RightsizingRecommendationConfiguration{
			RecommendationTarget: types.RecommendationTargetSameInstanceFamily,
			BenefitsConsidered:   true,
		},
	}

	output, err := p.costExplorer.GetRightsizingRecommendation(ctx, input)
	if err != nil {
		return nil, err
	}

	var recommendations []provider.Recommendation
	for _, rec := range output.RightsizingRecommendations {
		if rec.RightsizingType == types.RightsizingTypeModify && rec.ModifyRecommendationDetail != nil {
			savings := 0.0
			if rec.ModifyRecommendationDetail.TargetInstances != nil {
				for _, target := range rec.ModifyRecommendationDetail.TargetInstances {
					if target.EstimatedMonthlySavings != nil {
						fmt.Sscanf(*target.EstimatedMonthlySavings, "%f", &savings)
					}
				}
			}

			resourceID := "unknown"
			if rec.CurrentInstance != nil && rec.CurrentInstance.ResourceId != nil {
				resourceID = *rec.CurrentInstance.ResourceId
			}

			recommendations = append(recommendations, provider.Recommendation{
				ID:                resourceID,
				Type:              provider.RecommendationTypeRightsizing,
				ResourceID:        resourceID,
				ResourceType:      "EC2 Instance",
				CurrentConfig:     getInstanceType(rec.CurrentInstance),
				RecommendedConfig: getTargetInstanceType(rec.ModifyRecommendationDetail),
				EstimatedSavings:  savings,
				Currency:          "USD",
				Impact:            "medium",
			})
		}
	}

	return recommendations, nil
}

func (p *Provider) getReservedInstanceRecommendations(ctx context.Context) ([]provider.Recommendation, error) {
	input := &costexplorer.GetReservationPurchaseRecommendationInput{
		Service:              aws.String("Amazon Elastic Compute Cloud - Compute"),
		LookbackPeriodInDays: types.LookbackPeriodInDaysSixtyDays,
		TermInYears:          types.TermInYearsOneYear,
		PaymentOption:        types.PaymentOptionNoUpfront,
	}

	output, err := p.costExplorer.GetReservationPurchaseRecommendation(ctx, input)
	if err != nil {
		return nil, err
	}

	var recommendations []provider.Recommendation
	for _, rec := range output.Recommendations {
		for _, detail := range rec.RecommendationDetails {
			savings := 0.0
			if detail.EstimatedMonthlySavingsAmount != nil {
				fmt.Sscanf(*detail.EstimatedMonthlySavingsAmount, "%f", &savings)
			}

			instanceType := "unknown"
			if detail.InstanceDetails != nil && detail.InstanceDetails.EC2InstanceDetails != nil && detail.InstanceDetails.EC2InstanceDetails.InstanceType != nil {
				instanceType = *detail.InstanceDetails.EC2InstanceDetails.InstanceType
			}

			recommendations = append(recommendations, provider.Recommendation{
				ID:                fmt.Sprintf("ri-%s", instanceType),
				Type:              provider.RecommendationTypeReservedInstances,
				ResourceType:      "EC2 Reserved Instance",
				RecommendedConfig: fmt.Sprintf("%s RI", instanceType),
				EstimatedSavings:  savings * 12, // Annual savings
				Currency:          "USD",
				Impact:            "high",
				Details: map[string]any{
					"instance_type":     instanceType,
					"recommended_count": getRecommendedCount(detail),
				},
			})
		}
	}

	return recommendations, nil
}

func (p *Provider) getSavingsPlansRecommendations(ctx context.Context) ([]provider.Recommendation, error) {
	input := &costexplorer.GetSavingsPlansPurchaseRecommendationInput{
		SavingsPlansType:     types.SupportedSavingsPlansTypeComputeSp,
		LookbackPeriodInDays: types.LookbackPeriodInDaysSixtyDays,
		TermInYears:          types.TermInYearsOneYear,
		PaymentOption:        types.PaymentOptionNoUpfront,
	}

	output, err := p.costExplorer.GetSavingsPlansPurchaseRecommendation(ctx, input)
	if err != nil {
		return nil, err
	}

	var recommendations []provider.Recommendation
	if output.SavingsPlansPurchaseRecommendation != nil {
		for _, detail := range output.SavingsPlansPurchaseRecommendation.SavingsPlansPurchaseRecommendationDetails {
			savings := 0.0
			if detail.EstimatedMonthlySavingsAmount != nil {
				fmt.Sscanf(*detail.EstimatedMonthlySavingsAmount, "%f", &savings)
			}

			offeringID := "unknown"
			if detail.SavingsPlansDetails != nil && detail.SavingsPlansDetails.OfferingId != nil {
				offeringID = *detail.SavingsPlansDetails.OfferingId
			}

			hourlyCommitment := "0"
			if detail.HourlyCommitmentToPurchase != nil {
				hourlyCommitment = *detail.HourlyCommitmentToPurchase
			}

			recommendations = append(recommendations, provider.Recommendation{
				ID:                fmt.Sprintf("sp-%s", offeringID),
				Type:              provider.RecommendationTypeSavingsPlans,
				ResourceType:      "Savings Plan",
				RecommendedConfig: fmt.Sprintf("Compute Savings Plan $%s/hr", hourlyCommitment),
				EstimatedSavings:  savings * 12,
				Currency:          "USD",
				Impact:            "high",
				Details: map[string]any{
					"hourly_commitment": hourlyCommitment,
				},
			})
		}
	}

	return recommendations, nil
}

func (p *Provider) buildFilter(filters provider.CostFilters) *types.Expression {
	var expressions []types.Expression

	if len(filters.Services) > 0 {
		expressions = append(expressions, types.Expression{
			Dimensions: &types.DimensionValues{
				Key:    types.DimensionService,
				Values: filters.Services,
			},
		})
	}

	if len(filters.AccountIDs) > 0 {
		expressions = append(expressions, types.Expression{
			Dimensions: &types.DimensionValues{
				Key:    types.DimensionLinkedAccount,
				Values: filters.AccountIDs,
			},
		})
	}

	if len(filters.Regions) > 0 {
		expressions = append(expressions, types.Expression{
			Dimensions: &types.DimensionValues{
				Key:    types.DimensionRegion,
				Values: filters.Regions,
			},
		})
	}

	if len(expressions) == 0 {
		return nil
	}

	if len(expressions) == 1 {
		return &expressions[0]
	}

	return &types.Expression{
		And: expressions,
	}
}

// Close cleans up provider resources.

func (p *Provider) Close() error {
	return nil
}

func getTargetInstanceType(detail *types.ModifyRecommendationDetail) string {
	if detail != nil && len(detail.TargetInstances) > 0 {
		if detail.TargetInstances[0].ResourceDetails != nil &&
			detail.TargetInstances[0].ResourceDetails.EC2ResourceDetails != nil &&
			detail.TargetInstances[0].ResourceDetails.EC2ResourceDetails.InstanceType != nil {
			return *detail.TargetInstances[0].ResourceDetails.EC2ResourceDetails.InstanceType
		}
	}
	return "unknown"
}

func getInstanceType(instance *types.CurrentInstance) string {
	if instance != nil && instance.ResourceDetails != nil &&
		instance.ResourceDetails.EC2ResourceDetails != nil &&
		instance.ResourceDetails.EC2ResourceDetails.InstanceType != nil {
		return *instance.ResourceDetails.EC2ResourceDetails.InstanceType
	}
	return "unknown"
}

func getRecommendedCount(detail types.ReservationPurchaseRecommendationDetail) string {
	if detail.RecommendedNumberOfInstancesToPurchase != nil {
		return *detail.RecommendedNumberOfInstancesToPurchase
	}
	return "0"
}
