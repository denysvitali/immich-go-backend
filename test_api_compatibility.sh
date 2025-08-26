#!/bin/bash

# Test API Compatibility Script for Immich Go Backend
# This script tests basic API endpoints to verify Immich compatibility

API_URL="http://localhost:8080/api"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=========================================="
echo "Immich Go Backend API Compatibility Test"
echo "=========================================="
echo ""

# Function to test endpoint
test_endpoint() {
    local method=$1
    local endpoint=$2
    local description=$3
    
    response=$(curl -s -o /dev/null -w "%{http_code}" -X $method "${API_URL}${endpoint}")
    
    if [ "$response" = "000" ]; then
        echo -e "${RED}✗${NC} $description - Server not responding"
    elif [ "$response" = "200" ] || [ "$response" = "201" ] || [ "$response" = "204" ]; then
        echo -e "${GREEN}✓${NC} $description - Success ($response)"
    elif [ "$response" = "401" ] || [ "$response" = "403" ]; then
        echo -e "${YELLOW}⚠${NC} $description - Auth required ($response)"
    elif [ "$response" = "404" ]; then
        echo -e "${RED}✗${NC} $description - Not found ($response)"
    elif [ "$response" = "405" ]; then
        echo -e "${RED}✗${NC} $description - Method not allowed ($response)"
    else
        echo -e "${YELLOW}?${NC} $description - Response: $response"
    fi
}

echo "Testing Core Endpoints..."
echo "--------------------------"

# Server Info
test_endpoint "GET" "/server/version" "Server Version"
test_endpoint "GET" "/server/info" "Server Info"
test_endpoint "GET" "/server/ping" "Server Ping"
test_endpoint "GET" "/server/features" "Server Features"
test_endpoint "GET" "/server/config" "Server Config"
test_endpoint "GET" "/server/statistics" "Server Statistics"
test_endpoint "GET" "/server/media-types" "Supported Media Types"

echo ""
echo "Testing Auth Endpoints..."
echo "--------------------------"

# Authentication
test_endpoint "POST" "/auth/login" "Login"
test_endpoint "POST" "/auth/logout" "Logout"
test_endpoint "POST" "/auth/signup" "Signup"
test_endpoint "POST" "/auth/validateToken" "Validate Token"
test_endpoint "POST" "/auth/changePassword" "Change Password"

echo ""
echo "Testing User Endpoints..."
echo "--------------------------"

# Users
test_endpoint "GET" "/users" "List Users"
test_endpoint "GET" "/users/me" "Get Current User"
test_endpoint "PUT" "/users" "Update User"

echo ""
echo "Testing Asset Endpoints..."
echo "---------------------------"

# Assets
test_endpoint "POST" "/assets" "Upload Asset"
test_endpoint "GET" "/assets" "List Assets"
test_endpoint "GET" "/assets/statistics" "Asset Statistics"
test_endpoint "GET" "/assets/time-buckets" "Asset Time Buckets"
test_endpoint "GET" "/assets/map-marker" "Asset Map Markers"
test_endpoint "POST" "/assets/jobs" "Run Asset Jobs"

echo ""
echo "Testing Album Endpoints..."
echo "---------------------------"

# Albums
test_endpoint "GET" "/albums" "List Albums"
test_endpoint "POST" "/albums" "Create Album"
test_endpoint "GET" "/albums/count" "Album Count"

echo ""
echo "Testing Search Endpoints..."
echo "----------------------------"

# Search
test_endpoint "GET" "/search" "Search Assets"
test_endpoint "GET" "/search/suggestions" "Search Suggestions"
test_endpoint "GET" "/search/explore" "Search Explore"
test_endpoint "POST" "/search/metadata" "Search Metadata"
test_endpoint "GET" "/search/cities" "Search Cities"
test_endpoint "GET" "/search/places" "Search Places"

echo ""
echo "Testing Library Endpoints..."
echo "-----------------------------"

# Libraries
test_endpoint "GET" "/libraries" "List Libraries"
test_endpoint "POST" "/libraries" "Create Library"
test_endpoint "POST" "/libraries/scan" "Scan Libraries"

