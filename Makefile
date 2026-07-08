# Immich Go Backend Makefile
# Requires Nix development environment

.PHONY: help proto-gen proto-clean proto-check setup dev-shell build test clean all perf-load perf-storage perf-db

# Default target
help: ## Show this help message
	@echo "Immich Go Backend - Available targets:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Note: Most targets require being in a Nix development environment."
	@echo "Run 'make dev-shell' or 'nix develop' first."

# Protocol Buffers targets
proto-gen: ## Generate protocol buffer Go files using buf
	@echo "🔨 Generating protocol buffers..."
	@./scripts/generate-protos.sh

proto-clean: ## Clean generated protocol buffer files
	@echo "🧹 Cleaning generated protocol buffer files..."
	@echo "✅ Cleaned generated files"

proto-check: ## Verify protocol buffer definitions and generated files
	@echo "🔍 Checking protocol buffer definitions..."
	@if command -v buf >/dev/null 2>&1; then \
		buf lint; \
		buf breaking --against '.git#branch=main'; \
	else \
		echo "❌ buf not found. Run 'make dev-shell' first."; \
		exit 1; \
	fi

sqlc-gen: ## Generate SQL code using sqlc
	@echo "🔨 Generating SQL code..."
	@if command -v sqlc >/dev/null 2>&1; then \
		sqlc generate; \
	else \
		echo "❌ sqlc not found. Run 'make dev-shell' first."; \
		exit 1; \
	fi

# Development environment
dev-shell: ## Enter Nix development shell
	@echo "🚀 Entering Nix development environment..."
	@if [ -f "flake.nix" ]; then \
		nix develop; \
	elif [ -f "shell.nix" ]; then \
		nix-shell; \
	else \
		echo "❌ No Nix configuration found"; \
		exit 1; \
	fi

setup: ## Set up development environment and generate initial files
	@echo "🔧 Setting up development environment..."
	@$(MAKE) proto-gen
	@echo "📦 Installing Go dependencies..."
	@go mod tidy
	@echo "✅ Setup complete!"

# Build targets
build: proto-gen ## Build the application
	@echo "🔨 Building application..."
	@go build -o bin/immich-go-backend ./cmd
	@echo "✅ Build complete: bin/immich-go-backend"

# Test targets
test: proto-gen ## Run tests
	@echo "🧪 Running tests..."
	@go test ./...

test-verbose: proto-gen ## Run tests with verbose output
	@echo "🧪 Running tests (verbose)..."
	@go test -v ./...

# Utility targets
clean: proto-clean ## Clean all generated files and build artifacts
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf bin/
	@echo "✅ Clean complete"

fmt: ## Format Go code
	@echo "🎨 Formatting Go code..."
	@go fmt ./...

lint: ## Run linters
	@echo "🔍 Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "⚠️  golangci-lint not found, running basic checks..."; \
		go vet ./...; \
	fi

mod-tidy: ## Tidy Go modules
	@echo "📦 Tidying Go modules..."
	@go mod tidy

# CI/CD targets
ci-check: proto-gen lint test ## Run all CI checks
	@echo "✅ All CI checks passed"

all: clean setup build test ## Clean, setup, build, and test everything
	@echo "🎉 All tasks completed successfully!"

# Performance / load (optional; not part of default CI)
# See scripts/perf/README.md
perf-load: ## Smoke load against IMMICH_URL (health + read-only; needs running server)
	@echo "📈 Running load smoke (IMMICH_URL=$${IMMICH_URL:-http://localhost:3001})..."
	@./scripts/perf/load-smoke.sh

perf-storage: ## Local storage Go benchmarks (-tags bench; no Docker)
	@echo "📈 Running storage benchmarks..."
	@go test -tags bench -bench=BenchmarkStorage -benchmem -count=1 -run='^$$' ./scripts/perf/

perf-db: ## DB/SQLC Go benchmarks (-tags bench; skips without Docker)
	@echo "📈 Running DB benchmarks (skips if Docker unavailable)..."
	@go test -tags bench -bench=BenchmarkDB -benchmem -count=1 -run='^$$' ./scripts/perf/
