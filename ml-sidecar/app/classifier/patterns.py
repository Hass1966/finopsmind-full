"""
Usage Pattern Detection Module

Analyzes CloudWatch metrics to detect specific usage patterns:
- Idle periods (% time at <5% CPU)
- Burst patterns (spikes and quiet periods)
- Request duration patterns
- Memory patterns
- Diurnal/weekly cycles
"""

from dataclasses import dataclass, field
from datetime import datetime, timedelta
from typing import List, Dict, Optional, Tuple
from enum import Enum
import statistics
import math


class PatternType(Enum):
    """Types of usage patterns that can be detected."""
    IDLE_DOMINANT = "idle_dominant"  # Mostly idle, occasional activity
    STEADY_STATE = "steady_state"    # Consistent usage
    BURSTY = "bursty"                # Spikes and valleys
    DIURNAL = "diurnal"              # Day/night patterns
    WEEKLY = "weekly"                # Weekday/weekend patterns
    BATCH = "batch"                  # Periodic batch processing
    GROWING = "growing"              # Increasing trend
    DECLINING = "declining"          # Decreasing trend
    ERRATIC = "erratic"              # No clear pattern


@dataclass
class MetricPoint:
    """Single metric data point."""
    timestamp: datetime
    value: float


@dataclass
class IdlePattern:
    """Analysis of idle periods."""
    idle_percent: float  # % of time at <5% CPU
    avg_idle_duration_hours: float
    max_idle_duration_hours: float
    idle_periods: List[Tuple[datetime, datetime]]
    is_idle_dominant: bool


@dataclass
class BurstPattern:
    """Analysis of burst activity."""
    is_bursty: bool
    burst_count: int
    avg_burst_duration_minutes: float
    avg_burst_intensity: float  # Peak/baseline ratio
    avg_quiet_duration_hours: float
    burst_periods: List[Tuple[datetime, datetime, float]]  # start, end, peak value


@dataclass
class DiurnalPattern:
    """Analysis of day/night patterns."""
    has_diurnal_pattern: bool
    peak_hour: int  # Hour of day (0-23)
    trough_hour: int
    peak_to_trough_ratio: float
    hourly_averages: Dict[int, float]  # Hour -> average value


@dataclass
class WeeklyPattern:
    """Analysis of weekly patterns."""
    has_weekly_pattern: bool
    peak_day: int  # Day of week (0=Monday)
    trough_day: int
    weekday_avg: float
    weekend_avg: float
    daily_averages: Dict[int, float]  # Day -> average value


@dataclass 
class MemoryPattern:
    """Analysis of memory usage patterns."""
    avg_utilization: float
    max_utilization: float
    min_utilization: float
    has_memory_leak: bool
    leak_rate_percent_per_day: Optional[float]
    memory_pressure_events: int  # Times above 90%
    is_memory_stable: bool


@dataclass
class TrendPattern:
    """Analysis of usage trends."""
    direction: str  # "growing", "declining", "stable"
    slope_per_day: float
    r_squared: float  # How well trend fits
    predicted_30_day_value: float


@dataclass
class PatternAnalysis:
    """Complete pattern analysis for a workload."""
    instance_id: str
    analysis_period_days: int
    primary_pattern: PatternType
    idle: IdlePattern
    burst: BurstPattern
    diurnal: DiurnalPattern
    weekly: WeeklyPattern
    memory: MemoryPattern
    trend: TrendPattern
    confidence: float
    recommendations: List[str]