echo ""
echo "Testing Sessions Endpoints..."
echo "------------------------------"

# Sessions
test_endpoint "GET" "/sessions" "List Sessions"
test_endpoint "DELETE" "/sessions" "Delete All Sessions"

echo ""
echo "Testing Sync Endpoints..."
echo "--------------------------"

# Sync
test_endpoint "POST" "/sync/delta" "Delta Sync"
test_endpoint "POST" "/sync/full" "Full Sync"
test_endpoint "POST" "/sync/acknowledge" "Sync Acknowledge"

echo ""
echo "Testing Download Endpoints..."
echo "------------------------------"

# Download
test_endpoint "POST" "/download/archive" "Download Archive"
test_endpoint "POST" "/download/info" "Download Info"

echo ""
echo "Testing Job Queue Endpoints..."
echo "-------------------------------"

# Jobs
test_endpoint "GET" "/jobs" "List Jobs"
test_endpoint "PUT" "/jobs" "Update Job"

echo ""
echo "Testing People & Faces..."
echo "--------------------------"

# People
test_endpoint "GET" "/people" "List People"
test_endpoint "POST" "/people" "Create Person"

# Faces
test_endpoint "GET" "/faces" "List Faces"

echo ""
echo "Testing Other Services..."
echo "-------------------------"

# Memory
test_endpoint "GET" "/memories" "List Memories"
test_endpoint "POST" "/memories" "Create Memory"

# Timeline
test_endpoint "GET" "/timeline/buckets" "Timeline Buckets"
test_endpoint "GET" "/timeline/bucket" "Timeline Bucket"

# Map
test_endpoint "GET" "/map/style" "Map Style"
test_endpoint "GET" "/map/markers" "Map Markers"

# Tags
test_endpoint "GET" "/tags" "List Tags"
test_endpoint "POST" "/tags" "Create Tag"

# Trash
test_endpoint "POST" "/trash/empty" "Empty Trash"
test_endpoint "POST" "/trash/restore" "Restore Trash"

# Partners
test_endpoint "GET" "/partners" "List Partners"
test_endpoint "POST" "/partners" "Create Partner"

# Activity
test_endpoint "GET" "/activities" "List Activities"
test_endpoint "POST" "/activities" "Create Activity"

# Stacks
test_endpoint "GET" "/stacks" "List Stacks"
test_endpoint "POST" "/stacks" "Create Stack"

# Duplicates
test_endpoint "GET" "/duplicates" "List Duplicates"

# View
test_endpoint "GET" "/view/folders" "View Folders"
test_endpoint "GET" "/view/folder" "View Folder"

# System Metadata
test_endpoint "GET" "/system-metadata/admin-onboarding" "Admin Onboarding"
test_endpoint "GET" "/system-metadata/config" "System Metadata Config"

# Shared Links
test_endpoint "GET" "/shared-links" "List Shared Links"
test_endpoint "POST" "/shared-links" "Create Shared Link"

# System Config
test_endpoint "GET" "/system-config" "Get System Config"
test_endpoint "PUT" "/system-config" "Update System Config"

# OAuth
test_endpoint "GET" "/oauth/authorize" "OAuth Authorize"
test_endpoint "POST" "/oauth/callback" "OAuth Callback"

# API Keys
test_endpoint "GET" "/api-keys" "List API Keys"
test_endpoint "POST" "/api-keys" "Create API Key"

# Notifications
test_endpoint "GET" "/notifications" "List Notifications"
test_endpoint "PUT" "/notifications" "Update Notification"

echo ""
echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo ""
echo "This test checked basic connectivity to all major API endpoints."
echo "Auth-required endpoints (401/403) are expected and normal."
echo "Any 404 or 405 responses indicate missing or incorrectly configured endpoints."
echo ""
echo "For full Immich compatibility:"
echo "1. All core endpoints should respond (200/201/204 or 401 for auth)"
echo "2. No 404 errors for essential services"
echo "3. WebSocket support at /api/socket.io/"
echo ""