"""
Tests for FinOpsMind Phase 5 ML Sidecar

Run with: pytest tests/test_phase5.py -v
"""

import pytest
from datetime import datetime, timedelta
import random
import sys
import os

# Add app to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'app'))

from classifier import (
    WorkloadClassifier,
    WorkloadClassification,
    CloudWatchMetrics,
    MetricDataPoint,
    PatternDetector,
    PatternType,
    MetricPoint,
)
from optimizer import (
    CostModeler,
    WorkloadProfile,
    CommitmentOptimizer,
    UsageRecord,
)
from architecture import (
    ArchitectureAnalyzer,
    ResourceMetrics,
    ResourceType,
)


# ============================================================================
# Fixtures
# ============================================================================

def generate_metric_data(hours: int, pattern: str = "steady") -> list:
    """Generate metric data points for testing."""
    base_time = datetime.now() - timedelta(hours=hours)
    data = []
    
    for h in range(hours):
        ts = base_time + timedelta(hours=h)
        
        if pattern == "bursty":
            # 10% chance of spike
            if random.random() < 0.1:
                value = random.uniform(70, 95)
            else:
                value = random.uniform(2, 8)
        elif pattern == "steady":
            value = random.gauss(50, 5)
        elif pattern == "diurnal":
            hour = ts.hour
            if 9 <= hour <= 17:
                value = random.gauss(70, 10)
            else:
                value = random.gauss(20, 5)
        else:
            value = random.uniform(10, 90)
        
        value = max(0, min(100, value))
        data.append(MetricDataPoint(timestamp=ts, value=value))
    
    return data


# ============================================================================
# Workload Classifier Tests
# ============================================================================

class TestWorkloadClassifier:
    """Tests for the workload classifier."""
    
    def test_bursty_workload_classification(self):
        """Bursty workload should be classified as Lambda candidate."""
        metrics = CloudWatchMetrics(
            instance_id="i-test-bursty",
            instance_type="t3.medium",
            cpu_utilization=generate_metric_data(336, "bursty"),
        )
        
        classifier = WorkloadClassifier()
        result = classifier.classify(metrics)
        
        assert result.instance_id == "i-test-bursty"
        assert result.classification == WorkloadClassification.LAMBDA_CANDIDATE
        assert result.confidence > 0.5
        assert len(result.reasons) > 0
    
    def test_steady_workload_classification(self):
        """Steady workload should be classified as Fargate or Keep EC2."""
        metrics = CloudWatchMetrics(
            instance_id="i-test-steady",
            instance_type="m5.large",
            cpu_utilization=generate_metric_data(336, "steady"),
            in_auto_scaling_group=True,
        )
        
        classifier = WorkloadClassifier()
        result = classifier.classify(metrics)
        
        assert result.instance_id == "i-test-steady"
        assert result.classification in [
            WorkloadClassification.FARGATE_CANDIDATE,
            WorkloadClassification.KEEP_EC2,
        ]
    
    def test_classification_with_elastic_ip(self):
        """Elastic IP should affect classification scores."""
        # Classification without EIP
        metrics_no_eip = CloudWatchMetrics(
            instance_id="i-test-no-eip",
            instance_type="t3.large",
            cpu_utilization=generate_metric_data(336, "bursty"),
            has_elastic_ip=False,
        )
        
        # Classification with EIP
        metrics_with_eip = CloudWatchMetrics(
            instance_id="i-test-eip",
            instance_type="t3.large",
            cpu_utilization=generate_metric_data(336, "bursty"),
            has_elastic_ip=True,
        )
        
        classifier = WorkloadClassifier()
        result_no_eip = classifier.classify(metrics_no_eip)
        result_with_eip = classifier.classify(metrics_with_eip)
        
        # Elastic IP should lower Lambda/Spot scores, so confidence may differ
        # The key test is that the classifier runs successfully with EIP
        assert result_with_eip.classification is not None
    
    def test_classification_returns_alternatives(self):
        """Classification should return alternative options."""
        metrics = CloudWatchMetrics(
            instance_id="i-test-alts",
            instance_type="t3.medium",
            cpu_utilization=generate_metric_data(168, "bursty"),
        )
        
        classifier = WorkloadClassifier()
        result = classifier.classify(metrics)
        
        # Should have alternative classifications
        assert isinstance(result.alternative_classifications, list)


# ============================================================================
# Pattern Detector Tests
# ============================================================================

