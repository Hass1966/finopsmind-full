package ml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Client is the HTTP client for the ML sidecar service
type Client struct {
	baseURL    string
	httpClient *http.Client
	cache      *forecastCache
}

// forecastCache stores cached forecast results
type forecastCache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
}

// CacheEntry represents a cached forecast
type CacheEntry struct {
	Result    *ForecastResponse
	ExpiresAt time.Time
}

// CostDataPoint represents a single cost data point
type CostDataPoint struct {
	Date             string             `json:"date"`
	Cost             float64            `json:"cost"`
	ServiceBreakdown map[string]float64 `json:"service_breakdown,omitempty"`
}

// ForecastRequest is the request payload for forecasting
type ForecastRequest struct {
	HistoricalData     []CostDataPoint `json:"historical_data"`
	Periods            int             `json:"periods"`
	WeeklySeasonality  bool            `json:"weekly_seasonality"`
	MonthlySeasonality bool            `json:"monthly_seasonality"`
	AccountID          string          `json:"account_id,omitempty"`
}

// ForecastPrediction represents a single forecast prediction
type ForecastPrediction struct {
	Date          string  `json:"date"`
	PredictedCost float64 `json:"predicted_cost"`
	LowerBound80  float64 `json:"lower_bound_80"`
	UpperBound80  float64 `json:"upper_bound_80"`
	LowerBound95  float64 `json:"lower_bound_95"`
	UpperBound95  float64 `json:"upper_bound_95"`
}

// ForecastSummary contains summary statistics for the forecast
type ForecastSummary struct {
	TotalPredictedCost float64 `json:"total_predicted_cost"`
	AverageDailyCost   float64 `json:"average_daily_cost"`
	ForecastStart      string  `json:"forecast_start"`
	ForecastEnd        string  `json:"forecast_end"`
}

// ForecastResponse is the response from the forecast endpoint
type ForecastResponse struct {
	ForecastGeneratedAt string               `json:"forecast_generated_at"`
	Periods             int                  `json:"periods"`
	Predictions         []ForecastPrediction `json:"predictions"`
	Summary             ForecastSummary      `json:"summary"`
	Cached              bool                 `json:"cached"`
	Note                string               `json:"note,omitempty"`
}

// AnomalyRequest is the request payload for anomaly detection
type AnomalyRequest struct {
	CostData        []CostDataPoint `json:"cost_data"`
	TrainingData    []CostDataPoint `json:"training_data,omitempty"`
	Contamination   float64         `json:"contamination"`
	ReturnAllScores bool            `json:"return_all_scores"`
}

// RootCause contains root cause analysis for an anomaly
type RootCause struct {
	PrimaryFactor       string            `json:"primary_factor"`
	ContributingFactors []string          `json:"contributing_factors,omitempty"`
	Details             map[string]any    `json:"details,omitempty"`
}

// AnomalyResult represents a single anomaly detection result
type AnomalyResult struct {
	Date         string    `json:"date"`
	Cost         float64   `json:"cost"`
	IsAnomaly    bool      `json:"is_anomaly"`
	AnomalyScore float64   `json:"anomaly_score"`
	Severity     string    `json:"severity"`
	RootCause    *RootCause `json:"root_cause,omitempty"`
}

// AnomalyThresholds contains threshold information
type AnomalyThresholds struct {
	Contamination  float64           `json:"contamination,omitempty"`
	Method         string            `json:"method,omitempty"`
	LowerBound     float64           `json:"lower_bound,omitempty"`
	UpperBound     float64           `json:"upper_bound,omitempty"`
	SeverityLevels map[string]string `json:"severity_levels,omitempty"`
}

