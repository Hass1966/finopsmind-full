// Package azure provides Azure cloud provider implementation.
package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/finopsmind/backend/internal/config"
	"github.com/finopsmind/backend/internal/model"
	"github.com/finopsmind/backend/internal/provider"
)

func init() {
	provider.AzureFromCredsFunc = func(creds model.AzureCredentials, logger *slog.Logger) (provider.Provider, error) {
		return NewProviderFromCredentials(creds, logger)
	}
}

// Provider implements the Azure cloud provider.
type Provider struct {
	cfg        config.AzureConfig
	httpClient *http.Client
	logger     *slog.Logger

	tokenMu     sync.Mutex
	token       string
	tokenExpiry time.Time
}

// NewProvider creates a new Azure provider.
func NewProvider(cfg config.AzureConfig, logger *slog.Logger) (*Provider, error) {
	if cfg.TenantID == "" || cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.SubscriptionID == "" {
		return nil, fmt.Errorf("azure: tenant_id, client_id, client_secret, and subscription_id are required")
	}

	return &Provider{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
	}, nil
}

// NewProviderFromCredentials creates an Azure provider from per-tenant credentials.
func NewProviderFromCredentials(creds model.AzureCredentials, logger *slog.Logger) (*Provider, error) {
	return NewProvider(config.AzureConfig{
		Enabled:        true,
		TenantID:       creds.TenantID,
		ClientID:       creds.ClientID,
		ClientSecret:   creds.ClientSecret,
		SubscriptionID: creds.SubscriptionID,
	}, logger)
}

func (p *Provider) Name() string             { return "azure" }
func (p *Provider) Type() model.CloudProvider { return model.CloudProviderAzure }
func (p *Provider) Close() error             { return nil }

// Health checks Azure connectivity by requesting a token.
func (p *Provider) Health(ctx context.Context) provider.HealthStatus {
	_, err := p.getToken(ctx)
	status := provider.HealthStatus{
		LastChecked: time.Now(),
		Details:     map[string]any{"subscription": p.cfg.SubscriptionID},
	}
	if err != nil {
		status.Healthy = false
		status.Message = fmt.Sprintf("Azure health check failed: %v", err)
	} else {
		status.Healthy = true
		status.Message = "Azure provider healthy"
	}
	return status
}

// GetCosts retrieves cost data from Azure Cost Management API.
func (p *Provider) GetCosts(ctx context.Context, req provider.CostRequest) (*provider.CostResponse, error) {
	p.logger.Info("fetching Azure costs",
		"start", req.StartDate.Format("2006-01-02"),
		"end", req.EndDate.Format("2006-01-02"),
	)

	token, err := p.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("azure: failed to get token: %w", err)
	}

	granularity := "Daily"
	switch req.Granularity {
	case model.GranularityMonthly:
		granularity = "Monthly"
	}

	// Build grouping
	grouping := []map[string]string{}
	for _, g := range req.GroupBy {
		switch g {
		case "service":
			grouping = append(grouping, map[string]string{"type": "Dimension", "name": "ServiceName"})
		case "account":
			grouping = append(grouping, map[string]string{"type": "Dimension", "name": "SubscriptionName"})
		case "region":
			grouping = append(grouping, map[string]string{"type": "Dimension", "name": "ResourceLocation"})
		}
	}
	if len(grouping) == 0 {
		grouping = append(grouping, map[string]string{"type": "Dimension", "name": "ServiceName"})
	}

	body := map[string]any{
		"type":      "ActualCost",
		"timeframe": "Custom",
		"timePeriod": map[string]string{
			"from": req.StartDate.Format("2006-01-02T00:00:00Z"),
			"to":   req.EndDate.Format("2006-01-02T00:00:00Z"),
		},
		"dataset": map[string]any{
			"granularity": granularity,
			"aggregation": map[string]any{
				"totalCost": map[string]string{
					"name":     "Cost",
					"function": "Sum",
				},
			},
			"grouping": grouping,
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("azure: failed to marshal request: %w", err)
	}

	apiURL := fmt.Sprintf(
		"https://management.azure.com/subscriptions/%s/providers/Microsoft.CostManagement/query?api-version=2023-11-01",
		p.cfg.SubscriptionID,
	)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("azure: API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("azure: API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result costQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("azure: failed to decode response: %w", err)
	}

	// Parse response into CostItems
	costs, totalAmount := parseCostResponse(result)

	return &provider.CostResponse{
		Costs:       costs,
		TotalAmount: totalAmount,
		Currency:    "USD",
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
	}, nil
}

