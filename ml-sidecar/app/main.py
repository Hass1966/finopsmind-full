"""
FinOpsMind ML Sidecar - Phase 5 API

FastAPI application providing ML-powered recommendations:
- POST /classify/workload - Classify EC2 workload
- POST /optimize/commitment - Calculate optimal RI/SP mix
- POST /analyze/architecture - Architecture recommendations
- POST /model/cost - What-if cost calculation
"""

from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field
from typing import List, Dict, Optional
from datetime import datetime
from enum import Enum
import logging
import sys
import os

# Add app directory to path for imports
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

# Import our modules
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

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Initialize FastAPI app
app = FastAPI(
    title="FinOpsMind ML Sidecar",
    description="ML-powered FinOps recommendations for AWS workloads",
    version="0.5.0",
    docs_url="/docs",
    redoc_url="/redoc",
)

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


# ============================================================================
# Request/Response Models
# ============================================================================

class MetricPointRequest(BaseModel):
    """Single metric data point."""
    timestamp: datetime
    value: float


class WorkloadClassifyRequest(BaseModel):
    """Request to classify EC2 workload."""
    instance_id: str
    instance_type: str
    cpu_utilization: List[MetricPointRequest]
    memory_utilization: Optional[List[MetricPointRequest]] = None
    network_in: Optional[List[MetricPointRequest]] = None
    network_out: Optional[List[MetricPointRequest]] = None
    disk_read_ops: Optional[List[MetricPointRequest]] = None
    disk_write_ops: Optional[List[MetricPointRequest]] = None
    has_elastic_ip: bool = False
    has_persistent_storage: bool = False
    in_auto_scaling_group: bool = False
    instance_age_days: int = 0


class AlternativeClassificationResponse(BaseModel):
    """Alternative classification with confidence."""
    classification: str
    confidence: float


class WorkloadClassifyResponse(BaseModel):
    """Workload classification response."""
    instance_id: str
    classification: str
    confidence: float
    reasons: List[str]
    metrics_summary: Dict[str, float]
    alternative_classifications: List[AlternativeClassificationResponse]
    estimated_savings_percent: Optional[float]
    recommended_target: Optional[str]
    warnings: List[str]
    pattern_analysis: Optional[Dict] = None


class CostModelRequest(BaseModel):
    """Request for cost modeling."""
    instance_id: str
    instance_type: str
    avg_cpu_percent: float
    max_cpu_percent: float
    avg_memory_percent: float
    max_memory_percent: float
    avg_requests_per_hour: float = 0
    avg_request_duration_ms: float = 0
    active_hours_per_day: float = 24
    region: str = "us-east-1"


class CostModelResponse(BaseModel):
    """Cost modeling response."""
    instance_id: str
    current_monthly_cost: float
    ec2_on_demand: float
    ec2_spot: float
    lambda_cost: Optional[float]
    lambda_viable: bool
    lambda_memory_mb: Optional[int]
    fargate_cost: float
    fargate_spot_cost: float
    fargate_config: str
    best_option: str
    best_option_cost: float
    potential_savings_dollars: float
    potential_savings_percent: float
    recommendations: List[str]
    caveats: List[str]


class UsageRecordRequest(BaseModel):
    """Single usage record for commitment optimization."""
    timestamp: datetime
    instance_type: str
    region: str
    usage_hours: float = Field(ge=0, le=1)
    on_demand_cost: float


class CommitmentOptimizeRequest(BaseModel):
    """Request for commitment optimization."""
    usage_records: List[UsageRecordRequest]
    on_demand_prices: Dict[str, float]


class RIRecommendationResponse(BaseModel):
    """Reserved Instance recommendation."""
    instance_type: str
    region: str
    quantity: int
    term_years: int
    payment_option: str
    monthly_savings: float
    annual_savings: float
    break_even_months: float
    coverage_percent: float
    risk_assessment: str


class SPRecommendationResponse(BaseModel):
    """Savings Plan recommendation."""
    plan_type: str
    hourly_commitment: float
    term_years: int
    monthly_cost: float
    monthly_savings: float
    annual_savings: float
    coverage_percent: float
    flexibility_score: float


class CommitmentOptimizeResponse(BaseModel):
    """Commitment optimization response."""
    current_monthly_cost: float
    optimized_monthly_cost: float
    monthly_savings: float
    savings_percent: float
    ri_recommendations: List[RIRecommendationResponse]
    sp_recommendations: List[SPRecommendationResponse]
    recommended_strategy: str
    on_demand_coverage_percent: float
    committed_coverage_percent: float
    analysis_notes: List[str]
    warnings: List[str]


