package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/finopsmind/backend/internal/ml"
	"github.com/finopsmind/backend/internal/model"
)

// AnomalyHandler handles anomaly detection API requests
type AnomalyHandler struct {
	mlClient *ml.Client
	store    model.CostStore
}

// NewAnomalyHandler creates a new AnomalyHandler
func NewAnomalyHandler(mlClient *ml.Client, store model.CostStore) *AnomalyHandler {
	return &AnomalyHandler{
		mlClient: mlClient,
		store:    store,
	}
}

// GetAnomalies handles GET /api/v1/anomalies
func (h *AnomalyHandler) GetAnomalies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	accountID := r.URL.Query().Get("account_id")
	if accountID == "" {
		writeError(w, http.StatusBadRequest, "account_id is required")
		return
	}

	// Days to analyze (default 30)
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		var err error
		days, err = strconv.Atoi(d)
		if err != nil || days < 7 || days > 90 {
			writeError(w, http.StatusBadRequest, "days must be between 7 and 90")
			return
		}
	}

	// Contamination rate (default 0.1)
	contamination := 0.1
	if c := r.URL.Query().Get("contamination"); c != "" {
		var err error
		contamination, err = strconv.ParseFloat(c, 64)
		if err != nil || contamination < 0.01 || contamination > 0.5 {
			writeError(w, http.StatusBadRequest, "contamination must be between 0.01 and 0.5")
			return
		}
	}

	// Get cost data from store
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)

	costs, err := h.store.GetDailyCosts(ctx, accountID, startDate, endDate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve cost data: "+err.Error())
		return
	}

	if len(costs) < 7 {
		writeError(w, http.StatusBadRequest, "insufficient data (need at least 7 days)")
		return
	}

	// Convert to ML client format
	dataPoints := make([]ml.CostDataPoint, len(costs))
	for i, cost := range costs {
		dataPoints[i] = ml.CostDataPoint{
			Date:             cost.Date.Format("2006-01-02"),
			Cost:             cost.TotalCost,
			ServiceBreakdown: cost.ServiceBreakdown,
		}
	}

	// Call ML sidecar
	mlReq := &ml.AnomalyRequest{
		CostData:        dataPoints,
		Contamination:   contamination,
		ReturnAllScores: true,
	}

	anomalies, err := h.mlClient.DetectAnomalies(ctx, mlReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "anomaly detection failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, anomalies)
}

// PostAnomalies handles POST /api/v1/anomalies/detect with custom data
func (h *AnomalyHandler) PostAnomalies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		CostData []struct {
			Date             string             `json:"date"`
			Cost             float64            `json:"cost"`
			ServiceBreakdown map[string]float64 `json:"service_breakdown,omitempty"`
		} `json:"cost_data"`
		TrainingData []struct {
			Date string  `json:"date"`
			Cost float64 `json:"cost"`
		} `json:"training_data,omitempty"`
		Contamination   float64 `json:"contamination"`
		ReturnAllScores bool    `json:"return_all_scores"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if len(req.CostData) < 7 {
		writeError(w, http.StatusBadRequest, "need at least 7 data points")
		return
	}

	if req.Contamination == 0 {
		req.Contamination = 0.1
	}

	// Convert to ML client format
	dataPoints := make([]ml.CostDataPoint, len(req.CostData))
	for i, d := range req.CostData {
		dataPoints[i] = ml.CostDataPoint{
			Date:             d.Date,
			Cost:             d.Cost,
			ServiceBreakdown: d.ServiceBreakdown,
		}
	}

	var trainingData []ml.CostDataPoint
	if len(req.TrainingData) > 0 {
		trainingData = make([]ml.CostDataPoint, len(req.TrainingData))
		for i, d := range req.TrainingData {
			trainingData[i] = ml.CostDataPoint{
				Date: d.Date,
				Cost: d.Cost,
			}
		}
	}

	mlReq := &ml.AnomalyRequest{
		CostData:        dataPoints,
		TrainingData:    trainingData,
		Contamination:   req.Contamination,
		ReturnAllScores: req.ReturnAllScores,
	}

	anomalies, err := h.mlClient.DetectAnomalies(ctx, mlReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "anomaly detection failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, anomalies)
}

// GetAnomalyScore handles POST /api/v1/anomalies/score for real-time scoring
func (h *AnomalyHandler) GetAnomalyScore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		Date string  `json:"date"`
		Cost float64 `json:"cost"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	dataPoint := ml.CostDataPoint{
		Date: req.Date,
		Cost: req.Cost,
	}

	result, err := h.mlClient.ScoreSinglePoint(ctx, dataPoint)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "scoring failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}
