# go-mcp-evals

A Go library and CLI for evaluating Model Context Protocol (MCP) servers using Claude. This tool connects to an MCP server, runs an agentic evaluation loop where Claude uses the server's tools to answer questions, and grades the responses across five dimensions: accuracy, completeness, relevance, clarity, and reasoning.

## Use Cases

**As a library**: Programmatically evaluate MCP servers in Go code, integrate evaluation results into CI/CD pipelines, or build custom evaluation workflows.

**As a CLI**: Run evaluations from YAML/JSON configuration files with immediate pass/fail feedback, detailed scoring breakdowns, and optional trace output for debugging.

## Installation

### Using install script (recommended)

```bash
curl -sSfL https://raw.githubusercontent.com/wolfeidau/go-mcp-evals/main/install.sh | sh
```

### Using Go

```bash
go install github.com/wolfeidau/go-mcp-evals/cmd/mcp-evals@latest
```

## Quick Start

Create an evaluation config file (e.g., `evals.yaml`):

```yaml
model: claude-3-5-sonnet-20241022
mcp_server:
  command: npx
  args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]

evals:
  - name: list-files
    description: Test filesystem listing
    prompt: "List files in the current directory"
    expected_result: "Should enumerate files with details"
```

Run evaluations:

```bash
export ANTHROPIC_API_KEY=your-api-key
mcp-evals run --config evals.yaml
```

## CLI Commands

- `run` - Execute evaluations (default command)
- `validate` - Validate config file against JSON schema
- `schema` - Generate JSON schema for configuration
- `help` - Show help information

See `mcp-evals <command> --help` for detailed usage.

## Configuration

Evaluation configs support both YAML and JSON formats:

- `model` - Anthropic model ID (required)
- `grading_model` - Optional separate model for grading
- `timeout` - Per-evaluation timeout (e.g., "2m", "30s")
- `max_steps` - Maximum agentic loop iterations (default: 10)
- `max_tokens` - Maximum tokens per LLM request (default: 4096)
- `mcp_server` - Server command, args, and environment
- `evals` - List of test cases with name, prompt, and expected result

## How It Works

1. Connects to the specified MCP server via command/transport
2. Retrieves available tools from the MCP server
3. Runs an agentic loop (max 10 steps) where Claude:
   - Receives the evaluation prompt and available MCP tools
   - Calls tools via the MCP protocol as needed
   - Accumulates tool results and continues reasoning
4. Evaluates the final response using a separate LLM call that scores five dimensions on a 1-5 scale
5. Returns structured results with pass/fail status (passing threshold: average score ≥ 3.0)

## License

Apache License, Version 2.0 - Copyright [Mark Wolfe](https://www.wolfe.id.au)