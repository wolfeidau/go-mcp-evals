# Performance Optimization Specification

## Overview

This document specifies performance optimizations for the go-mcp-evals library, focusing on prompt caching to reduce costs and latency in evaluation workflows.

## Problem Statement

Analysis of evaluation traces reveals significant opportunities for optimization:

### Current Performance Characteristics

**Example: `troubleshoot_build` evaluation**
- Total duration: 75.2 seconds
- Execution steps: 68.7 seconds (7 steps, 9 tool calls)
- Grading call: 6.5 seconds
- Grading input tokens: **59,527 tokens**
- Grading output tokens: 171 tokens

**Cost Analysis (Without Caching)**
- Per grading call: ~$0.18 (Claude Sonnet 3.5)
- Batch of 10 evals: ~$1.80 in grading costs alone
- Most tokens are **stable content** (tool definitions, system prompts, tool outputs)

### Customer Impact

1. **High costs** for repeated evaluations using the same MCP tools
2. **Slow feedback loops** due to redundant token processing
3. **Limited visibility** into where time and money are spent
4. **Missed optimization opportunities** due to lack of cache metrics

## Solution: Prompt Caching

Anthropic's prompt caching enables caching of stable content (tool definitions, system prompts, conversation history) with significant cost and performance benefits.

### Benefits

**Cost Savings**
- Cache writes: 25% premium over base input tokens (one-time cost)
- Cache reads: 90% discount (10% of base input token price)
- **Expected savings: 50-80% on repeated evaluations**

**Performance Improvements**
- Faster processing of cached tokens
- Reduced API latency for subsequent requests
- Enables larger context windows within budget constraints

**Industry Adoption**
- Standard practice for AI agent frameworks
- Explicitly recommended by Anthropic for agentic tool use
- Production-proven in major AI platforms

## Implementation Plan

### Phase 1: Enable Prompt Caching

#### 1.1 Cache Tool Definitions

**Location**: `mcp_evals.go:188-217` (tool parameter construction)

**Implementation**:
```go
// After building toolParams array, add cache control to the last tool
if len(toolParams) > 0 {
    lastIdx := len(toolParams) - 1
    toolParams[lastIdx].CacheControl = anthropic.NewCacheControlEphemeralParam()
}
```

**Rationale**:
- Tool definitions are **identical** across all evals using the same MCP server
- Typically 1000+ tokens per tool definition
- Change infrequently (only when MCP server updates)
- Cache breakpoint should be placed **after** all stable content

#### 1.2 Cache System Prompt

**Location**: `mcp_evals.go:237-245` (agentic loop message creation)

**Implementation**:
```go
System: []anthropic.TextBlockParam{
    {
        Text:         SystemPrompt,
        Type:         constant.Text,
        CacheControl: anthropic.NewCacheControlEphemeralParam(),
    },
},
```

**Rationale**:
- System prompt is static across all evaluations
- First content block that should be cached
- Combined with cached tools, creates large cached prefix

#### 1.3 Cache Grading System Prompt

**Location**: `mcp_evals.go:435-443` (grading message creation)

**Implementation**:
```go
System: []anthropic.TextBlockParam{
    {
        Text:         EvalSystemPrompt,
        Type:         constant.Text,
        CacheControl: anthropic.NewCacheControlEphemeralParam(),
    },
},
```

**Rationale**:
- Grading system prompt is static
- Grading calls consume the most tokens (59k+ in examples)
- High ROI for caching

#### 1.4 Configuration Options

**Add to `EvalClientConfig`**:
```go
type EvalClientConfig struct {
    // ... existing fields ...

    // EnablePromptCaching enables Anthropic prompt caching for tool definitions
    // and system prompts. This can reduce costs by 50-80% for repeated evaluations.
    // Default: true
    EnablePromptCaching bool

    // CacheTTL specifies the cache time-to-live.
    // Options: "5m" (default, free) or "1h" (premium, for long-running workflows)
    // Default: "5m"
    CacheTTL string
}
```

**Default Behavior**:
- Enable caching by default (opt-out, not opt-in)
- Use 5-minute TTL (optimal for batched evals)
- Allow users to disable for cost comparison testing

### Phase 2: Capture Cache Metrics

#### 2.1 Extend Trace Data Structures

