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
        exec nix --extra-experimental-features nix-command --extra-experimental-features flakes develop --command bash "$0" "$@"
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
if go build ./...; then
    echo -e "${GREEN}   ✅ Generated code compiles successfully${NC}"
else
    echo -e "${YELLOW}   ⚠️  Generated code compilation check failed (this may be normal if main.go has missing dependencies)${NC}"
fi

echo ""
echo -e "${GREEN}🎉 Protocol buffer generation completed!${NC}"
echo "================================================"
echo ""
echo -e "${BLUE}📁 Generated files location:${NC} internal/proto/"
echo -e "${BLUE}🔧 Next steps:${NC}"
echo "   - Review generated Go files"
echo "   - Update import paths in your Go code if needed"
echo "   - Run 'go build' to verify everything compiles"
echo "   - Run tests to ensure functionality works correctly"
echo ""
