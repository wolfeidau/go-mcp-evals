# Reporting Specification

## Overview

This specification defines the enhanced reporting system for MCP evaluations, including both real-time run output and post-execution report generation from trace files.

## Goals

1. **Print reports from existing traces** - Enable users to regenerate reports from previously saved trace JSON files
2. **Enhanced visual output** - Use the lipgloss theme system for colorized, styled output
3. **MCP-specific metrics** - Display metrics critical to MCP evaluation based on industry research

## Research Findings

### Industry Best Practices

Based on research into leading LLM evaluation frameworks (OpenAI Evals, DeepEval, MCPEval, MCPBench):

#### Core Metrics Display
- **Pass/Fail status** with clear visual indicators (colors)
- **Score thresholds** (our system: >= 3.0 = PASS)
- **Multiple quality dimensions** (accuracy, relevance, completeness, clarity, reasoning)
- **Token usage tracking** (critical for cost monitoring)
- **Latency/duration metrics** (execution time)

#### MCP-Specific Metrics
Research papers on MCP evaluation (MCPEval, MCPBench, Twilio MCP testing) identified these critical metrics:

1. **Tool Call Success Rate**: Ratio of successful vs failed tool calls
   - Top MCP servers: 64% accuracy
   - Poor MCP servers: 10% accuracy
   - This is a PRIMARY indicator of MCP quality

2. **Tool Execution Latency**: Time per tool call
   - Top performers: <15 seconds per tool
   - Highly variable across MCP implementations

3. **Token Economics**:
   - Input/output token counts
   - **Cached tokens**: MCP agents pull in large amounts of reference data (API specs, tool definitions) that gets cached
   - Cost estimates based on token usage

4. **Step Efficiency**: Number of agentic loop iterations to completion
   - Fewer steps with successful tool calls = better MCP design

5. **Data Accuracy**: For data-fetching MCPs, correctness of retrieved information
   - Evaluated by LLM grader in our system

## Trace Data Available

From `EvalTrace` structure in [mcp_evals.go](../mcp_evals.go):

```go
type EvalTrace struct {
    Steps             []AgenticStep  // Each step in the agentic loop
    Grading           *GradingTrace  // Grading interaction details
    TotalDuration     time.Duration  // Total execution time
    TotalInputTokens  int            // Sum of input tokens across all steps
    TotalOutputTokens int            // Sum of output tokens across all steps
    StepCount         int            // Number of agentic steps executed
    ToolCallCount     int            // Total number of tool calls made
}

type AgenticStep struct {
    StepNumber    int           // 1-indexed step number
    StartTime     time.Time     // When this step started
    EndTime       time.Time     // When this step completed
    Duration      time.Duration // Step execution duration
    ModelResponse string        // Text content from assistant
    StopReason    string        // end_turn, tool_use, max_tokens, etc.
    ToolCalls     []ToolCall    // Tools executed in this step
    InputTokens   int           // Input tokens for this step
    OutputTokens  int           // Output tokens for this step
    Error         string        // Error message if step failed
}

type ToolCall struct {
    ToolID    string          // Unique ID from content block
    ToolName  string          // MCP tool name
    StartTime time.Time       // When tool execution started
    EndTime   time.Time       // When tool execution completed
    Duration  time.Duration   // Tool execution duration
    Input     json.RawMessage // Tool arguments as JSON
    Output    json.RawMessage // Tool result as JSON
    Success   bool            // Whether tool executed successfully
    Error     string          // Error message if tool failed
}

type GradingTrace struct {
    UserPrompt       string        // Original eval prompt
    ModelResponse    string        // Model's answer being graded
    ExpectedResult   string        // Expected result description
    GradingPrompt    string        // Full prompt sent to grader
    RawGradingOutput string        // Complete LLM response before parsing
    StartTime        time.Time     // When grading started
    EndTime          time.Time     // When grading completed
    Duration         time.Duration // Grading duration
    InputTokens      int           // Input tokens for grading
    OutputTokens     int           // Output tokens for grading
    Error            string        // Error message if grading failed
}
```

## Proposed Output Design

### Summary Table

Enhanced table with MCP-specific metrics:

```
Name                 Status      Avg    Steps  Tools  Success%  Tokens (I→O)    Duration
───────────────────────────────────────────────────────────────────────────────────────
weather-forecast     PASS        4.2      3      5      100%     1,234 → 892    2.3s
code-search          PASS        3.8      5      8       88%     2,456 → 1,234  4.1s
data-fetch           FAIL        2.4      7     12       67%     3,890 → 2,100  8.7s
api-integration      ERROR       -        -      -        -       -              -

Quality Breakdown (when verbose):
  Accuracy  Completeness  Relevance  Clarity  Reasoning
     4          5            4         4         4
```

### Color Scheme (using theme.go)