class ResourceMetricsRequest(BaseModel):
    """Resource metrics for architecture analysis."""
    resource_id: str
    resource_type: str
    avg_cpu_percent: float = 0
    max_cpu_percent: float = 0
    avg_memory_percent: float = 0
    avg_connections: float = 0
    max_connections: float = 0
    avg_iops: float = 0
    max_iops: float = 0
    storage_gb: float = 0
    monthly_cost: float = 0
    engine: str = ""
    multi_az: bool = False
    read_replicas: int = 0
    cache_hits_percent: float = 0
    tags: Dict[str, str] = {}


class ArchitectureAnalyzeRequest(BaseModel):
    """Request for architecture analysis."""
    resources: List[ResourceMetricsRequest]
    dependencies: Optional[Dict[str, List[str]]] = None


class PatternResponse(BaseModel):
    """Architecture pattern response."""
    pattern_name: str
    description: str
    resources_involved: List[str]
    confidence: float
    modernization_opportunity: bool
    recommendations: List[str]


class MigrationCandidateResponse(BaseModel):
    """Migration candidate response."""
    resource_id: str
    resource_type: str
    current_config: str
    recommended_target: str
    confidence: float
    estimated_savings_percent: float
    estimated_savings_monthly: float
    migration_complexity: str
    prerequisites: List[str]
    benefits: List[str]
    risks: List[str]


class ArchitectureAnalyzeResponse(BaseModel):
    """Architecture analysis response."""
    patterns_detected: List[PatternResponse]
    migration_candidates: List[MigrationCandidateResponse]
    refactoring_opportunities: List[Dict]
    total_potential_savings: float
    modernization_score: float
    recommendations: List[str]
    warnings: List[str]


# ============================================================================
# API Endpoints
# ============================================================================

@app.get("/")
async def root():
    """Health check endpoint."""
    return {
        "service": "FinOpsMind ML Sidecar",
        "version": "0.5.0",
        "status": "healthy",
        "endpoints": [
            "/classify/workload",
            "/optimize/commitment",
            "/analyze/architecture",
            "/model/cost",
        ],
    }


@app.get("/health")
async def health():
    """Detailed health check."""
    return {
        "status": "healthy",
        "components": {
            "classifier": "ok",
            "optimizer": "ok",
            "architecture": "ok",
        },
    }


@app.post("/classify/workload", response_model=WorkloadClassifyResponse)
async def classify_workload(request: WorkloadClassifyRequest):
    """
    Classify EC2 workload for potential migration.
    
    Analyzes CloudWatch metrics to determine if workload is suitable for:
    - Lambda (bursty, short-lived, stateless)
    - Fargate (containerizable, predictable)
    - Spot (fault tolerant, flexible timing)
    - Keep on EC2 (none of the above)
    """
    try:
        # Convert request to CloudWatchMetrics
        metrics = CloudWatchMetrics(
            instance_id=request.instance_id,
            instance_type=request.instance_type,
            cpu_utilization=[
                MetricDataPoint(timestamp=p.timestamp, value=p.value)
                for p in request.cpu_utilization
            ],
            memory_utilization=[
                MetricDataPoint(timestamp=p.timestamp, value=p.value)
                for p in (request.memory_utilization or [])
            ],
            network_in=[
                MetricDataPoint(timestamp=p.timestamp, value=p.value)
                for p in (request.network_in or [])
            ],
            network_out=[
                MetricDataPoint(timestamp=p.timestamp, value=p.value)
                for p in (request.network_out or [])
            ],
            disk_read_ops=[
                MetricDataPoint(timestamp=p.timestamp, value=p.value)
                for p in (request.disk_read_ops or [])
            ],
            disk_write_ops=[
                MetricDataPoint(timestamp=p.timestamp, value=p.value)
                for p in (request.disk_write_ops or [])
            ],
            has_elastic_ip=request.has_elastic_ip,
            has_persistent_storage=request.has_persistent_storage,
            in_auto_scaling_group=request.in_auto_scaling_group,
            instance_age_days=request.instance_age_days,
        )
        
        # Classify workload
        classifier = WorkloadClassifier()
        result = classifier.classify(metrics)
        
        # Pattern analysis
        pattern_analysis = None
        if request.cpu_utilization:
            detector = PatternDetector()
            pattern_result = detector.analyze(
                request.instance_id,
                [MetricPoint(timestamp=p.timestamp, value=p.value) for p in request.cpu_utilization],
                [MetricPoint(timestamp=p.timestamp, value=p.value) for p in (request.memory_utilization or [])],
            )
            pattern_analysis = {
                "primary_pattern": pattern_result.primary_pattern.value,
                "idle_percent": pattern_result.idle.idle_percent,
                "is_bursty": pattern_result.burst.is_bursty,
                "burst_count": pattern_result.burst.burst_count,
                "has_diurnal_pattern": pattern_result.diurnal.has_diurnal_pattern,
                "has_weekly_pattern": pattern_result.weekly.has_weekly_pattern,
                "trend_direction": pattern_result.trend.direction,
            }
        
        return WorkloadClassifyResponse(
            instance_id=result.instance_id,
            classification=result.classification.value,
            confidence=result.confidence,
            reasons=result.reasons,
            metrics_summary=result.metrics_summary,
            alternative_classifications=[
                AlternativeClassificationResponse(classification=c[0].value, confidence=c[1])
                for c in result.alternative_classifications
            ],
            estimated_savings_percent=result.estimated_savings_percent,
            recommended_target=result.recommended_target,
            warnings=result.warnings,
            pattern_analysis=pattern_analysis,
        )
        
    except Exception as e:
        logger.error(f"Workload classification error: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/model/cost", response_model=CostModelResponse)
