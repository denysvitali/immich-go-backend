#!/bin/bash

# Simple script to test Docker build locally
# This is for development/testing purposes only

set -euo pipefail

echo "🐳 Testing Docker build for immich-go-backend"
echo "=============================================="

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed or not in PATH"
    exit 1
fi

# Check if Docker daemon is running
if ! docker info &> /dev/null; then
    echo "❌ Docker daemon is not running"
    exit 1
fi

echo "✅ Docker is available and running"

# Build the Docker image
echo "🔨 Building Docker image..."
docker build -t immich-go-backend:test .

echo "✅ Docker build completed successfully!"

# Optional: Test the image
echo "🧪 Testing the built image..."
docker run --rm immich-go-backend:test --help || echo "⚠️  Application help command failed (this might be expected)"

echo "🎉 Docker build test completed!"