"""FinOpsMind ML Sidecar - Forecasting and Anomaly Detection Service."""

import os
import logging
from datetime import datetime, timedelta
from typing import List
from contextlib import asynccontextmanager

import numpy as np
from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field

logging.basicConfig(level=logging.INFO, format="%(asctime)s - %(name)s - %(levelname)s - %(message)s")
logger = logging.getLogger("finopsmind-ml")

# Check optional dependencies
PROPHET_AVAILABLE = False
SKLEARN_AVAILABLE = False

try:
    from prophet import Prophet
    PROPHET_AVAILABLE = True
except ImportError:
    pass

try:
    from sklearn.ensemble import IsolationForest
    SKLEARN_AVAILABLE = True
except ImportError:
    pass


# Models
class CostDataPoint(BaseModel):
    date: str
    amount: float

class ForecastRequest(BaseModel):
    organization_id: str
    data: List[CostDataPoint]
    forecast_days: int = Field(default=30, ge=7, le=90)
    granularity: str = "daily"
    confidence_level: float = Field(default=0.95, ge=0.5, le=0.99)

class ForecastPoint(BaseModel):
    date: str
    predicted: float
    lower_bound: float
    upper_bound: float

class ForecastResponse(BaseModel):
    organization_id: str
    generated_at: datetime
    model_version: str
    forecasts: List[ForecastPoint]
    total_forecasted: float
    confidence_level: float

class AnomalyDetectionRequest(BaseModel):
    organization_id: str
    data: List[CostDataPoint]
    sensitivity: float = Field(default=0.1, ge=0.01, le=0.5)

class DetectedAnomaly(BaseModel):
    date: str
    actual_amount: float
    expected_amount: float
    deviation: float
    deviation_pct: float
    score: float
    severity: str

class AnomalyDetectionResponse(BaseModel):
    organization_id: str
    analyzed_at: datetime
    model_version: str
    anomalies: List[DetectedAnomaly]
    total_analyzed: int
    anomaly_count: int
    threshold: float

class HealthResponse(BaseModel):
    status: str
    version: str
    models: dict


# Forecasting Service
class ForecastingService:
    def __init__(self):
        self.model_version = "prophet-1.0" if PROPHET_AVAILABLE else "linear-regression-1.0"
    
    def forecast(self, data: List[CostDataPoint], forecast_days: int, confidence_level: float) -> List[ForecastPoint]:
        if len(data) < 7:
            raise ValueError("Need at least 7 data points")
        
        if PROPHET_AVAILABLE:
            return self._prophet_forecast(data, forecast_days, confidence_level)
        return self._linear_forecast(data, forecast_days, confidence_level)
    
    def _prophet_forecast(self, data, forecast_days, confidence_level):
        import pandas as pd
        df = pd.DataFrame([{"ds": d.date, "y": d.amount} for d in data])
        df["ds"] = pd.to_datetime(df["ds"])
        
        model = Prophet(interval_width=confidence_level, daily_seasonality=False, weekly_seasonality=True)
        model.fit(df)
        
        future = model.make_future_dataframe(periods=forecast_days)
        forecast = model.predict(future).tail(forecast_days)
        
        return [ForecastPoint(
            date=row["ds"].strftime("%Y-%m-%d"),
            predicted=max(0, row["yhat"]),
            lower_bound=max(0, row["yhat_lower"]),
            upper_bound=max(0, row["yhat_upper"])
        ) for _, row in forecast.iterrows()]
    
    def _linear_forecast(self, data, forecast_days, confidence_level):
        amounts = np.array([d.amount for d in data])
        x = np.arange(len(amounts))
        slope, intercept = np.polyfit(x, amounts, 1)
        std_error = np.std(amounts - (slope * x + intercept))
        z_score = 1.96 if confidence_level >= 0.95 else 1.645
        
        last_date = datetime.strptime(data[-1].date, "%Y-%m-%d")
        predictions = []
        
        for i in range(forecast_days):
            future_x = len(amounts) + i
            predicted = slope * future_x + intercept
            margin = z_score * std_error
            forecast_date = last_date + timedelta(days=i + 1)
            predictions.append(ForecastPoint(
                date=forecast_date.strftime("%Y-%m-%d"),
                predicted=max(0, predicted),
                lower_bound=max(0, predicted - margin),
                upper_bound=max(0, predicted + margin)
            ))
        return predictions


