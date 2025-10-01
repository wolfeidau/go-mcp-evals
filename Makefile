.PHONY: test test-e2e build lint lint-check clean build-test-server check

# Run unit tests
test:
	go test -v ./...

# Run e2e tests with API key from 1Password
# Update the op read path to match your 1Password structure
test-e2e:
	@echo "Fetching ANTHROPIC_API_KEY from 1Password..."
	@ANTHROPIC_API_KEY=$$(op item get "Anthropic API Key" --field credential --reveal) \
		go test -v -tags=e2e -timeout=5m ./...

# Run e2e tests with manual API key (alternative to 1Password)
test-e2e-manual:
	@if [ -z "$$ANTHROPIC_API_KEY" ]; then \
		echo "Error: ANTHROPIC_API_KEY environment variable is not set"; \
		exit 1; \
	fi
	go test -v -tags=e2e -timeout=5m ./...

# Build CLI
build:
	mkdir -p bin
	go build -o bin/mcp-evals ./cmd/mcp-evals

# Run linter with auto-fix
lint:
	golangci-lint run --fix

# Run linter without fixes (for CI)
lint-check:
	golangci-lint run

# Run tests with coverage
test-coverage:
	go test -cover ./...

# Clean build artifacts
clean:
	rm -rf bin/
	go clean -testcache

# Build test server (for manual testing)
build-test-server:
	mkdir -p bin
	go build -o bin/test-server ./testdata/mcp-test-server

# Run all checks (lint + test)
check: lint-check test

# Run all checks including e2e
check-all: lint-check test test-e2e