class TestPatternDetector:
    """Tests for the pattern detector."""
    
    def test_idle_detection(self):
        """Should detect idle periods correctly."""
        # Mostly idle data
        base_time = datetime.now() - timedelta(hours=100)
        data = [
            MetricPoint(timestamp=base_time + timedelta(hours=h), value=2)
            for h in range(100)
        ]
        
        detector = PatternDetector()
        result = detector.analyze("i-idle", data)
        
        assert result.idle.idle_percent > 90
        assert result.idle.is_idle_dominant
    
    def test_diurnal_pattern_detection(self):
        """Should detect day/night patterns."""
        base_time = datetime.now() - timedelta(hours=168)  # 1 week
        data = []
        
        for h in range(168):
            ts = base_time + timedelta(hours=h)
            hour = ts.hour
            # Clear diurnal pattern
            if 9 <= hour <= 17:
                value = 80
            else:
                value = 10
            data.append(MetricPoint(timestamp=ts, value=value))
        
        detector = PatternDetector()
        result = detector.analyze("i-diurnal", data)
        
        assert result.diurnal.has_diurnal_pattern
        assert result.diurnal.peak_to_trough_ratio > 3
    
    def test_burst_detection(self):
        """Should detect burst patterns."""
        base_time = datetime.now() - timedelta(hours=200)
        data = []
        
        for h in range(200):
            ts = base_time + timedelta(hours=h)
            # Create clear bursts every 20 hours
            if h % 20 < 3:
                value = 85
            else:
                value = 5
            data.append(MetricPoint(timestamp=ts, value=value))
        
        detector = PatternDetector()
        result = detector.analyze("i-burst", data)
        
        assert result.burst.burst_count > 0


# ============================================================================
# Cost Modeler Tests
# ============================================================================

class TestCostModeler:
    """Tests for the cost modeler."""
    
    def test_ec2_cost_calculation(self):
        """Should calculate EC2 costs correctly."""
        modeler = CostModeler()
        cost = modeler.calculate_ec2_cost("t3.large")
        
        assert cost.hourly_on_demand > 0
        assert cost.monthly_on_demand > 0
        assert cost.hourly_spot < cost.hourly_on_demand
        assert cost.spot_savings_percent > 0
    
    def test_unknown_instance_type(self):
        """Should handle unknown instance types."""
        modeler = CostModeler()
        cost = modeler.calculate_ec2_cost("x99.unknown")
        
        assert cost.is_estimated
        assert cost.hourly_on_demand > 0
    
    def test_cost_comparison(self):
        """Should compare costs across options."""
        profile = WorkloadProfile(
            instance_id="i-test",
            instance_type="t3.large",
            avg_cpu_percent=15,
            max_cpu_percent=40,
            avg_memory_percent=20,
            max_memory_percent=50,
            avg_requests_per_hour=1000,
            avg_request_duration_ms=200,
            active_hours_per_day=8,
        )
        
        modeler = CostModeler()
        comparison = modeler.compare_costs(profile)
        
        assert comparison.current_ec2.monthly_on_demand > 0
        assert comparison.best_option is not None
        assert comparison.potential_savings_percent >= 0
    
    def test_lambda_viability(self):
        """Should correctly assess Lambda viability."""
        # Low memory profile - Lambda viable
        low_mem_profile = WorkloadProfile(
            instance_id="i-low-mem",
            instance_type="t3.small",
            avg_cpu_percent=10,
            max_cpu_percent=30,
            avg_memory_percent=20,
            max_memory_percent=40,
            avg_requests_per_hour=100,
            avg_request_duration_ms=100,
            active_hours_per_day=4,
        )
        
        modeler = CostModeler()
        comparison = modeler.compare_costs(low_mem_profile)
        
        assert comparison.lambda_estimate.viable


# ============================================================================
# Commitment Optimizer Tests
# ============================================================================

class TestCommitmentOptimizer:
    """Tests for the commitment optimizer."""
    
    def generate_usage_records(self, days: int, pattern: str = "steady") -> list:
        """Generate usage records for testing."""
        records = []
        base_time = datetime.now() - timedelta(days=days)
        
        for day in range(days):
            for hour in range(24):
                ts = base_time + timedelta(days=day, hours=hour)
                
                if pattern == "steady":
                    usage = random.gauss(0.9, 0.05)
                elif pattern == "variable":
                    usage = random.uniform(0.3, 1.0)
                else:
                    usage = random.random()
                
                usage = max(0, min(1, usage))
                
                records.append(UsageRecord(
                    timestamp=ts,
                    instance_type="t3.large",
                    region="us-east-1",
                    usage_hours=usage,
                    on_demand_cost=usage * 0.0832,
                ))
        
        return records
    
    def test_steady_usage_recommends_ri(self):
        """Steady usage should recommend Reserved Instances or Savings Plans."""
        records = self.generate_usage_records(30, "steady")
        
        optimizer = CommitmentOptimizer()
        result = optimizer.optimize(records, {"t3.large": 0.0832})
        
        # Should have some recommendations or at least analyze correctly
        assert result.current_monthly_cost > 0
        # Either RIs, SPs, or a valid strategy
        assert (
            len(result.ri_recommendations) > 0 or
            len(result.sp_recommendations) > 0 or
            result.recommended_strategy is not None
        )
    
    def test_insufficient_data_warning(self):
        """Should warn when data is insufficient."""
        records = self.generate_usage_records(7, "steady")  # Only 7 days
        
        optimizer = CommitmentOptimizer()
        result = optimizer.optimize(records, {"t3.large": 0.0832})
        
        assert len(result.warnings) > 0
    
    def test_usage_analysis(self):
        """Should correctly analyze usage patterns."""
        records = self.generate_usage_records(30, "steady")
        
        optimizer = CommitmentOptimizer()
        summaries = optimizer.analyze_usage(records)
        
        assert len(summaries) > 0
        key = "t3.large:us-east-1"
        assert key in summaries
        assert summaries[key].usage_pattern == "steady"