# Anomaly Detection Service
class AnomalyDetectionService:
    def __init__(self):
        self.model_version = "isolation-forest-1.0" if SKLEARN_AVAILABLE else "z-score-1.0"
    
    def detect(self, data: List[CostDataPoint], sensitivity: float) -> List[DetectedAnomaly]:
        if len(data) < 14:
            raise ValueError("Need at least 14 data points")
        
        if SKLEARN_AVAILABLE:
            return self._isolation_forest_detect(data, sensitivity)
        return self._zscore_detect(data, sensitivity)
    
    def _isolation_forest_detect(self, data, sensitivity):
        amounts = np.array([[d.amount] for d in data])
        model = IsolationForest(contamination=sensitivity, random_state=42)
        predictions = model.fit_predict(amounts)
        scores = model.decision_function(amounts)
        scores_normalized = (scores - scores.min()) / (scores.max() - scores.min() + 1e-10)
        
        anomalies = []
        mean_amount = np.mean(amounts)
        
        for i, (pred, score) in enumerate(zip(predictions, scores_normalized)):
            if pred == -1:
                actual = data[i].amount
                deviation = actual - mean_amount
                deviation_pct = (deviation / mean_amount) * 100 if mean_amount > 0 else 0
                anomalies.append(DetectedAnomaly(
                    date=data[i].date,
                    actual_amount=actual,
                    expected_amount=round(float(mean_amount), 2),
                    deviation=round(deviation, 2),
                    deviation_pct=round(deviation_pct, 2),
                    score=round(1 - score, 4),
                    severity=self._classify_severity(abs(deviation_pct))
                ))
        return anomalies
    
    def _zscore_detect(self, data, sensitivity):
        amounts = np.array([d.amount for d in data])
        mean, std = np.mean(amounts), np.std(amounts)
        threshold = 3.0 - (sensitivity * 4)
        
        anomalies = []
        for i, amount in enumerate(amounts):
            z_score = abs((amount - mean) / std) if std > 0 else 0
            if z_score > threshold:
                deviation = amount - mean
                deviation_pct = (deviation / mean) * 100 if mean > 0 else 0
                anomalies.append(DetectedAnomaly(
                    date=data[i].date,
                    actual_amount=amount,
                    expected_amount=round(mean, 2),
                    deviation=round(deviation, 2),
                    deviation_pct=round(deviation_pct, 2),
                    score=round(min(z_score / 5, 1), 4),
                    severity=self._classify_severity(abs(deviation_pct))
                ))
        return anomalies
    
    def _classify_severity(self, deviation_pct):
        if deviation_pct >= 100: return "critical"
        if deviation_pct >= 50: return "high"
        if deviation_pct >= 25: return "medium"
        return "low"


forecasting_service = ForecastingService()
anomaly_service = AnomalyDetectionService()

@asynccontextmanager
async def lifespan(app: FastAPI):
    logger.info(f"Starting ML Sidecar - Prophet: {PROPHET_AVAILABLE}, sklearn: {SKLEARN_AVAILABLE}")
    yield

app = FastAPI(title="FinOpsMind ML Sidecar", version="1.0.0", lifespan=lifespan)
app.add_middleware(CORSMiddleware, allow_origins=["*"], allow_methods=["*"], allow_headers=["*"])

@app.get("/health", response_model=HealthResponse)
async def health():
    return HealthResponse(status="healthy", version="1.0.0", models={
        "prophet": PROPHET_AVAILABLE, "isolation_forest": SKLEARN_AVAILABLE,
        "forecasting": forecasting_service.model_version, "anomaly_detection": anomaly_service.model_version
    })

@app.post("/api/v1/forecast", response_model=ForecastResponse)
async def forecast(request: ForecastRequest):
    try:
        forecasts = forecasting_service.forecast(request.data, request.forecast_days, request.confidence_level)
        return ForecastResponse(
            organization_id=request.organization_id, generated_at=datetime.utcnow(),
            model_version=forecasting_service.model_version, forecasts=forecasts,
            total_forecasted=round(sum(f.predicted for f in forecasts), 2), confidence_level=request.confidence_level
        )
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))

@app.post("/api/v1/anomalies/detect", response_model=AnomalyDetectionResponse)
async def detect_anomalies(request: AnomalyDetectionRequest):
    try:
        anomalies = anomaly_service.detect(request.data, request.sensitivity)
        return AnomalyDetectionResponse(
            organization_id=request.organization_id, analyzed_at=datetime.utcnow(),
            model_version=anomaly_service.model_version, anomalies=anomalies,
            total_analyzed=len(request.data), anomaly_count=len(anomalies), threshold=request.sensitivity
        )
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=int(os.getenv("PORT", "8000")))
