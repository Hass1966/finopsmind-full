"""
Workload Classifier for EC2 Migration Recommendations

Classifies EC2 workloads into migration candidates based on CloudWatch metrics.
Uses heuristic/rule-based approach initially, designed for ML enhancement later.

Classifications:
- lambda_candidate: Bursty, short-lived, stateless workloads
- fargate_candidate: Containerizable, predictable workloads
- spot_candidate: Fault tolerant, flexible timing workloads
- keep_ec2: None of the above - keep on EC2
"""

from dataclasses import dataclass, field
from datetime import datetime, timedelta
from typing import List, Dict, Optional, Tuple
from enum import Enum
import statistics
import math


class WorkloadClassification(Enum):
    LAMBDA_CANDIDATE = "lambda_candidate"
    FARGATE_CANDIDATE = "fargate_candidate"
    SPOT_CANDIDATE = "spot_candidate"
    KEEP_EC2 = "keep_ec2"


@dataclass
class MetricDataPoint:
    """Single metric data point from CloudWatch."""
    timestamp: datetime
    value: float


@dataclass
class CloudWatchMetrics:
    """14-day CloudWatch metrics for an EC2 instance."""
    instance_id: str
    instance_type: str
    cpu_utilization: List[MetricDataPoint] = field(default_factory=list)
    memory_utilization: List[MetricDataPoint] = field(default_factory=list)
    network_in: List[MetricDataPoint] = field(default_factory=list)
    network_out: List[MetricDataPoint] = field(default_factory=list)
    disk_read_ops: List[MetricDataPoint] = field(default_factory=list)
    disk_write_ops: List[MetricDataPoint] = field(default_factory=list)
    disk_read_bytes: List[MetricDataPoint] = field(default_factory=list)
    disk_write_bytes: List[MetricDataPoint] = field(default_factory=list)
    
    # Optional metadata
    has_elastic_ip: bool = False
    has_persistent_storage: bool = False
    in_auto_scaling_group: bool = False
    instance_age_days: int = 0


@dataclass
class ClassificationResult:
    """Result of workload classification."""
    instance_id: str
    classification: WorkloadClassification
    confidence: float  # 0.0 to 1.0
    reasons: List[str]
    metrics_summary: Dict[str, float]
    alternative_classifications: List[Tuple[WorkloadClassification, float]]
    estimated_savings_percent: Optional[float] = None
    recommended_target: Optional[str] = None  # e.g., "Lambda", "Fargate 0.5vCPU/1GB"
    warnings: List[str] = field(default_factory=list)


