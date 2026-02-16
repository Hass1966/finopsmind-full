"""
Isolation Forest-based anomaly detection for FinOpsMind.
Detects cost anomalies and identifies root causes.
"""

import logging
from datetime import datetime
from typing import Dict, List, Optional, Tuple
import numpy as np
import pandas as pd

try:
    from sklearn.ensemble import IsolationForest
    from sklearn.preprocessing import StandardScaler
    SKLEARN_AVAILABLE = True
except ImportError:
    SKLEARN_AVAILABLE = False
    logging.warning("scikit-learn not installed. Using fallback anomaly detection.")

logger = logging.getLogger(__name__)


class CostAnomalyDetector:
    """
    Anomaly detection for cloud costs using Isolation Forest.
    Identifies unusual cost patterns and attempts to determine root causes.
    """
    
    def __init__(
        self,
        contamination: float = 0.1,
        n_estimators: int = 100,
        max_samples: str = 'auto',
        random_state: int = 42,
    ):
        """
        Initialize the anomaly detector.
        
        Args:
            contamination: Expected proportion of anomalies (0.0 to 0.5)
            n_estimators: Number of trees in the forest
            max_samples: Number of samples per tree
            random_state: Random seed for reproducibility
        """
        self.contamination = contamination
        self.n_estimators = n_estimators
        self.max_samples = max_samples
        self.random_state = random_state
        
        self.model: Optional[IsolationForest] = None
        self.scaler: Optional[StandardScaler] = None
        self._is_fitted = False
        self._feature_names: List[str] = []
        self._training_stats: Dict = {}
        
    def _prepare_features(
        self,
        cost_data: List[Dict],
        include_services: bool = True
    ) -> Tuple[pd.DataFrame, pd.DataFrame]:
        """
        Extract features from cost time series data.
        
        Args:
            cost_data: List of cost records with date, cost, and optional service breakdown
            include_services: Whether to include per-service costs as features
            
        Returns:
            Tuple of (features DataFrame, original data DataFrame)
        """
        df = pd.DataFrame(cost_data)
        
        # Handle date column
        date_col = None
        for col in ['date', 'ds', 'timestamp', 'time']:
            if col in df.columns:
                date_col = col
                break
        
        if date_col:
            df['date'] = pd.to_datetime(df[date_col])
            df = df.sort_values('date').reset_index(drop=True)
        
        # Get cost column
        cost_col = None
        for col in ['cost', 'total_cost', 'amount', 'value', 'y']:
            if col in df.columns:
                cost_col = col
                break
        
        if cost_col is None:
            raise ValueError("Data must contain a cost column")
        
        df['total_cost'] = df[cost_col].astype(float)
        
        # Build feature set
        features = pd.DataFrame()
        
        # Basic cost features
        features['total_cost'] = df['total_cost']
        
        # Rolling statistics (if enough data)
        if len(df) >= 7:
            features['rolling_mean_7d'] = df['total_cost'].rolling(7, min_periods=1).mean()
            features['rolling_std_7d'] = df['total_cost'].rolling(7, min_periods=1).std().fillna(0)
            features['cost_vs_rolling_mean'] = features['total_cost'] - features['rolling_mean_7d']
        else:
            features['rolling_mean_7d'] = df['total_cost'].mean()
            features['rolling_std_7d'] = df['total_cost'].std() if len(df) > 1 else 0
            features['cost_vs_rolling_mean'] = features['total_cost'] - features['rolling_mean_7d']
        
        # Day of week effect
        if 'date' in df.columns:
            features['day_of_week'] = df['date'].dt.dayofweek
            features['is_weekend'] = (features['day_of_week'] >= 5).astype(int)
        
        # Day-over-day change
        features['cost_change'] = df['total_cost'].diff().fillna(0)
        features['cost_change_pct'] = df['total_cost'].pct_change().fillna(0).replace([np.inf, -np.inf], 0)
        
        # Service-level features if available
        service_cols = [col for col in df.columns if col.startswith('service_') or col in [
            'ec2', 'rds', 's3', 'lambda', 'cloudfront', 'compute', 'storage', 'network', 'database'
        ]]
        
        if include_services and service_cols:
            for col in service_cols:
                features[f'svc_{col}'] = df[col].astype(float)
        
        # Store feature names
        self._feature_names = features.columns.tolist()
        
        return features, df
    
    def fit(self, cost_data: List[Dict]) -> 'CostAnomalyDetector':
        """
        Train the anomaly detection model on historical cost data.
        
        Args:
            cost_data: List of cost records
            
        Returns:
            self for method chaining
        """
        features, df = self._prepare_features(cost_data)
        
        if len(features) < 14:
            raise ValueError("Need at least 14 days of data for reliable anomaly detection")
        
        # Store training statistics for root cause analysis
        self._training_stats = {
            'mean_cost': df['total_cost'].mean(),
            'std_cost': df['total_cost'].std(),
            'median_cost': df['total_cost'].median(),
            'q1': df['total_cost'].quantile(0.25),
            'q3': df['total_cost'].quantile(0.75),
        }
        
        if not SKLEARN_AVAILABLE:
            self._fallback_stats = self._training_stats.copy()
            self._fallback_stats['iqr'] = self._training_stats['q3'] - self._training_stats['q1']
            self._is_fitted = True
            return self
        
        # Scale features
        self.scaler = StandardScaler()
        scaled_features = self.scaler.fit_transform(features)
        
        # Train Isolation Forest
        self.model = IsolationForest(
            contamination=self.contamination,
            n_estimators=self.n_estimators,
            max_samples=self.max_samples,
            random_state=self.random_state,
            n_jobs=-1,
        )
        
        logger.info(f"Training Isolation Forest on {len(features)} data points")
        self.model.fit(scaled_features)
        self._is_fitted = True
        
        return self
    
    def detect(
        self,
        cost_data: List[Dict],
        return_scores: bool = True
    ) -> Dict:
        """
        Detect anomalies in cost data.
        
        Args:
            cost_data: List of cost records to analyze
            return_scores: Whether to return anomaly scores
            
        Returns:
            Dictionary with anomaly detection results
        """
        if not self._is_fitted:
            raise RuntimeError("Model must be fitted before detection")
        
        features, df = self._prepare_features(cost_data)
        
        if not SKLEARN_AVAILABLE:
            return self._fallback_detect(features, df)
        
        # Scale features
        scaled_features = self.scaler.transform(features)
        
        # Get predictions (-1 for anomaly, 1 for normal)
        predictions = self.model.predict(scaled_features)
        
        # Get anomaly scores (more negative = more anomalous)
        scores = self.model.decision_function(scaled_features)
        
        # Convert scores to probability-like values (0-1, higher = more anomalous)
        # Normalize: score < 0 means anomaly, transform to probability
        anomaly_probs = 1 / (1 + np.exp(scores * 5))  # Sigmoid transformation
        
        # Build results
        anomalies = []
        all_results = []
        
        for i in range(len(df)):
            is_anomaly = predictions[i] == -1
            score = float(anomaly_probs[i])
            
            record = {
                'date': df.iloc[i]['date'].strftime('%Y-%m-%d') if 'date' in df.columns else f"point_{i}",
                'cost': float(df.iloc[i]['total_cost']),
                'is_anomaly': is_anomaly,
                'anomaly_score': round(score, 4),
                'severity': self._classify_severity(score),
            }
            
            if is_anomaly:
                # Identify root cause
                record['root_cause'] = self._identify_root_cause(
                    features.iloc[i],
                    df.iloc[i] if i < len(df) else None
                )
                anomalies.append(record)
            
            if return_scores:
                all_results.append(record)
        
        return {
            'detection_timestamp': datetime.utcnow().isoformat(),
            'total_points_analyzed': len(df),
            'anomalies_detected': len(anomalies),
            'anomaly_rate': round(len(anomalies) / len(df), 4) if df.shape[0] > 0 else 0,
            'anomalies': anomalies,
            'all_results': all_results if return_scores else None,
            'thresholds': {
                'contamination': self.contamination,
                'severity_levels': {
                    'critical': '>= 0.9',
                    'high': '>= 0.7',
                    'medium': '>= 0.5',
                    'low': '< 0.5'
                }
            }
        }
    
    def _fallback_detect(self, features: pd.DataFrame, df: pd.DataFrame) -> Dict:
        """
        Simple fallback anomaly detection using IQR method.
        """
        stats = self._fallback_stats
        iqr = stats['iqr']
        lower_bound = stats['q1'] - 1.5 * iqr
        upper_bound = stats['q3'] + 1.5 * iqr
        
        anomalies = []
        all_results = []
        
        for i in range(len(df)):
            cost = float(df.iloc[i]['total_cost'])
            
            # Calculate how far outside bounds (normalized)
            if cost > upper_bound:
                deviation = (cost - upper_bound) / (iqr if iqr > 0 else 1)
                is_anomaly = True
            elif cost < lower_bound:
                deviation = (lower_bound - cost) / (iqr if iqr > 0 else 1)
                is_anomaly = True
            else:
                deviation = 0
                is_anomaly = False
            
            # Convert to probability-like score
            score = min(1.0, deviation / 3)  # Cap at 1.0
            
            record = {
                'date': df.iloc[i]['date'].strftime('%Y-%m-%d') if 'date' in df.columns else f"point_{i}",
                'cost': cost,
                'is_anomaly': is_anomaly,
                'anomaly_score': round(score, 4),
                'severity': self._classify_severity(score),
            }
            
            if is_anomaly:
                record['root_cause'] = {
                    'primary_factor': 'cost_spike' if cost > stats['mean_cost'] else 'cost_drop',
                    'deviation_from_median': round(cost - stats['median_cost'], 2),
                    'percentage_deviation': round((cost - stats['mean_cost']) / stats['mean_cost'] * 100, 2) if stats['mean_cost'] > 0 else 0,
                }
                anomalies.append(record)
            
            all_results.append(record)
        
        return {
            'detection_timestamp': datetime.utcnow().isoformat(),
            'total_points_analyzed': len(df),
            'anomalies_detected': len(anomalies),
            'anomaly_rate': round(len(anomalies) / len(df), 4) if len(df) > 0 else 0,
            'anomalies': anomalies,
            'all_results': all_results,
            'thresholds': {
                'method': 'IQR (fallback)',
                'lower_bound': round(lower_bound, 2),
                'upper_bound': round(upper_bound, 2),
            },
            'note': 'Using IQR fallback method (scikit-learn not available)'
        }
    
    def _classify_severity(self, score: float) -> str:
        """Classify anomaly severity based on score."""
        if score >= 0.9:
            return 'critical'
        elif score >= 0.7:
            return 'high'
        elif score >= 0.5:
            return 'medium'
        else:
            return 'low'
    
    def _identify_root_cause(
        self,
        features: pd.Series,
        original_record: Optional[pd.Series]
    ) -> Dict:
        """
        Attempt to identify the root cause of an anomaly.
        """
        root_cause = {
            'primary_factor': 'unknown',
            'contributing_factors': [],
            'details': {}
        }
        
        # Check for cost spike vs drop
        if 'cost_vs_rolling_mean' in features:
            deviation = features['cost_vs_rolling_mean']
            if deviation > 0:
                root_cause['primary_factor'] = 'cost_spike'
                root_cause['details']['deviation_from_average'] = round(deviation, 2)
            else:
                root_cause['primary_factor'] = 'cost_drop'
                root_cause['details']['deviation_from_average'] = round(deviation, 2)
        
        # Check for large day-over-day change
        if 'cost_change_pct' in features:
            change_pct = features['cost_change_pct'] * 100
            if abs(change_pct) > 50:
                root_cause['contributing_factors'].append(f'Large day-over-day change: {change_pct:.1f}%')
                root_cause['details']['day_over_day_change_pct'] = round(change_pct, 2)
        
        # Check for weekend effect
        if 'is_weekend' in features and features['is_weekend']:
            root_cause['contributing_factors'].append('Weekend traffic pattern')
        
        # Check service-level contributions
        service_features = [f for f in features.index if f.startswith('svc_')]
        if service_features and original_record is not None:
            service_costs = {}
            for sf in service_features:
                service_name = sf.replace('svc_', '').replace('service_', '')
                if sf in features:
                    service_costs[service_name] = features[sf]
            
            if service_costs:
                # Find top contributing service
                top_service = max(service_costs.items(), key=lambda x: x[1])
                root_cause['contributing_factors'].append(f'Highest cost service: {top_service[0]}')
                root_cause['details']['service_breakdown'] = {
                    k: round(v, 2) for k, v in sorted(
                        service_costs.items(), key=lambda x: x[1], reverse=True
                    )[:5]
                }
        
        # Calculate percentage over baseline
        if self._training_stats:
            cost = features['total_cost']
            mean = self._training_stats['mean_cost']
            if mean > 0:
                pct_over = (cost - mean) / mean * 100
                root_cause['details']['percentage_over_baseline'] = round(pct_over, 2)
        
        return root_cause
    
    def score_single(self, cost_record: Dict) -> Dict:
        """
        Score a single cost record for anomaly probability.
        Useful for real-time scoring.
        """
        result = self.detect([cost_record], return_scores=True)
        if result['all_results']:
            return result['all_results'][0]
        return {'error': 'Could not score record'}


def detect_anomalies(
    cost_data: List[Dict],
    training_data: Optional[List[Dict]] = None,
    contamination: float = 0.1,
) -> Dict:
    """
    Convenience function to detect anomalies in cost data.
    
    Args:
        cost_data: Data to analyze for anomalies
        training_data: Historical data to train on (if different from cost_data)
        contamination: Expected proportion of anomalies
        
    Returns:
        Anomaly detection results
    """
    detector = CostAnomalyDetector(contamination=contamination)
    
    # Use training_data if provided, otherwise use first 80% of cost_data
    if training_data:
        detector.fit(training_data)
    elif len(cost_data) >= 20:
        split_idx = int(len(cost_data) * 0.8)
        detector.fit(cost_data[:split_idx])
    else:
        detector.fit(cost_data)
    
    return detector.detect(cost_data)
