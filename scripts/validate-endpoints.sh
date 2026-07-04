#!/bin/bash
# Endpoint Validation Script
# Tests all public and health endpoints to verify routing and security

set -e

BASE_URL="${1:-http://localhost:80}"
FAILED=0
PASSED=0

echo "======================================"
echo "TenzoShare Endpoint Validation"
echo "Base URL: $BASE_URL"
echo "======================================"
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

test_endpoint() {
    local method="$1"
    local path="$2"
    local expected_status="$3"
    local description="$4"
    
    echo -n "Testing: $description ... "
    
    response=$(curl -s -w "\n%{http_code}" -X "$method" "$BASE_URL$path" 2>&1 || echo "000")
    status_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n-1)
    
    if [ "$status_code" = "$expected_status" ]; then
        echo -e "${GREEN}✓ PASS${NC} (HTTP $status_code)"
        ((PASSED++))
    else
        echo -e "${RED}✗ FAIL${NC} (Expected HTTP $expected_status, got $status_code)"
        echo "  Response: $body"
        ((FAILED++))
    fi
}

test_json_response() {
    local method="$1"
    local path="$2"
    local expected_field="$3"
    local description="$4"
    
    echo -n "Testing: $description ... "
    
    response=$(curl -s -X "$method" "$BASE_URL$path" 2>&1)
    
    if echo "$response" | jq -e "$expected_field" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ PASS${NC} (Valid JSON with $expected_field)"
        ((PASSED++))
    else
        echo -e "${RED}✗ FAIL${NC} (Missing expected field: $expected_field)"
        echo "  Response: $response"
        ((FAILED++))
    fi
}

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "1. PUBLIC ENDPOINTS (No Auth Required)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Admin service public endpoints
test_json_response "GET" "/api/v1/platform/config" ".link_protection_policy" "Platform Config"
test_json_response "GET" "/api/v1/branding" ".primary_color" "Branding Config"

# Health checks (should return 200 with JSON)
test_json_response "GET" "/api/v1/auth/health" ".status" "Auth Health"
test_json_response "GET" "/api/v1/transfers/health" ".status" "Transfer Health"
test_json_response "GET" "/api/v1/files/health" ".status" "Storage Health"
test_json_response "GET" "/api/v1/uploads/health" ".status" "Upload Health"
test_json_response "GET" "/api/v1/notification/health" ".status" "Notification Health"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "2. PROTECTED ENDPOINTS (Should Reject)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# These should return 401 without auth
test_endpoint "GET" "/api/v1/users/me" "401" "GET /api/v1/users/me (no auth)"
test_endpoint "GET" "/api/v1/transfers" "401" "GET /api/v1/transfers (no auth)"
test_endpoint "GET" "/api/v1/files" "401" "GET /api/v1/files (no auth)"
test_endpoint "GET" "/api/v1/admin/users" "401" "GET /api/v1/admin/users (no auth)"
test_endpoint "GET" "/api/v1/admin/stats" "401" "GET /api/v1/admin/stats (no auth)"
test_endpoint "GET" "/api/v1/audit/stats" "401" "GET /api/v1/audit/stats (no auth)"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "3. PUBLIC API ENDPOINTS (Bad Request)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# These should return 400 (bad request) or 422 (validation) when missing data
test_endpoint "POST" "/api/v1/auth/register" "400" "POST /api/v1/auth/register (no data)"
test_endpoint "POST" "/api/v1/auth/login" "400" "POST /api/v1/auth/login (no data)"
test_endpoint "POST" "/api/v1/auth/password-reset/request" "400" "POST /api/v1/auth/password-reset/request (no data)"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "4. ROUTING CHECKS"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Test that different path prefixes route correctly
echo -n "Testing: Auth service routing ... "
auth_response=$(curl -s "$BASE_URL/api/v1/auth/health")
if echo "$auth_response" | jq -e '.service == "auth"' > /dev/null 2>&1; then
    echo -e "${GREEN}✓ PASS${NC}"
    ((PASSED++))
else
    echo -e "${RED}✗ FAIL${NC}"
    ((FAILED++))
fi

echo -n "Testing: Transfer service routing ... "
transfer_response=$(curl -s "$BASE_URL/api/v1/transfers/health")
if echo "$transfer_response" | jq -e '.service == "transfer"' > /dev/null 2>&1; then
    echo -e "${GREEN}✓ PASS${NC}"
    ((PASSED++))
else
    echo -e "${RED}✗ FAIL${NC}"
    ((FAILED++))
fi

echo -n "Testing: Storage service routing ... "
storage_response=$(curl -s "$BASE_URL/api/v1/files/health")
if echo "$storage_response" | jq -e '.service == "storage"' > /dev/null 2>&1; then
    echo -e "${GREEN}✓ PASS${NC}"
    ((PASSED++))
else
    echo -e "${RED}✗ FAIL${NC}"
    ((FAILED++))
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "SUMMARY"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed. Review the output above.${NC}"
    exit 1
fi