class PatternDetector:
    """
    Detects and analyzes usage patterns in CloudWatch metrics.
    
    Provides detailed pattern analysis to support workload classification
    and optimization recommendations.
    """
    
    # Thresholds
    IDLE_THRESHOLD_CPU = 5.0  # CPU below this = idle
    BURST_THRESHOLD_MULTIPLIER = 2.0  # Peak must be 2x baseline for burst
    MIN_BURST_DURATION_MINUTES = 5
    DIURNAL_SIGNIFICANCE = 1.5  # Peak/trough ratio for significance
    MEMORY_LEAK_THRESHOLD = 1.0  # % per day increase
    
    def __init__(self, config: Optional[Dict] = None):
        """Initialize with optional configuration overrides."""
        self.config = config or {}
        self._apply_config()
    
    def _apply_config(self):
        """Apply configuration overrides."""
        if "idle_threshold" in self.config:
            self.IDLE_THRESHOLD_CPU = self.config["idle_threshold"]
        if "burst_multiplier" in self.config:
            self.BURST_THRESHOLD_MULTIPLIER = self.config["burst_multiplier"]
    
    def analyze(
        self,
        instance_id: str,
        cpu_metrics: List[MetricPoint],
        memory_metrics: Optional[List[MetricPoint]] = None,
    ) -> PatternAnalysis:
        """
        Perform complete pattern analysis on workload metrics.
        
        Args:
            instance_id: EC2 instance identifier
            cpu_metrics: CPU utilization time series
            memory_metrics: Optional memory utilization time series
            
        Returns:
            Comprehensive pattern analysis
        """
        # Analyze each pattern type
        idle = self._analyze_idle_pattern(cpu_metrics)
        burst = self._analyze_burst_pattern(cpu_metrics)
        diurnal = self._analyze_diurnal_pattern(cpu_metrics)
        weekly = self._analyze_weekly_pattern(cpu_metrics)
        trend = self._analyze_trend(cpu_metrics)
        
        # Memory analysis if available
        if memory_metrics:
            memory = self._analyze_memory_pattern(memory_metrics)
        else:
            memory = MemoryPattern(
                avg_utilization=0,
                max_utilization=0,
                min_utilization=0,
                has_memory_leak=False,
                leak_rate_percent_per_day=None,
                memory_pressure_events=0,
                is_memory_stable=True,
            )
        
        # Determine primary pattern
        primary_pattern, confidence = self._determine_primary_pattern(
            idle, burst, diurnal, weekly, trend
        )
        
        # Generate recommendations
        recommendations = self._generate_recommendations(
            primary_pattern, idle, burst, diurnal, weekly, memory, trend
        )
        
        # Calculate analysis period
        if cpu_metrics:
            period = (cpu_metrics[-1].timestamp - cpu_metrics[0].timestamp).days
        else:
            period = 0
        
        return PatternAnalysis(
            instance_id=instance_id,
            analysis_period_days=period,
            primary_pattern=primary_pattern,
            idle=idle,
            burst=burst,
            diurnal=diurnal,
            weekly=weekly,
            memory=memory,
            trend=trend,
            confidence=confidence,
            recommendations=recommendations,
        )
    
    def _analyze_idle_pattern(self, metrics: List[MetricPoint]) -> IdlePattern:
        """Analyze idle periods in the metrics."""
        if not metrics:
            return IdlePattern(
                idle_percent=0,
                avg_idle_duration_hours=0,
                max_idle_duration_hours=0,
                idle_periods=[],
                is_idle_dominant=False,
            )
        
        # Find idle periods
        idle_periods = []
        in_idle = False
        idle_start = None
        
        for point in metrics:
            if point.value < self.IDLE_THRESHOLD_CPU:
                if not in_idle:
                    in_idle = True
                    idle_start = point.timestamp
            else:
                if in_idle:
                    idle_periods.append((idle_start, point.timestamp))
                    in_idle = False
        
        # Close final idle period if still idle
        if in_idle and idle_start:
            idle_periods.append((idle_start, metrics[-1].timestamp))
        
        # Calculate statistics
        idle_count = sum(1 for p in metrics if p.value < self.IDLE_THRESHOLD_CPU)
        idle_percent = (idle_count / len(metrics)) * 100
        
        if idle_periods:
            durations = [(end - start).total_seconds() / 3600 for start, end in idle_periods]
            avg_duration = statistics.mean(durations)
            max_duration = max(durations)
        else:
            avg_duration = 0
            max_duration = 0
        
        return IdlePattern(
            idle_percent=idle_percent,
            avg_idle_duration_hours=avg_duration,
            max_idle_duration_hours=max_duration,
            idle_periods=idle_periods,
            is_idle_dominant=idle_percent > 70,
        )
    
    def _analyze_burst_pattern(self, metrics: List[MetricPoint]) -> BurstPattern:
        """Analyze burst patterns (spikes and quiet periods)."""
        if len(metrics) < 10:
            return BurstPattern(
                is_bursty=False,
                burst_count=0,
                avg_burst_duration_minutes=0,
                avg_burst_intensity=0,
                avg_quiet_duration_hours=0,
                burst_periods=[],
            )
        
        # Calculate baseline (25th percentile)
        values = [p.value for p in metrics]
        baseline = self._percentile(values, 25)
        burst_threshold = max(baseline * self.BURST_THRESHOLD_MULTIPLIER, 20)
        
        # Find burst periods
        burst_periods = []
        in_burst = False
        burst_start = None
        burst_peak = 0
        
        for point in metrics:
            if point.value > burst_threshold:
                if not in_burst:
                    in_burst = True
                    burst_start = point.timestamp
                    burst_peak = point.value
                else:
                    burst_peak = max(burst_peak, point.value)
            else:
                if in_burst:
                    burst_periods.append((burst_start, point.timestamp, burst_peak))
                    in_burst = False
                    burst_peak = 0
        
        # Close final burst if still in burst
        if in_burst and burst_start:
            burst_periods.append((burst_start, metrics[-1].timestamp, burst_peak))
        
        # Filter out very short bursts (noise)
        min_duration = timedelta(minutes=self.MIN_BURST_DURATION_MINUTES)
        burst_periods = [
            (s, e, p) for s, e, p in burst_periods 
            if (e - s) >= min_duration
        ]
        
        # Calculate statistics
        if burst_periods:
            durations = [(e - s).total_seconds() / 60 for s, e, _ in burst_periods]
            avg_duration = statistics.mean(durations)
            avg_intensity = statistics.mean([p / max(baseline, 1) for _, _, p in burst_periods])
            
            # Calculate quiet periods between bursts
            quiet_durations = []
            for i in range(1, len(burst_periods)):
                quiet = (burst_periods[i][0] - burst_periods[i-1][1]).total_seconds() / 3600
                quiet_durations.append(quiet)
            avg_quiet = statistics.mean(quiet_durations) if quiet_durations else 0
        else:
            avg_duration = 0
            avg_intensity = 0
            avg_quiet = 0
        
        # Determine if truly bursty (significant bursts with quiet periods)
        is_bursty = (
            len(burst_periods) >= 3 and
            avg_intensity > 2.0 and
            avg_quiet > 1.0
        )
        
        return BurstPattern(
            is_bursty=is_bursty,
            burst_count=len(burst_periods),
            avg_burst_duration_minutes=avg_duration,
            avg_burst_intensity=avg_intensity,
            avg_quiet_duration_hours=avg_quiet,
            burst_periods=burst_periods,
        )
    
    def _analyze_diurnal_pattern(self, metrics: List[MetricPoint]) -> DiurnalPattern:
        """Analyze day/night patterns."""
        if len(metrics) < 48:  # Need at least 2 days
            return DiurnalPattern(
                has_diurnal_pattern=False,
                peak_hour=0,
                trough_hour=0,
                peak_to_trough_ratio=1.0,
                hourly_averages={},
            )
        
        # Group by hour of day
        hourly_values: Dict[int, List[float]] = {h: [] for h in range(24)}
        for point in metrics:
            hour = point.timestamp.hour
            hourly_values[hour].append(point.value)
        
        # Calculate hourly averages
        hourly_averages = {
            hour: statistics.mean(values) if values else 0
            for hour, values in hourly_values.items()
        }
        
        # Find peak and trough
        peak_hour = max(hourly_averages, key=hourly_averages.get)
        trough_hour = min(hourly_averages, key=hourly_averages.get)
        
        peak_value = hourly_averages[peak_hour]
        trough_value = hourly_averages[trough_hour]
        
        ratio = peak_value / max(trough_value, 0.1)
        
        return DiurnalPattern(
            has_diurnal_pattern=ratio >= self.DIURNAL_SIGNIFICANCE,
            peak_hour=peak_hour,
            trough_hour=trough_hour,
            peak_to_trough_ratio=ratio,
            hourly_averages=hourly_averages,
        )
    
    def _analyze_weekly_pattern(self, metrics: List[MetricPoint]) -> WeeklyPattern:
        """Analyze weekday/weekend patterns."""
        if len(metrics) < 168:  # Need at least 1 week
            return WeeklyPattern(
                has_weekly_pattern=False,
                peak_day=0,
                trough_day=0,
                weekday_avg=0,
                weekend_avg=0,
                daily_averages={},
            )
        
        # Group by day of week
        daily_values: Dict[int, List[float]] = {d: [] for d in range(7)}
        for point in metrics:
            day = point.timestamp.weekday()
            daily_values[day].append(point.value)
        
        # Calculate daily averages
        daily_averages = {
            day: statistics.mean(values) if values else 0
            for day, values in daily_values.items()
        }
        
        # Find peak and trough
        peak_day = max(daily_averages, key=daily_averages.get)
        trough_day = min(daily_averages, key=daily_averages.get)
        
        # Weekday vs weekend
        weekday_values = [daily_averages[d] for d in range(5)]
        weekend_values = [daily_averages[5], daily_averages[6]]
        
        weekday_avg = statistics.mean(weekday_values) if weekday_values else 0
        weekend_avg = statistics.mean(weekend_values) if weekend_values else 0
        
        # Significant difference between weekday and weekend
        has_pattern = abs(weekday_avg - weekend_avg) / max(weekday_avg, weekend_avg, 1) > 0.3
        
        return WeeklyPattern(
            has_weekly_pattern=has_pattern,
            peak_day=peak_day,
            trough_day=trough_day,
            weekday_avg=weekday_avg,
            weekend_avg=weekend_avg,
            daily_averages=daily_averages,
        )
    
    def _analyze_memory_pattern(self, metrics: List[MetricPoint]) -> MemoryPattern:
        """Analyze memory usage patterns."""
        if not metrics:
            return MemoryPattern(
                avg_utilization=0,
                max_utilization=0,
                min_utilization=0,
                has_memory_leak=False,
                leak_rate_percent_per_day=None,
                memory_pressure_events=0,
                is_memory_stable=True,
            )
        
        values = [p.value for p in metrics]
        
        avg_util = statistics.mean(values)
        max_util = max(values)
        min_util = min(values)
        
        # Count pressure events (>90% utilization)
        pressure_events = sum(1 for v in values if v > 90)
        
        # Check for memory leak (sustained increase over time)
        leak_rate = self._detect_memory_leak(metrics)
        has_leak = leak_rate is not None and leak_rate > self.MEMORY_LEAK_THRESHOLD
        
        # Stability check
        stddev = statistics.stdev(values) if len(values) > 1 else 0
        is_stable = stddev < 10  # Less than 10% standard deviation
        
        return MemoryPattern(
            avg_utilization=avg_util,
            max_utilization=max_util,
            min_utilization=min_util,
            has_memory_leak=has_leak,
            leak_rate_percent_per_day=leak_rate,
            memory_pressure_events=pressure_events,
            is_memory_stable=is_stable,
        )
    
    def _detect_memory_leak(self, metrics: List[MetricPoint]) -> Optional[float]:
        """Detect memory leak by looking for sustained increase."""
        if len(metrics) < 48:
            return None
        
        # Calculate daily averages
        daily_avgs = {}
        for point in metrics:
            day = point.timestamp.date()
            if day not in daily_avgs:
                daily_avgs[day] = []
            daily_avgs[day].append(point.value)
        
        daily_means = [statistics.mean(v) for v in daily_avgs.values()]
        
        if len(daily_means) < 3:
            return None
        
        # Simple linear regression to find slope
        n = len(daily_means)
        x_mean = (n - 1) / 2
        y_mean = statistics.mean(daily_means)
        
        numerator = sum((i - x_mean) * (daily_means[i] - y_mean) for i in range(n))
        denominator = sum((i - x_mean) ** 2 for i in range(n))
        
        if denominator == 0:
            return None
        
        slope = numerator / denominator  # % change per day
        
        # Only report if positive (leak) and significant
        if slope > 0.5:
            return slope
        return None
    
    def _analyze_trend(self, metrics: List[MetricPoint]) -> TrendPattern:
        """Analyze overall usage trend."""
        if len(metrics) < 48:
            return TrendPattern(
                direction="stable",
                slope_per_day=0,
                r_squared=0,
                predicted_30_day_value=0,
            )
        
        # Calculate daily averages for trend
        daily_avgs = {}
        for point in metrics:
            day = point.timestamp.date()
            if day not in daily_avgs:
                daily_avgs[day] = []
            daily_avgs[day].append(point.value)
        
        daily_means = [statistics.mean(v) for v in daily_avgs.values()]
        
        if len(daily_means) < 3:
            current = daily_means[-1] if daily_means else 0
            return TrendPattern(
                direction="stable",
                slope_per_day=0,
                r_squared=0,
                predicted_30_day_value=current,
            )
        
        # Linear regression
        n = len(daily_means)
        x = list(range(n))
        y = daily_means
        
        x_mean = statistics.mean(x)
        y_mean = statistics.mean(y)
        
        numerator = sum((x[i] - x_mean) * (y[i] - y_mean) for i in range(n))
        denominator = sum((x[i] - x_mean) ** 2 for i in range(n))
        
        if denominator == 0:
            slope = 0
        else:
            slope = numerator / denominator
        
        intercept = y_mean - slope * x_mean
        
        # R-squared
        y_pred = [slope * x[i] + intercept for i in range(n)]
        ss_res = sum((y[i] - y_pred[i]) ** 2 for i in range(n))
        ss_tot = sum((y[i] - y_mean) ** 2 for i in range(n))
        r_squared = 1 - (ss_res / ss_tot) if ss_tot > 0 else 0
        
        # Predict 30 days out
        predicted_30 = slope * (n + 30) + intercept
        predicted_30 = max(0, min(100, predicted_30))  # Clamp to valid range
        
        # Determine direction
        if abs(slope) < 0.5:
            direction = "stable"
        elif slope > 0:
            direction = "growing"
        else:
            direction = "declining"
        
        return TrendPattern(
            direction=direction,
            slope_per_day=slope,
            r_squared=max(0, r_squared),
            predicted_30_day_value=predicted_30,
        )
    
    def _percentile(self, values: List[float], p: int) -> float:
        """Calculate percentile."""
        if not values:
            return 0
        sorted_values = sorted(values)
        index = (len(sorted_values) - 1) * p / 100
        lower = int(index)
        upper = lower + 1
        if upper >= len(sorted_values):
            return sorted_values[-1]
        weight = index - lower
        return sorted_values[lower] * (1 - weight) + sorted_values[upper] * weight
    
    def _determine_primary_pattern(
        self,
        idle: IdlePattern,
        burst: BurstPattern,
        diurnal: DiurnalPattern,
        weekly: WeeklyPattern,
        trend: TrendPattern,
    ) -> Tuple[PatternType, float]:
        """Determine the primary usage pattern and confidence."""
        
        scores = {
            PatternType.IDLE_DOMINANT: 0.0,
            PatternType.BURSTY: 0.0,
            PatternType.STEADY_STATE: 0.0,
            PatternType.DIURNAL: 0.0,
            PatternType.WEEKLY: 0.0,
            PatternType.BATCH: 0.0,
            PatternType.GROWING: 0.0,
            PatternType.DECLINING: 0.0,
        }
        
        # Idle dominant
        if idle.is_idle_dominant:
            scores[PatternType.IDLE_DOMINANT] = 0.8 + (idle.idle_percent - 70) / 100
        
        # Bursty
        if burst.is_bursty:
            scores[PatternType.BURSTY] = min(
                0.5 + burst.avg_burst_intensity / 10 + burst.burst_count / 50,
                0.95
            )
        
        # Diurnal
        if diurnal.has_diurnal_pattern:
            scores[PatternType.DIURNAL] = min(
                0.5 + (diurnal.peak_to_trough_ratio - 1.5) / 3,
                0.9
            )
        
        # Weekly
        if weekly.has_weekly_pattern:
            diff_ratio = abs(weekly.weekday_avg - weekly.weekend_avg) / max(weekly.weekday_avg, 1)
            scores[PatternType.WEEKLY] = min(0.5 + diff_ratio, 0.85)
        
        # Trend patterns
        if trend.direction == "growing" and trend.r_squared > 0.5:
            scores[PatternType.GROWING] = 0.5 + trend.r_squared * 0.4
        elif trend.direction == "declining" and trend.r_squared > 0.5:
            scores[PatternType.DECLINING] = 0.5 + trend.r_squared * 0.4
        
        # Batch pattern (bursty + periodic)
        if burst.is_bursty and (diurnal.has_diurnal_pattern or weekly.has_weekly_pattern):
            scores[PatternType.BATCH] = min(
                scores[PatternType.BURSTY] + 0.2,
                0.95
            )
        
        # Steady state (low variance, not idle)
        if not idle.is_idle_dominant and not burst.is_bursty:
            if diurnal.peak_to_trough_ratio < 1.5:
                scores[PatternType.STEADY_STATE] = 0.7
        
        # Find highest score
        best_pattern = max(scores, key=scores.get)
        confidence = scores[best_pattern]
        
        # If no clear pattern, mark as erratic
        if confidence < 0.4:
            return PatternType.ERRATIC, 0.5
        
        return best_pattern, confidence
    
    def _generate_recommendations(
        self,
        primary: PatternType,
        idle: IdlePattern,
        burst: BurstPattern,
        diurnal: DiurnalPattern,
        weekly: WeeklyPattern,
        memory: MemoryPattern,
        trend: TrendPattern,
    ) -> List[str]:
        """Generate optimization recommendations based on patterns."""
        recommendations = []
        
        # Pattern-specific recommendations
        if primary == PatternType.IDLE_DOMINANT:
            recommendations.append(
                f"Consider Lambda/Fargate: {idle.idle_percent:.0f}% idle time - "
                "pay-per-use pricing could reduce costs significantly"
            )
            if idle.avg_idle_duration_hours > 4:
                recommendations.append(
                    "Consider scheduled stop/start during predictable idle periods"
                )
        
        elif primary == PatternType.BURSTY:
            if burst.avg_burst_duration_minutes < 15:
                recommendations.append(
                    "Short bursts suitable for Lambda (< 15 min execution)"
                )
            else:
                recommendations.append(
                    "Burst pattern suitable for Fargate with auto-scaling"
                )
        
        elif primary == PatternType.BATCH:
            recommendations.append(
                "Batch processing pattern detected - consider Spot Instances "
                "for up to 90% cost savings"
            )
            if diurnal.has_diurnal_pattern:
                recommendations.append(
                    f"Schedule batch jobs during off-peak hours (trough at {diurnal.trough_hour}:00)"
                )
        
        elif primary == PatternType.DIURNAL:
            recommendations.append(
                f"Scale down during off-peak hours ({diurnal.trough_hour}:00) - "
                "consider scheduled scaling policies"
            )
        
        elif primary == PatternType.WEEKLY:
            if weekly.weekend_avg < weekly.weekday_avg * 0.5:
                recommendations.append(
                    "Weekend usage significantly lower - consider weekend shutdown/scale-down"
                )
        
        elif primary == PatternType.GROWING:
            recommendations.append(
                f"Usage trending up ({trend.slope_per_day:.1f}%/day) - "
                "plan for capacity increase or Reserved Instance purchase"
            )
        
        elif primary == PatternType.DECLINING:
            recommendations.append(
                f"Usage trending down ({abs(trend.slope_per_day):.1f}%/day) - "
                "consider right-sizing or decommissioning"
            )
        
        elif primary == PatternType.STEADY_STATE:
            recommendations.append(
                "Steady usage pattern - ideal candidate for Reserved Instances "
                "or Savings Plans"
            )
        
        # Memory-specific recommendations
        if memory.has_memory_leak:
            recommendations.append(
                f"⚠️ Potential memory leak detected ({memory.leak_rate_percent_per_day:.1f}%/day) - "
                "investigate application memory management"
            )
        
        if memory.memory_pressure_events > 10:
            recommendations.append(
                f"Memory pressure detected ({memory.memory_pressure_events} events >90%) - "
                "consider instance with more memory"
            )
        
        return recommendations


