#!/bin/bash

# Immich API Compatibility Test Script
# This script tests the immich-go-backend for compatibility with Immich clients

set -e

# Configuration
API_URL="${API_URL:-http://localhost:8080/api}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin123}"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test results
PASSED=0
FAILED=0
WARNINGS=0

# Helper functions
test_endpoint() {
    local method=$1
    local endpoint=$2
    local data=$3
    local expected_status=$4
    local description=$5
    local auth_header=$6
    
    echo -n "Testing $method $endpoint - $description... "
    
    if [ -n "$auth_header" ]; then
        if [ -n "$data" ]; then
            response=$(curl -s -w "\n%{http_code}" -X $method "$API_URL$endpoint" \
                -H "Content-Type: application/json" \
                -H "$auth_header" \
                -d "$data" 2>/dev/null || true)
        else
            response=$(curl -s -w "\n%{http_code}" -X $method "$API_URL$endpoint" \
                -H "$auth_header" 2>/dev/null || true)
        fi
    else
        if [ -n "$data" ]; then
            response=$(curl -s -w "\n%{http_code}" -X $method "$API_URL$endpoint" \
                -H "Content-Type: application/json" \
                -d "$data" 2>/dev/null || true)
        else
            response=$(curl -s -w "\n%{http_code}" -X $method "$API_URL$endpoint" 2>/dev/null || true)
        fi
    fi
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n-1)
    
    if [ "$http_code" = "$expected_status" ]; then
        echo -e "${GREEN}PASS${NC} (HTTP $http_code)"
        ((PASSED++))
        return 0
    elif [ "$http_code" = "000" ]; then
        echo -e "${RED}FAIL${NC} (Connection refused - is the server running?)"
        ((FAILED++))
        return 1
    else
        echo -e "${RED}FAIL${NC} (Expected HTTP $expected_status, got $http_code)"
        if [ -n "$body" ]; then
            echo "  Response: $body"
        fi
        ((FAILED++))
        return 1
    fi
}

test_endpoint_warn() {
    local method=$1
    local endpoint=$2
    local data=$3
    local expected_status=$4
    local description=$5
    local auth_header=$6
    
    echo -n "Testing $method $endpoint - $description... "
    
    if [ -n "$auth_header" ]; then
        if [ -n "$data" ]; then
            response=$(curl -s -w "\n%{http_code}" -X $method "$API_URL$endpoint" \
                -H "Content-Type: application/json" \
                -H "$auth_header" \
                -d "$data" 2>/dev/null || true)
        else
            response=$(curl -s -w "\n%{http_code}" -X $method "$API_URL$endpoint" \
                -H "$auth_header" 2>/dev/null || true)
        fi
    else
        if [ -n "$data" ]; then
            response=$(curl -s -w "\n%{http_code}" -X $method "$API_URL$endpoint" \
                -H "Content-Type: application/json" \
                -d "$data" 2>/dev/null || true)
        else
            response=$(curl -s -w "\n%{http_code}" -X $method "$API_URL$endpoint" 2>/dev/null || true)
        fi
    fi
    
    http_code=$(echo "$response" | tail -n1)
    
    if [ "$http_code" = "$expected_status" ]; then
        echo -e "${GREEN}PASS${NC} (HTTP $http_code)"
        ((PASSED++))
        return 0
    else
        echo -e "${YELLOW}WARN${NC} (Expected HTTP $expected_status, got $http_code - non-critical)"
        ((WARNINGS++))
        return 1
    fi
}

echo "=========================================="
echo "Immich API Compatibility Test"
echo "Testing against: $API_URL"
echo "=========================================="
echo

# Test server connectivity
echo "1. Testing Server Connectivity"
echo "------------------------------"
test_endpoint "GET" "/server/ping" "" "200" "Server ping"
test_endpoint "GET" "/server/info" "" "200" "Server info"
test_endpoint "GET" "/server/version" "" "200" "Server version"
test_endpoint "GET" "/server/supported-media-types" "" "200" "Supported media types"
test_endpoint "GET" "/server/statistics" "" "200" "Server statistics"
echo

# Test authentication endpoints
echo "2. Testing Authentication"
echo "-------------------------"

# First, try to create admin user
echo "Creating admin user..."
admin_signup_data='{"email":"'$ADMIN_EMAIL'","password":"'$ADMIN_PASSWORD'","name":"Admin User"}'
if test_endpoint "POST" "/auth/signup-admin" "$admin_signup_data" "200" "Admin signup" 2>/dev/null; then
    echo "Admin user created successfully"
else
    echo "Admin user might already exist, continuing..."
fi

