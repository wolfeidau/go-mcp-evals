# Flight Recorder Specification

## Problem Statement

Users of the MCP evaluation library need detailed visibility into evaluation execution to understand:
- How the LLM arrived at its final answer
- Which tools were called and in what sequence
- Token consumption and timing for each step
- The complete interaction history that led to the grade
- Performance bottlenecks and error conditions

Currently, `EvalRunResult` only captures the final answer and grade, making it impossible to debug failures, optimize performance, or audit the decision-making process.

## Solution: Comprehensive Trace Capture

Add a structured "flight recorder" system that captures every interaction during evaluation execution without breaking the existing API.

## Data Structures

### EvalTrace

Top-level container for all trace data from an evaluation run.

```go
type EvalTrace struct {
    // Agentic loop execution
    Steps []AgenticStep `json:"steps"`

    // Grading execution
    Grading *GradingTrace `json:"grading,omitempty"`

    // Overall metrics
    TotalDuration    time.Duration `json:"total_duration"`
    TotalInputTokens int           `json:"total_input_tokens"`
    TotalOutputTokens int          `json:"total_output_tokens"`
    StepCount        int           `json:"step_count"`
    ToolCallCount    int           `json:"tool_call_count"`
}
```

### AgenticStep

Records a single iteration of the agentic loop (prompt → thinking → tool calls → results).

```go
type AgenticStep struct {
    // Step identification
    StepNumber int       `json:"step_number"`
    StartTime  time.Time `json:"start_time"`
    EndTime    time.Time `json:"end_time"`
    Duration   time.Duration `json:"duration"`

    // Model interaction
    ModelResponse string `json:"model_response"` // Text content from assistant
    StopReason    string `json:"stop_reason"`    // end_turn, tool_use, max_tokens, etc.

    // Tool execution
    ToolCalls []ToolCall `json:"tool_calls,omitempty"`

    // Token usage for this step
    InputTokens  int `json:"input_tokens"`
    OutputTokens int `json:"output_tokens"`

    // Error tracking
    Error string `json:"error,omitempty"`
}
```

### ToolCall

Captures details of a single tool invocation.

```go
type ToolCall struct {
    // Tool identification
    ToolID   string `json:"tool_id"`   // Unique ID from content block
    ToolName string `json:"tool_name"` // MCP tool name

    // Execution timing
    StartTime time.Time     `json:"start_time"`
    EndTime   time.Time     `json:"end_time"`
    Duration  time.Duration `json:"duration"`

    // Input/Output
    Input  json.RawMessage `json:"input"`  // Tool arguments as JSON
    Output json.RawMessage `json:"output"` // Tool result as JSON

    // Status
    Success bool   `json:"success"`
    Error   string `json:"error,omitempty"`
}
```

### GradingTrace

Records the grading interaction with the LLM.

```go
type GradingTrace struct {
    // Grading prompt components
    UserPrompt       string `json:"user_prompt"`        // Original eval prompt
    ModelResponse    string `json:"model_response"`     // Model's answer being graded
    ExpectedResult   string `json:"expected_result"`    // Expected result description

    // Grading system prompt and response
    GradingPrompt    string `json:"grading_prompt"`     // Full prompt sent to grader
    RawGradingOutput string `json:"raw_grading_output"` // Complete LLM response before parsing

    // Metrics
    StartTime     time.Time     `json:"start_time"`
    EndTime       time.Time     `json:"end_time"`
    Duration      time.Duration `json:"duration"`
    InputTokens   int           `json:"input_tokens"`
    OutputTokens  int           `json:"output_tokens"`

    // Error tracking
    Error string `json:"error,omitempty"`
}
```

## API Changes

### EvalRunResult Enhancement

Add trace field to existing result structure:

```go
type EvalRunResult struct {
    Eval   Eval         `json:"eval"`
    Result *EvalResult  `json:"result,omitempty"`
    Grade  *GradeResult `json:"grade,omitempty"`
    Error  error        `json:"-"`

    // NEW: Complete execution trace
    Trace  *EvalTrace   `json:"trace,omitempty"`
}
```

