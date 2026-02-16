"""
Cost Model for What-If Calculations

Calculates and compares costs across different compute options:
- Lambda cost for workload
- Fargate cost for workload
- Spot savings potential
- Current EC2 cost baseline
"""

from dataclasses import dataclass, field
from datetime import datetime, timedelta
from typing import List, Dict, Optional, Tuple
from enum import Enum
import math


# AWS Pricing (US-East-1 as baseline - can be overridden)
EC2_PRICING = {
    # Instance type: (hourly_on_demand, hourly_spot_avg)
    "t3.micro": (0.0104, 0.0031),
    "t3.small": (0.0208, 0.0062),
    "t3.medium": (0.0416, 0.0125),
    "t3.large": (0.0832, 0.0250),
    "t3.xlarge": (0.1664, 0.0499),
    "t3.2xlarge": (0.3328, 0.0998),
    "m5.large": (0.096, 0.0384),
    "m5.xlarge": (0.192, 0.0768),
    "m5.2xlarge": (0.384, 0.1536),
    "m5.4xlarge": (0.768, 0.3072),
    "m6i.large": (0.096, 0.0384),
    "m6i.xlarge": (0.192, 0.0768),
    "m6i.2xlarge": (0.384, 0.1536),
    "c5.large": (0.085, 0.0340),
    "c5.xlarge": (0.170, 0.0680),
    "c5.2xlarge": (0.340, 0.1360),
    "c6i.large": (0.085, 0.0340),
    "c6i.xlarge": (0.170, 0.0680),
    "r5.large": (0.126, 0.0504),
    "r5.xlarge": (0.252, 0.1008),
    "r5.2xlarge": (0.504, 0.2016),
}

# Instance specs (vCPU, Memory GB)
EC2_SPECS = {
    "t3.micro": (2, 1),
    "t3.small": (2, 2),
    "t3.medium": (2, 4),
    "t3.large": (2, 8),
    "t3.xlarge": (4, 16),
    "t3.2xlarge": (8, 32),
    "m5.large": (2, 8),
    "m5.xlarge": (4, 16),
    "m5.2xlarge": (8, 32),
    "m5.4xlarge": (16, 64),
    "m6i.large": (2, 8),
    "m6i.xlarge": (4, 16),
    "m6i.2xlarge": (8, 32),
    "c5.large": (2, 4),
    "c5.xlarge": (4, 8),
    "c5.2xlarge": (8, 16),
    "c6i.large": (2, 4),
    "c6i.xlarge": (4, 8),
    "r5.large": (2, 16),
    "r5.xlarge": (4, 32),
    "r5.2xlarge": (8, 64),
}

# Lambda pricing
LAMBDA_PRICE_PER_GB_SECOND = 0.0000166667
LAMBDA_PRICE_PER_REQUEST = 0.0000002
LAMBDA_FREE_TIER_GB_SECONDS = 400000
LAMBDA_FREE_TIER_REQUESTS = 1000000

# Fargate pricing (per vCPU-hour and GB-hour)
FARGATE_VCPU_PER_HOUR = 0.04048
FARGATE_MEMORY_GB_PER_HOUR = 0.004445
FARGATE_SPOT_DISCOUNT = 0.70  # 70% discount

# Fargate configurations
FARGATE_CONFIGS = [
    {"vcpu": 0.25, "memory_gb": 0.5},
    {"vcpu": 0.25, "memory_gb": 1},
    {"vcpu": 0.25, "memory_gb": 2},
    {"vcpu": 0.5, "memory_gb": 1},
    {"vcpu": 0.5, "memory_gb": 2},
    {"vcpu": 0.5, "memory_gb": 3},
    {"vcpu": 0.5, "memory_gb": 4},
    {"vcpu": 1, "memory_gb": 2},
    {"vcpu": 1, "memory_gb": 3},
    {"vcpu": 1, "memory_gb": 4},
    {"vcpu": 1, "memory_gb": 5},
    {"vcpu": 1, "memory_gb": 6},
    {"vcpu": 1, "memory_gb": 7},
    {"vcpu": 1, "memory_gb": 8},
    {"vcpu": 2, "memory_gb": 4},
    {"vcpu": 2, "memory_gb": 8},
    {"vcpu": 2, "memory_gb": 16},
    {"vcpu": 4, "memory_gb": 8},
    {"vcpu": 4, "memory_gb": 16},
    {"vcpu": 4, "memory_gb": 30},
]


@dataclass
class WorkloadProfile:
    """Profile of workload resource requirements."""
    instance_id: str
    instance_type: str
    avg_cpu_percent: float
    max_cpu_percent: float
    avg_memory_percent: float
    max_memory_percent: float
    avg_requests_per_hour: float  # Estimated from network metrics
    avg_request_duration_ms: float  # Estimated execution time
    active_hours_per_day: float  # Hours with >5% CPU
    monthly_hours: float = 730  # Total hours in month


