package jobs

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/finopsmind/backend/internal/ml"
	"github.com/finopsmind/backend/internal/model"
)

// MLJobRunner runs scheduled ML tasks
type MLJobRunner struct {
	mlClient *ml.Client
	store    model.CostStore
	accounts []string // List of account IDs to process
}

// NewMLJobRunner creates a new ML job runner
func NewMLJobRunner(mlClient *ml.Client, store model.CostStore, accounts []string) *MLJobRunner {
	return &MLJobRunner{
		mlClient: mlClient,
		store:    store,
		accounts: accounts,
	}
}

// RunDailyForecasting runs forecasting for all accounts
// Should be scheduled to run daily (e.g., via cron at 00:00 UTC)
func (r *MLJobRunner) RunDailyForecasting(ctx context.Context) error {
	log.Println("Starting daily forecasting job")
	
	for _, accountID := range r.accounts {
		if err := r.runForecastForAccount(ctx, accountID); err != nil {
			log.Printf("Forecasting failed for account %s: %v", accountID, err)
			continue
		}
		log.Printf("Forecasting completed for account %s", accountID)
	}
	
	log.Println("Daily forecasting job completed")
	return nil
}

// runForecastForAccount runs forecasting for a single account
func (r *MLJobRunner) runForecastForAccount(ctx context.Context, accountID string) error {
	// Get last 90 days of cost data
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -90)
	
	costs, err := r.store.GetDailyCosts(ctx, accountID, startDate, endDate)
	if err != nil {
		return err
	}
	
	if len(costs) < 14 {
		log.Printf("Insufficient data for account %s (only %d days)", accountID, len(costs))
		return nil
	}
	
	// Convert to ML format
	dataPoints := make([]ml.CostDataPoint, len(costs))
	for i, cost := range costs {
		dataPoints[i] = ml.CostDataPoint{
			Date: cost.Date.Format("2006-01-02"),
			Cost: cost.TotalCost,
		}
	}
	
	// Generate forecast
	mlReq := &ml.ForecastRequest{
		HistoricalData:     dataPoints,
		Periods:            30,
		WeeklySeasonality:  true,
		MonthlySeasonality: true,
		AccountID:          accountID,
	}
	
	forecast, err := r.mlClient.GenerateForecast(ctx, mlReq)
	if err != nil {
		return err
	}
	
	// Store forecast
	predictionsJSON, _ := json.Marshal(forecast.Predictions)
	summaryJSON, _ := json.Marshal(forecast.Summary)
	
	forecastStart, _ := time.Parse("2006-01-02", forecast.Summary.ForecastStart)
	forecastEnd, _ := time.Parse("2006-01-02", forecast.Summary.ForecastEnd)
	
	dbForecast := &model.CostForecast{
		AccountID:          accountID,
		GeneratedAt:        time.Now(),
		ForecastStartDate:  forecastStart,
		ForecastEndDate:    forecastEnd,
		Predictions:        predictionsJSON,
		Summary:            summaryJSON,
		TotalPredictedCost: forecast.Summary.TotalPredictedCost,
	}
	
	return r.store.SaveForecast(ctx, dbForecast)
}

// RunHourlyAnomalyDetection runs anomaly detection for all accounts
// Should be scheduled to run hourly
func (r *MLJobRunner) RunHourlyAnomalyDetection(ctx context.Context) error {
	log.Println("Starting hourly anomaly detection job")
	
	for _, accountID := range r.accounts {
		if err := r.runAnomalyDetectionForAccount(ctx, accountID); err != nil {
			log.Printf("Anomaly detection failed for account %s: %v", accountID, err)
			continue
		}
		log.Printf("Anomaly detection completed for account %s", accountID)
	}
	
	log.Println("Hourly anomaly detection job completed")
	return nil
}

// runAnomalyDetectionForAccount runs anomaly detection for a single account
func (r *MLJobRunner) runAnomalyDetectionForAccount(ctx context.Context, accountID string) error {
	// Get last 30 days of cost data
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)
	
	costs, err := r.store.GetDailyCosts(ctx, accountID, startDate, endDate)
	if err != nil {
		return err
	}
	
	if len(costs) < 7 {
		log.Printf("Insufficient data for anomaly detection on account %s", accountID)
		return nil
	}
	
	// Convert to ML format
	dataPoints := make([]ml.CostDataPoint, len(costs))
	for i, cost := range costs {
		dataPoints[i] = ml.CostDataPoint{
			Date:             cost.Date.Format("2006-01-02"),
			Cost:             cost.TotalCost,
			ServiceBreakdown: cost.ServiceBreakdown,
		}
	}
	
	// Detect anomalies
	mlReq := &ml.AnomalyRequest{
		CostData:        dataPoints,
		Contamination:   0.1,
		ReturnAllScores: false,
	}
	
	result, err := r.mlClient.DetectAnomalies(ctx, mlReq)
	if err != nil {
		return err
	}
	
	// Store new anomalies
	for _, anomaly := range result.Anomalies {
		anomalyDate, _ := time.Parse("2006-01-02", anomaly.Date)
		
		// Convert root cause to map
		var rootCause map[string]any
		if anomaly.RootCause != nil {
			rootCause = map[string]any{
				"primary_factor":       anomaly.RootCause.PrimaryFactor,
				"contributing_factors": anomaly.RootCause.ContributingFactors,
				"details":              anomaly.RootCause.Details,
			}
		}
		
		dbAnomaly := &model.CostAnomaly{
			AccountID:    accountID,
			Date:         anomalyDate,
			Cost:         anomaly.Cost,
			AnomalyScore: anomaly.AnomalyScore,
			Severity:     anomaly.Severity,
			RootCause:    rootCause,
			Acknowledged: false,
		}
		
		if err := r.store.SaveAnomaly(ctx, dbAnomaly); err != nil {
			log.Printf("Failed to save anomaly: %v", err)
		}
	}
	
	return nil
}

// StartScheduler starts the background job scheduler
func (r *MLJobRunner) StartScheduler(ctx context.Context) {
	// Run forecasting daily at midnight UTC
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		
		// Calculate time until next midnight
		now := time.Now().UTC()
		nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
		initialDelay := nextMidnight.Sub(now)
		
		// Wait until midnight
		time.Sleep(initialDelay)
		
		// Run immediately at midnight
		r.RunDailyForecasting(ctx)
		
		// Then run every 24 hours
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.RunDailyForecasting(ctx)
			}
		}
	}()
	
	// Run anomaly detection hourly
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		
		// Run immediately on startup
		r.RunHourlyAnomalyDetection(ctx)
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.RunHourlyAnomalyDetection(ctx)
			}
		}
	}()
	
	log.Println("ML job scheduler started")
}