**Update `AgenticStep`**:
```go
type AgenticStep struct {
    // ... existing fields ...

    // Cache metrics from Anthropic API
    CacheCreationInputTokens int           `json:"cache_creation_input_tokens"` // Tokens used to create cache
    CacheReadInputTokens     int           `json:"cache_read_input_tokens"`     // Tokens read from cache
    CacheCreationDetails     *CacheDetails `json:"cache_creation_details,omitempty"` // Breakdown by TTL
}

type CacheDetails struct {
    Ephemeral5m int64 `json:"ephemeral_5m_input_tokens"` // 5-minute cache tokens
    Ephemeral1h int64 `json:"ephemeral_1h_input_tokens"` // 1-hour cache tokens
}
```

**Update `GradingTrace`**:
```go
type GradingTrace struct {
    // ... existing fields ...

    // Cache metrics
    CacheCreationInputTokens int           `json:"cache_creation_input_tokens"`
    CacheReadInputTokens     int           `json:"cache_read_input_tokens"`
    CacheCreationDetails     *CacheDetails `json:"cache_creation_details,omitempty"`
}
```

**Update `EvalTrace`**:
```go
type EvalTrace struct {
    // ... existing fields ...

    // Aggregate cache metrics across all steps
    TotalCacheCreationTokens int `json:"total_cache_creation_tokens"`
    TotalCacheReadTokens     int `json:"total_cache_read_tokens"`
}
```

#### 2.2 Capture Metrics from API Responses

**Location**: `mcp_evals.go:271-272` (agentic step metrics)

**Implementation**:
```go
// Record step data from message
step.StopReason = string(message.StopReason)
step.InputTokens = int(message.Usage.InputTokens)
step.OutputTokens = int(message.Usage.OutputTokens)

// Capture cache metrics
step.CacheCreationInputTokens = int(message.Usage.CacheCreationInputTokens)
step.CacheReadInputTokens = int(message.Usage.CacheReadInputTokens)

// Optionally capture TTL breakdown
if message.Usage.CacheCreation.Ephemeral5mInputTokens > 0 ||
   message.Usage.CacheCreation.Ephemeral1hInputTokens > 0 {
    step.CacheCreationDetails = &CacheDetails{
        Ephemeral5m: message.Usage.CacheCreation.Ephemeral5mInputTokens,
        Ephemeral1h: message.Usage.CacheCreation.Ephemeral1hInputTokens,
    }
}
```

**Location**: `mcp_evals.go:457-458` (grading metrics)

**Implementation**:
```go
// Capture raw response and token usage
rawResponse := resp.Content[0].AsAny().(anthropic.TextBlock).Text
trace.RawGradingOutput = rawResponse
trace.InputTokens = int(resp.Usage.InputTokens)
trace.OutputTokens = int(resp.Usage.OutputTokens)

// Capture cache metrics
trace.CacheCreationInputTokens = int(resp.Usage.CacheCreationInputTokens)
trace.CacheReadInputTokens = int(resp.Usage.CacheReadInputTokens)

// Optionally capture TTL breakdown
if resp.Usage.CacheCreation.Ephemeral5mInputTokens > 0 ||
   resp.Usage.CacheCreation.Ephemeral1hInputTokens > 0 {
    trace.CacheCreationDetails = &CacheDetails{
        Ephemeral5m: resp.Usage.CacheCreation.Ephemeral5mInputTokens,
        Ephemeral1h: resp.Usage.CacheCreation.Ephemeral1hInputTokens,
    }
}
```

**Location**: `mcp_evals.go:339-344` (aggregate trace metrics)

**Implementation**:
```go
// Calculate trace metrics
trace.StepCount = len(trace.Steps)
for _, step := range trace.Steps {
    trace.TotalInputTokens += step.InputTokens
    trace.TotalOutputTokens += step.OutputTokens
    trace.ToolCallCount += len(step.ToolCalls)

    // Aggregate cache metrics
    trace.TotalCacheCreationTokens += step.CacheCreationInputTokens
    trace.TotalCacheReadTokens += step.CacheReadInputTokens
}

// Include grading cache metrics
if trace.Grading != nil {
    trace.TotalCacheCreationTokens += trace.Grading.CacheCreationInputTokens
    trace.TotalCacheReadTokens += trace.Grading.CacheReadInputTokens
}
```

### Phase 3: Enhanced Reporting

#### 3.1 Display Grading Timing

**Location**: `internal/reporting/report.go:346-365` (grading details section)

