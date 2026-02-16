"""
Architecture Analyzer for Migration Recommendations

Analyzes AWS architecture patterns and recommends modernization:
- RDS → Aurora Serverless candidate detection
- ElastiCache → DAX candidate detection
- Monolith → Microservices indicators
- Event-driven refactoring opportunities
"""

from dataclasses import dataclass, field
from typing import List, Dict, Optional, Tuple
from enum import Enum


class ResourceType(Enum):
    """AWS resource types for analysis."""
    EC2 = "ec2"
    RDS = "rds"
    ELASTICACHE = "elasticache"
    LAMBDA = "lambda"
    ECS = "ecs"
    EKS = "eks"
    FARGATE = "fargate"
    S3 = "s3"
    DYNAMODB = "dynamodb"
    SQS = "sqs"
    SNS = "sns"
    API_GATEWAY = "api_gateway"
    ALB = "alb"
    CLOUDFRONT = "cloudfront"


class MigrationTarget(Enum):
    """Potential migration targets."""
    AURORA_SERVERLESS = "aurora_serverless"
    DAX = "dax"
    DYNAMODB = "dynamodb"
    LAMBDA = "lambda"
    FARGATE = "fargate"
    EKS = "eks"
    EVENT_BRIDGE = "event_bridge"
    STEP_FUNCTIONS = "step_functions"
    API_GATEWAY_HTTP = "api_gateway_http"


@dataclass
class ResourceMetrics:
    """Metrics for a resource."""
    resource_id: str
    resource_type: ResourceType
    avg_cpu_percent: float = 0
    max_cpu_percent: float = 0
    avg_memory_percent: float = 0
    avg_connections: float = 0
    max_connections: float = 0
    avg_iops: float = 0
    max_iops: float = 0
    avg_throughput_mbps: float = 0
    storage_gb: float = 0
    monthly_cost: float = 0
    engine: str = ""
    multi_az: bool = False
    read_replicas: int = 0
    cache_hits_percent: float = 0
    connected_services: List[str] = field(default_factory=list)
    tags: Dict[str, str] = field(default_factory=dict)


@dataclass
class ArchitecturePattern:
    """Detected architecture pattern."""
    pattern_name: str
    description: str
    resources_involved: List[str]
    confidence: float
    modernization_opportunity: bool
    recommendations: List[str]


@dataclass
class MigrationCandidate:
    """Migration candidate with analysis."""
    resource_id: str
    resource_type: ResourceType
    current_config: str
    recommended_target: MigrationTarget
    confidence: float
    estimated_savings_percent: float
    estimated_savings_monthly: float
    migration_complexity: str
    prerequisites: List[str]
    benefits: List[str]
    risks: List[str]


@dataclass
class ArchitectureAnalysis:
    """Complete architecture analysis result."""
    patterns_detected: List[ArchitecturePattern]
    migration_candidates: List[MigrationCandidate]
    refactoring_opportunities: List[Dict]
    total_potential_savings: float
    modernization_score: float
    recommendations: List[str]
    warnings: List[str]


