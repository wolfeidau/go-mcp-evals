# E2E Testing Specification

## Status: ✅ IMPLEMENTED & PASSING

All planned tests have been implemented and are passing successfully.

## Overview

This document outlines the end-to-end testing strategy for the go-mcp-evals library. The goal is to validate the complete evaluation flow from MCP server startup through tool calling to final grading.

### Implementation Summary

- **Test Server**: 4 tools implemented (add, echo, get_current_time, get_env)
- **Test Cases**: 4 test cases implemented and passing
- **Build System**: Makefile with 1Password integration
- **Coverage**: Basic eval, multi-tool, env vars, grading validation

## Test Architecture

### 1. Test MCP Server

**Location**: `testdata/mcp-test-server/main.go`

A minimal MCP server implementation using `github.com/modelcontextprotocol/go-sdk` with simple, deterministic tools for validation.

#### Tools Implemented

1. **add** - Addition tool ✅
   - Input: `{a: number, b: number}`
   - Output: `{result: number}`
   - Purpose: Tests basic numeric computation and tool calling

2. **get_current_time** - Time tool ✅
   - Input: `{}`
   - Output: `{time: string, format: string}`
   - Purpose: Tests tools with no input parameters

3. **echo** - Echo tool ✅
   - Input: `{message: string}`
   - Output: `{echoed: string}`
   - Purpose: Tests string handling and simple passthrough

4. **get_env** - Environment variable tool ✅
   - Input: `{name: string}`
   - Output: `{name: string, value: string, set: bool}`
   - Purpose: Tests environment variable passthrough from EvalClientConfig to MCP server

#### Server Implementation

See [testdata/mcp-test-server/main.go](../testdata/mcp-test-server/main.go) for the full implementation.

The server implements four tools using the MCP Go SDK:
- `Add()` - adds two numbers
- `Echo()` - echoes a message
- `GetCurrentTime()` - returns current time in RFC3339 format
- `GetEnv()` - retrieves environment variable values (validates env passthrough)

### 2. E2E Test Implementation

**Location**: `mcp_evals_e2e_test.go`

**Build Tag**: `//go:build e2e`

#### Test Cases Implemented

1. **TestE2E_BasicEvaluation** ✅
   - Start test MCP server
   - Run evaluation with prompt: "What is 5 plus 3?"
   - Verify Claude calls the `add` tool
   - Verify final answer contains "8"
   - Verify grading succeeds

2. **TestE2E_MultipleTools** ✅
   - Prompt: "Echo the message 'hello world' and tell me what time it is"
   - Verify both `echo` and `get_current_time` tools are called
   - Verify answer contains expected content

3. **TestE2E_EnvironmentVariables** ✅
   - Pass `TEST_API_TOKEN` environment variable to MCP server
   - Prompt: "What is the value of the TEST_API_TOKEN environment variable?"
   - Verify Claude calls the `get_env` tool
   - Verify answer contains the test token value
   - Validates env var passthrough from EvalClientConfig.Env to MCP server process

4. **TestE2E_GradingScores** ✅
   - Run evaluation with known correct answer
   - Verify all grade dimensions are present
   - Verify scores are within reasonable ranges (1-5)
   - Verify overall comment is non-empty

#### Test Structure

See [mcp_evals_e2e_test.go](../mcp_evals_e2e_test.go) for the full implementation. Key points:

- Uses `//go:build e2e` tag to separate from unit tests
- Builds test server in temporary directory for each test
- Uses `strings.Contains()` for result validation
- `validateGrade()` checks scores are in range [0-5] and `OverallComment` is non-empty
- API differences from original plan:
  - `Env` is `[]string` not `map[string]string` (format: `"KEY=value"`)
  - `NewEvalClient()` returns `*EvalClient` (no error)
  - `RunEval()` returns `*EvalResult` (not `string`)
  - `Grade()` takes `*EvalResult` (not separate prompt and result strings)
  - `GradeResult` fields are `int` not `float64`
  - `GradeResult` has `OverallComment` field (not individual explanation fields)

### 3. Makefile Integration

**Location**: `Makefile`

```makefile
.PHONY: test test-e2e build lint clean

# Run unit tests
test:
	go test -v ./...

# Run e2e tests with API key from 1Password
test-e2e:
	@echo "Fetching ANTHROPIC_API_KEY from 1Password..."
	@ANTHROPIC_API_KEY=$$(op read "op://path/to/your/anthropic-api-key") \
		go test -v -tags=e2e -timeout=5m ./...

# Build CLI
build:
	go build -o bin/mcp-evals ./cmd/mcp-evals

# Run linter
lint:
	golangci-lint run --fix

# Run linter without fixes
lint-check:
	golangci-lint run

# Clean build artifacts
clean:
	rm -rf bin/
	go clean -testcache

# Build test server (for manual testing)
build-test-server:
	go build -o bin/test-server ./testdata/mcp-test-server

# Run all checks (lint + test)
check: lint-check test
```

### 4. 1Password Integration

The Makefile assumes the ANTHROPIC_API_KEY is stored in 1Password. To set this up:

1. Store your API key in 1Password
2. Get the secret reference path using: `op item get "Anthropic API Key" --reveal`
3. Update the Makefile with the correct path: `op://Vault/Item/field`

Example paths:
- `op://Private/Anthropic/api_key`
- `op://Development/API Keys/anthropic_api_key`

Alternatively, you can pass the API key directly:

```bash
ANTHROPIC_API_KEY=your-key-here make test-e2e
```

## Test Validation Criteria

### Success Criteria

1. **Server Connection**: Test server starts and MCP client connects successfully ✅
2. **Tool Discovery**: All four tools are discovered and converted to Anthropic format ✅
3. **Tool Execution**: Claude successfully calls tools and receives results ✅
4. **Environment Variables**: Env vars passed through EvalClientConfig.Env reach the MCP server ✅
5. **Answer Quality**: Final answers are coherent and factually correct ✅
6. **Grading**: All five grade dimensions return valid scores with overall comment ✅

### Performance Expectations

- E2E test completion: < 2 minutes per test
- Tool call latency: < 5 seconds per call
- Server startup: < 1 second

## Running the Tests

```bash
# Unit tests only
make test

# E2E tests (requires 1Password CLI and stored API key)
make test-e2e

# E2E tests with manual API key
ANTHROPIC_API_KEY=sk-... make test-e2e

# All checks (lint + unit tests)
make check
```

## Future Enhancements

1. **Parallel Test Server**: Add test server with streaming/async tools
2. **Error Scenarios**: Test cases for tool errors, timeouts, malformed responses
3. **Performance Tests**: Benchmark tool calling overhead
4. **Comparison Tests**: Run same eval against different models
5. **Batch Evaluation**: Test multiple prompts in sequence

## Dependencies

- `github.com/modelcontextprotocol/go-sdk` - MCP server implementation
- `github.com/anthropics/anthropic-sdk-go` - Claude API client
- 1Password CLI (`op`) - API key management
- Go 1.25+ - Build toolchain