# Export for use in main.py
__all__ = [
    "PatternType",
    "PatternAnalysis",
    "IdlePattern",
    "BurstPattern",
    "DiurnalPattern",
    "WeeklyPattern",
    "MemoryPattern",
    "TrendPattern",
    "PatternDetector",
    "MetricPoint",
]


if __name__ == "__main__":
    import random
    
    # Generate test data with bursty pattern
    def generate_bursty_data(hours: int = 336) -> List[MetricPoint]:
        base = datetime.now() - timedelta(hours=hours)
        data = []
        for h in range(hours):
            ts = base + timedelta(hours=h)
            # Mostly low with occasional spikes
            if random.random() < 0.1:
                value = random.uniform(70, 95)
            else:
                value = random.uniform(2, 8)
            data.append(MetricPoint(timestamp=ts, value=value))
        return data
    
    # Generate test data with diurnal pattern
    def generate_diurnal_data(hours: int = 336) -> List[MetricPoint]:
        base = datetime.now() - timedelta(hours=hours)
        data = []
        for h in range(hours):
            ts = base + timedelta(hours=h)
            hour = ts.hour
            # Peak during business hours (9-17)
            if 9 <= hour <= 17:
                value = random.gauss(60, 10)
            else:
                value = random.gauss(15, 5)
            value = max(0, min(100, value))
            data.append(MetricPoint(timestamp=ts, value=value))
        return data
    
    # Test pattern detector
    detector = PatternDetector()
    
    print("=" * 60)
    print("Testing Pattern Detector")
    print("=" * 60)
    
    # Test bursty pattern
    print("\n--- Bursty Pattern Test ---")
    bursty_data = generate_bursty_data()
    result = detector.analyze("i-bursty123", bursty_data)
    print(f"Primary Pattern: {result.primary_pattern.value}")
    print(f"Confidence: {result.confidence:.2f}")
    print(f"Idle: {result.idle.idle_percent:.1f}% idle time")
    print(f"Bursts: {result.burst.burst_count} bursts detected")
    print("Recommendations:")
    for rec in result.recommendations:
        print(f"  • {rec}")
    
    # Test diurnal pattern
    print("\n--- Diurnal Pattern Test ---")
    diurnal_data = generate_diurnal_data()
    result = detector.analyze("i-diurnal456", diurnal_data)
    print(f"Primary Pattern: {result.primary_pattern.value}")
    print(f"Confidence: {result.confidence:.2f}")
    print(f"Peak Hour: {result.diurnal.peak_hour}:00")
    print(f"Trough Hour: {result.diurnal.trough_hour}:00")
    print(f"Peak/Trough Ratio: {result.diurnal.peak_to_trough_ratio:.2f}")
    print("Recommendations:")
    for rec in result.recommendations:
        print(f"  • {rec}")