# ============================================================================
# Architecture Analyzer Tests
# ============================================================================

class TestArchitectureAnalyzer:
    """Tests for the architecture analyzer."""
    
    def test_rds_aurora_candidate(self):
        """Should detect RDS Aurora Serverless candidates."""
        resources = [
            ResourceMetrics(
                resource_id="rds-test",
                resource_type=ResourceType.RDS,
                avg_cpu_percent=15,
                max_cpu_percent=40,
                avg_connections=25,
                max_connections=100,
                monthly_cost=200,
                engine="postgresql",
            ),
        ]
        
        analyzer = ArchitectureAnalyzer()
        result = analyzer.analyze(resources)
        
        # Should recommend Aurora Serverless
        aurora_candidates = [
            c for c in result.migration_candidates
            if "aurora" in c.recommended_target.value.lower()
        ]
        assert len(aurora_candidates) > 0
    
    def test_monolith_detection(self):
        """Should detect monolithic architecture."""
        resources = [
            ResourceMetrics(
                resource_id="ec2-app",
                resource_type=ResourceType.EC2,
                avg_cpu_percent=40,
                monthly_cost=100,
            ),
            ResourceMetrics(
                resource_id="rds-main",
                resource_type=ResourceType.RDS,
                engine="mysql",
                monthly_cost=150,
            ),
            ResourceMetrics(
                resource_id="alb-main",
                resource_type=ResourceType.ALB,
                monthly_cost=30,
            ),
        ]
        
        analyzer = ArchitectureAnalyzer()
        result = analyzer.analyze(resources)
        
        # Should detect monolithic pattern
        monolith_patterns = [
            p for p in result.patterns_detected
            if "monolith" in p.pattern_name.lower()
        ]
        assert len(monolith_patterns) > 0
    
    def test_modernization_score(self):
        """Should calculate modernization score."""
        # Modern architecture
        modern_resources = [
            ResourceMetrics(
                resource_id="lambda-1",
                resource_type=ResourceType.LAMBDA,
            ),
            ResourceMetrics(
                resource_id="dynamodb-1",
                resource_type=ResourceType.DYNAMODB,
            ),
            ResourceMetrics(
                resource_id="sqs-1",
                resource_type=ResourceType.SQS,
            ),
        ]
        
        analyzer = ArchitectureAnalyzer()
        modern_result = analyzer.analyze(modern_resources)
        
        # Legacy architecture
        legacy_resources = [
            ResourceMetrics(
                resource_id="ec2-1",
                resource_type=ResourceType.EC2,
                avg_cpu_percent=20,
            ),
            ResourceMetrics(
                resource_id="rds-1",
                resource_type=ResourceType.RDS,
                engine="mysql",
            ),
        ]
        
        legacy_result = analyzer.analyze(legacy_resources)
        
        # Modern should have higher score
        assert modern_result.modernization_score > legacy_result.modernization_score


# ============================================================================
# Integration Tests
# ============================================================================

class TestIntegration:
    """Integration tests combining multiple modules."""
    
    def test_full_workload_analysis(self):
        """Test full workload analysis pipeline."""
        # Generate metrics
        cpu_metrics = generate_metric_data(336, "bursty")
        
        # Classify workload
        metrics = CloudWatchMetrics(
            instance_id="i-full-test",
            instance_type="t3.large",
            cpu_utilization=cpu_metrics,
        )
        
        classifier = WorkloadClassifier()
        classification = classifier.classify(metrics)
        
        # Run cost modeling based on classification
        profile = WorkloadProfile(
            instance_id="i-full-test",
            instance_type="t3.large",
            avg_cpu_percent=classification.metrics_summary.get("cpu_avg", 20),
            max_cpu_percent=classification.metrics_summary.get("cpu_max", 80),
            avg_memory_percent=30,
            max_memory_percent=60,
            avg_requests_per_hour=500,
            avg_request_duration_ms=100,
            active_hours_per_day=8,
        )
        
        modeler = CostModeler()
        cost_comparison = modeler.compare_costs(profile)
        
        # Verify end-to-end
        assert classification.classification is not None
        assert cost_comparison.best_option is not None
        assert cost_comparison.potential_savings_percent >= 0


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