This is **backward compatible** - existing code continues to work, trace is opt-in for detailed analysis.

## Implementation Details

### 1. Instrumentation in RunEval

Wrap the agentic loop to capture each step:

```go
func (c *EvalClient) RunEval(ctx context.Context, eval Eval) (EvalRunResult, error) {
    trace := &EvalTrace{
        Steps: make([]AgenticStep, 0, 10),
    }
    overallStart := time.Now()

    // ... existing setup code ...

    for i := 0; i < maxSteps; i++ {
        stepStart := time.Now()
        step := AgenticStep{
            StepNumber: i + 1,
            StartTime:  stepStart,
        }

        // Execute message API call
        messageResp, err := c.client.Messages.New(ctx, params)

        // Record response data
        step.StopReason = string(messageResp.StopReason)
        step.InputTokens = int(messageResp.Usage.InputTokens)
        step.OutputTokens = int(messageResp.Usage.OutputTokens)

        // Process content blocks
        for _, block := range messageResp.Content {
            switch block := block.AsUnion().(type) {
            case anthropic.TextBlock:
                step.ModelResponse += block.Text
            case anthropic.ToolUseBlock:
                // Execute tool and record
                toolCall := c.executeAndTraceToolCall(ctx, block, mcpClient)
                step.ToolCalls = append(step.ToolCalls, toolCall)
            }
        }

        step.EndTime = time.Now()
        step.Duration = step.EndTime.Sub(stepStart)
        trace.Steps = append(trace.Steps, step)

        // ... existing loop logic ...
    }

    // Record overall metrics
    trace.TotalDuration = time.Since(overallStart)
    trace.StepCount = len(trace.Steps)
    // ... calculate totals ...

    return EvalRunResult{
        Trace: trace,
        // ... other fields ...
    }
}
```

### 2. Instrumentation in grade

Capture the grading interaction:

```go
func (c *EvalClient) grade(ctx context.Context, eval Eval, result string) (*GradeResult, *GradingTrace, error) {
    trace := &GradingTrace{
        UserPrompt:     eval.Prompt,
        ModelResponse:  result,
        ExpectedResult: eval.ExpectedResult,
        StartTime:      time.Now(),
    }

    // Build grading prompt
    gradingPrompt := c.buildGradingPrompt(eval, result)
    trace.GradingPrompt = gradingPrompt

    // Execute grading
    resp, err := c.client.Messages.New(ctx, params)
    if err != nil {
        trace.Error = err.Error()
        return nil, trace, err
    }

    // Capture raw response
    trace.RawGradingOutput = resp.Content[0].Text
    trace.InputTokens = int(resp.Usage.InputTokens)
    trace.OutputTokens = int(resp.Usage.OutputTokens)
    trace.EndTime = time.Now()
    trace.Duration = trace.EndTime.Sub(trace.StartTime)

    // Parse grade
    grade, err := parseGradeResponse(resp.Content[0].Text)

    return grade, trace, err
}
```

### 3. Tool Call Tracing

Extract tool execution into traceable function:

```go
func (c *EvalClient) executeAndTraceToolCall(
    ctx context.Context,
    toolUseBlock anthropic.ToolUseBlock,
    mcpClient *client.StdioMCPClient,
) ToolCall {
    toolCall := ToolCall{
        ToolID:    toolUseBlock.ID,
        ToolName:  toolUseBlock.Name,
        StartTime: time.Now(),
        Input:     json.RawMessage(toolUseBlock.Input),
    }

    // Execute MCP tool call
    result, err := mcpClient.CallTool(ctx, toolUseBlock.Name, toolUseBlock.Input)

    toolCall.EndTime = time.Now()
    toolCall.Duration = toolCall.EndTime.Sub(toolCall.StartTime)

    if err != nil {
        toolCall.Success = false
        toolCall.Error = err.Error()
    } else {
        toolCall.Success = true
        toolCall.Output, _ = json.Marshal(result)
    }

    return toolCall
}
```

