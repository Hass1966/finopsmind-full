"""
FinOpsMind ML Sidecar - FastAPI application
Provides ML endpoints for cost forecasting and anomaly detection.
"""

import logging
import os
from datetime import datetime
from typing import Dict, List, Optional
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException, BackgroundTasks
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field

from forecasting.prophet import CostForecaster, generate_forecast
from anomaly.isolation_forest import CostAnomalyDetector, detect_anomalies

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Cache for trained models
model_cache: Dict[str, any] = {
    'forecaster': None,
    'anomaly_detector': None,
    'forecast_cache': {},
    'last_forecast_time': None,
    'last_anomaly_time': None,
}


# ==================== Pydantic Models ====================

class CostDataPoint(BaseModel):
    """Single cost data point."""
    date: str = Field(..., description="Date in YYYY-MM-DD format")
    cost: float = Field(..., description="Cost value", ge=0)
    service_breakdown: Optional[Dict[str, float]] = Field(
        None, description="Optional breakdown by service"
    )


class ForecastRequest(BaseModel):
    """Request model for cost forecasting."""
    historical_data: List[CostDataPoint] = Field(
        ..., description="Historical cost data (30-90 days recommended)"
    )
    periods: int = Field(30, description="Number of days to forecast", ge=1, le=90)
    weekly_seasonality: bool = Field(True, description="Include weekly seasonality")
    monthly_seasonality: bool = Field(True, description="Include monthly seasonality")
    account_id: Optional[str] = Field(None, description="Account ID for caching")


class ForecastResponse(BaseModel):
    """Response model for cost forecasting."""
    forecast_generated_at: str
    periods: int
    predictions: List[Dict]
    summary: Dict
    cached: bool = False
    note: Optional[str] = None


class AnomalyRequest(BaseModel):
    """Request model for anomaly detection."""
    cost_data: List[CostDataPoint] = Field(
        ..., description="Cost time series to analyze"
    )
    training_data: Optional[List[CostDataPoint]] = Field(
        None, description="Optional separate training data"
    )
    contamination: float = Field(
        0.1, description="Expected proportion of anomalies", ge=0.01, le=0.5
    )
    return_all_scores: bool = Field(
        True, description="Return scores for all points, not just anomalies"
    )


class AnomalyResponse(BaseModel):
    """Response model for anomaly detection."""
    detection_timestamp: str
    total_points_analyzed: int
    anomalies_detected: int
    anomaly_rate: float
    anomalies: List[Dict]
    all_results: Optional[List[Dict]]
    thresholds: Dict
    note: Optional[str] = None


class HealthResponse(BaseModel):
    """Health check response."""
    status: str
    timestamp: str
    version: str
    models_loaded: Dict[str, bool]
    cache_status: Dict


# ==================== Lifespan Management ====================

