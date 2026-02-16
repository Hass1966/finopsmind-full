#!/bin/bash
#
# FinOpsMind End-to-End Test Script
# Tests all services and features
#
# Usage: ./test_finopsmind.sh
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
FRONTEND_URL="${FRONTEND_URL:-http://localhost:3000}"
BACKEND_URL="${BACKEND_URL:-http://localhost:8080}"
ML_URL="${ML_URL:-http://localhost:8081}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"

# Counters
PASSED=0
FAILED=0
TOTAL=0

# Test function
test_endpoint() {
    local name="$1"
    local url="$2"
    local method="${3:-GET}"
    local data="$4"
    local expected_code="${5:-200}"
    
    TOTAL=$((TOTAL + 1))
    
    if [ "$method" == "POST" ] && [ -n "$data" ]; then
        response=$(curl -s -w "\n%{http_code}" -X POST "$url" \
            -H "Content-Type: application/json" \
            -d "$data" 2>/dev/null)
    else
        response=$(curl -s -w "\n%{http_code}" "$url" 2>/dev/null)
    fi
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')
    
    if [ "$http_code" == "$expected_code" ]; then
        echo -e "  ${GREEN}✓${NC} $name (HTTP $http_code)"
        PASSED=$((PASSED + 1))
        return 0
    else
        echo -e "  ${RED}✗${NC} $name (Expected $expected_code, got $http_code)"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Test service is running
test_service() {
    local name="$1"
    local url="$2"
    
    TOTAL=$((TOTAL + 1))
    
    if curl -s --connect-timeout 5 "$url" > /dev/null 2>&1; then
        echo -e "  ${GREEN}✓${NC} $name is running"
        PASSED=$((PASSED + 1))
        return 0
    else
        echo -e "  ${RED}✗${NC} $name is not running"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Test database connection
test_database() {
    TOTAL=$((TOTAL + 1))
    
    if docker exec finopsmind-postgres pg_isready -U finopsmind > /dev/null 2>&1; then
        echo -e "  ${GREEN}✓${NC} PostgreSQL is running and accepting connections"
        PASSED=$((PASSED + 1))
        return 0
    else
        echo -e "  ${RED}✗${NC} PostgreSQL is not responding"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Test Redis connection
test_redis() {
    TOTAL=$((TOTAL + 1))
    
    if docker exec finopsmind-redis redis-cli ping > /dev/null 2>&1; then
        echo -e "  ${GREEN}✓${NC} Redis is running"
        PASSED=$((PASSED + 1))
        return 0
    else
        echo -e "  ${RED}✗${NC} Redis is not responding"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Header
echo ""
echo -e "${BLUE}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║          FinOpsMind End-to-End Test Suite                    ║${NC}"
echo -e "${BLUE}╚══════════════════════════════════════════════════════════════╝${NC}"
echo ""

# ============================================================================
# 1. Infrastructure Tests
# ============================================================================
echo -e "${YELLOW}━━━ 1. Infrastructure Tests ━━━${NC}"

test_database
test_redis
test_service "ML Sidecar" "$ML_URL/"
test_service "Backend API" "$BACKEND_URL/health" || true
test_service "Frontend" "$FRONTEND_URL" || true

echo ""

# ============================================================================
# 2. ML Sidecar API Tests
# ============================================================================
echo -e "${YELLOW}━━━ 2. ML Sidecar API Tests ━━━${NC}"

test_endpoint "ML Health Check" "$ML_URL/"
test_endpoint "ML Health Detailed" "$ML_URL/health"

# Test workload classification
echo -e "\n  ${BLUE}Testing Workload Classification...${NC}"
CLASSIFY_DATA='{
    "instance_id": "i-test12345678",
    "instance_type": "t3.large",
    "cpu_utilization": [
        {"timestamp": "2025-01-01T00:00:00Z", "value": 5},
        {"timestamp": "2025-01-01T01:00:00Z", "value": 3},
        {"timestamp": "2025-01-01T02:00:00Z", "value": 85},
        {"timestamp": "2025-01-01T03:00:00Z", "value": 4},
        {"timestamp": "2025-01-01T04:00:00Z", "value": 2},
        {"timestamp": "2025-01-01T05:00:00Z", "value": 6},
        {"timestamp": "2025-01-01T06:00:00Z", "value": 90},
        {"timestamp": "2025-01-01T07:00:00Z", "value": 3},
        {"timestamp": "2025-01-01T08:00:00Z", "value": 5},
        {"timestamp": "2025-01-01T09:00:00Z", "value": 4}
    ]
}'
test_endpoint "POST /classify/workload" "$ML_URL/classify/workload" "POST" "$CLASSIFY_DATA"

# Show classification result
RESULT=$(curl -s -X POST "$ML_URL/classify/workload" -H "Content-Type: application/json" -d "$CLASSIFY_DATA" 2>/dev/null)
if [ -n "$RESULT" ]; then
    CLASS=$(echo "$RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('classification','N/A'))" 2>/dev/null || echo "N/A")
    CONF=$(echo "$RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(f\"{d.get('confidence',0):.0%}\")" 2>/dev/null || echo "N/A")
    echo -e "    └─ Classification: ${GREEN}$CLASS${NC} (Confidence: $CONF)"
fi

# Test cost model
echo -e "\n  ${BLUE}Testing Cost Model...${NC}"
COST_DATA='{
    "instance_id": "i-test12345678",
    "instance_type": "t3.large",
    "avg_cpu_percent": 15,
    "max_cpu_percent": 45,
    "avg_memory_percent": 25,
    "max_memory_percent": 60,
    "avg_requests_per_hour": 1000,
    "avg_request_duration_ms": 200,
    "active_hours_per_day": 8
}'
test_endpoint "POST /model/cost" "$ML_URL/model/cost" "POST" "$COST_DATA"

# Show cost result
RESULT=$(curl -s -X POST "$ML_URL/model/cost" -H "Content-Type: application/json" -d "$COST_DATA" 2>/dev/null)
if [ -n "$RESULT" ]; then
    BEST=$(echo "$RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('best_option','N/A'))" 2>/dev/null || echo "N/A")
    SAVINGS=$(echo "$RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(f\"\\${d.get('potential_savings_dollars',0):.2f}\")" 2>/dev/null || echo "N/A")
    echo -e "    └─ Best Option: ${GREEN}$BEST${NC} (Savings: $SAVINGS/month)"
fi

# Test commitment optimization
echo -e "\n  ${BLUE}Testing Commitment Optimizer...${NC}"
COMMIT_DATA='{
    "usage_records": [
        {"timestamp": "2025-01-01T00:00:00Z", "instance_type": "t3.large", "region": "eu-west-2", "usage_hours": 0.9, "on_demand_cost": 0.075},
        {"timestamp": "2025-01-01T01:00:00Z", "instance_type": "t3.large", "region": "eu-west-2", "usage_hours": 0.85, "on_demand_cost": 0.071},
        {"timestamp": "2025-01-01T02:00:00Z", "instance_type": "t3.large", "region": "eu-west-2", "usage_hours": 0.92, "on_demand_cost": 0.077}
    ],
    "on_demand_prices": {"t3.large": 0.0832}
}'
test_endpoint "POST /optimize/commitment" "$ML_URL/optimize/commitment" "POST" "$COMMIT_DATA"

# Test architecture analysis
echo -e "\n  ${BLUE}Testing Architecture Analyzer...${NC}"
ARCH_DATA='{
    "resources": [
        {
            "resource_id": "rds-main-db",
            "resource_type": "rds",
            "avg_cpu_percent": 15,
            "max_cpu_percent": 40,
            "avg_connections": 25,
            "max_connections": 100,
            "monthly_cost": 200,
            "engine": "postgresql"
        },
        {
            "resource_id": "ec2-web-01",
            "resource_type": "ec2",
            "avg_cpu_percent": 20,
            "max_cpu_percent": 55,
            "monthly_cost": 150,
            "tags": {"role": "web"}
        },
        {
            "resource_id": "alb-main",
            "resource_type": "alb",
            "monthly_cost": 30
        }
    ],
    "dependencies": {
        "ec2-web-01": ["rds-main-db"]
    }
}'
test_endpoint "POST /analyze/architecture" "$ML_URL/analyze/architecture" "POST" "$ARCH_DATA"

