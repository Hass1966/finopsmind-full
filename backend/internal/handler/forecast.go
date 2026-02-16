package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/finopsmind/backend/internal/ml"
	"github.com/finopsmind/backend/internal/model"
)

// ForecastHandler handles cost forecast API requests
type ForecastHandler struct {
	mlClient *ml.Client
	store    model.CostStore
}

// NewForecastHandler creates a new ForecastHandler
func NewForecastHandler(mlClient *ml.Client, store model.CostStore) *ForecastHandler {
	return &ForecastHandler{
		mlClient: mlClient,
		store:    store,
	}
}

// ForecastRequest represents the API request for forecasting
type ForecastRequest struct {
	AccountID          string `json:"account_id"`
	Periods            int    `json:"periods"`
	WeeklySeasonality  bool   `json:"weekly_seasonality"`
	MonthlySeasonality bool   `json:"monthly_seasonality"`
}

// GetForecast handles GET /api/v1/forecast
func (h *ForecastHandler) GetForecast(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	accountID := r.URL.Query().Get("account_id")
	if accountID == "" {
		writeError(w, http.StatusBadRequest, "account_id is required")
		return
	}

	periods := 30
	if p := r.URL.Query().Get("periods"); p != "" {
		var err error
		periods, err = strconv.Atoi(p)
		if err != nil || periods < 1 || periods > 90 {
			writeError(w, http.StatusBadRequest, "periods must be between 1 and 90")
			return
		}
	}

	// Get historical cost data from store (last 90 days)
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -90)
	
	historicalCosts, err := h.store.GetDailyCosts(ctx, accountID, startDate, endDate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve historical costs: "+err.Error())
		return
	}

	if len(historicalCosts) < 14 {
		writeError(w, http.StatusBadRequest, "insufficient historical data (need at least 14 days)")
		return
	}

	// Convert to ML client format
	dataPoints := make([]ml.CostDataPoint, len(historicalCosts))
	for i, cost := range historicalCosts {
		dataPoints[i] = ml.CostDataPoint{
			Date: cost.Date.Format("2006-01-02"),
			Cost: cost.TotalCost,
		}
	}

	// Call ML sidecar
	mlReq := &ml.ForecastRequest{
		HistoricalData:     dataPoints,
		Periods:            periods,
		WeeklySeasonality:  true,
		MonthlySeasonality: true,
		AccountID:          accountID,
	}

	forecast, err := h.mlClient.GenerateForecast(ctx, mlReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "forecast generation failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, forecast)
}

// PostForecast handles POST /api/v1/forecast with custom data
func (h *ForecastHandler) PostForecast(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		HistoricalData []struct {
			Date string  `json:"date"`
			Cost float64 `json:"cost"`
		} `json:"historical_data"`
		Periods            int  `json:"periods"`
		WeeklySeasonality  bool `json:"weekly_seasonality"`
		MonthlySeasonality bool `json:"monthly_seasonality"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if len(req.HistoricalData) < 14 {
		writeError(w, http.StatusBadRequest, "need at least 14 data points")
		return
	}

	if req.Periods == 0 {
		req.Periods = 30
	}

	// Convert to ML client format
	dataPoints := make([]ml.CostDataPoint, len(req.HistoricalData))
	for i, d := range req.HistoricalData {
		dataPoints[i] = ml.CostDataPoint{
			Date: d.Date,
			Cost: d.Cost,
		}
	}

	mlReq := &ml.ForecastRequest{
		HistoricalData:     dataPoints,
		Periods:            req.Periods,
		WeeklySeasonality:  req.WeeklySeasonality,
		MonthlySeasonality: req.MonthlySeasonality,
	}

	forecast, err := h.mlClient.GenerateForecast(ctx, mlReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "forecast generation failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, forecast)
}