// GetRecommendations retrieves optimization recommendations from Azure Advisor.
func (p *Provider) GetRecommendations(ctx context.Context, req provider.RecommendationRequest) (*provider.RecommendationResponse, error) {
	p.logger.Info("fetching Azure recommendations")

	token, err := p.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("azure: failed to get token: %w", err)
	}

	apiURL := fmt.Sprintf(
		"https://management.azure.com/subscriptions/%s/providers/Microsoft.Advisor/recommendations?api-version=2023-01-01&$filter=Category eq 'Cost'",
		p.cfg.SubscriptionID,
	)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("azure: API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("azure: API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result advisorResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("azure: failed to decode response: %w", err)
	}

	var recommendations []provider.Recommendation
	var totalSavings float64

	for _, rec := range result.Value {
		savings := rec.Properties.ExtendedProperties.AnnualSavingsAmount
		recommendation := provider.Recommendation{
			ID:                rec.ID,
			Type:              provider.RecommendationTypeRightsizing,
			ResourceID:        rec.Properties.ResourceMetadata.ResourceID,
			ResourceType:      rec.Properties.ImpactedField,
			CurrentConfig:     rec.Properties.ShortDescription.Problem,
			RecommendedConfig: rec.Properties.ShortDescription.Solution,
			EstimatedSavings:  savings,
			Currency:          "USD",
			Impact:            strings.ToLower(string(rec.Properties.Impact)),
		}
		recommendations = append(recommendations, recommendation)
		totalSavings += savings
	}

	return &provider.RecommendationResponse{
		Recommendations: recommendations,
		TotalSavings:    totalSavings,
		Currency:        "USD",
	}, nil
}

// getToken acquires an OAuth2 token using client credentials flow.
func (p *Provider) getToken(ctx context.Context) (string, error) {
	p.tokenMu.Lock()
	defer p.tokenMu.Unlock()

	if p.token != "" && time.Now().Before(p.tokenExpiry) {
		return p.token, nil
	}

	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", p.cfg.TenantID)

	data := url.Values{}
	data.Set("client_id", p.cfg.ClientID)
	data.Set("client_secret", p.cfg.ClientSecret)
	data.Set("scope", "https://management.azure.com/.default")
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	p.token = tokenResp.AccessToken
	p.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)

	return p.token, nil
}

// --- Response types for Azure APIs ---

type costQueryResponse struct {
	Properties struct {
		Columns []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"columns"`
		Rows [][]any `json:"rows"`
	} `json:"properties"`
}

type advisorResponse struct {
	Value []struct {
		ID         string `json:"id"`
		Properties struct {
			Category      string `json:"category"`
			Impact        string `json:"impact"`
			ImpactedField string `json:"impactedField"`
			ShortDescription struct {
				Problem  string `json:"problem"`
				Solution string `json:"solution"`
			} `json:"shortDescription"`
			ResourceMetadata struct {
				ResourceID string `json:"resourceId"`
			} `json:"resourceMetadata"`
			ExtendedProperties struct {
				AnnualSavingsAmount float64 `json:"annualSavingsAmount"`
			} `json:"extendedProperties"`
		} `json:"properties"`
	} `json:"value"`
}

func parseCostResponse(result costQueryResponse) ([]provider.CostItem, float64) {
	var costs []provider.CostItem
	var totalAmount float64

	// Find column indices
	costIdx := -1
	dateIdx := -1
	serviceIdx := -1

	for i, col := range result.Properties.Columns {
		switch col.Name {
		case "Cost", "PreTaxCost":
			costIdx = i
		case "UsageDate", "BillingPeriod":
			dateIdx = i
		case "ServiceName":
			serviceIdx = i
		}
	}

	for _, row := range result.Properties.Rows {
		item := provider.CostItem{}

		if costIdx >= 0 && costIdx < len(row) {
			switch v := row[costIdx].(type) {
			case float64:
				item.Amount = v
			case json.Number:
				f, _ := v.Float64()
				item.Amount = f
			}
		}

		if dateIdx >= 0 && dateIdx < len(row) {
			switch v := row[dateIdx].(type) {
			case float64:
				// Azure returns dates as numbers like 20240115
				dateStr := fmt.Sprintf("%.0f", v)
				if t, err := time.Parse("20060102", dateStr); err == nil {
					item.Date = t
				}
			case string:
				if t, err := time.Parse("20060102", v); err == nil {
					item.Date = t
				}
			}
		}

		if serviceIdx >= 0 && serviceIdx < len(row) {
			if v, ok := row[serviceIdx].(string); ok {
				item.Service = v
			}
		}

		costs = append(costs, item)
		totalAmount += item.Amount
	}

	return costs, totalAmount
}
