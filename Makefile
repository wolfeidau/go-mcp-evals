.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: test
test: ## Run unit tests
	go test -v ./...

.PHONY: test-e2e
test-e2e: ## Run e2e tests with API key from 1Password
	@echo "Fetching ANTHROPIC_API_KEY from 1Password..."
	@ANTHROPIC_API_KEY=$$(op item get "Anthropic API Key" --field credential --reveal) \
		go test -v -tags=e2e -timeout=5m ./...

.PHONY: test-e2e-manual
test-e2e-manual: ## Run e2e tests with manual API key (set ANTHROPIC_API_KEY)
	@if [ -z "$$ANTHROPIC_API_KEY" ]; then \
		echo "Error: ANTHROPIC_API_KEY environment variable is not set"; \
		exit 1; \
	fi
	go test -v -tags=e2e -timeout=5m ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	go test -cover ./...

.PHONY: test-coverage-html
test-coverage-html: ## Generate HTML coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"

.PHONY: test-build
test-build: ## Build test binary
	go test -c -o bin/mcp-evals.test .

.PHONY: build
build: ## Build CLI binary
	mkdir -p bin
	go build -o bin/mcp-evals ./cmd/mcp-evals

.PHONY: build-test-server
build-test-server: ## Build test server for manual testing
	mkdir -p bin
	go build -o bin/test-server ./testdata/mcp-test-server

.PHONY: lint
lint: ## Run golangci-lint (no auto-fix)
	golangci-lint run

.PHONY: lint-fix
lint-fix: ## Run golangci-lint with auto-fix
	golangci-lint run --fix

.PHONY: fmt
fmt: ## Format Go code
	go fmt ./...

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean -testcache

.PHONY: install
install: ## Install CLI to GOPATH/bin
	go install ./cmd/mcp-evals

.PHONY: schema
schema: ## Generate JSON schema for eval configuration
	go run ./cmd/mcp-evals schema > eval-config-schema.json
	@echo "Schema generated at eval-config-schema.json"

.PHONY: check
check: lint test ## Run all checks (lint + test)

.PHONY: check-all
check-all: lint test test-e2e ## Run all checks including e2e

.PHONY: all
all: fmt lint test build ## Run fmt, lint, test, and build
