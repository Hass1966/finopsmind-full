"""
Reserved Instance and Savings Plan Optimizer

Analyzes usage patterns to calculate optimal:
- Reserved Instance coverage
- Savings Plan commitment
- Recommended purchase mix
"""

from dataclasses import dataclass, field
from datetime import datetime, timedelta
from typing import List, Dict, Optional, Tuple
from enum import Enum
import statistics
import math


class CommitmentType(Enum):
    """Types of AWS commitment options."""
    ON_DEMAND = "on_demand"
    RESERVED_1Y_NO_UPFRONT = "ri_1y_no_upfront"
    RESERVED_1Y_PARTIAL = "ri_1y_partial"
    RESERVED_1Y_ALL_UPFRONT = "ri_1y_all_upfront"
    RESERVED_3Y_NO_UPFRONT = "ri_3y_no_upfront"
    RESERVED_3Y_PARTIAL = "ri_3y_partial"
    RESERVED_3Y_ALL_UPFRONT = "ri_3y_all_upfront"
    SAVINGS_PLAN_COMPUTE_1Y = "sp_compute_1y"
    SAVINGS_PLAN_COMPUTE_3Y = "sp_compute_3y"
    SAVINGS_PLAN_EC2_1Y = "sp_ec2_1y"
    SAVINGS_PLAN_EC2_3Y = "sp_ec2_3y"


# Discount percentages by commitment type (approximate, varies by instance)
COMMITMENT_DISCOUNTS = {
    CommitmentType.ON_DEMAND: 0,
    CommitmentType.RESERVED_1Y_NO_UPFRONT: 31,
    CommitmentType.RESERVED_1Y_PARTIAL: 38,
    CommitmentType.RESERVED_1Y_ALL_UPFRONT: 42,
    CommitmentType.RESERVED_3Y_NO_UPFRONT: 42,
    CommitmentType.RESERVED_3Y_PARTIAL: 52,
    CommitmentType.RESERVED_3Y_ALL_UPFRONT: 60,
    CommitmentType.SAVINGS_PLAN_COMPUTE_1Y: 37,
    CommitmentType.SAVINGS_PLAN_COMPUTE_3Y: 52,
    CommitmentType.SAVINGS_PLAN_EC2_1Y: 42,
    CommitmentType.SAVINGS_PLAN_EC2_3Y: 60,
}


@dataclass
class UsageRecord:
    """Hourly usage record for an instance or workload."""
    timestamp: datetime
    instance_type: str
    region: str
    usage_hours: float  # 0-1 for partial hours
    on_demand_cost: float


@dataclass
class UsageSummary:
    """Summary of usage patterns for commitment analysis."""
    instance_type: str
    region: str
    total_hours: float
    avg_hourly_usage: float
    min_hourly_usage: float
    max_hourly_usage: float
    p10_usage: float  # 10th percentile (stable baseline)
    p50_usage: float
    p90_usage: float
    total_on_demand_cost: float
    days_analyzed: int
    usage_pattern: str  # "steady", "variable", "bursty", "declining"


@dataclass
class RIRecommendation:
    """Reserved Instance purchase recommendation."""
    instance_type: str
    region: str
    quantity: int
    term_years: int
    payment_option: str  # "no_upfront", "partial", "all_upfront"
    commitment_type: CommitmentType
    monthly_savings: float
    annual_savings: float
    break_even_months: float
    coverage_percent: float
    risk_assessment: str  # "low", "medium", "high"


@dataclass
class SavingsPlanRecommendation:
    """Savings Plan purchase recommendation."""
    plan_type: str  # "compute", "ec2"
    hourly_commitment: float
    term_years: int
    commitment_type: CommitmentType
    monthly_cost: float
    monthly_savings: float
    annual_savings: float
    coverage_percent: float
    flexibility_score: float  # Higher = more flexible


@dataclass
class CommitmentRecommendation:
    """Complete commitment optimization recommendation."""
    current_monthly_cost: float
    optimized_monthly_cost: float
    monthly_savings: float
    savings_percent: float
    
    ri_recommendations: List[RIRecommendation]
    sp_recommendations: List[SavingsPlanRecommendation]
    
    recommended_strategy: str
    on_demand_coverage_percent: float
    committed_coverage_percent: float
    
    analysis_notes: List[str]
    warnings: List[str]