async def model_cost(request: CostModelRequest):
    """
    Calculate what-if costs for workload migration.
    
    Compares current EC2 cost against:
    - Lambda (if viable)
    - Fargate
    - Spot Instances
    """
    try:
        # Create workload profile
        profile = WorkloadProfile(
            instance_id=request.instance_id,
            instance_type=request.instance_type,
            avg_cpu_percent=request.avg_cpu_percent,
            max_cpu_percent=request.max_cpu_percent,
            avg_memory_percent=request.avg_memory_percent,
            max_memory_percent=request.max_memory_percent,
            avg_requests_per_hour=request.avg_requests_per_hour,
            avg_request_duration_ms=request.avg_request_duration_ms,
            active_hours_per_day=request.active_hours_per_day,
        )
        
        # Run cost comparison
        modeler = CostModeler(region=request.region)
        comparison = modeler.compare_costs(profile)
        
        return CostModelResponse(
            instance_id=comparison.instance_id,
            current_monthly_cost=comparison.current_ec2.monthly_on_demand,
            ec2_on_demand=comparison.current_ec2.monthly_on_demand,
            ec2_spot=comparison.current_ec2.monthly_spot,
            lambda_cost=comparison.lambda_estimate.monthly_cost_after_free_tier if comparison.lambda_estimate.viable else None,
            lambda_viable=comparison.lambda_estimate.viable,
            lambda_memory_mb=comparison.lambda_estimate.memory_mb if comparison.lambda_estimate.viable else None,
            fargate_cost=comparison.fargate_estimate.monthly_cost,
            fargate_spot_cost=comparison.fargate_estimate.monthly_cost_spot,
            fargate_config=f"{comparison.fargate_estimate.vcpu}vCPU / {comparison.fargate_estimate.memory_gb}GB",
            best_option=comparison.best_option,
            best_option_cost=comparison.best_option_monthly_cost,
            potential_savings_dollars=comparison.potential_savings_dollars,
            potential_savings_percent=comparison.potential_savings_percent,
            recommendations=comparison.recommendations,
            caveats=comparison.caveats,
        )
        
    except Exception as e:
        logger.error(f"Cost modeling error: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/optimize/commitment", response_model=CommitmentOptimizeResponse)
async def optimize_commitment(request: CommitmentOptimizeRequest):
    """
    Calculate optimal Reserved Instance and Savings Plan mix.
    
    Analyzes usage patterns to recommend:
    - RI coverage for stable workloads
    - Savings Plan for flexible coverage
    - On-demand buffer for variable needs
    """
    try:
        # Convert request to UsageRecords
        records = [
            UsageRecord(
                timestamp=r.timestamp,
                instance_type=r.instance_type,
                region=r.region,
                usage_hours=r.usage_hours,
                on_demand_cost=r.on_demand_cost,
            )
            for r in request.usage_records
        ]
        
        # Run optimization
        optimizer = CommitmentOptimizer()
        result = optimizer.optimize(records, request.on_demand_prices)
        
        return CommitmentOptimizeResponse(
            current_monthly_cost=result.current_monthly_cost,
            optimized_monthly_cost=result.optimized_monthly_cost,
            monthly_savings=result.monthly_savings,
            savings_percent=result.savings_percent,
            ri_recommendations=[
                RIRecommendationResponse(
                    instance_type=ri.instance_type,
                    region=ri.region,
                    quantity=ri.quantity,
                    term_years=ri.term_years,
                    payment_option=ri.payment_option,
                    monthly_savings=ri.monthly_savings,
                    annual_savings=ri.annual_savings,
                    break_even_months=ri.break_even_months,
                    coverage_percent=ri.coverage_percent,
                    risk_assessment=ri.risk_assessment,
                )
                for ri in result.ri_recommendations
            ],
            sp_recommendations=[
                SPRecommendationResponse(
                    plan_type=sp.plan_type,
                    hourly_commitment=sp.hourly_commitment,
                    term_years=sp.term_years,
                    monthly_cost=sp.monthly_cost,
                    monthly_savings=sp.monthly_savings,
                    annual_savings=sp.annual_savings,
                    coverage_percent=sp.coverage_percent,
                    flexibility_score=sp.flexibility_score,
                )
                for sp in result.sp_recommendations
            ],
            recommended_strategy=result.recommended_strategy,
            on_demand_coverage_percent=result.on_demand_coverage_percent,
            committed_coverage_percent=result.committed_coverage_percent,
            analysis_notes=result.analysis_notes,
            warnings=result.warnings,
        )
        
    except Exception as e:
        logger.error(f"Commitment optimization error: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/analyze/architecture", response_model=ArchitectureAnalyzeResponse)