**Current State**: Grading section shows scores but not timing/performance

**Enhancement**:
```
## Grading Details

⏱️  Duration: 6.5s
📊 Tokens: 59.5k → 171 (cache: 54k read, 0 write)
💰 Cost: ~$0.06 (90% cache hit)

### Scores
Accuracy:     5/5 ⭐⭐⭐⭐⭐
Completeness: 5/5 ⭐⭐⭐⭐⭐
...
```

**Rationale**:
- Makes the "mystery 30s call" visible
- Shows immediate cost impact of caching
- Helps identify optimization opportunities

#### 3.2 Cache-Aware Token Display

**Format**: `<total> (<cached>/<created>)`

**Examples**:
- `59.5k (54k cached, 0 created)` - cache hit, minimal cost
- `59.5k (0 cached, 54k created)` - first run, paid premium
- `10.2k (8.5k cached, 1.7k created)` - partial cache hit

**Implementation** (per step):
```go
func formatTokens(step AgenticStep) string {
    total := step.InputTokens
    cached := step.CacheReadInputTokens
    created := step.CacheCreationInputTokens

    if cached == 0 && created == 0 {
        return fmt.Sprintf("%s", formatNumber(total))
    }

    parts := []string{}
    if cached > 0 {
        parts = append(parts, fmt.Sprintf("%s cached", formatNumber(cached)))
    }
    if created > 0 {
        parts = append(parts, fmt.Sprintf("%s created", formatNumber(created)))
    }

    return fmt.Sprintf("%s (%s)", formatNumber(total), strings.Join(parts, ", "))
}
```

#### 3.3 Overall Cache Statistics

**Add to summary section**:
```
┌─────────────────────────────────────────────────┐
│ Cache Performance                               │
├─────────────────────────────────────────────────┤
│ Cache writes:  54.0k tokens (+25% cost)         │
│ Cache reads:   486k tokens (-90% cost)          │
│ Cache hit rate: 90%                             │
│ Estimated savings: $1.45 (81%)                  │
└─────────────────────────────────────────────────┘
```

**Rationale**:
- Demonstrates value of prompt caching
- Helps users understand cost optimization
- Identifies evals that don't benefit from caching

## Performance Characteristics

### Cache Behavior

**5-Minute TTL (Default)**:
- Optimal for: Batched evaluations (10-50 evals in sequence)
- Cache lifetime: Refreshed on each use, expires after 5 min idle
- Cost: No premium (standard cache pricing)
- Use case: CI/CD pipelines, batch testing, development workflows

**1-Hour TTL (Premium)**:
- Optimal for: Long-running agent sessions, distributed evaluations
- Cache lifetime: Up to 1 hour
- Cost: Higher cache write premium
- Use case: Production monitoring, user-facing agents, sparse eval runs

### Expected Performance Improvements

**First Evaluation (Cold Cache)**:
- Cache creation overhead: ~2-5% latency increase
- Cost: 25% premium on cached tokens
- Benefit: Subsequent evals will see savings

**Subsequent Evaluations (Warm Cache)**:
- Cost reduction: 50-80% on cached tokens
- Latency reduction: 20-40% faster processing
- Scaling: Benefits increase with more evaluations

**Example Batch (10 Evals)**:
- Without caching: $1.80 grading cost
- With caching: $0.77 grading cost
- **Savings: $1.03 (57%)**

### Cache Hit Optimization

**Best Practices**:
1. Run evaluations sequentially (not parallel) to maximize cache reuse
2. Group evaluations by MCP server to share tool definition cache
3. Use consistent model selection (cache is model-specific)
4. Avoid modifying tool definitions between eval runs

**Cache Invalidation**:
Cache is automatically invalidated when:
- Content changes (different tools, system prompt)
- Cache TTL expires
- Organization context changes
- Model is upgraded

## Monitoring & Observability

### Key Metrics to Track

1. **Cache Hit Rate**: `TotalCacheReadTokens / (TotalCacheReadTokens + InputTokens - TotalCacheCreationTokens)`
2. **Cost Savings**: `(CacheReadTokens * 0.9 * BasePrice) - (CacheCreationTokens * 0.25 * BasePrice)`
3. **Cache Efficiency**: Ratio of cached tokens to total tokens per eval
4. **Grading Duration**: Track whether caching reduces grading latency

### Logging