class CommitmentOptimizer:
    """
    Analyzes usage patterns and recommends optimal RI/Savings Plan mix.
    
    Strategy:
    1. Analyze historical usage to find stable baseline
    2. Recommend RIs for predictable, instance-specific workloads
    3. Recommend Savings Plans for variable or multi-service workloads
    4. Keep buffer for on-demand to handle spikes
    """
    
    # Configuration
    MINIMUM_ANALYSIS_DAYS = 14
    RI_COVERAGE_TARGET = 0.70  # Cover 70% with RIs
    ON_DEMAND_BUFFER = 0.15  # Keep 15% on-demand for flexibility
    
    def __init__(self, config: Optional[Dict] = None):
        """Initialize optimizer with optional configuration."""
        self.config = config or {}
        self._apply_config()
    
    def _apply_config(self):
        """Apply configuration overrides."""
        if "ri_coverage_target" in self.config:
            self.RI_COVERAGE_TARGET = self.config["ri_coverage_target"]
        if "on_demand_buffer" in self.config:
            self.ON_DEMAND_BUFFER = self.config["on_demand_buffer"]
    
    def analyze_usage(self, records: List[UsageRecord]) -> Dict[str, UsageSummary]:
        """
        Analyze usage records and create summaries by instance type/region.
        
        Args:
            records: List of hourly usage records
            
        Returns:
            Dictionary of usage summaries keyed by instance_type:region
        """
        # Group by instance type and region
        grouped: Dict[str, List[UsageRecord]] = {}
        for record in records:
            key = f"{record.instance_type}:{record.region}"
            if key not in grouped:
                grouped[key] = []
            grouped[key].append(record)
        
        summaries = {}
        for key, group_records in grouped.items():
            instance_type, region = key.split(":")
            
            # Calculate statistics
            usage_values = [r.usage_hours for r in group_records]
            costs = [r.on_demand_cost for r in group_records]
            
            # Determine usage pattern
            if not usage_values:
                continue
            
            avg_usage = statistics.mean(usage_values)
            stddev = statistics.stdev(usage_values) if len(usage_values) > 1 else 0
            cv = stddev / avg_usage if avg_usage > 0 else 0  # Coefficient of variation
            
            if cv < 0.2:
                pattern = "steady"
            elif cv < 0.5:
                pattern = "variable"
            else:
                pattern = "bursty"
            
            # Check for declining trend
            if len(usage_values) > 48:
                first_half = statistics.mean(usage_values[:len(usage_values)//2])
                second_half = statistics.mean(usage_values[len(usage_values)//2:])
                if second_half < first_half * 0.8:
                    pattern = "declining"
            
            # Calculate days analyzed
            if group_records:
                first_ts = min(r.timestamp for r in group_records)
                last_ts = max(r.timestamp for r in group_records)
                days = (last_ts - first_ts).days + 1
            else:
                days = 0
            
            summaries[key] = UsageSummary(
                instance_type=instance_type,
                region=region,
                total_hours=sum(usage_values),
                avg_hourly_usage=avg_usage,
                min_hourly_usage=min(usage_values),
                max_hourly_usage=max(usage_values),
                p10_usage=self._percentile(usage_values, 10),
                p50_usage=self._percentile(usage_values, 50),
                p90_usage=self._percentile(usage_values, 90),
                total_on_demand_cost=sum(costs),
                days_analyzed=days,
                usage_pattern=pattern,
            )
        
        return summaries
    
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
    
    def recommend_ris(
        self,
        summary: UsageSummary,
        on_demand_hourly: float,
    ) -> List[RIRecommendation]:
        """
        Generate RI recommendations for a usage pattern.
        
        Args:
            summary: Usage summary for instance type/region
            on_demand_hourly: On-demand hourly price
            
        Returns:
            List of RI recommendations
        """
        recommendations = []
        
        # Don't recommend RIs for:
        # - Declining usage patterns
        # - Very short analysis periods
        # - Very low usage
        if summary.usage_pattern == "declining":
            return recommendations
        
        if summary.days_analyzed < self.MINIMUM_ANALYSIS_DAYS:
            return recommendations
        
        if summary.avg_hourly_usage < 0.1:  # Less than 10% utilization
            return recommendations
        
        # Use P10 as the safe baseline for RI coverage
        safe_baseline = summary.p10_usage
        
        # Calculate quantity (round down to be conservative)
        ri_quantity = int(safe_baseline)
        
        if ri_quantity < 1:
            return recommendations
        
        # Generate recommendations for different terms/payment options
        for term_years, payment_option, commitment_type in [
            (1, "no_upfront", CommitmentType.RESERVED_1Y_NO_UPFRONT),
            (1, "all_upfront", CommitmentType.RESERVED_1Y_ALL_UPFRONT),
            (3, "no_upfront", CommitmentType.RESERVED_3Y_NO_UPFRONT),
            (3, "all_upfront", CommitmentType.RESERVED_3Y_ALL_UPFRONT),
        ]:
            discount = COMMITMENT_DISCOUNTS[commitment_type] / 100
            
            # Calculate savings
            monthly_hours = 730
            monthly_on_demand = on_demand_hourly * ri_quantity * monthly_hours
            monthly_ri_cost = monthly_on_demand * (1 - discount)
            monthly_savings = monthly_on_demand - monthly_ri_cost
            annual_savings = monthly_savings * 12
            
            # Break-even calculation
            if payment_option == "all_upfront":
                upfront_cost = monthly_ri_cost * 12 * term_years
                break_even = upfront_cost / monthly_savings if monthly_savings > 0 else 999
            else:
                break_even = 0  # No upfront = immediate savings
            
            # Coverage calculation
            coverage = (ri_quantity / summary.avg_hourly_usage * 100) if summary.avg_hourly_usage > 0 else 0
            
            # Risk assessment
            if summary.usage_pattern == "steady":
                risk = "low"
            elif summary.usage_pattern == "variable":
                risk = "medium"
            else:
                risk = "high"
            
            recommendations.append(RIRecommendation(
                instance_type=summary.instance_type,
                region=summary.region,
                quantity=ri_quantity,
                term_years=term_years,
                payment_option=payment_option,
                commitment_type=commitment_type,
                monthly_savings=monthly_savings,
                annual_savings=annual_savings,
                break_even_months=break_even,
                coverage_percent=coverage,
                risk_assessment=risk,
            ))
        
        return recommendations
    
    def recommend_savings_plans(
        self,
        summaries: Dict[str, UsageSummary],
        total_monthly_spend: float,
    ) -> List[SavingsPlanRecommendation]:
        """
        Generate Savings Plan recommendations based on aggregate usage.
        
        Args:
            summaries: All usage summaries
            total_monthly_spend: Total monthly on-demand spend
            
        Returns:
            List of Savings Plan recommendations
        """
        recommendations = []
        
        # Calculate total stable hourly spend
        total_p10_cost = sum(
            s.p10_usage * (s.total_on_demand_cost / s.total_hours) if s.total_hours > 0 else 0
            for s in summaries.values()
        )
        
        if total_p10_cost < 1:  # Less than $1/hour baseline
            return recommendations
        
        # Determine flexibility needs
        instance_types = set(s.instance_type for s in summaries.values())
        regions = set(s.region for s in summaries.values())
        
        # More instance types/regions = prefer Compute SP for flexibility
        flexibility_needed = len(instance_types) > 3 or len(regions) > 1
        
        # Generate SP recommendations
        for term_years in [1, 3]:
            # Compute Savings Plan (more flexible)
            compute_type = CommitmentType.SAVINGS_PLAN_COMPUTE_1Y if term_years == 1 else CommitmentType.SAVINGS_PLAN_COMPUTE_3Y
            compute_discount = COMMITMENT_DISCOUNTS[compute_type] / 100
            
            # Use 70% of P10 baseline for SP commitment (conservative)
            sp_commitment = total_p10_cost * 0.7
            
            monthly_sp_cost = sp_commitment * 730  # Hourly to monthly
            monthly_savings = monthly_sp_cost * compute_discount
            
            recommendations.append(SavingsPlanRecommendation(
                plan_type="Compute",
                hourly_commitment=sp_commitment,
                term_years=term_years,
                commitment_type=compute_type,
                monthly_cost=monthly_sp_cost,
                monthly_savings=monthly_savings,
                annual_savings=monthly_savings * 12,
                coverage_percent=(sp_commitment / (total_monthly_spend / 730)) * 100 if total_monthly_spend > 0 else 0,
                flexibility_score=0.9 if flexibility_needed else 0.7,
            ))
            
            # EC2 Instance Savings Plan (better discount, less flexible)
            ec2_type = CommitmentType.SAVINGS_PLAN_EC2_1Y if term_years == 1 else CommitmentType.SAVINGS_PLAN_EC2_3Y
            ec2_discount = COMMITMENT_DISCOUNTS[ec2_type] / 100
            
            monthly_savings_ec2 = monthly_sp_cost * ec2_discount
            
            recommendations.append(SavingsPlanRecommendation(
                plan_type="EC2 Instance",
                hourly_commitment=sp_commitment,
                term_years=term_years,
                commitment_type=ec2_type,
                monthly_cost=monthly_sp_cost,
                monthly_savings=monthly_savings_ec2,
                annual_savings=monthly_savings_ec2 * 12,
                coverage_percent=(sp_commitment / (total_monthly_spend / 730)) * 100 if total_monthly_spend > 0 else 0,
                flexibility_score=0.5 if flexibility_needed else 0.8,
            ))
        
        return recommendations
    
    def optimize(
        self,
        records: List[UsageRecord],
        on_demand_prices: Dict[str, float],  # instance_type -> hourly price
    ) -> CommitmentRecommendation:
        """
        Generate complete commitment optimization recommendation.
        
        Args:
            records: Historical usage records
            on_demand_prices: On-demand hourly prices by instance type
            
        Returns:
            Complete optimization recommendation
        """
        # Analyze usage patterns
        summaries = self.analyze_usage(records)
        
        if not summaries:
            return self._empty_recommendation()
        
        # Calculate current costs
        current_monthly = sum(s.total_on_demand_cost for s in summaries.values())
        if current_monthly == 0:
            return self._empty_recommendation()
        
        # Generate RI recommendations for each instance type
        all_ri_recs = []
        for key, summary in summaries.items():
            instance_type = summary.instance_type
            if instance_type in on_demand_prices:
                ri_recs = self.recommend_ris(summary, on_demand_prices[instance_type])
                all_ri_recs.extend(ri_recs)
        
        # Generate Savings Plan recommendations
        sp_recs = self.recommend_savings_plans(summaries, current_monthly)
        
        # Select best recommendations
        best_ri = self._select_best_ris(all_ri_recs)
        best_sp = self._select_best_sp(sp_recs)
        
        # Calculate optimized costs
        ri_savings = sum(r.monthly_savings for r in best_ri)
        sp_savings = best_sp.monthly_savings if best_sp else 0
        
        # Don't double-count savings (SP and RI can overlap)
        total_savings = min(ri_savings + sp_savings, current_monthly * 0.6)  # Cap at 60% savings
        
        optimized_monthly = current_monthly - total_savings
        
        # Determine strategy
        if best_ri and best_sp:
            strategy = "Hybrid: RIs for stable workloads, Savings Plan for flexibility"
        elif best_ri:
            strategy = "Reserved Instances for predictable workloads"
        elif best_sp:
            strategy = "Savings Plan for flexible coverage"
        else:
            strategy = "Maintain On-Demand for variable/declining usage"
        
        # Coverage calculations
        ri_coverage = sum(r.coverage_percent for r in best_ri) / len(best_ri) if best_ri else 0
        sp_coverage = best_sp.coverage_percent if best_sp else 0
        committed_coverage = min(ri_coverage + sp_coverage, 100)
        on_demand_coverage = 100 - committed_coverage
        
        # Analysis notes
        notes = self._generate_analysis_notes(summaries, best_ri, best_sp)
        
        # Warnings
        warnings = self._generate_warnings(summaries, best_ri)
        
        return CommitmentRecommendation(
            current_monthly_cost=current_monthly,
            optimized_monthly_cost=optimized_monthly,
            monthly_savings=total_savings,
            savings_percent=(total_savings / current_monthly * 100) if current_monthly > 0 else 0,
            ri_recommendations=best_ri,
            sp_recommendations=[best_sp] if best_sp else [],
            recommended_strategy=strategy,
            on_demand_coverage_percent=on_demand_coverage,
            committed_coverage_percent=committed_coverage,
            analysis_notes=notes,
            warnings=warnings,
        )
    
    def _select_best_ris(self, recommendations: List[RIRecommendation]) -> List[RIRecommendation]:
        """Select the best RI recommendations."""
        if not recommendations:
            return []
        
        # Group by instance type
        by_type: Dict[str, List[RIRecommendation]] = {}
        for rec in recommendations:
            if rec.instance_type not in by_type:
                by_type[rec.instance_type] = []
            by_type[rec.instance_type].append(rec)
        
        # Select best for each type (prefer 1-year no-upfront for lower risk)
        selected = []
        for instance_type, type_recs in by_type.items():
            # Filter to low-medium risk only
            safe_recs = [r for r in type_recs if r.risk_assessment in ["low", "medium"]]
            if not safe_recs:
                continue
            
            # Prefer 1-year terms for flexibility
            one_year = [r for r in safe_recs if r.term_years == 1]
            if one_year:
                best = max(one_year, key=lambda r: r.monthly_savings)
            else:
                best = max(safe_recs, key=lambda r: r.monthly_savings)
            
            selected.append(best)
        
        return selected
    
    def _select_best_sp(self, recommendations: List[SavingsPlanRecommendation]) -> Optional[SavingsPlanRecommendation]:
        """Select the best Savings Plan recommendation."""
        if not recommendations:
            return None
        
        # Score each option by savings * flexibility
        scored = [
            (rec, rec.monthly_savings * rec.flexibility_score)
            for rec in recommendations
        ]
        
        # Return highest scored
        return max(scored, key=lambda x: x[1])[0]
    
    def _generate_analysis_notes(
        self,
        summaries: Dict[str, UsageSummary],
        ri_recs: List[RIRecommendation],
        sp_rec: Optional[SavingsPlanRecommendation],
    ) -> List[str]:
        """Generate analysis notes."""
        notes = []
        
        # Usage pattern observations
        patterns = [s.usage_pattern for s in summaries.values()]
        steady_count = patterns.count("steady")
        variable_count = patterns.count("variable")
        
        if steady_count > len(patterns) / 2:
            notes.append(f"{steady_count} of {len(patterns)} workloads show steady usage - ideal for RIs")
        
        if variable_count > 0:
            notes.append(f"{variable_count} workloads show variable usage - Savings Plans provide flexibility")
        
        # RI recommendations
        if ri_recs:
            total_ri_savings = sum(r.annual_savings for r in ri_recs)
            notes.append(f"Recommended RIs could save ${total_ri_savings:.0f}/year")
        
        # SP recommendation
        if sp_rec:
            notes.append(f"{sp_rec.plan_type} Savings Plan ({sp_rec.term_years}Y) recommended for ${sp_rec.annual_savings:.0f}/year savings")
        
        return notes
    
    def _generate_warnings(
        self,
        summaries: Dict[str, UsageSummary],
        ri_recs: List[RIRecommendation],
    ) -> List[str]:
        """Generate warnings about recommendations."""
        warnings = []
        
        # Short analysis period
        for summary in summaries.values():
            if summary.days_analyzed < self.MINIMUM_ANALYSIS_DAYS:
                warnings.append(
                    f"Only {summary.days_analyzed} days analyzed for {summary.instance_type} - "
                    "recommend 30+ days for accurate recommendations"
                )
                break  # Only warn once
        
        # Declining usage
        declining = [s for s in summaries.values() if s.usage_pattern == "declining"]
        if declining:
            warnings.append(
                f"{len(declining)} workloads show declining usage - "
                "avoid long-term commitments"
            )
        
        # High-risk RIs
        high_risk = [r for r in ri_recs if r.risk_assessment == "high"]
        if high_risk:
            warnings.append(
                f"{len(high_risk)} RI recommendations are high-risk due to variable usage"
            )
        
        return warnings
    
    def _empty_recommendation(self) -> CommitmentRecommendation:
        """Return empty recommendation when no data available."""
        return CommitmentRecommendation(
            current_monthly_cost=0,
            optimized_monthly_cost=0,
            monthly_savings=0,
            savings_percent=0,
            ri_recommendations=[],
            sp_recommendations=[],
            recommended_strategy="Insufficient data for recommendations",
            on_demand_coverage_percent=100,
            committed_coverage_percent=0,
            analysis_notes=["No usage data available for analysis"],
            warnings=["Provide at least 14 days of usage data"],
        )


__all__ = [
    "CommitmentOptimizer",
    "CommitmentType",
    "UsageRecord",
    "UsageSummary",
    "RIRecommendation",
    "SavingsPlanRecommendation",
    "CommitmentRecommendation",
]


if __name__ == "__main__":
    from datetime import datetime, timedelta
    import random
    
    print("=" * 60)
    print("Testing Commitment Optimizer")
    print("=" * 60)
    
    # Generate test usage data
    def generate_steady_usage(instance_type: str, region: str, days: int = 30) -> List[UsageRecord]:
        records = []
        base_time = datetime.now() - timedelta(days=days)
        
        for day in range(days):
            for hour in range(24):
                ts = base_time + timedelta(days=day, hours=hour)
                # Steady usage around 0.8-1.0 instances
                usage = random.gauss(0.9, 0.05)
                usage = max(0.7, min(1.0, usage))
                
                # On-demand price for t3.large
                hourly_price = 0.0832
                
                records.append(UsageRecord(
                    timestamp=ts,
                    instance_type=instance_type,
                    region=region,
                    usage_hours=usage,
                    on_demand_cost=usage * hourly_price,
                ))
        
        return records
    
    # Generate test data
    records = generate_steady_usage("t3.large", "us-east-1", days=30)
    
    # Add variable workload
    records.extend(generate_steady_usage("m5.xlarge", "us-east-1", days=30))
    
    # Run optimizer
    optimizer = CommitmentOptimizer()
    
    on_demand_prices = {
        "t3.large": 0.0832,
        "m5.xlarge": 0.192,
    }
    
    result = optimizer.optimize(records, on_demand_prices)
    
    print(f"\n--- Optimization Results ---")
    print(f"Current Monthly Cost: ${result.current_monthly_cost:.2f}")
    print(f"Optimized Monthly Cost: ${result.optimized_monthly_cost:.2f}")
    print(f"Monthly Savings: ${result.monthly_savings:.2f} ({result.savings_percent:.0f}%)")
    print(f"\nStrategy: {result.recommended_strategy}")
    print(f"Committed Coverage: {result.committed_coverage_percent:.0f}%")
    print(f"On-Demand Buffer: {result.on_demand_coverage_percent:.0f}%")
    
    if result.ri_recommendations:
        print("\n--- RI Recommendations ---")
        for ri in result.ri_recommendations:
            print(f"  {ri.instance_type}: {ri.quantity}x {ri.term_years}Y {ri.payment_option}")
            print(f"    Monthly Savings: ${ri.monthly_savings:.2f}")
            print(f"    Risk: {ri.risk_assessment}")
    
    if result.sp_recommendations:
        print("\n--- Savings Plan Recommendations ---")
        for sp in result.sp_recommendations:
            print(f"  {sp.plan_type} ({sp.term_years}Y): ${sp.hourly_commitment:.3f}/hour")
            print(f"    Monthly Savings: ${sp.monthly_savings:.2f}")
    
    if result.analysis_notes:
        print("\n--- Analysis Notes ---")
        for note in result.analysis_notes:
            print(f"  • {note}")
    
    if result.warnings:
        print("\n--- Warnings ---")
        for warning in result.warnings:
            print(f"  ⚠ {warning}")