# Login
echo -n "Testing login... "
login_response=$(curl -s -X POST "$API_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"email":"'$ADMIN_EMAIL'","password":"'$ADMIN_PASSWORD'"}' 2>/dev/null || true)

if echo "$login_response" | grep -q "token"; then
    echo -e "${GREEN}PASS${NC}"
    ((PASSED++))
    # Extract token from response
    AUTH_TOKEN=$(echo "$login_response" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
    AUTH_HEADER="Authorization: Bearer $AUTH_TOKEN"
else
    echo -e "${RED}FAIL${NC} - Could not login"
    echo "Response: $login_response"
    ((FAILED++))
    AUTH_HEADER=""
fi

if [ -n "$AUTH_HEADER" ]; then
    test_endpoint "GET" "/auth/validate-token" "" "200" "Validate token" "$AUTH_HEADER"
fi
echo

# Test user endpoints
echo "3. Testing User Endpoints"
echo "-------------------------"
if [ -n "$AUTH_HEADER" ]; then
    test_endpoint "GET" "/users/me" "" "200" "Get current user" "$AUTH_HEADER"
    test_endpoint "GET" "/users" "" "200" "List users" "$AUTH_HEADER"
fi
echo

# Test asset endpoints
echo "4. Testing Asset Endpoints"
echo "--------------------------"
if [ -n "$AUTH_HEADER" ]; then
    test_endpoint "GET" "/assets" "" "200" "List assets" "$AUTH_HEADER"
    test_endpoint "GET" "/assets/statistics" "" "200" "Asset statistics" "$AUTH_HEADER"
    # Note: We can't test asset upload without actual file handling
    echo "Note: Asset upload/download tests require actual files"
fi
echo

# Test album endpoints
echo "5. Testing Album Endpoints"
echo "--------------------------"
if [ -n "$AUTH_HEADER" ]; then
    test_endpoint "GET" "/albums" "" "200" "List albums" "$AUTH_HEADER"
    
    # Try to create an album
    album_data='{"name":"Test Album","description":"Test album from API test"}'
    echo -n "Creating test album... "
    album_response=$(curl -s -X POST "$API_URL/albums" \
        -H "Content-Type: application/json" \
        -H "$AUTH_HEADER" \
        -d "$album_data" 2>/dev/null || true)
    
    if echo "$album_response" | grep -q "id"; then
        echo -e "${GREEN}PASS${NC}"
        ((PASSED++))
        ALBUM_ID=$(echo "$album_response" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
        
        if [ -n "$ALBUM_ID" ]; then
            test_endpoint "GET" "/albums/$ALBUM_ID" "" "200" "Get album details" "$AUTH_HEADER"
            test_endpoint "DELETE" "/albums/$ALBUM_ID" "" "200" "Delete album" "$AUTH_HEADER"
        fi
    else
        echo -e "${RED}FAIL${NC}"
        ((FAILED++))
    fi
fi
echo

# Test other endpoints (warning only)
echo "6. Testing Additional Endpoints (Non-Critical)"
echo "----------------------------------------------"
if [ -n "$AUTH_HEADER" ]; then
    test_endpoint_warn "GET" "/shared-links" "" "200" "List shared links" "$AUTH_HEADER"
    test_endpoint_warn "GET" "/search/suggestions" "" "200" "Search suggestions" "$AUTH_HEADER"
    test_endpoint_warn "GET" "/libraries" "" "200" "List libraries" "$AUTH_HEADER"
    test_endpoint_warn "GET" "/timeline" "" "200" "Timeline" "$AUTH_HEADER"
    test_endpoint_warn "GET" "/memories" "" "200" "Memories" "$AUTH_HEADER"
    test_endpoint_warn "GET" "/people" "" "200" "People" "$AUTH_HEADER"
    test_endpoint_warn "GET" "/faces" "" "200" "Faces" "$AUTH_HEADER"
    test_endpoint_warn "GET" "/tags" "" "200" "Tags" "$AUTH_HEADER"
    test_endpoint_warn "GET" "/map/markers" "" "200" "Map markers" "$AUTH_HEADER"
    test_endpoint_warn "GET" "/activities" "" "200" "Activities" "$AUTH_HEADER"
    test_endpoint_warn "GET" "/partners" "" "200" "Partners" "$AUTH_HEADER"
    test_endpoint_warn "GET" "/notifications" "" "200" "Notifications" "$AUTH_HEADER"
fi
echo

# Test logout
if [ -n "$AUTH_HEADER" ]; then
    test_endpoint "POST" "/auth/logout" "" "200" "Logout" "$AUTH_HEADER"
fi

# Summary
echo
echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"
echo -e "Warnings: ${YELLOW}$WARNINGS${NC}"
echo

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All critical tests passed!${NC}"
    echo "The backend appears to be compatible with Immich clients."
    exit 0
else
    echo -e "${RED}Some critical tests failed.${NC}"
    echo "Please check the failed endpoints and ensure the server is properly configured."
    exit 1
fi