## Usage Examples

### Basic Trace Access

```go
result, err := client.RunEval(ctx, eval)
if err != nil {
    log.Fatal(err)
}

// Access trace data
fmt.Printf("Completed in %d steps using %d tokens\n",
    result.Trace.StepCount,
    result.Trace.TotalInputTokens + result.Trace.TotalOutputTokens)

// Examine each step
for _, step := range result.Trace.Steps {
    fmt.Printf("Step %d: %s (%.2fs, %d tools)\n",
        step.StepNumber,
        step.StopReason,
        step.Duration.Seconds(),
        len(step.ToolCalls))
}
```

### Export Trace to JSON

```go
traceJSON, err := json.MarshalIndent(result.Trace, "", "  ")
if err != nil {
    log.Fatal(err)
}
os.WriteFile("eval_trace.json", traceJSON, 0644)
```

### Performance Analysis

```go
// Find slowest tool calls
var slowestTool ToolCall
var slowestDuration time.Duration

for _, step := range result.Trace.Steps {
    for _, tc := range step.ToolCalls {
        if tc.Duration > slowestDuration {
            slowestDuration = tc.Duration
            slowestTool = tc
        }
    }
}

fmt.Printf("Slowest tool: %s took %.2fs\n",
    slowestTool.ToolName,
    slowestTool.Duration.Seconds())
```

### Debug Failed Evaluations

```go
if result.Error != nil {
    // Find where it failed
    for _, step := range result.Trace.Steps {
        if step.Error != "" {
            fmt.Printf("Failed at step %d: %s\n", step.StepNumber, step.Error)
        }
        for _, tc := range step.ToolCalls {
            if !tc.Success {
                fmt.Printf("Tool %s failed: %s\n", tc.ToolName, tc.Error)
            }
        }
    }
}
```

## Future Enhancements

### Real-Time Progress Callbacks

Add optional callback mechanism for streaming trace events:

```go
type EvalProgressCallback func(event TraceEvent)

type EvalClientConfig struct {
    // ... existing fields ...
    ProgressCallback EvalProgressCallback
}

// User can implement callback for real-time monitoring
config.ProgressCallback = func(event TraceEvent) {
    switch e := event.(type) {
    case *StepStartEvent:
        fmt.Printf("Starting step %d\n", e.StepNumber)
    case *ToolCallEvent:
        fmt.Printf("Calling tool: %s\n", e.ToolName)
    case *StepCompleteEvent:
        fmt.Printf("Step %d complete (%d tokens)\n", e.StepNumber, e.OutputTokens)
    }
}
```

### Trace Export Formats

Support multiple export formats:
- JSON (structured data)
- Markdown (human-readable report)
- HTML (interactive visualization)
- OpenTelemetry spans (distributed tracing integration)

### Message History Capture

Optionally capture the complete message array passed to Claude at each step:

```go
type AgenticStep struct {
    // ... existing fields ...
    MessageHistory []anthropic.MessageParam `json:"message_history,omitempty"`
}
```

This enables full replay and debugging but increases memory usage.

## Non-Goals

- **Performance overhead**: Trace capture should add minimal latency (<1% overhead)
- **Breaking changes**: Existing code continues to work without modification
- **Storage**: Library doesn't persist traces; that's the caller's responsibility
- **Visualization**: Library provides data structures; visualization is external

## Testing Strategy

1. **Unit tests**: Verify trace structures are populated correctly
2. **E2E tests**: Add assertions on trace data in existing e2e tests
3. **Benchmark tests**: Measure overhead of trace capture
4. **Integration tests**: Verify trace data matches actual execution

## Success Metrics

1. Users can debug why evaluations failed by examining trace data
2. Users can identify performance bottlenecks (slow tools, excessive steps)
3. Users can export complete audit trails for compliance
4. Trace capture adds <1% performance overhead
5. Zero breaking changes to existing API
