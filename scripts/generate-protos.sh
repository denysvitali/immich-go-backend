#!/usr/bin/env bash

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo -e "${BLUE}🔧 Generating Protocol Buffers for Immich Go Backend${NC}"
echo "=================================================="
echo ""

# Change to project root
cd "$PROJECT_ROOT"

# Check if we're in a Nix environment
if [[ -z "${IN_NIX_SHELL:-}" ]]; then
    echo -e "${YELLOW}⚠️  Not in Nix shell. Entering development environment...${NC}"
    
    # Check if flake.nix exists (preferred) or fall back to shell.nix
    if [[ -f "flake.nix" ]]; then
        echo -e "${BLUE}📦 Using Nix flake environment...${NC}"
        exec nix develop --command bash "$0" "$@"
    elif [[ -f "shell.nix" ]]; then
        echo -e "${BLUE}📦 Using Nix shell environment...${NC}"
        exec nix-shell --command "bash $0 $@"
    else
        echo -e "${RED}❌ No Nix configuration found (flake.nix or shell.nix)${NC}"
        exit 1
    fi
fi

echo -e "${GREEN}✅ Running in Nix development environment${NC}"
echo ""

# Verify required tools are available
echo -e "${BLUE}🔍 Verifying required tools...${NC}"

check_tool() {
    local tool=$1
    local cmd=${2:-$tool}
    
    if command -v "$cmd" &> /dev/null; then
        echo -e "   ✅ $tool: $(command -v "$cmd")"
    else
        echo -e "   ${RED}❌ $tool: not found${NC}"
        return 1
    fi
}

check_tool "protoc"
check_tool "protoc-gen-go"
check_tool "protoc-gen-go-grpc"
check_tool "buf"
check_tool "go"

echo ""

# Clean previous generated files
echo -e "${BLUE}🧹 Cleaning previous generated files...${NC}"
if [[ -d "src/proto/generated" ]]; then
    rm -rf src/proto/generated
    echo "   Removed src/proto/generated"
fi

# Create output directory
mkdir -p src/proto/generated
echo "   Created src/proto/generated"
echo ""

# Generate protobuf files using buf
echo -e "${BLUE}🔨 Generating protobuf files with buf...${NC}"

# Check if buf.gen.yaml exists
if [[ ! -f "buf.gen.yaml" ]]; then
    echo -e "${RED}❌ buf.gen.yaml not found${NC}"
    exit 1
fi

# Check if buf.yaml exists
if [[ ! -f "buf.yaml" ]]; then
    echo -e "${RED}❌ buf.yaml not found${NC}"
    exit 1
fi

# Run buf generate
echo "   Running: buf generate"
if buf generate; then
    echo -e "${GREEN}   ✅ Protocol buffer generation completed successfully${NC}"
else
    echo -e "${RED}   ❌ Protocol buffer generation failed${NC}"
    exit 1
fi

echo ""

# Verify generated files
echo -e "${BLUE}🔍 Verifying generated files...${NC}"

if [[ -d "src/proto/generated" ]]; then
    generated_files=$(find src/proto/generated -name "*.go" | wc -l)
    if [[ $generated_files -gt 0 ]]; then
        echo -e "${GREEN}   ✅ Generated $generated_files Go files${NC}"
        echo "   Generated files:"
        find src/proto/generated -name "*.go" -type f | sort | sed 's/^/      /'
    else
        echo -e "${YELLOW}   ⚠️  No Go files generated${NC}"
    fi
else
    echo -e "${RED}   ❌ Generated directory not found${NC}"
    exit 1
fi

echo ""

# Run go mod tidy to ensure dependencies are up to date
echo -e "${BLUE}📦 Updating Go module dependencies...${NC}"
if go mod tidy; then
    echo -e "${GREEN}   ✅ Go module dependencies updated${NC}"
else
    echo -e "${YELLOW}   ⚠️  Failed to update Go dependencies (this may be normal)${NC}"
fi

echo ""

# Final verification - check if files compile
echo -e "${BLUE}🔍 Verifying generated code compiles...${NC}"
if go build -o /tmp/immich-go-backend-test ./...; then
    echo -e "${GREEN}   ✅ Generated code compiles successfully${NC}"
    rm -f /tmp/immich-go-backend-test
else
    echo -e "${YELLOW}   ⚠️  Generated code compilation check failed (this may be normal if main.go has missing dependencies)${NC}"
fi

echo ""
echo -e "${GREEN}🎉 Protocol buffer generation completed!${NC}"
echo "================================================"
echo ""
echo -e "${BLUE}📁 Generated files location:${NC} src/proto/generated/"
echo -e "${BLUE}🔧 Next steps:${NC}"
echo "   - Review generated Go files"
echo "   - Update import paths in your Go code if needed"
echo "   - Run 'go build' to verify everything compiles"
echo "   - Run tests to ensure functionality works correctly"
echo ""
