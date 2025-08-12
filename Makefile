.PHONY: help build test lint clean config-reference config-example config-all install-tools

# Default target
help: ## Show this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build targets
build: ## Build the application
	go build -v -ldflags="-w -s" -o the-archive .

build-debug: ## Build the application with debug symbols
	go build -v -o the-archive .

install: ## Install the application
	go install -ldflags="-w -s" .

# Test targets
test: ## Run all tests
	go test -v -race ./...

test-coverage: ## Run tests with coverage
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html

test-short: ## Run tests without race detection
	go test -short ./...

benchmark: ## Run benchmarks
	go test -bench=. -benchmem ./...

# Code quality targets
lint: ## Run linter
	golangci-lint run

lint-fix: ## Run linter with auto-fix
	golangci-lint run --fix

fmt: ## Format code
	go fmt ./...
	gofumpt -w .

# Configuration targets
config-reference: ## Generate comprehensive config reference with documentation
	go run ./cmd/generate-config -mode=reference -output=config.reference.yaml

config-example: ## Generate simple example config
	go run ./cmd/generate-config -mode=example -output=config.example.yaml

config-minimal: ## Generate minimal config
	go run ./cmd/generate-config -mode=minimal -output=config.minimal.yaml

config-all: config-reference config-example config-minimal ## Generate all config files

config-update-tests: ## Update test fixtures with current defaults
	go run ./cmd/generate-config -update-tests

config-test: ## Test configuration system including golden files
	go test ./internal/config/... -v

# Development targets
dev: ## Run in development mode
	go run . -config=config.yaml

watch: ## Watch for changes and rebuild (requires entr)
	find . -name "*.go" | entr -r make build

# Cleanup targets
clean: ## Clean build artifacts
	rm -f the-archive coverage.out coverage.html
	go clean -cache -testcache

clean-all: clean ## Clean everything including generated configs
	rm -f config.*.yaml

# Installation targets
install-tools: ## Install development tools
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install mvdan.cc/gofumpt@latest

# Release targets
release-check: lint test ## Run checks before release
	@echo "✅ All checks passed, ready for release"

# Database targets
db-migrate: ## Run database migrations
	go run ./cmd/migrate

# Configuration validation
config-validate: ## Validate current config.yaml
	@if [ -f config.yaml ]; then \
		echo "Validating config.yaml..."; \
		go run . -config=config.yaml -validate 2>/dev/null && echo "✅ Config is valid" || echo "❌ Config is invalid"; \
	else \
		echo "❌ config.yaml not found"; \
	fi

# Documentation
docs-serve: ## Serve documentation locally (if docs exist)
	@if [ -d docs ]; then \
		echo "Serving docs at http://localhost:8000"; \
		python3 -m http.server 8000 -d docs; \
	else \
		echo "No docs directory found"; \
	fi

# Git helpers  
git-hooks: ## Install git hooks
	@if [ -d .git ]; then \
		echo "Installing git hooks..."; \
		echo "#!/bin/sh\nmake lint test" > .git/hooks/pre-commit; \
		chmod +x .git/hooks/pre-commit; \
		echo "✅ Git hooks installed"; \
	else \
		echo "❌ Not a git repository"; \
	fi