Add structured logging for cache events:
```go
log.Info().
    Int("cache_creation_tokens", step.CacheCreationInputTokens).
    Int("cache_read_tokens", step.CacheReadInputTokens).
    Float64("cache_hit_rate", hitRate).
    Msg("Step completed with cache metrics")
```

## Testing Strategy

### Test Scenarios

1. **Cache Creation Test**
   - First eval run should show `CacheCreationInputTokens > 0`
   - Subsequent run should show `CacheReadInputTokens > 0`
   - Verify token counts match expected values

2. **Cache Expiration Test**
   - Run eval, wait 6 minutes, run again
   - Should see cache recreation (new `CacheCreationInputTokens`)

3. **Cache Disabled Test**
   - Set `EnablePromptCaching: false`
   - Verify no cache metrics in trace
   - Compare costs with caching enabled

4. **Cost Verification**
   - Run batch of 10 evals with caching enabled
   - Verify cost savings match expected 50-80% range
   - Compare with baseline (caching disabled)

### Backward Compatibility

All changes are backward compatible:
- New config fields have sensible defaults (caching enabled)
- Trace format is extended (new fields, existing fields unchanged)
- Reports gracefully handle missing cache data
- Existing tests continue to pass

## Rollout Plan

### Phase 1: Implementation (Week 1)
- [ ] Add cache control to tool definitions
- [ ] Add cache control to system prompts
- [ ] Add configuration options
- [ ] Capture cache metrics in traces
- [ ] Write unit tests

### Phase 2: Reporting (Week 2)
- [ ] Add grading timing display
- [ ] Implement cache-aware token formatting
- [ ] Add overall cache statistics
- [ ] Update report tests

### Phase 3: Validation (Week 3)
- [ ] Run cost comparison tests
- [ ] Verify cache behavior across TTL options
- [ ] Document performance characteristics
- [ ] Update README with caching information

### Phase 4: Release
- [ ] Update CHANGELOG with breaking changes note (if any)
- [ ] Create migration guide
- [ ] Publish blog post on cost optimization
- [ ] Monitor production usage and gather feedback

## Success Criteria

1. **Cost Reduction**: 50-80% savings on batched evaluations
2. **Performance**: 20-40% reduction in grading latency
3. **Visibility**: Cache metrics displayed in all reports
4. **Adoption**: Caching enabled by default, opt-out available
5. **Reliability**: No regressions in eval correctness or trace accuracy

## Future Enhancements

### Extended Thinking Cache Support

When extended thinking is enabled, cache the thinking blocks:
```go
Thinking: anthropic.ThinkingConfigParamUnionParamOfThinkingConfig{
    ThinkingConfig: anthropic.ThinkingConfigParam{
        Type:         anthropic.ThinkingConfigTypeEnabled,
        BudgetTokens: 10000,
        CacheControl: anthropic.NewCacheControlEphemeralParam(), // Future
    },
},
```

### Smart Cache Invalidation

Detect when tool definitions change and automatically invalidate cache:
```go
// Compare current tool hash with previous run
// Bypass cache if tools have changed
if toolsChanged {
    // Skip cache control for this run
}
```

### Cache Warming

Pre-warm cache before batch runs:
```go
// Send minimal request to populate cache
// All subsequent evals benefit immediately
warmCache(ctx, tools, systemPrompt)
```

## References

- [Anthropic Prompt Caching Documentation](https://docs.claude.com/en/docs/build-with-claude/prompt-caching)
- [Anthropic Agent Capabilities Blog](https://www.anthropic.com/news/agent-capabilities-api)
- [Extended Thinking Documentation](https://docs.anthropic.com/en/docs/build-with-claude/extended-thinking)
- [Cost Calculator](https://docs.anthropic.com/en/docs/about-claude/models#model-comparison)

## Appendix: Cache Pricing

**Claude 3.5 Sonnet (as of 2025)**:
- Base input: $3.00 / MTok
- Base output: $15.00 / MTok
- Cache write: $3.75 / MTok (25% premium)
- Cache read: $0.30 / MTok (90% discount)

**Example Calculation**:
```
Grading call: 59,527 input tokens

Without cache:
59,527 tokens * $3.00/MTok = $0.179

With cache (after first run):
54,000 cached * $0.30/MTok = $0.016
5,527 fresh * $3.00/MTok = $0.017
Total = $0.033

Savings: $0.146 (81%)
```