@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan manager."""
    logger.info("Starting FinOpsMind ML Sidecar...")
    logger.info("ML endpoints ready")
    yield
    logger.info("Shutting down ML Sidecar...")


# ==================== FastAPI App ====================

app = FastAPI(
    title="FinOpsMind ML Sidecar",
    description="Machine Learning service for cost forecasting and anomaly detection",
    version="1.0.0",
    lifespan=lifespan,
)

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # Configure appropriately for production
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


# ==================== Endpoints ====================

@app.get("/health", response_model=HealthResponse)
async def health_check():
    """
    Health check endpoint.
    Returns status of the ML service and loaded models.
    """
    return HealthResponse(
        status="healthy",
        timestamp=datetime.utcnow().isoformat(),
        version="1.0.0",
        models_loaded={
            "forecaster": model_cache['forecaster'] is not None,
            "anomaly_detector": model_cache['anomaly_detector'] is not None,
        },
        cache_status={
            "forecast_cache_size": len(model_cache['forecast_cache']),
            "last_forecast_time": model_cache['last_forecast_time'],
            "last_anomaly_time": model_cache['last_anomaly_time'],
        }
    )


@app.post("/forecast", response_model=ForecastResponse)
async def generate_cost_forecast(request: ForecastRequest):
    """
    Generate cost forecast using Prophet model.
    
    Takes 30-90 days of historical cost data and generates a forecast
    for the specified number of periods with confidence intervals.
    
    - **historical_data**: List of {date, cost} records
    - **periods**: Number of days to forecast (default: 30)
    - **weekly_seasonality**: Include weekly patterns (default: True)
    - **monthly_seasonality**: Include monthly patterns (default: True)
    """
    try:
        # Check cache if account_id provided
        cache_key = f"{request.account_id}_{request.periods}" if request.account_id else None
        if cache_key and cache_key in model_cache['forecast_cache']:
            cached = model_cache['forecast_cache'][cache_key]
            # Return cached if less than 1 hour old
            cache_time = datetime.fromisoformat(cached['forecast_generated_at'])
            if (datetime.utcnow() - cache_time).seconds < 3600:
                logger.info(f"Returning cached forecast for {cache_key}")
                return ForecastResponse(**cached, cached=True)
        
        # Validate data length
        if len(request.historical_data) < 14:
            raise HTTPException(
                status_code=400,
                detail="Need at least 14 days of historical data for forecasting"
            )
        
        # Convert to list of dicts
        historical = [
            {"date": dp.date, "cost": dp.cost}
            for dp in request.historical_data
        ]
        
        # Generate forecast
        logger.info(f"Generating {request.periods}-day forecast from {len(historical)} data points")
        
        result = generate_forecast(
            historical_data=historical,
            periods=request.periods,
            weekly_seasonality=request.weekly_seasonality,
            monthly_seasonality=request.monthly_seasonality,
        )
        
        # Update cache
        model_cache['last_forecast_time'] = datetime.utcnow().isoformat()
        if cache_key:
            model_cache['forecast_cache'][cache_key] = result
        
        return ForecastResponse(**result, cached=False)
        
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"Forecast error: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Forecast generation failed: {str(e)}")


@app.post("/anomalies/detect", response_model=AnomalyResponse)
async def detect_cost_anomalies(request: AnomalyRequest):
    """
    Detect anomalies in cost data using Isolation Forest.
    
    Analyzes cost time series to identify unusual spending patterns
    and attempts to determine root causes.
    
    - **cost_data**: Cost time series to analyze
    - **training_data**: Optional separate training data
    - **contamination**: Expected proportion of anomalies (default: 0.1)
    - **return_all_scores**: Return scores for all points (default: True)
    """
    try:
        # Validate data length
        if len(request.cost_data) < 7:
            raise HTTPException(
                status_code=400,
                detail="Need at least 7 days of data for anomaly detection"
            )
        
        # Convert to list of dicts
        cost_data = [
            {"date": dp.date, "cost": dp.cost, **(dp.service_breakdown or {})}
            for dp in request.cost_data
        ]
        
        training_data = None
        if request.training_data:
            training_data = [
                {"date": dp.date, "cost": dp.cost, **(dp.service_breakdown or {})}
                for dp in request.training_data
            ]
        
        # Detect anomalies
        logger.info(f"Detecting anomalies in {len(cost_data)} data points")
        
        result = detect_anomalies(
            cost_data=cost_data,
            training_data=training_data,
            contamination=request.contamination,
        )
        
        # Update cache
        model_cache['last_anomaly_time'] = datetime.utcnow().isoformat()
        
        # Filter all_results if not requested
        if not request.return_all_scores:
            result['all_results'] = None
        
        return AnomalyResponse(**result)
        
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"Anomaly detection error: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Anomaly detection failed: {str(e)}")


@app.post("/anomalies/score")
async def score_single_point(data_point: CostDataPoint):
    """
    Score a single data point for anomaly probability.
    Requires a pre-trained model in cache.
    """
    if model_cache['anomaly_detector'] is None:
        raise HTTPException(
            status_code=400,
            detail="No trained model available. Call /anomalies/detect first with training data."
        )
    
    try:
        detector = model_cache['anomaly_detector']
        result = detector.score_single({
            "date": data_point.date,
            "cost": data_point.cost,
        })
        return result
    except Exception as e:
        logger.error(f"Scoring error: {str(e)}")
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/train/forecaster")
async def train_forecaster(request: ForecastRequest, background_tasks: BackgroundTasks):
    """
    Pre-train a forecaster model for faster predictions.
    Training happens in the background.
    """
    def do_training(historical_data: List[Dict]):
        try:
            forecaster = CostForecaster(
                weekly_seasonality=request.weekly_seasonality,
                monthly_seasonality=request.monthly_seasonality,
            )
            forecaster.fit(historical_data)
            model_cache['forecaster'] = forecaster
            logger.info("Forecaster training complete")
        except Exception as e:
            logger.error(f"Background training failed: {e}")
    
    historical = [
        {"date": dp.date, "cost": dp.cost}
        for dp in request.historical_data
    ]
    
    background_tasks.add_task(do_training, historical)
    
    return {"status": "training_started", "data_points": len(historical)}


@app.post("/train/anomaly-detector")
async def train_anomaly_detector(request: AnomalyRequest, background_tasks: BackgroundTasks):
    """
    Pre-train an anomaly detector for faster scoring.
    Training happens in the background.
    """
    def do_training(cost_data: List[Dict]):
        try:
            detector = CostAnomalyDetector(contamination=request.contamination)
            detector.fit(cost_data)
            model_cache['anomaly_detector'] = detector
            logger.info("Anomaly detector training complete")
        except Exception as e:
            logger.error(f"Background training failed: {e}")
    
    cost_data = [
        {"date": dp.date, "cost": dp.cost}
        for dp in request.cost_data
    ]
    
    background_tasks.add_task(do_training, cost_data)
    
    return {"status": "training_started", "data_points": len(cost_data)}


@app.delete("/cache")
async def clear_cache():
    """Clear all cached models and forecasts."""
    model_cache['forecaster'] = None
    model_cache['anomaly_detector'] = None
    model_cache['forecast_cache'] = {}
    model_cache['last_forecast_time'] = None
    model_cache['last_anomaly_time'] = None
    
    return {"status": "cache_cleared"}


# ==================== Main ====================

if __name__ == "__main__":
    import uvicorn
    
    port = int(os.getenv("ML_SIDECAR_PORT", "8081"))
    host = os.getenv("ML_SIDECAR_HOST", "0.0.0.0")
    
    uvicorn.run(
        "main:app",
        host=host,
        port=port,
        reload=os.getenv("ENVIRONMENT", "development") == "development",
        log_level="info",
    )