async def analyze_architecture(request: ArchitectureAnalyzeRequest):
    """
    Analyze architecture for migration recommendations.
    
    Detects patterns and recommends:
    - RDS → Aurora Serverless migration
    - ElastiCache → DAX migration
    - EC2 → Container migration
    - Event-driven refactoring opportunities
    """
    try:
        # Convert resource types
        type_mapping = {
            "ec2": ResourceType.EC2,
            "rds": ResourceType.RDS,
            "elasticache": ResourceType.ELASTICACHE,
            "lambda": ResourceType.LAMBDA,
            "ecs": ResourceType.ECS,
            "eks": ResourceType.EKS,
            "s3": ResourceType.S3,
            "dynamodb": ResourceType.DYNAMODB,
            "sqs": ResourceType.SQS,
            "sns": ResourceType.SNS,
            "api_gateway": ResourceType.API_GATEWAY,
            "alb": ResourceType.ALB,
            "cloudfront": ResourceType.CLOUDFRONT,
        }
        
        # Convert request to ResourceMetrics
        resources = []
        for r in request.resources:
            resource_type = type_mapping.get(r.resource_type.lower())
            if not resource_type:
                continue
                
            resources.append(ResourceMetrics(
                resource_id=r.resource_id,
                resource_type=resource_type,
                avg_cpu_percent=r.avg_cpu_percent,
                max_cpu_percent=r.max_cpu_percent,
                avg_memory_percent=r.avg_memory_percent,
                avg_connections=r.avg_connections,
                max_connections=r.max_connections,
                avg_iops=r.avg_iops,
                max_iops=r.max_iops,
                storage_gb=r.storage_gb,
                monthly_cost=r.monthly_cost,
                engine=r.engine,
                multi_az=r.multi_az,
                read_replicas=r.read_replicas,
                cache_hits_percent=r.cache_hits_percent,
                tags=r.tags,
            ))
        
        # Run analysis
        analyzer = ArchitectureAnalyzer()
        result = analyzer.analyze(resources, request.dependencies)
        
        return ArchitectureAnalyzeResponse(
            patterns_detected=[
                PatternResponse(
                    pattern_name=p.pattern_name,
                    description=p.description,
                    resources_involved=p.resources_involved,
                    confidence=p.confidence,
                    modernization_opportunity=p.modernization_opportunity,
                    recommendations=p.recommendations,
                )
                for p in result.patterns_detected
            ],
            migration_candidates=[
                MigrationCandidateResponse(
                    resource_id=c.resource_id,
                    resource_type=c.resource_type.value,
                    current_config=c.current_config,
                    recommended_target=c.recommended_target.value,
                    confidence=c.confidence,
                    estimated_savings_percent=c.estimated_savings_percent,
                    estimated_savings_monthly=c.estimated_savings_monthly,
                    migration_complexity=c.migration_complexity,
                    prerequisites=c.prerequisites,
                    benefits=c.benefits,
                    risks=c.risks,
                )
                for c in result.migration_candidates
            ],
            refactoring_opportunities=result.refactoring_opportunities,
            total_potential_savings=result.total_potential_savings,
            modernization_score=result.modernization_score,
            recommendations=result.recommendations,
            warnings=result.warnings,
        )
        
    except Exception as e:
        logger.error(f"Architecture analysis error: {e}")
        raise HTTPException(status_code=500, detail=str(e))


# ============================================================================
# Run with uvicorn
# ============================================================================

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8081)