class WorkloadClassifier:
    """
    Classifies EC2 workloads for potential migration using heuristic rules.
    
    Thresholds are configurable and can be tuned based on real-world results.
    Designed to be replaced/enhanced with ML model later.
    """
    
    # Lambda candidate thresholds
    LAMBDA_MAX_AVG_CPU = 20.0  # Average CPU must be low
    LAMBDA_MAX_MEMORY_MB = 10240  # Lambda max is 10GB
    LAMBDA_MIN_IDLE_PERCENT = 70.0  # High idle time suggests event-driven
    LAMBDA_MAX_BURST_DURATION_MINUTES = 15  # Bursts should be short
    LAMBDA_MIN_BURST_RATIO = 3.0  # Peak/average ratio for burstiness
    
    # Fargate candidate thresholds
    FARGATE_MIN_AVG_CPU = 5.0  # Some consistent usage
    FARGATE_MAX_AVG_CPU = 80.0  # Not maxed out
    FARGATE_MAX_CPU_STDDEV = 25.0  # Relatively predictable
    FARGATE_MIN_UPTIME_PERCENT = 80.0  # Runs most of the time
    
    # Spot candidate thresholds
    SPOT_MAX_REQUIRED_UPTIME = 95.0  # Can tolerate interruptions
    SPOT_MIN_BATCH_PATTERN_SCORE = 0.6  # Shows batch processing patterns
    SPOT_FLEXIBLE_TIMING_SCORE = 0.5  # Timing flexibility indicator
    
    def __init__(self, config: Optional[Dict] = None):
        """Initialize classifier with optional config overrides."""
        self.config = config or {}
        self._apply_config_overrides()
    
    def _apply_config_overrides(self):
        """Apply any configuration overrides to thresholds."""
        if "lambda_max_avg_cpu" in self.config:
            self.LAMBDA_MAX_AVG_CPU = self.config["lambda_max_avg_cpu"]
        if "fargate_max_cpu_stddev" in self.config:
            self.FARGATE_MAX_CPU_STDDEV = self.config["fargate_max_cpu_stddev"]
        # Add more overrides as needed
    
    def classify(self, metrics: CloudWatchMetrics) -> ClassificationResult:
        """
        Classify an EC2 workload based on its CloudWatch metrics.
        
        Args:
            metrics: 14-day CloudWatch metrics for the instance
            
        Returns:
            ClassificationResult with classification, confidence, and reasoning
        """
        # Extract metric statistics
        stats = self._compute_statistics(metrics)
        
        # Score each classification
        lambda_score, lambda_reasons = self._score_lambda_candidate(metrics, stats)
        fargate_score, fargate_reasons = self._score_fargate_candidate(metrics, stats)
        spot_score, spot_reasons = self._score_spot_candidate(metrics, stats)
        keep_ec2_score, keep_ec2_reasons = self._score_keep_ec2(metrics, stats)
        
        # Determine best classification
        scores = [
            (WorkloadClassification.LAMBDA_CANDIDATE, lambda_score, lambda_reasons),
            (WorkloadClassification.FARGATE_CANDIDATE, fargate_score, fargate_reasons),
            (WorkloadClassification.SPOT_CANDIDATE, spot_score, spot_reasons),
            (WorkloadClassification.KEEP_EC2, keep_ec2_score, keep_ec2_reasons),
        ]
        
        # Sort by score descending
        scores.sort(key=lambda x: x[1], reverse=True)
        
        best_classification, best_score, best_reasons = scores[0]
        
        # Build alternative classifications
        alternatives = [(s[0], s[1]) for s in scores[1:] if s[1] > 0.3]
        
        # Generate warnings
        warnings = self._generate_warnings(metrics, stats, best_classification)
        
        # Calculate estimated savings and recommended target
        savings, target = self._estimate_savings(metrics, stats, best_classification)
        
        return ClassificationResult(
            instance_id=metrics.instance_id,
            classification=best_classification,
            confidence=min(best_score, 1.0),
            reasons=best_reasons,
            metrics_summary=stats,
            alternative_classifications=alternatives,
            estimated_savings_percent=savings,
            recommended_target=target,
            warnings=warnings,
        )
    
    def _compute_statistics(self, metrics: CloudWatchMetrics) -> Dict[str, float]:
        """Compute summary statistics from metrics."""
        stats = {}
        
        # CPU statistics
        cpu_values = [p.value for p in metrics.cpu_utilization]
        if cpu_values:
            stats["cpu_avg"] = statistics.mean(cpu_values)
            stats["cpu_max"] = max(cpu_values)
            stats["cpu_min"] = min(cpu_values)
            stats["cpu_stddev"] = statistics.stdev(cpu_values) if len(cpu_values) > 1 else 0
            stats["cpu_p95"] = self._percentile(cpu_values, 95)
            stats["cpu_p50"] = self._percentile(cpu_values, 50)
            stats["cpu_idle_percent"] = len([v for v in cpu_values if v < 5]) / len(cpu_values) * 100
        
        # Memory statistics
        mem_values = [p.value for p in metrics.memory_utilization]
        if mem_values:
            stats["memory_avg"] = statistics.mean(mem_values)
            stats["memory_max"] = max(mem_values)
            stats["memory_stddev"] = statistics.stdev(mem_values) if len(mem_values) > 1 else 0
        
        # Network statistics
        net_in = [p.value for p in metrics.network_in]
        net_out = [p.value for p in metrics.network_out]
        if net_in:
            stats["network_in_avg"] = statistics.mean(net_in)
            stats["network_in_max"] = max(net_in)
        if net_out:
            stats["network_out_avg"] = statistics.mean(net_out)
            stats["network_out_max"] = max(net_out)
        
        # Disk I/O statistics
        disk_read = [p.value for p in metrics.disk_read_ops]
        disk_write = [p.value for p in metrics.disk_write_ops]
        if disk_read:
            stats["disk_read_ops_avg"] = statistics.mean(disk_read)
        if disk_write:
            stats["disk_write_ops_avg"] = statistics.mean(disk_write)
            stats["disk_write_ops_max"] = max(disk_write)
        
        # Derived metrics
        stats["burst_ratio"] = stats.get("cpu_max", 0) / max(stats.get("cpu_avg", 1), 0.1)
        stats["variability_score"] = stats.get("cpu_stddev", 0) / max(stats.get("cpu_avg", 1), 0.1)
        
        return stats
    
    def _percentile(self, values: List[float], percentile: int) -> float:
        """Calculate percentile of a list of values."""
        if not values:
            return 0.0
        sorted_values = sorted(values)
        index = (len(sorted_values) - 1) * percentile / 100
        lower = int(index)
        upper = lower + 1
        if upper >= len(sorted_values):
            return sorted_values[-1]
        weight = index - lower
        return sorted_values[lower] * (1 - weight) + sorted_values[upper] * weight
    
    def _score_lambda_candidate(
        self, metrics: CloudWatchMetrics, stats: Dict[str, float]
    ) -> Tuple[float, List[str]]:
        """
        Score workload as Lambda candidate.
        
        Lambda is ideal for:
        - Bursty, event-driven workloads
        - Short execution times (<15 min)
        - Stateless processing
        - High idle time with occasional spikes
        """
        score = 0.0
        reasons = []
        
        # Check average CPU (should be low)
        cpu_avg = stats.get("cpu_avg", 100)
        if cpu_avg <= self.LAMBDA_MAX_AVG_CPU:
            score += 0.25
            reasons.append(f"Low average CPU ({cpu_avg:.1f}%) indicates event-driven pattern")
        
        # Check idle percentage (should be high)
        idle_pct = stats.get("cpu_idle_percent", 0)
        if idle_pct >= self.LAMBDA_MIN_IDLE_PERCENT:
            score += 0.25
            reasons.append(f"High idle time ({idle_pct:.1f}%) suggests sporadic usage")
        
        # Check burst ratio (should be high - indicating spiky workload)
        burst_ratio = stats.get("burst_ratio", 0)
        if burst_ratio >= self.LAMBDA_MIN_BURST_RATIO:
            score += 0.2
            reasons.append(f"High burst ratio ({burst_ratio:.1f}x) indicates bursty workload")
        
        # Check for stateless indicators (low disk writes)
        disk_write_avg = stats.get("disk_write_ops_avg", 0)
        if disk_write_avg < 100:  # Low disk writes suggest stateless
            score += 0.15
            reasons.append("Low disk write activity suggests stateless processing")
        
        # Penalize if persistent storage attached
        if metrics.has_persistent_storage:
            score -= 0.2
            reasons.append("Persistent storage attached - may require state management")
        
        # Penalize if in ASG (usually indicates steady-state service)
        if metrics.in_auto_scaling_group:
            score -= 0.1
            reasons.append("Part of Auto Scaling Group - may be steady-state service")
        
        # Memory check for Lambda limits
        memory_avg = stats.get("memory_avg", 0)
        if memory_avg > 80:  # Using >80% of instance memory
            score -= 0.15
            reasons.append("High memory usage may exceed Lambda limits")
        
        return max(score, 0), reasons
    
    def _score_fargate_candidate(
        self, metrics: CloudWatchMetrics, stats: Dict[str, float]
    ) -> Tuple[float, List[str]]:
        """
        Score workload as Fargate candidate.
        
        Fargate is ideal for:
        - Containerizable workloads
        - Predictable, steady-state usage
        - Consistent resource requirements
        - Long-running services
        """
        score = 0.0
        reasons = []
        
        # Check for consistent usage (moderate CPU, low variance)
        cpu_avg = stats.get("cpu_avg", 0)
        cpu_stddev = stats.get("cpu_stddev", 100)
        
        if self.FARGATE_MIN_AVG_CPU <= cpu_avg <= self.FARGATE_MAX_AVG_CPU:
            score += 0.2
            reasons.append(f"Consistent CPU usage ({cpu_avg:.1f}%) suitable for containers")
        
        if cpu_stddev <= self.FARGATE_MAX_CPU_STDDEV:
            score += 0.2
            reasons.append(f"Low CPU variance (σ={cpu_stddev:.1f}) indicates predictable workload")
        
        # Check uptime pattern (should be high for Fargate)
        idle_pct = stats.get("cpu_idle_percent", 100)
        uptime_pct = 100 - idle_pct
        if uptime_pct >= self.FARGATE_MIN_UPTIME_PERCENT:
            score += 0.2
            reasons.append(f"High uptime ({uptime_pct:.1f}%) suitable for always-on container")
        
        # Check memory stability
        memory_stddev = stats.get("memory_stddev", 100)
        if memory_stddev < 15:
            score += 0.15
            reasons.append("Stable memory usage suitable for fixed container allocation")
        
        # Network activity suggests service workload
        net_in_avg = stats.get("network_in_avg", 0)
        net_out_avg = stats.get("network_out_avg", 0)
        if net_in_avg > 1000 or net_out_avg > 1000:  # Some network activity
            score += 0.1
            reasons.append("Network activity indicates service-type workload")
        
        # Bonus for ASG membership (containerization natural fit)
        if metrics.in_auto_scaling_group:
            score += 0.1
            reasons.append("ASG membership suggests container-friendly architecture")
        
        # Penalize high disk I/O (containers should be stateless)
        disk_write_max = stats.get("disk_write_ops_max", 0)
        if disk_write_max > 1000:
            score -= 0.15
            reasons.append("High disk I/O may require persistent volume strategy")
        
        return max(score, 0), reasons
    
    def _score_spot_candidate(
        self, metrics: CloudWatchMetrics, stats: Dict[str, float]
    ) -> Tuple[float, List[str]]:
        """
        Score workload as Spot Instance candidate.
        
        Spot is ideal for:
        - Fault-tolerant workloads
        - Batch processing
        - Flexible timing requirements
        - Workloads that can checkpoint
        """
        score = 0.0
        reasons = []
        
        # Check for batch processing patterns (periodic high usage)
        variability = stats.get("variability_score", 0)
        if variability > 1.0:
            score += 0.2
            reasons.append("High variability suggests batch processing pattern")
        
        # Check if workload shows periodic patterns
        cpu_values = [p.value for p in metrics.cpu_utilization]
        if self._detect_periodic_pattern(cpu_values):
            score += 0.2
            reasons.append("Periodic usage pattern detected - suitable for scheduled Spot")
        
        # High burst ratio but longer duration than Lambda
        burst_ratio = stats.get("burst_ratio", 0)
        cpu_avg = stats.get("cpu_avg", 0)
        if burst_ratio > 2.0 and cpu_avg > self.LAMBDA_MAX_AVG_CPU:
            score += 0.15
            reasons.append("Bursty but sustained workload suitable for Spot")
        
        # Check for compute-heavy workload (good Spot candidate)
        cpu_p95 = stats.get("cpu_p95", 0)
        if cpu_p95 > 70:
            score += 0.15
            reasons.append(f"Compute-intensive (P95 CPU: {cpu_p95:.1f}%) benefits from Spot pricing")
        
        # Low network I/O suggests batch (not real-time service)
        net_out_avg = stats.get("network_out_avg", 0)
        if net_out_avg < 10000:  # Low outbound traffic
            score += 0.1
            reasons.append("Low network egress suggests non-interactive workload")
        
        # Instance age - older instances often good Spot candidates
        if metrics.instance_age_days > 30:
            score += 0.1
            reasons.append("Long-running instance may benefit from Spot migration")
        
        # Penalize if Elastic IP (suggests need for stable endpoint)
        if metrics.has_elastic_ip:
            score -= 0.2
            reasons.append("Elastic IP suggests need for stable endpoint - Spot interruptions risky")
        
        return max(score, 0), reasons
    
    def _score_keep_ec2(
        self, metrics: CloudWatchMetrics, stats: Dict[str, float]
    ) -> Tuple[float, List[str]]:
        """
        Score workload for keeping on EC2.
        
        Keep on EC2 when:
        - High, consistent utilization
        - Needs specific instance features
        - Performance-critical workloads
        - Complex networking requirements
        """
        score = 0.3  # Base score - EC2 is always viable
        reasons = []
        
        # High consistent CPU usage
        cpu_avg = stats.get("cpu_avg", 0)
        cpu_stddev = stats.get("cpu_stddev", 0)
        
        if cpu_avg > 70 and cpu_stddev < 15:
            score += 0.25
            reasons.append(f"High, stable CPU usage ({cpu_avg:.1f}%) - well-suited for EC2")
        
        # High memory utilization
        memory_avg = stats.get("memory_avg", 0)
        if memory_avg > 70:
            score += 0.15
            reasons.append(f"High memory usage ({memory_avg:.1f}%) - may need dedicated instance")
        
        # Heavy disk I/O
        disk_read_avg = stats.get("disk_read_ops_avg", 0)
        disk_write_avg = stats.get("disk_write_ops_avg", 0)
        if disk_read_avg > 500 or disk_write_avg > 500:
            score += 0.15
            reasons.append("Significant disk I/O - EC2 with EBS optimized recommended")
        
        # Elastic IP suggests stable endpoint requirement
        if metrics.has_elastic_ip:
            score += 0.1
            reasons.append("Elastic IP indicates need for stable endpoint")
        
        # Persistent storage
        if metrics.has_persistent_storage:
            score += 0.1
            reasons.append("Persistent storage attached - may require EC2 for data locality")
        
        return min(score, 1.0), reasons
    
    def _detect_periodic_pattern(self, values: List[float], min_periods: int = 2) -> bool:
        """
        Simple periodic pattern detection using autocorrelation.
        
        Returns True if periodic pattern detected.
        """
        if len(values) < 48:  # Need at least 2 days of hourly data
            return False
        
        # Simple check: look for similar patterns at common intervals
        # (hourly data: 24 = daily, 168 = weekly)
        for period in [24, 168]:
            if len(values) < period * min_periods:
                continue
            
            # Compare first period with subsequent periods
            correlations = []
            for i in range(1, min_periods):
                start = i * period
                end = start + period
                if end > len(values):
                    break
                
                # Simple correlation check
                first_period = values[:period]
                this_period = values[start:end]
                
                if len(first_period) == len(this_period):
                    corr = self._simple_correlation(first_period, this_period)
                    correlations.append(corr)
            
            if correlations and statistics.mean(correlations) > 0.7:
                return True
        
        return False
    
    def _simple_correlation(self, x: List[float], y: List[float]) -> float:
        """Calculate simple Pearson correlation coefficient."""
        if len(x) != len(y) or len(x) == 0:
            return 0.0
        
        n = len(x)
        mean_x = sum(x) / n
        mean_y = sum(y) / n
        
        numerator = sum((x[i] - mean_x) * (y[i] - mean_y) for i in range(n))
        denom_x = math.sqrt(sum((xi - mean_x) ** 2 for xi in x))
        denom_y = math.sqrt(sum((yi - mean_y) ** 2 for yi in y))
        
        if denom_x == 0 or denom_y == 0:
            return 0.0
        
        return numerator / (denom_x * denom_y)
    
    def _generate_warnings(
        self,
        metrics: CloudWatchMetrics,
        stats: Dict[str, float],
        classification: WorkloadClassification,
    ) -> List[str]:
        """Generate warnings about potential issues with the classification."""
        warnings = []
        
        # Insufficient data warning
        cpu_points = len(metrics.cpu_utilization)
        if cpu_points < 336:  # Less than 14 days of hourly data
            warnings.append(
                f"Limited data points ({cpu_points}) - recommend 14 days for accurate classification"
            )
        
        # High variance warning for Fargate
        if classification == WorkloadClassification.FARGATE_CANDIDATE:
            cpu_stddev = stats.get("cpu_stddev", 0)
            if cpu_stddev > 20:
                warnings.append(
                    "Moderate CPU variance may require careful Fargate sizing"
                )
        
        # Memory concerns for Lambda
        if classification == WorkloadClassification.LAMBDA_CANDIDATE:
            memory_avg = stats.get("memory_avg", 0)
            if memory_avg > 60:
                warnings.append(
                    "Memory usage may approach Lambda limits - monitor actual memory requirements"
                )
        
        # Spot interruption risk
        if classification == WorkloadClassification.SPOT_CANDIDATE:
            if metrics.has_elastic_ip:
                warnings.append(
                    "Elastic IP detected - consider interruption handling strategy"
                )
        
        return warnings
    
    def _estimate_savings(
        self,
        metrics: CloudWatchMetrics,
        stats: Dict[str, float],
        classification: WorkloadClassification,
    ) -> Tuple[Optional[float], Optional[str]]:
        """Estimate potential savings and recommended target configuration."""
        
        if classification == WorkloadClassification.LAMBDA_CANDIDATE:
            # Lambda pricing is complex - rough estimate based on idle time
            idle_pct = stats.get("cpu_idle_percent", 0)
            # Higher idle = more savings (pay only when running)
            savings = min(idle_pct * 0.8, 80)  # Up to 80% savings
            
            memory_avg = stats.get("memory_avg", 50)
            if memory_avg < 30:
                target = "Lambda 512MB"
            elif memory_avg < 60:
                target = "Lambda 1024MB"
            else:
                target = "Lambda 2048MB"
            
            return savings, target
        
        elif classification == WorkloadClassification.FARGATE_CANDIDATE:
            # Fargate savings depend on right-sizing
            cpu_avg = stats.get("cpu_avg", 50)
            if cpu_avg < 25:
                savings = 40  # Significant oversized
                target = "Fargate 0.25vCPU / 0.5GB"
            elif cpu_avg < 50:
                savings = 25
                target = "Fargate 0.5vCPU / 1GB"
            else:
                savings = 15
                target = "Fargate 1vCPU / 2GB"
            
            return savings, target
        
        elif classification == WorkloadClassification.SPOT_CANDIDATE:
            # Spot typically 60-90% cheaper
            savings = 70  # Conservative estimate
            target = f"Spot {metrics.instance_type}"
            return savings, target
        
        else:
            # Keep on EC2 - check for right-sizing
            cpu_avg = stats.get("cpu_avg", 50)
            if cpu_avg < 20:
                return 30, f"Downsize {metrics.instance_type}"
            elif cpu_avg > 80:
                return None, f"Consider upsizing {metrics.instance_type}"
            else:
                return None, None