**Status Colors:**
- **PASS**: `Guac` / `BrightGreen` (green)
- **FAIL**: `Cardinal` / `Watermelon` (red/pink)
- **ERROR**: `Cardinal` / `Watermelon` (red/pink)
- **NO GRADE**: `Squid` / `Smoke` (gray)

**Score Colors:**
- **1-2** (poor): `Cardinal` / `Watermelon` (red)
- **3** (threshold): `Squid` / `Smoke` (gray/neutral)
- **4-5** (good): `Guac` / `BrightGreen` (green)

**Section Headers:**
- Use `Section` style: `DarkGreen` / `BrightGreen`, bold, underlined

**Metric Values:**
- Normal: `Charcoal` / `Ash`
- Important values (token counts, duration): `Charple` (purple) for emphasis

### Overall Statistics

```
╭──────────────────────────────────────╮
│         EVALUATION SUMMARY           │
╰──────────────────────────────────────╯

Total Evaluations: 12
  ✓ Pass:   8 (67%)
  ✗ Fail:   3 (25%)
  ⚠ Error:  1 (8%)

Performance Metrics:
  Total Duration:     45.2s
  Total Tokens:       45,678 (I) → 23,456 (O)
  Avg Tokens/Eval:    3,806 (I) → 1,954 (O)

Tool Execution:
  Total Tool Calls:   89
  Success Rate:       85% (76/89)
  Avg Call Duration:  1.2s
  Failed Calls:       13
```

### Detailed View (Verbose Mode)

When `--verbose` flag is used, show per-eval breakdown:

```
╭─ Eval: weather-forecast ─────────────────────────────────╮
│ Status: PASS (4.2/5)                                      │
│                                                           │
│ Execution Trace:                                          │
│   Step 1: (0.8s, 412→289 tokens)                         │
│     Tool: get_current_weather                            │
│       ✓ Success (0.3s)                                   │
│       Input: {"location": "San Francisco"}               │
│                                                           │
│   Step 2: (0.7s, 389→267 tokens)                         │
│     Tool: get_forecast                                   │
│       ✓ Success (0.4s)                                   │
│                                                           │
│   Step 3: (0.8s, 433→336 tokens) - final answer          │
│                                                           │
│ Grading Details:                                          │
│   Accuracy:      4  ████░                                │
│   Completeness:  5  █████                                │
│   Relevance:     4  ████░                                │
│   Clarity:       4  ████░                                │
│   Reasoning:     4  ████░                                │
│                                                           │
│   Comments: "The response accurately uses weather data   │
│   from tools and provides a clear, complete forecast."   │
╰───────────────────────────────────────────────────────────╯
```

## Implementation Design

### 1. New Report Command

Add to [internal/commands/run.go](../internal/commands/run.go):

```go
type ReportCmd struct {
    TraceFiles []string `help:"Path(s) to trace JSON file(s)" required:"" type:"existingfile"`
    Verbose    bool     `help:"Show detailed per-eval breakdown" short:"v"`
}

func (r *ReportCmd) Run(globals *Globals) error {
    // Load trace files
    results := []evaluations.EvalRunResult{}
    for _, path := range r.TraceFiles {
        result, err := loadTraceFile(path)
        if err != nil {
            return err
        }
        results = append(results, result)
    }

    // Generate styled report
    return printStyledReport(results, r.Verbose)
}
```

### 2. Refactor Reporting Functions

Extract from `printSummary()` into shared styled functions:

```go
// Shared report rendering using lipgloss styles
func printStyledReport(results []evaluations.EvalRunResult, verbose bool) error {
    styles := help.DefaultStyles()

    // Print header
    printReportHeader(styles)

    // Print summary table
    printSummaryTable(results, styles)

    // Print overall statistics
    printOverallStats(results, styles)

    // Print detailed view if verbose
    if verbose {
        printDetailedBreakdown(results, styles)
    }

    return nil
}

func printSummaryTable(results []evaluations.EvalRunResult, styles help.Styles) {
    // Calculate column widths
    // Build table with proper alignment
    // Apply color scheme to status/scores
}

func printOverallStats(results []evaluations.EvalRunResult, styles help.Styles) {
    // Aggregate metrics
    totalDuration := 0
    totalInputTokens := 0
    totalOutputTokens := 0
    totalToolCalls := 0
    successfulToolCalls := 0

    for _, result := range results {
        if result.Trace != nil {
            totalDuration += result.Trace.TotalDuration
            totalInputTokens += result.Trace.TotalInputTokens
            totalOutputTokens += result.Trace.TotalOutputTokens
            totalToolCalls += result.Trace.ToolCallCount

            // Count successful tool calls
            for _, step := range result.Trace.Steps {
                for _, tool := range step.ToolCalls {
                    if tool.Success {
                        successfulToolCalls++
                    }
                }
            }
        }
    }

    // Print formatted stats with styling
}

func printDetailedBreakdown(results []evaluations.EvalRunResult, styles help.Styles) {
    for _, result := range results {
        printEvalDetail(result, styles)
    }
}
```