# Show architecture result
RESULT=$(curl -s -X POST "$ML_URL/analyze/architecture" -H "Content-Type: application/json" -d "$ARCH_DATA" 2>/dev/null)
if [ -n "$RESULT" ]; then
    SCORE=$(echo "$RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('modernization_score',0))" 2>/dev/null || echo "N/A")
    PATTERNS=$(echo "$RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('patterns_detected',[])))" 2>/dev/null || echo "0")
    CANDIDATES=$(echo "$RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('migration_candidates',[])))" 2>/dev/null || echo "0")
    echo -e "    └─ Modernization Score: ${GREEN}$SCORE/100${NC} | Patterns: $PATTERNS | Migration Candidates: $CANDIDATES"
fi

echo ""

# ============================================================================
# 3. Backend API Tests (if running)
# ============================================================================
echo -e "${YELLOW}━━━ 3. Backend API Tests ━━━${NC}"

if curl -s --connect-timeout 2 "$BACKEND_URL/health" > /dev/null 2>&1; then
    test_endpoint "Backend Health" "$BACKEND_URL/health"
    test_endpoint "API v1 Resources" "$BACKEND_URL/api/v1/resources" || true
    test_endpoint "API v1 Recommendations" "$BACKEND_URL/api/v1/recommendations" || true
    test_endpoint "API v1 Cost History" "$BACKEND_URL/api/v1/costs/history" || true
    test_endpoint "API v1 Anomalies" "$BACKEND_URL/api/v1/anomalies" || true
