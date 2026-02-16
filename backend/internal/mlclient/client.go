// Package mlclient provides a client for the ML sidecar service.
package mlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/finopsmind/backend/internal/config"
	"github.com/finopsmind/backend/internal/model"
)

// Client provides access to the ML sidecar.
type Client struct {
	baseURL    string
	httpClient *http.Client
	cb         *CircuitBreaker
	enabled    bool
}

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	mu            sync.RWMutex
	failures      int
	maxFailures   int
	state         string // closed, open, half-open
	lastFailure   time.Time
	resetTimeout  time.Duration
	halfOpenLimit int
	halfOpenCount int
}

// NewClient creates a new ML client.
func NewClient(cfg config.MLSidecarConfig) *Client {
	return &Client{
		baseURL: cfg.URL,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		cb: &CircuitBreaker{
			maxFailures:   cfg.CircuitBreaker.MaxFailures,
			resetTimeout:  cfg.CircuitBreaker.ResetTimeout,
			halfOpenLimit: cfg.CircuitBreaker.HalfOpenLimit,
			state:         "closed",
		},
		enabled: cfg.Enabled,
	}
}

// Health checks the ML sidecar health.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	if !c.enabled {
		return &HealthResponse{Status: "disabled"}, nil
	}

	resp, err := c.doRequest(ctx, "GET", "/health", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, fmt.Errorf("failed to decode health response: %w", err)
	}

	return &health, nil
}

// Forecast requests a cost forecast from the ML sidecar.
func (c *Client) Forecast(ctx context.Context, req *ForecastRequest) (*model.ForecastResponse, error) {
	if !c.enabled {
		return nil, fmt.Errorf("ML sidecar is disabled")
	}

	if err := c.cb.Allow(); err != nil {
		return nil, err
	}

	body, _ := json.Marshal(req)
	resp, err := c.doRequest(ctx, "POST", "/api/v1/forecast", body)
	if err != nil {
		c.cb.RecordFailure()
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.cb.RecordFailure()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("forecast failed: %s", string(bodyBytes))
	}

	c.cb.RecordSuccess()

	var forecast model.ForecastResponse
	if err := json.NewDecoder(resp.Body).Decode(&forecast); err != nil {
		return nil, fmt.Errorf("failed to decode forecast response: %w", err)
	}

	return &forecast, nil
}

// DetectAnomalies requests anomaly detection from the ML sidecar.
func (c *Client) DetectAnomalies(ctx context.Context, req *AnomalyDetectionRequest) (*AnomalyDetectionResponse, error) {
	if !c.enabled {
		return nil, fmt.Errorf("ML sidecar is disabled")
	}

	if err := c.cb.Allow(); err != nil {
		return nil, err
	}

	body, _ := json.Marshal(req)
	resp, err := c.doRequest(ctx, "POST", "/api/v1/anomalies/detect", body)
	if err != nil {
		c.cb.RecordFailure()
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.cb.RecordFailure()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anomaly detection failed: %s", string(bodyBytes))
	}

	c.cb.RecordSuccess()

	var result AnomalyDetectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode anomaly response: %w", err)
	}

	return &result, nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}

// CircuitBreaker methods

func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case "open":
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = "half-open"
			cb.halfOpenCount = 0
		} else {
			return fmt.Errorf("circuit breaker is open")
		}
	case "half-open":
		if cb.halfOpenCount >= cb.halfOpenLimit {
			return fmt.Errorf("circuit breaker is half-open, limit reached")
		}
		cb.halfOpenCount++
	}

	return nil
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.state = "closed"
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.maxFailures {
		cb.state = "open"
	}
}

func (cb *CircuitBreaker) State() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Request/Response types

type ForecastRequest struct {
	OrganizationID string           `json:"organization_id"`
	HistoricalDays int              `json:"historical_days"`
	ForecastDays   int              `json:"forecast_days"`
	Granularity    string           `json:"granularity"`
	ServiceFilter  string           `json:"service_filter,omitempty"`
	AccountFilter  string           `json:"account_filter,omitempty"`
	Data           []CostDataPoint  `json:"data,omitempty"`
}

type CostDataPoint struct {
	Date   string  `json:"date"`
	Amount float64 `json:"amount"`
}

type AnomalyDetectionRequest struct {
	OrganizationID string          `json:"organization_id"`
	Data           []CostDataPoint `json:"data"`
	Sensitivity    float64         `json:"sensitivity"`
}

type AnomalyDetectionResponse struct {
	OrganizationID string     `json:"organization_id"`
	AnalyzedAt     time.Time  `json:"analyzed_at"`
	ModelVersion   string     `json:"model_version"`
	Anomalies      []Anomaly  `json:"anomalies"`
	TotalAnalyzed  int        `json:"total_analyzed"`
	AnomalyCount   int        `json:"anomaly_count"`
	Threshold      float64    `json:"threshold"`
}

type Anomaly struct {
	Date           string  `json:"date"`
	ActualAmount   float64 `json:"actual_amount"`
	ExpectedAmount float64 `json:"expected_amount"`
	Deviation      float64 `json:"deviation"`
	DeviationPct   float64 `json:"deviation_pct"`
	Score          float64 `json:"score"`
	Severity       string  `json:"severity"`
}

type HealthResponse struct {
	Status  string            `json:"status"`
	Version string            `json:"version"`
	Models  map[string]bool   `json:"models"`
}