class ArchitectureAnalyzer:
    """Analyzes AWS architecture for modernization opportunities."""
    
    AURORA_MIN_IDLE_PERCENT = 30
    AURORA_MAX_PEAK_ACU = 128
    DAX_MIN_READ_PERCENT = 80
    DAX_MIN_CACHE_HIT_RATE = 60
    
    def __init__(self, config: Optional[Dict] = None):
        self.config = config or {}
    
    def analyze(self, resources: List[ResourceMetrics], dependencies: Optional[Dict[str, List[str]]] = None) -> ArchitectureAnalysis:
        dependencies = dependencies or {}
        patterns = self._detect_patterns(resources, dependencies)
        candidates = []
        
        for rds in [r for r in resources if r.resource_type == ResourceType.RDS]:
            candidate = self._analyze_rds_migration(rds)
            if candidate:
                candidates.append(candidate)
        
        for cache in [r for r in resources if r.resource_type == ResourceType.ELASTICACHE]:
            candidate = self._analyze_cache_migration(cache, resources)
            if candidate:
                candidates.append(candidate)
        
        for ec2 in [r for r in resources if r.resource_type == ResourceType.EC2]:
            candidate = self._analyze_ec2_modernization(ec2, dependencies)
            if candidate:
                candidates.append(candidate)
        
        refactoring = self._find_refactoring_opportunities(resources, dependencies)
        total_savings = sum(c.estimated_savings_monthly for c in candidates)
        modernization_score = self._calculate_modernization_score(resources, patterns)
        recommendations = self._generate_recommendations(patterns, candidates, refactoring)
        warnings = self._generate_warnings(resources, candidates)
        
        return ArchitectureAnalysis(
            patterns_detected=patterns,
            migration_candidates=candidates,
            refactoring_opportunities=refactoring,
            total_potential_savings=total_savings,
            modernization_score=modernization_score,
            recommendations=recommendations,
            warnings=warnings,
        )
    
    def _detect_patterns(self, resources: List[ResourceMetrics], dependencies: Dict[str, List[str]]) -> List[ArchitecturePattern]:
        patterns = []
        ec2_count = len([r for r in resources if r.resource_type == ResourceType.EC2])
        has_single_alb = len([r for r in resources if r.resource_type == ResourceType.ALB]) == 1
        has_single_rds = len([r for r in resources if r.resource_type == ResourceType.RDS]) == 1
        
        if ec2_count <= 3 and has_single_alb and has_single_rds:
            patterns.append(ArchitecturePattern(
                pattern_name="Monolithic Architecture",
                description="Single-tier application with centralized database",
                resources_involved=[r.resource_id for r in resources if r.resource_type in [ResourceType.EC2, ResourceType.RDS]],
                confidence=0.8,
                modernization_opportunity=True,
                recommendations=[
                    "Consider decomposing into microservices for independent scaling",
                    "Evaluate container orchestration (ECS/EKS) for deployment flexibility",
                    "Implement event-driven patterns for decoupling",
                ],
            ))
        
        lambdas = [r for r in resources if r.resource_type == ResourceType.LAMBDA]
        if len(lambdas) > 5 and not any(r.resource_type in [ResourceType.SQS, ResourceType.SNS] for r in resources):
            patterns.append(ArchitecturePattern(
                pattern_name="Synchronous API Chain",
                description="Multiple Lambda functions without async messaging",
                resources_involved=[r.resource_id for r in lambdas],
                confidence=0.7,
                modernization_opportunity=True,
                recommendations=[
                    "Introduce SQS for decoupling Lambda invocations",
                    "Consider EventBridge for event-driven orchestration",
                    "Evaluate Step Functions for complex workflows",
                ],
            ))
        
        overprovisioned_dbs = [r.resource_id for r in resources if r.resource_type == ResourceType.RDS and r.avg_cpu_percent < 20 and r.avg_connections < 50]
        if overprovisioned_dbs:
            patterns.append(ArchitecturePattern(
                pattern_name="Over-provisioned Databases",
                description="RDS instances running at low utilization",
                resources_involved=overprovisioned_dbs,
                confidence=0.85,
                modernization_opportunity=True,
                recommendations=[
                    "Consider Aurora Serverless for variable workloads",
                    "Right-size RDS instances based on actual usage",
                    "Evaluate read replicas necessity",
                ],
            ))
        
        return patterns
    
    def _analyze_rds_migration(self, rds: ResourceMetrics) -> Optional[MigrationCandidate]:
        reasons_for = []
        reasons_against = []
        
        if rds.avg_cpu_percent < 30:
            reasons_for.append("Low average CPU utilization")
        else:
            reasons_against.append("Consistent high CPU usage")
        
        connection_variance = (rds.max_connections - rds.avg_connections) / max(rds.avg_connections, 1)
        if connection_variance > 2:
            reasons_for.append("Variable connection patterns")
        
        compatible_engines = ["mysql", "postgresql", "aurora-mysql", "aurora-postgresql"]
        if rds.engine.lower() in compatible_engines:
            reasons_for.append(f"Compatible engine ({rds.engine})")
        else:
            reasons_against.append(f"Engine {rds.engine} not supported by Aurora Serverless")
            return None
        
        confidence = len(reasons_for) / (len(reasons_for) + len(reasons_against) + 1)
        if confidence < 0.5:
            return None
        
        avg_utilization = (rds.avg_cpu_percent + rds.avg_memory_percent) / 2 / 100
        potential_savings_percent = max(0, (1 - avg_utilization) * 50)
        monthly_savings = rds.monthly_cost * (potential_savings_percent / 100)
        
        complexity = "high" if rds.multi_az or rds.read_replicas > 0 else ("low" if "aurora" in rds.engine.lower() else "medium")
        
        return MigrationCandidate(
            resource_id=rds.resource_id,
            resource_type=ResourceType.RDS,
            current_config=f"{rds.engine} - {rds.storage_gb}GB",
            recommended_target=MigrationTarget.AURORA_SERVERLESS,
            confidence=confidence,
            estimated_savings_percent=potential_savings_percent,
            estimated_savings_monthly=monthly_savings,
            migration_complexity=complexity,
            prerequisites=["Verify application compatibility with Aurora", "Test connection handling with serverless scaling", "Plan for maintenance window"],
            benefits=reasons_for + ["Pay-per-use pricing for variable workloads", "Automatic scaling to handle peaks", "Reduced operational overhead"],
            risks=["Cold start latency after idle periods", "Different connection pooling behavior", "Potential compatibility issues with some features"] + reasons_against,
        )
    
    def _analyze_cache_migration(self, cache: ResourceMetrics, all_resources: List[ResourceMetrics]) -> Optional[MigrationCandidate]:
        has_dynamodb = any(r.resource_type == ResourceType.DYNAMODB for r in all_resources)
        if not has_dynamodb:
            return None
        
        reasons_for = []
        if cache.cache_hits_percent > self.DAX_MIN_CACHE_HIT_RATE:
            reasons_for.append(f"Good cache hit rate ({cache.cache_hits_percent:.0f}%)")
        
        if "redis" in cache.tags.get("engine", "").lower():
            reasons_for.append("Redis cluster can potentially migrate to DAX")
        
        confidence = 0.6 if has_dynamodb else 0.3
        if not reasons_for:
            return None
        
        potential_savings_percent = 25
        monthly_savings = cache.monthly_cost * (potential_savings_percent / 100)
        
        return MigrationCandidate(
            resource_id=cache.resource_id,
            resource_type=ResourceType.ELASTICACHE,
            current_config=f"ElastiCache - {cache.tags.get('engine', 'Unknown')}",
            recommended_target=MigrationTarget.DAX,
            confidence=confidence,
            estimated_savings_percent=potential_savings_percent,
            estimated_savings_monthly=monthly_savings,
            migration_complexity="medium",
            prerequisites=["Verify all cached data is from DynamoDB", "Update application to use DAX client", "Test read/write consistency requirements"],
            benefits=reasons_for + ["Native DynamoDB integration", "Automatic cache invalidation", "Microsecond response times"],
            risks=["DAX-specific client required", "Different consistency model", "Limited to DynamoDB use cases"],
        )
    
    def _analyze_ec2_modernization(self, ec2: ResourceMetrics, dependencies: Dict[str, List[str]]) -> Optional[MigrationCandidate]:
        reasons_for = []
        
        if ec2.avg_cpu_percent < 30:
            reasons_for.append("Low CPU utilization suitable for right-sizing")
        
        if "web" in ec2.tags.get("role", "").lower() or "api" in ec2.tags.get("role", "").lower():
            reasons_for.append("Web/API workload suitable for containers")
        
        if ec2.avg_connections > 100:
            reasons_for.append("High connection count indicates service workload")
        
        target = MigrationTarget.EKS if len(dependencies.get(ec2.resource_id, [])) > 3 else MigrationTarget.FARGATE
        complexity = "high" if target == MigrationTarget.EKS else "medium"
        
        confidence = len(reasons_for) / 4
        if confidence < 0.25:
            return None
        
        avg_utilization = ec2.avg_cpu_percent / 100
        potential_savings_percent = max(0, (1 - avg_utilization) * 35)
        monthly_savings = ec2.monthly_cost * (potential_savings_percent / 100)
        
        return MigrationCandidate(
            resource_id=ec2.resource_id,
            resource_type=ResourceType.EC2,
            current_config=f"EC2 - {ec2.tags.get('instance_type', 'Unknown')}",
            recommended_target=target,
            confidence=confidence,
            estimated_savings_percent=potential_savings_percent,
            estimated_savings_monthly=monthly_savings,
            migration_complexity=complexity,
            prerequisites=["Containerize application (Docker)", "Define resource requirements", "Set up container registry (ECR)", "Configure networking (VPC, security groups)"],
            benefits=reasons_for + ["Improved deployment flexibility", "Better resource utilization", "Simplified scaling"],
            risks=["Application may require refactoring", "Learning curve for container orchestration", "Initial migration effort"],
        )
    
    def _find_refactoring_opportunities(self, resources: List[ResourceMetrics], dependencies: Dict[str, List[str]]) -> List[Dict]:
        opportunities = []
        tightly_coupled = [rid for rid, deps in dependencies.items() if len(deps) > 3]
        
        if tightly_coupled:
            opportunities.append({
                "type": "event_driven_decoupling",
                "description": "Introduce event-driven architecture to reduce coupling",
                "affected_resources": tightly_coupled,
                "recommendations": ["Implement EventBridge for service communication", "Use SQS for async processing", "Consider Step Functions for complex workflows"],
                "estimated_effort": "high",
            })
        
        lambdas = [r for r in resources if r.resource_type == ResourceType.LAMBDA]
        if len(lambdas) > 10:
            opportunities.append({
                "type": "api_consolidation",
                "description": "Consolidate Lambda functions behind API Gateway",
                "affected_resources": [r.resource_id for r in lambdas[:10]],
                "recommendations": ["Use API Gateway HTTP APIs for REST endpoints", "Implement Lambda layers for shared code", "Consider Lambda@Edge for edge processing"],
                "estimated_effort": "medium",
            })
        
        return opportunities
    
    def _calculate_modernization_score(self, resources: List[ResourceMetrics], patterns: List[ArchitecturePattern]) -> float:
        score = 50
        modern_types = [ResourceType.LAMBDA, ResourceType.FARGATE, ResourceType.DYNAMODB]
        modern_count = len([r for r in resources if r.resource_type in modern_types])
        score += min(modern_count * 5, 25)
        
        event_types = [ResourceType.SQS, ResourceType.SNS]
        event_count = len([r for r in resources if r.resource_type in event_types])
        score += min(event_count * 3, 15)
        
        modernization_patterns = [p for p in patterns if p.modernization_opportunity]
        score -= len(modernization_patterns) * 5
        
        return max(0, min(100, score))
    
    def _generate_recommendations(self, patterns: List[ArchitecturePattern], candidates: List[MigrationCandidate], refactoring: List[Dict]) -> List[str]:
        recommendations = []
        
        for c in [c for c in candidates if c.confidence > 0.7][:3]:
            recommendations.append(f"Consider migrating {c.resource_id} to {c.recommended_target.value} (estimated ${c.estimated_savings_monthly:.0f}/month savings)")
        
        for pattern in patterns:
            if pattern.modernization_opportunity:
                recommendations.extend(pattern.recommendations[:2])
        
        for opp in refactoring:
            recommendations.extend(opp["recommendations"][:1])
        
        return list(dict.fromkeys(recommendations))[:10]
    
    def _generate_warnings(self, resources: List[ResourceMetrics], candidates: List[MigrationCandidate]) -> List[str]:
        warnings = []
        
        high_complexity = [c for c in candidates if c.migration_complexity == "high"]
        if high_complexity:
            warnings.append(f"{len(high_complexity)} migration candidates have high complexity - recommend thorough testing")
        
        missing_metrics = [r for r in resources if r.avg_cpu_percent == 0]
        if missing_metrics:
            warnings.append(f"{len(missing_metrics)} resources have incomplete metrics - analysis may be limited")
        
        return warnings


__all__ = [
    "ArchitectureAnalyzer", "ResourceType", "MigrationTarget",
    "ResourceMetrics", "ArchitecturePattern", "MigrationCandidate",
    "ArchitectureAnalysis",
]
