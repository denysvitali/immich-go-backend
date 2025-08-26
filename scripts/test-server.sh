#!/bin/bash
# Test script for Immich Go Backend

echo "Immich Go Backend Test Script"
echo "=============================="

# Check if PostgreSQL is running
nc -zv localhost 5432 &>/dev/null
if [ $? -ne 0 ]; then
    echo "❌ PostgreSQL is not running on localhost:5432"
    echo "   Please start PostgreSQL first:"
    echo "   docker-compose up -d postgres redis"
    exit 1
else
    echo "✅ PostgreSQL is running"
fi

# Check if Redis is running
nc -zv localhost 6379 &>/dev/null
if [ $? -ne 0 ]; then
    echo "⚠️  Redis is not running on localhost:6379"
    echo "   Some features may not work without Redis"
else
    echo "✅ Redis is running"
fi

# Build the binary
echo ""
echo "Building binary..."
go build -o bin/immich-go-backend ./cmd
if [ $? -eq 0 ]; then
    echo "✅ Build successful"
else
    echo "❌ Build failed"
    exit 1
fi

# Run migrations
echo ""
echo "Running database migrations..."
./bin/immich-go-backend migrate
if [ $? -eq 0 ]; then
    echo "✅ Migrations successful"
else
    echo "❌ Migrations failed"
    exit 1
fi

# Start the server in the background
echo ""
echo "Starting server..."
./bin/immich-go-backend serve &
SERVER_PID=$!

# Wait for server to start
sleep 3

# Test endpoints
echo ""
echo "Testing API endpoints:"
echo "----------------------"

# Test ping endpoint
response=$(curl -s -w "\n%{http_code}" http://localhost:3001/api/server/ping)
http_code=$(echo "$response" | tail -n1)
if [ "$http_code" = "200" ]; then
    echo "✅ /api/server/ping - OK"
else
    echo "❌ /api/server/ping - Failed (HTTP $http_code)"
fi

# Test version endpoint
response=$(curl -s -w "\n%{http_code}" http://localhost:3001/api/server/version)
http_code=$(echo "$response" | tail -n1)
if [ "$http_code" = "200" ]; then
    echo "✅ /api/server/version - OK"
else
    echo "❌ /api/server/version - Failed (HTTP $http_code)"
fi

# Test auth status endpoint
response=$(curl -s -w "\n%{http_code}" http://localhost:3001/api/auth/status)
http_code=$(echo "$response" | tail -n1)
if [ "$http_code" = "401" ] || [ "$http_code" = "200" ]; then
    echo "✅ /api/auth/status - OK"
else
    echo "❌ /api/auth/status - Failed (HTTP $http_code)"
fi

# Cleanup
echo ""
echo "Shutting down server..."
kill $SERVER_PID 2>/dev/null

echo ""
echo "Test complete!"
echo ""
echo "To test with Immich clients:"
echo "1. Configure the Immich mobile app to use http://your-ip:3001 as the server URL"
echo "2. Try to create an admin account and log in"
echo "3. Upload test photos and verify they appear"