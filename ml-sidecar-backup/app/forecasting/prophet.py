"""
Prophet-based cost forecasting module for FinOpsMind.
Generates daily cost forecasts with confidence intervals.
"""

import logging
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Tuple
import numpy as np
import pandas as pd

try:
    from prophet import Prophet
    PROPHET_AVAILABLE = True
except ImportError:
    PROPHET_AVAILABLE = False
    logging.warning("Prophet not installed. Using fallback forecasting.")

logger = logging.getLogger(__name__)


class CostForecaster:
    """
    Cost forecasting using Facebook Prophet.
    Handles weekly and monthly seasonality in cloud cost data.
    """
    
    def __init__(
        self,
        weekly_seasonality: bool = True,
        monthly_seasonality: bool = True,
        yearly_seasonality: bool = False,
        changepoint_prior_scale: float = 0.05,
        seasonality_prior_scale: float = 10.0,
    ):
        self.weekly_seasonality = weekly_seasonality
        self.monthly_seasonality = monthly_seasonality
        self.yearly_seasonality = yearly_seasonality
        self.changepoint_prior_scale = changepoint_prior_scale
        self.seasonality_prior_scale = seasonality_prior_scale
        self.model: Optional[Prophet] = None
        self._is_fitted = False
        
    def _prepare_data(self, historical_data: List[Dict]) -> pd.DataFrame:
        """
        Convert historical cost data to Prophet format.
        
        Args:
            historical_data: List of {"date": "YYYY-MM-DD", "cost": float}
            
        Returns:
            DataFrame with 'ds' and 'y' columns
        """
        df = pd.DataFrame(historical_data)
        
        # Handle various date column names
        date_col = None
        for col in ['date', 'ds', 'timestamp', 'time']:
            if col in df.columns:
                date_col = col
                break
        
        cost_col = None
        for col in ['cost', 'y', 'value', 'amount']:
            if col in df.columns:
                cost_col = col
                break
                
        if date_col is None or cost_col is None:
            raise ValueError("Data must contain date and cost columns")
            
        # Rename to Prophet format
        df = df.rename(columns={date_col: 'ds', cost_col: 'y'})
        df['ds'] = pd.to_datetime(df['ds'])
        df['y'] = df['y'].astype(float)
        
        # Sort by date
        df = df.sort_values('ds').reset_index(drop=True)
        
        # Remove duplicates (keep last value for each day)
        df = df.drop_duplicates(subset=['ds'], keep='last')
        
        return df[['ds', 'y']]
    
    def fit(self, historical_data: List[Dict]) -> 'CostForecaster':
        """
        Train the Prophet model on historical cost data.
        
        Args:
            historical_data: List of {"date": "YYYY-MM-DD", "cost": float}
            
        Returns:
            self for method chaining
        """
        df = self._prepare_data(historical_data)
        
        if len(df) < 14:
            raise ValueError("Need at least 14 days of data for reliable forecasting")
        
        if not PROPHET_AVAILABLE:
            # Store data for fallback
            self._fallback_data = df
            self._is_fitted = True
            return self
        
        # Initialize Prophet model
        self.model = Prophet(
            weekly_seasonality=self.weekly_seasonality,
            yearly_seasonality=self.yearly_seasonality,
            changepoint_prior_scale=self.changepoint_prior_scale,
            seasonality_prior_scale=self.seasonality_prior_scale,
            interval_width=0.95,  # 95% confidence interval
        )
        
        # Add monthly seasonality if requested
        if self.monthly_seasonality:
            self.model.add_seasonality(
                name='monthly',
                period=30.5,
                fourier_order=5
            )
        
        # Fit the model
        logger.info(f"Training Prophet model on {len(df)} data points")
        self.model.fit(df)
        self._is_fitted = True
        
        return self
    
    def predict(
        self,
        periods: int = 30,
        include_history: bool = False
    ) -> Dict:
        """
        Generate cost forecasts.
        
        Args:
            periods: Number of days to forecast (default 30)
            include_history: Whether to include historical fitted values
            
        Returns:
            Dictionary with forecast data including confidence intervals
        """
        if not self._is_fitted:
            raise RuntimeError("Model must be fitted before prediction")
        
        if not PROPHET_AVAILABLE:
            return self._fallback_predict(periods, include_history)
        
        # Create future dataframe
        future = self.model.make_future_dataframe(periods=periods)
        
        # Generate predictions
        forecast = self.model.predict(future)
        
        # Extract relevant columns
        if include_history:
            result_df = forecast
        else:
            # Only return future predictions
            cutoff = self.model.history['ds'].max()
            result_df = forecast[forecast['ds'] > cutoff]
        
        # Format output
        predictions = []
        for _, row in result_df.iterrows():
            predictions.append({
                'date': row['ds'].strftime('%Y-%m-%d'),
                'predicted_cost': round(max(0, row['yhat']), 2),
                'lower_bound_80': round(max(0, row['yhat_lower'] + 0.1 * (row['yhat'] - row['yhat_lower'])), 2),
                'upper_bound_80': round(row['yhat_upper'] - 0.1 * (row['yhat_upper'] - row['yhat']), 2),
                'lower_bound_95': round(max(0, row['yhat_lower']), 2),
                'upper_bound_95': round(row['yhat_upper'], 2),
            })
        
        # Calculate summary statistics
        total_predicted = sum(p['predicted_cost'] for p in predictions)
        avg_daily = total_predicted / len(predictions) if predictions else 0
        
        return {
            'forecast_generated_at': datetime.utcnow().isoformat(),
            'periods': periods,
            'predictions': predictions,
            'summary': {
                'total_predicted_cost': round(total_predicted, 2),
                'average_daily_cost': round(avg_daily, 2),
                'forecast_start': predictions[0]['date'] if predictions else None,
                'forecast_end': predictions[-1]['date'] if predictions else None,
            }
        }
    
    def _fallback_predict(self, periods: int, include_history: bool) -> Dict:
        """
        Simple fallback prediction when Prophet is not available.
        Uses linear regression with weekly seasonality.
        """
        df = self._fallback_data.copy()
        
        # Calculate basic statistics
        mean_cost = df['y'].mean()
        std_cost = df['y'].std()
        
        # Simple linear trend
        df['day_num'] = (df['ds'] - df['ds'].min()).dt.days
        if len(df) > 1:
            slope = np.polyfit(df['day_num'], df['y'], 1)[0]
        else:
            slope = 0
        
        # Calculate weekly pattern
        df['dow'] = df['ds'].dt.dayofweek
        weekly_pattern = df.groupby('dow')['y'].mean()
        weekly_adjustment = weekly_pattern - weekly_pattern.mean()
        
        # Generate predictions
        last_date = df['ds'].max()
        last_day_num = df['day_num'].max()
        
        predictions = []
        for i in range(1, periods + 1):
            pred_date = last_date + timedelta(days=i)
            day_num = last_day_num + i
            dow = pred_date.dayofweek
            
            # Base prediction with trend
            base_pred = mean_cost + slope * (day_num - df['day_num'].mean())
            
            # Add weekly adjustment
            if dow in weekly_adjustment.index:
                base_pred += weekly_adjustment[dow]
            
            predictions.append({
                'date': pred_date.strftime('%Y-%m-%d'),
                'predicted_cost': round(max(0, base_pred), 2),
                'lower_bound_80': round(max(0, base_pred - 1.28 * std_cost), 2),
                'upper_bound_80': round(base_pred + 1.28 * std_cost, 2),
                'lower_bound_95': round(max(0, base_pred - 1.96 * std_cost), 2),
                'upper_bound_95': round(base_pred + 1.96 * std_cost, 2),
            })
        
        total_predicted = sum(p['predicted_cost'] for p in predictions)
        
        return {
            'forecast_generated_at': datetime.utcnow().isoformat(),
            'periods': periods,
            'predictions': predictions,
            'summary': {
                'total_predicted_cost': round(total_predicted, 2),
                'average_daily_cost': round(total_predicted / periods, 2),
                'forecast_start': predictions[0]['date'],
                'forecast_end': predictions[-1]['date'],
            },
            'note': 'Using fallback forecasting (Prophet not available)'
        }
    
    def get_components(self) -> Optional[Dict]:
        """
        Get the decomposed forecast components (trend, seasonality).
        """
        if not self._is_fitted or not PROPHET_AVAILABLE or self.model is None:
            return None
        
        future = self.model.make_future_dataframe(periods=30)
        forecast = self.model.predict(future)
        
        components = {
            'trend': forecast[['ds', 'trend']].to_dict('records'),
        }
        
        if self.weekly_seasonality:
            components['weekly'] = forecast[['ds', 'weekly']].to_dict('records')
        
        if self.monthly_seasonality and 'monthly' in forecast.columns:
            components['monthly'] = forecast[['ds', 'monthly']].to_dict('records')
        
        return components


def generate_forecast(
    historical_data: List[Dict],
    periods: int = 30,
    weekly_seasonality: bool = True,
    monthly_seasonality: bool = True,
) -> Dict:
    """
    Convenience function to generate a forecast from historical data.
    
    Args:
        historical_data: List of {"date": "YYYY-MM-DD", "cost": float}
        periods: Number of days to forecast
        weekly_seasonality: Include weekly patterns
        monthly_seasonality: Include monthly patterns
        
    Returns:
        Forecast dictionary with predictions and confidence intervals
    """
    forecaster = CostForecaster(
        weekly_seasonality=weekly_seasonality,
        monthly_seasonality=monthly_seasonality,
    )
    
    forecaster.fit(historical_data)
    return forecaster.predict(periods=periods)