@dataclass
class EC2Cost:
    """EC2 cost breakdown."""
    instance_type: str
    hourly_on_demand: float
    monthly_on_demand: float
    hourly_spot: float
    monthly_spot: float
    spot_savings_percent: float
    is_estimated: bool = False


@dataclass
class LambdaCost:
    """Lambda cost estimation."""
    memory_mb: int
    monthly_invocations: int
    avg_duration_ms: float
    monthly_gb_seconds: float
    monthly_cost: float
    monthly_cost_after_free_tier: float
    viable: bool
    viability_issues: List[str]


@dataclass
class FargateCost:
    """Fargate cost estimation."""
    vcpu: float
    memory_gb: float
    monthly_hours: float
    monthly_cost: float
    monthly_cost_spot: float
    is_right_sized: bool
    sizing_recommendation: str


@dataclass
class CostComparison:
    """Complete cost comparison across compute options."""
    instance_id: str
    current_ec2: EC2Cost
    lambda_estimate: LambdaCost
    fargate_estimate: FargateCost
    spot_estimate: EC2Cost
    
    best_option: str
    best_option_monthly_cost: float
    potential_savings_dollars: float
    potential_savings_percent: float
    
    recommendations: List[str]
    caveats: List[str]


class CostModeler:
    """Calculate and compare costs across compute options."""
    
    def __init__(self, region: str = "us-east-1", custom_pricing: Optional[Dict] = None):
        self.region = region
        self.pricing = custom_pricing or {}
        self.region_multiplier = self._get_region_multiplier(region)
    
    def _get_region_multiplier(self, region: str) -> float:
        """Get pricing multiplier for region."""
        multipliers = {
            "us-east-1": 1.0,
            "us-east-2": 1.0,
            "us-west-1": 1.08,
            "us-west-2": 1.0,
            "eu-west-1": 1.10,
            "eu-west-2": 1.12,
            "eu-central-1": 1.15,
            "ap-southeast-1": 1.12,
            "ap-southeast-2": 1.15,
            "ap-northeast-1": 1.20,
        }
        return multipliers.get(region, 1.1)
    
    def calculate_ec2_cost(self, instance_type: str) -> EC2Cost:
        """Calculate EC2 on-demand and Spot costs."""
        if instance_type in EC2_PRICING:
            on_demand, spot = EC2_PRICING[instance_type]
            is_estimated = False
        else:
            on_demand, spot = self._estimate_ec2_price(instance_type)
            is_estimated = True
        
        on_demand *= self.region_multiplier
        spot *= self.region_multiplier
        
        monthly_on_demand = on_demand * 730
        monthly_spot = spot * 730
        
        spot_savings = ((on_demand - spot) / on_demand) * 100 if on_demand > 0 else 0
        
        return EC2Cost(
            instance_type=instance_type,
            hourly_on_demand=on_demand,
            monthly_on_demand=monthly_on_demand,
            hourly_spot=spot,
            monthly_spot=monthly_spot,
            spot_savings_percent=spot_savings,
            is_estimated=is_estimated,
        )
    
    def _estimate_ec2_price(self, instance_type: str) -> Tuple[float, float]:
        """Estimate price for unknown instance type."""
        parts = instance_type.split(".")
        if len(parts) != 2:
            return (0.10, 0.03)
        
        size = parts[1]
        size_multipliers = {
            "micro": 0.5, "small": 1, "medium": 2, "large": 4,
            "xlarge": 8, "2xlarge": 16, "4xlarge": 32, "8xlarge": 64,
        }
        
        family_base = {"t": 0.005, "m": 0.012, "c": 0.011, "r": 0.016, "i": 0.020}
        
        multiplier = size_multipliers.get(size, 8)
        base = family_base.get(parts[0][0], 0.012)
        
        on_demand = base * multiplier
        spot = on_demand * 0.3
        
        return (on_demand, spot)
    
    def calculate_lambda_cost(self, profile: WorkloadProfile) -> LambdaCost:
        """Calculate Lambda cost for workload."""
        viability_issues = []
        
        instance_memory_gb = self._get_instance_memory(profile.instance_type)
        required_memory_gb = instance_memory_gb * (profile.max_memory_percent / 100)
        
        if required_memory_gb > 10:
            viability_issues.append(f"Memory ({required_memory_gb:.1f}GB) exceeds Lambda max (10GB)")
        
        if profile.avg_request_duration_ms > 900000:
            viability_issues.append("Execution time may exceed Lambda 15-minute limit")
        
        memory_mb = self._recommend_lambda_memory(profile)
        
        monthly_invocations = int(profile.avg_requests_per_hour * 24 * 30)
        if monthly_invocations == 0:
            monthly_invocations = int(profile.active_hours_per_day * 30 * 60)
        
        avg_duration_seconds = (profile.avg_request_duration_ms or 500) / 1000
        
        gb_seconds = monthly_invocations * (memory_mb / 1024) * avg_duration_seconds
        
        compute_cost = gb_seconds * LAMBDA_PRICE_PER_GB_SECOND
        request_cost = monthly_invocations * LAMBDA_PRICE_PER_REQUEST
        monthly_cost = compute_cost + request_cost
        
        free_tier_compute = min(gb_seconds, LAMBDA_FREE_TIER_GB_SECONDS) * LAMBDA_PRICE_PER_GB_SECOND
        free_tier_requests = min(monthly_invocations, LAMBDA_FREE_TIER_REQUESTS) * LAMBDA_PRICE_PER_REQUEST
        monthly_cost_after_free = max(0, monthly_cost - free_tier_compute - free_tier_requests)
        
        monthly_cost *= self.region_multiplier
        monthly_cost_after_free *= self.region_multiplier
        
        return LambdaCost(
            memory_mb=memory_mb,
            monthly_invocations=monthly_invocations,
            avg_duration_ms=profile.avg_request_duration_ms or 500,
            monthly_gb_seconds=gb_seconds,
            monthly_cost=monthly_cost,
            monthly_cost_after_free_tier=monthly_cost_after_free,
            viable=len(viability_issues) == 0,
            viability_issues=viability_issues,
        )
    
    def _recommend_lambda_memory(self, profile: WorkloadProfile) -> int:
        """Recommend Lambda memory size based on workload."""
        instance_memory_gb = self._get_instance_memory(profile.instance_type)
        avg_memory_gb = instance_memory_gb * (profile.avg_memory_percent / 100)
        recommended_mb = int(avg_memory_gb * 1.5 * 1024)
        
        lambda_sizes = [128, 256, 512, 1024, 1536, 2048, 3008, 4096, 5120, 6144, 7168, 8192, 9216, 10240]
        
        for size in lambda_sizes:
            if size >= recommended_mb:
                return size
        
        return 10240
    
    def _get_instance_memory(self, instance_type: str) -> float:
        """Get instance memory in GB."""
        if instance_type in EC2_SPECS:
            return EC2_SPECS[instance_type][1]
        
        size = instance_type.split(".")[-1] if "." in instance_type else "large"
        size_memory = {"micro": 1, "small": 2, "medium": 4, "large": 8, "xlarge": 16, "2xlarge": 32}
        return size_memory.get(size, 8)
    
    def _get_instance_vcpu(self, instance_type: str) -> int:
        """Get instance vCPU count."""
        if instance_type in EC2_SPECS:
            return EC2_SPECS[instance_type][0]
        
        size = instance_type.split(".")[-1] if "." in instance_type else "large"
        size_vcpu = {"micro": 2, "small": 2, "medium": 2, "large": 2, "xlarge": 4, "2xlarge": 8}
        return size_vcpu.get(size, 2)
    
    def calculate_fargate_cost(self, profile: WorkloadProfile) -> FargateCost:
        """Calculate Fargate cost for workload."""
        instance_vcpu = self._get_instance_vcpu(profile.instance_type)
        instance_memory = self._get_instance_memory(profile.instance_type)
        
        required_vcpu = instance_vcpu * (profile.max_cpu_percent / 100) * 1.3
        required_memory = instance_memory * (profile.max_memory_percent / 100) * 1.3
        
        best_config = None
        for config in FARGATE_CONFIGS:
            if config["vcpu"] >= required_vcpu and config["memory_gb"] >= required_memory:
                if best_config is None or (config["vcpu"] * config["memory_gb"] < best_config["vcpu"] * best_config["memory_gb"]):
                    best_config = config
        
        if best_config is None:
            best_config = FARGATE_CONFIGS[-1]
            is_right_sized = False
            sizing_rec = f"Workload may exceed Fargate limits (needs {required_vcpu:.1f} vCPU, {required_memory:.1f}GB)"
        else:
            is_right_sized = True
            sizing_rec = f"Recommended: {best_config['vcpu']} vCPU, {best_config['memory_gb']}GB memory"
        
        monthly_hours = profile.active_hours_per_day * 30
        
        vcpu_cost = best_config["vcpu"] * FARGATE_VCPU_PER_HOUR * monthly_hours
        memory_cost = best_config["memory_gb"] * FARGATE_MEMORY_GB_PER_HOUR * monthly_hours
        monthly_cost = (vcpu_cost + memory_cost) * self.region_multiplier
        monthly_cost_spot = monthly_cost * (1 - FARGATE_SPOT_DISCOUNT)
        
        return FargateCost(
            vcpu=best_config["vcpu"],
            memory_gb=best_config["memory_gb"],
            monthly_hours=monthly_hours,
            monthly_cost=monthly_cost,
            monthly_cost_spot=monthly_cost_spot,
            is_right_sized=is_right_sized,
            sizing_recommendation=sizing_rec,
        )
    
    def compare_costs(self, profile: WorkloadProfile) -> CostComparison:
        """Compare costs across all compute options."""
        ec2_cost = self.calculate_ec2_cost(profile.instance_type)
        lambda_cost = self.calculate_lambda_cost(profile)
        fargate_cost = self.calculate_fargate_cost(profile)
        
        options = [
            ("EC2 On-Demand", ec2_cost.monthly_on_demand),
            ("EC2 Spot", ec2_cost.monthly_spot),
            ("Fargate On-Demand", fargate_cost.monthly_cost),
            ("Fargate Spot", fargate_cost.monthly_cost_spot),
        ]
        
        if lambda_cost.viable:
            options.append(("Lambda", lambda_cost.monthly_cost_after_free_tier))
        
        options.sort(key=lambda x: x[1])
        best_option, best_cost = options[0]
        
        current_cost = ec2_cost.monthly_on_demand
        savings_dollars = current_cost - best_cost
        savings_percent = (savings_dollars / current_cost) * 100 if current_cost > 0 else 0
        
        recommendations = []
        caveats = []
        
        if best_option == "Lambda" and lambda_cost.viable:
            recommendations.append(f"Migrate to Lambda for ${savings_dollars:.0f}/month savings ({savings_percent:.0f}%)")
            recommendations.append(f"Recommended Lambda config: {lambda_cost.memory_mb}MB memory")
        elif "Fargate" in best_option:
            recommendations.append(f"Migrate to {best_option} for ${savings_dollars:.0f}/month savings ({savings_percent:.0f}%)")
            recommendations.append(fargate_cost.sizing_recommendation)
        elif best_option == "EC2 Spot":
            recommendations.append(f"Switch to Spot Instances for ${savings_dollars:.0f}/month savings ({savings_percent:.0f}%)")
            caveats.append("Spot instances can be interrupted with 2-minute warning")
        
        if not lambda_cost.viable:
            caveats.extend(lambda_cost.viability_issues)
        
        if ec2_cost.is_estimated:
            caveats.append(f"Pricing for {profile.instance_type} is estimated")
        
        return CostComparison(
            instance_id=profile.instance_id,
            current_ec2=ec2_cost,
            lambda_estimate=lambda_cost,
            fargate_estimate=fargate_cost,
            spot_estimate=ec2_cost,
            best_option=best_option,
            best_option_monthly_cost=best_cost,
            potential_savings_dollars=savings_dollars,
            potential_savings_percent=savings_percent,
            recommendations=recommendations,
            caveats=caveats,
        )