// AnomalyResponse is the response from the anomaly detection endpoint
type AnomalyResponse struct {
	DetectionTimestamp  string            `json:"detection_timestamp"`
	TotalPointsAnalyzed int               `json:"total_points_analyzed"`
	AnomaliesDetected   int               `json:"anomalies_detected"`
	AnomalyRate         float64           `json:"anomaly_rate"`
	Anomalies           []AnomalyResult   `json:"anomalies"`
	AllResults          []AnomalyResult   `json:"all_results,omitempty"`
	Thresholds          AnomalyThresholds `json:"thresholds"`
	Note                string            `json:"note,omitempty"`
}

// HealthResponse is the response from the health endpoint
type HealthResponse struct {
	Status       string          `json:"status"`
	Timestamp    string          `json:"timestamp"`
	Version      string          `json:"version"`
	ModelsLoaded map[string]bool `json:"models_loaded"`
	CacheStatus  map[string]any  `json:"cache_status"`
}

// NewClient creates a new ML sidecar client
func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		cache: &forecastCache{
			entries: make(map[string]*CacheEntry),
		},
	}
}

// NewClientWithDefaults creates a client with default configuration
func NewClientWithDefaults() *Client {
	return NewClient("http://ml-sidecar:8081", 30*time.Second)
}

// Health checks the health of the ML sidecar
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("health check failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &health, nil
}

// GenerateForecast generates a cost forecast
func (c *Client) GenerateForecast(ctx context.Context, req *ForecastRequest) (*ForecastResponse, error) {
	// Check cache first
	if req.AccountID != "" {
		cacheKey := fmt.Sprintf("%s_%d", req.AccountID, req.Periods)
		if cached := c.getCachedForecast(cacheKey); cached != nil {
			cached.Cached = true
			return cached, nil
		}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/forecast", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("forecast failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var forecast ForecastResponse
	if err := json.NewDecoder(resp.Body).Decode(&forecast); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	// Cache the result
	if req.AccountID != "" {
		cacheKey := fmt.Sprintf("%s_%d", req.AccountID, req.Periods)
		c.setCachedForecast(cacheKey, &forecast, time.Hour)
	}

	return &forecast, nil
}

// DetectAnomalies detects anomalies in cost data
func (c *Client) DetectAnomalies(ctx context.Context, req *AnomalyRequest) (*AnomalyResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/anomalies/detect", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anomaly detection failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var anomalies AnomalyResponse
	if err := json.NewDecoder(resp.Body).Decode(&anomalies); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &anomalies, nil
}

// ScoreSinglePoint scores a single cost data point for anomalies
func (c *Client) ScoreSinglePoint(ctx context.Context, dataPoint CostDataPoint) (*AnomalyResult, error) {
	body, err := json.Marshal(dataPoint)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/anomalies/score", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("scoring failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var result AnomalyResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// TrainForecaster pre-trains a forecaster model
func (c *Client) TrainForecaster(ctx context.Context, req *ForecastRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/train/forecaster", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("training failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// TrainAnomalyDetector pre-trains an anomaly detector
func (c *Client) TrainAnomalyDetector(ctx context.Context, req *AnomalyRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/train/anomaly-detector", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("training failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// ClearCache clears the ML sidecar cache
func (c *Client) ClearCache(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/cache", nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cache clear failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Also clear local cache
	c.cache.mu.Lock()
	c.cache.entries = make(map[string]*CacheEntry)
	c.cache.mu.Unlock()

	return nil
}

// getCachedForecast retrieves a cached forecast if valid
func (c *Client) getCachedForecast(key string) *ForecastResponse {
	c.cache.mu.RLock()
	defer c.cache.mu.RUnlock()

	entry, ok := c.cache.entries[key]
	if !ok {
		return nil
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil
	}

	// Return a copy
	result := *entry.Result
	return &result
}

// setCachedForecast stores a forecast in the cache
func (c *Client) setCachedForecast(key string, result *ForecastResponse, ttl time.Duration) {
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()

	c.cache.entries[key] = &CacheEntry{
		Result:    result,
		ExpiresAt: time.Now().Add(ttl),
	}
}