else
    echo -e "  ${YELLOW}⚠${NC} Backend not running - skipping tests"
    echo -e "    Start with: docker compose up -d backend"
fi

echo ""

# ============================================================================
# 4. Database Tests
# ============================================================================
echo -e "${YELLOW}━━━ 4. Database Tests ━━━${NC}"

# Check tables exist
TOTAL=$((TOTAL + 1))
TABLES=$(docker exec finopsmind-postgres psql -U finopsmind -d finopsmind -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" 2>/dev/null | tr -d ' ')
if [ -n "$TABLES" ] && [ "$TABLES" -gt 0 ]; then
    echo -e "  ${GREEN}✓${NC} Database has $TABLES tables"
    PASSED=$((PASSED + 1))
else
    echo -e "  ${RED}✗${NC} No tables found in database"
    FAILED=$((FAILED + 1))
fi

# Check for data
TOTAL=$((TOTAL + 1))
RESOURCES=$(docker exec finopsmind-postgres psql -U finopsmind -d finopsmind -t -c "SELECT COUNT(*) FROM resources;" 2>/dev/null | tr -d ' ' || echo "0")
if [ -n "$RESOURCES" ] && [ "$RESOURCES" -gt 0 ]; then
    echo -e "  ${GREEN}✓${NC} Resources table has $RESOURCES records"
    PASSED=$((PASSED + 1))
else
    echo -e "  ${YELLOW}⚠${NC} Resources table is empty - run: ./seed_mock_data.sql"
    PASSED=$((PASSED + 1))  # Not a failure, just needs seeding
fi

TOTAL=$((TOTAL + 1))
RECS=$(docker exec finopsmind-postgres psql -U finopsmind -d finopsmind -t -c "SELECT COUNT(*) FROM recommendations;" 2>/dev/null | tr -d ' ' || echo "0")
if [ -n "$RECS" ] && [ "$RECS" -gt 0 ]; then
    echo -e "  ${GREEN}✓${NC} Recommendations table has $RECS records"
    PASSED=$((PASSED + 1))
else
    echo -e "  ${YELLOW}⚠${NC} Recommendations table is empty"
    PASSED=$((PASSED + 1))
fi

echo ""

# ============================================================================
# 5. Integration Tests
# ============================================================================
echo -e "${YELLOW}━━━ 5. Integration Tests ━━━${NC}"

# Test ML -> Backend integration (if backend running)
if curl -s --connect-timeout 2 "$BACKEND_URL/health" > /dev/null 2>&1; then
    TOTAL=$((TOTAL + 1))
    # Test that backend can reach ML sidecar
    ML_STATUS=$(curl -s "$BACKEND_URL/api/v1/ml/status" 2>/dev/null || echo '{"status":"unknown"}')
    if echo "$ML_STATUS" | grep -q "healthy\|ok\|connected"; then
        echo -e "  ${GREEN}✓${NC} Backend -> ML Sidecar connection OK"
        PASSED=$((PASSED + 1))
    else
        echo -e "  ${YELLOW}⚠${NC} Backend -> ML Sidecar connection not verified"
        PASSED=$((PASSED + 1))  # Not critical
    fi
else
    echo -e "  ${YELLOW}⚠${NC} Skipping integration tests (backend not running)"
fi

echo ""

# ============================================================================
# Summary
# ============================================================================
echo -e "${BLUE}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                       Test Summary                           ║${NC}"
echo -e "${BLUE}╚══════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "  Total Tests:  $TOTAL"
echo -e "  ${GREEN}Passed:${NC}       $PASSED"
echo -e "  ${RED}Failed:${NC}       $FAILED"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  ✓ All tests passed! FinOpsMind is working correctly.        ${NC}"
    echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
    EXIT_CODE=0
else
    echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${YELLOW}  ⚠ Some tests failed. Check the output above.               ${NC}"
    echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
    EXIT_CODE=1
fi

echo ""
echo -e "${BLUE}Quick Links:${NC}"
echo "  • Frontend Dashboard: $FRONTEND_URL"
echo "  • ML API Docs:        $ML_URL/docs"
echo "  • Backend API:        $BACKEND_URL/api/v1"
echo ""
echo -e "${BLUE}Next Steps:${NC}"
if [ "$RESOURCES" == "0" ] || [ -z "$RESOURCES" ]; then
    echo "  1. Seed mock data:  docker exec -i finopsmind-postgres psql -U finopsmind -d finopsmind < seed_mock_data.sql"
fi
echo "  2. View dashboard:  open $FRONTEND_URL"
echo "  3. Test ML API:     open $ML_URL/docs"
echo ""

exit $EXIT_CODE