def compare_workload_costs(profiles: List[WorkloadProfile], region: str = "us-east-1") -> List[CostComparison]:
    """Compare costs for multiple workloads."""
    modeler = CostModeler(region=region)
    return [modeler.compare_costs(p) for p in profiles]


__all__ = ["CostModeler", "WorkloadProfile", "EC2Cost", "LambdaCost", "FargateCost", "CostComparison", "compare_workload_costs"]


if __name__ == "__main__":
    print("=" * 60)
    print("Testing Cost Modeler")
    print("=" * 60)
    
    modeler = CostModeler(region="us-east-1")
    
    profile = WorkloadProfile(
        instance_id="i-test123",
        instance_type="t3.large",
        avg_cpu_percent=15,
        max_cpu_percent=45,
        avg_memory_percent=25,
        max_memory_percent=60,
        avg_requests_per_hour=1000,
        avg_request_duration_ms=200,
        active_hours_per_day=8,
    )
    
    print(f"\n--- Test: Low-utilization t3.large ---")
    print(f"Instance: {profile.instance_type}")
    print(f"Avg CPU: {profile.avg_cpu_percent}%, Avg Memory: {profile.avg_memory_percent}%")
    
    comparison = modeler.compare_costs(profile)
    
    print(f"\n--- Cost Comparison ---")
    print(f"EC2 On-Demand: ${comparison.current_ec2.monthly_on_demand:.2f}/month")
    print(f"EC2 Spot: ${comparison.current_ec2.monthly_spot:.2f}/month")
    print(f"Lambda: ${comparison.lambda_estimate.monthly_cost_after_free_tier:.2f}/month")
    print(f"Fargate: ${comparison.fargate_estimate.monthly_cost:.2f}/month")
    
    print(f"\n--- Best Option ---")
    print(f"Recommendation: {comparison.best_option}")
    print(f"Savings: ${comparison.potential_savings_dollars:.2f}/month ({comparison.potential_savings_percent:.0f}%)")