# Batch classification helper
def classify_workloads(
    workloads: List[CloudWatchMetrics],
    config: Optional[Dict] = None
) -> List[ClassificationResult]:
    """
    Classify multiple workloads.
    
    Args:
        workloads: List of CloudWatch metrics for each instance
        config: Optional configuration overrides
        
    Returns:
        List of classification results
    """
    classifier = WorkloadClassifier(config)
    return [classifier.classify(w) for w in workloads]


# Example usage and testing
if __name__ == "__main__":
    from datetime import datetime, timedelta
    import random
    
    # Generate sample bursty workload (Lambda candidate)
    def generate_bursty_metrics() -> CloudWatchMetrics:
        base_time = datetime.now() - timedelta(days=14)
        cpu_data = []
        
        for hour in range(336):  # 14 days of hourly data
            timestamp = base_time + timedelta(hours=hour)
            # Mostly idle with occasional spikes
            if random.random() < 0.1:  # 10% chance of spike
                value = random.uniform(60, 95)
            else:
                value = random.uniform(1, 5)
            cpu_data.append(MetricDataPoint(timestamp=timestamp, value=value))
        
        return CloudWatchMetrics(
            instance_id="i-bursty123",
            instance_type="t3.medium",
            cpu_utilization=cpu_data,
            memory_utilization=[MetricDataPoint(timestamp=d.timestamp, value=random.uniform(20, 40)) for d in cpu_data],
            network_in=[MetricDataPoint(timestamp=d.timestamp, value=random.uniform(100, 1000)) for d in cpu_data],
            network_out=[MetricDataPoint(timestamp=d.timestamp, value=random.uniform(100, 500)) for d in cpu_data],
            disk_read_ops=[MetricDataPoint(timestamp=d.timestamp, value=random.uniform(0, 50)) for d in cpu_data],
            disk_write_ops=[MetricDataPoint(timestamp=d.timestamp, value=random.uniform(0, 20)) for d in cpu_data],
        )
    
    # Generate sample steady-state workload (Fargate candidate)
    def generate_steady_metrics() -> CloudWatchMetrics:
        base_time = datetime.now() - timedelta(days=14)
        cpu_data = []
        
        for hour in range(336):
            timestamp = base_time + timedelta(hours=hour)
            # Steady usage with small variance
            value = random.gauss(45, 8)
            value = max(10, min(80, value))
            cpu_data.append(MetricDataPoint(timestamp=timestamp, value=value))
        
        return CloudWatchMetrics(
            instance_id="i-steady456",
            instance_type="m5.large",
            cpu_utilization=cpu_data,
            memory_utilization=[MetricDataPoint(timestamp=d.timestamp, value=random.gauss(55, 5)) for d in cpu_data],
            network_in=[MetricDataPoint(timestamp=d.timestamp, value=random.uniform(5000, 15000)) for d in cpu_data],
            network_out=[MetricDataPoint(timestamp=d.timestamp, value=random.uniform(10000, 30000)) for d in cpu_data],
            disk_read_ops=[MetricDataPoint(timestamp=d.timestamp, value=random.uniform(10, 100)) for d in cpu_data],
            disk_write_ops=[MetricDataPoint(timestamp=d.timestamp, value=random.uniform(10, 80)) for d in cpu_data],
            in_auto_scaling_group=True,
        )
    
    # Test classifications
    classifier = WorkloadClassifier()
    
    print("=" * 60)
    print("Testing Workload Classifier")
    print("=" * 60)
    
    # Test bursty workload
    bursty = generate_bursty_metrics()
    result = classifier.classify(bursty)
    print(f"\nInstance: {result.instance_id}")
    print(f"Classification: {result.classification.value}")
    print(f"Confidence: {result.confidence:.2f}")
    print(f"Estimated Savings: {result.estimated_savings_percent}%")
    print(f"Recommended Target: {result.recommended_target}")
    print("Reasons:")
    for reason in result.reasons:
        print(f"  - {reason}")
    if result.warnings:
        print("Warnings:")
        for warning in result.warnings:
            print(f"  ⚠ {warning}")
    
    # Test steady workload
    steady = generate_steady_metrics()
    result = classifier.classify(steady)
    print(f"\nInstance: {result.instance_id}")
    print(f"Classification: {result.classification.value}")
    print(f"Confidence: {result.confidence:.2f}")
    print(f"Estimated Savings: {result.estimated_savings_percent}%")
    print(f"Recommended Target: {result.recommended_target}")
    print("Reasons:")
    for reason in result.reasons:
        print(f"  - {reason}")