### 3. Helper Functions

```go
// Load trace file and reconstruct EvalRunResult
func loadTraceFile(path string) (evaluations.EvalRunResult, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return evaluations.EvalRunResult{}, err
    }

    var trace evaluations.EvalTrace
    if err := json.Unmarshal(data, &trace); err != nil {
        return evaluations.EvalRunResult{}, err
    }

    // Extract eval name from filename
    evalName := strings.TrimSuffix(filepath.Base(path), ".json")

    // Reconstruct result from trace
    // Note: We may need to enhance trace format to include
    // Eval metadata and Grade results
    return evaluations.EvalRunResult{
        Eval: evaluations.Eval{
            Name: evalName,
        },
        Trace: &trace,
    }, nil
}

// Calculate tool success rate
func calculateToolSuccessRate(trace *evaluations.EvalTrace) float64 {
    if trace.ToolCallCount == 0 {
        return 0.0
    }

    successful := 0
    for _, step := range trace.Steps {
        for _, tool := range step.ToolCalls {
            if tool.Success {
                successful++
            }
        }
    }

    return float64(successful) / float64(trace.ToolCallCount) * 100
}

// Format duration for display
func formatDuration(d time.Duration) string {
    if d < time.Second {
        return fmt.Sprintf("%dms", d.Milliseconds())
    }
    return fmt.Sprintf("%.1fs", d.Seconds())
}

// Format token counts with thousands separator
func formatTokens(count int) string {
    return fmt.Sprintf("%s", humanize.Comma(int64(count)))
}

// Get status color based on result
func getStatusColor(result evaluations.EvalRunResult, styles help.Styles) lipgloss.Style {
    if result.Error != nil {
        return styles.Error
    }

    if result.Grade == nil {
        return lipgloss.NewStyle().Foreground(help.Squid)
    }

    avg := avgScore(result.Grade)
    if avg >= 3.0 {
        return lipgloss.NewStyle().Foreground(help.Guac)
    }

    return styles.Error
}

// Get score color based on value
func getScoreColor(score int) lipgloss.Color {
    switch {
    case score >= 4:
        return help.Guac // Green
    case score == 3:
        return help.Squid // Gray
    default:
        return help.Cardinal // Red
    }
}
```

### 4. Update Trace Format (if needed)

Currently traces only store `EvalTrace` data. To generate full reports from traces, we may need to enhance the trace file format to include:

```json
{
  "eval": {
    "name": "weather-forecast",
    "description": "Test weather forecast MCP",
    "prompt": "What's the weather...",
    "expected_result": "..."
  },
  "grade": {
    "accuracy": 4,
    "completeness": 5,
    ...
  },
  "trace": {
    "steps": [...],
    ...
  }
}
```

This would require updating `writeTraces()` in [internal/commands/run.go](../internal/commands/run.go) to save the full `EvalRunResult` instead of just `Trace`.

### 5. Add Verbose Flag to RunCmd

```go
type RunCmd struct {
    Config  string `help:"Path to evaluation configuration file" required:"" type:"path"`
    APIKey  string `help:"Anthropic API key"`
    BaseURL string `help:"Base URL for Anthropic API"`
    Verbose bool   `help:"Show detailed per-eval breakdown" short:"v"`
}
```

## Command Usage

### Run evaluations with enhanced output

```bash
# Run with colorized summary
./mcp-evals run --config evals.yaml

# Run with detailed breakdown
./mcp-evals run --config evals.yaml --verbose

# Run and save traces
./mcp-evals run --config evals.yaml --trace-dir ./traces
```

### Generate report from existing traces

```bash
# Generate report from single trace
./mcp-evals report --trace-files ./traces/weather-forecast.json

# Generate report from multiple traces
./mcp-evals report --trace-files ./traces/*.json

# Generate detailed report
./mcp-evals report --trace-files ./traces/*.json --verbose
```

## Benefits

1. **Reproducibility**: Generate reports from saved traces without re-running expensive evals
2. **Cost tracking**: Prominent display of token usage for cost estimation
3. **MCP insights**: Tool success rates and execution times help identify MCP quality issues
4. **Visual clarity**: Color-coded output makes it easy to spot failures and performance issues
5. **Debugging**: Verbose mode provides step-by-step execution details for troubleshooting

## References

- OpenAI Evals: https://github.com/openai/evals
- DeepEval Framework: https://github.com/confident-ai/deepeval
- MCPEval Paper: https://arxiv.org/abs/2507.12806
- MCPBench Research: https://arxiv.org/html/2504.11094v2
- Twilio MCP Testing: https://www.twilio.com/en-us/blog/developers/twilio-alpha-mcp-server-real-world-performance
