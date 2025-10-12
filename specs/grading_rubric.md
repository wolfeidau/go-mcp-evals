# Grading Rubric Feature Specification

## Customer Problem

Evaluation grading in go-mcp-evals currently uses generic 1-5 scoring criteria (accuracy, completeness, relevance, clarity, reasoning) without domain-specific guidance. This creates three critical issues:

1. **Inconsistent scoring**: The same response quality may receive different grades depending on the grading LLM's interpretation
2. **Lack of specificity**: Generic criteria don't capture what "complete" means for troubleshooting vs. data retrieval vs. analysis tasks
3. **Difficult to iterate**: Evaluation authors cannot specify what matters most for their specific use case

## Solution Overview

Add support for **custom grading rubrics** that allow evaluation authors to define specific, measurable criteria for each dimension. Rubrics provide concrete guidance to the grading LLM, making scores more consistent, meaningful, and actionable.

## Design

### Data Structure

Add optional `grading_rubric` field to the `Eval` struct:

```go
// Eval represents a single evaluation test case
type Eval struct {
    Name           string         `yaml:"name" json:"name" jsonschema:"Unique identifier for this evaluation"`
    Description    string         `yaml:"description,omitempty" json:"description,omitempty" jsonschema:"Human-readable description of what this eval tests"`
    Prompt         string         `yaml:"prompt" json:"prompt" jsonschema:"The input prompt to send to the LLM"`
    ExpectedResult string         `yaml:"expected_result,omitempty" json:"expected_result,omitempty" jsonschema:"Expected behavior or result (used for documentation and grading context)"`
    GradingRubric  *GradingRubric `yaml:"grading_rubric,omitempty" json:"grading_rubric,omitempty" jsonschema:"Optional custom grading criteria for this evaluation"`
}

// GradingRubric defines specific evaluation criteria for grading
type GradingRubric struct {
    // Optional: Override which dimensions to grade (defaults to all 5 standard dimensions)
    Dimensions []string `yaml:"dimensions,omitempty" json:"dimensions,omitempty" jsonschema:"Which dimensions to grade: accuracy, completeness, relevance, clarity, reasoning"`

    // Criteria for each dimension - what to look for when grading
    Accuracy     *DimensionCriteria `yaml:"accuracy,omitempty" json:"accuracy,omitempty" jsonschema:"Specific criteria for accuracy scoring"`
    Completeness *DimensionCriteria `yaml:"completeness,omitempty" json:"completeness,omitempty" jsonschema:"Specific criteria for completeness scoring"`
    Relevance    *DimensionCriteria `yaml:"relevance,omitempty" json:"relevance,omitempty" jsonschema:"Specific criteria for relevance scoring"`
    Clarity      *DimensionCriteria `yaml:"clarity,omitempty" json:"clarity,omitempty" jsonschema:"Specific criteria for clarity scoring"`
    Reasoning    *DimensionCriteria `yaml:"reasoning,omitempty" json:"reasoning,omitempty" jsonschema:"Specific criteria for reasoning scoring"`

    // Optional: Minimum acceptable scores for pass/fail
    MinimumScores map[string]int `yaml:"minimum_scores,omitempty" json:"minimum_scores,omitempty" jsonschema:"Minimum acceptable score for each dimension (1-5)"`
}

// DimensionCriteria provides specific guidance for grading a dimension
type DimensionCriteria struct {
    Description string   `yaml:"description,omitempty" json:"description,omitempty" jsonschema:"What this dimension means for this specific eval"`
    MustHave    []string `yaml:"must_have,omitempty" json:"must_have,omitempty" jsonschema:"Required elements for high scores (4-5)"`
    NiceToHave  []string `yaml:"nice_to_have,omitempty" json:"nice_to_have,omitempty" jsonschema:"Optional elements that improve scores"`
    Penalties   []string `yaml:"penalties,omitempty" json:"penalties,omitempty" jsonschema:"Elements that reduce scores (errors, omissions, inaccuracies)"`
}
```

### YAML Configuration Example

```yaml
evals:
  - name: "troubleshoot_build"
    description: Test troubleshooting capabilities on a failed build
    prompt: "troubleshoot the https://buildkite.com/buildkite/buildkite/builds/161061 build"
    expected_result: "Should identify root cause and provide actionable remediation"

    grading_rubric:
      # Optional: Focus on specific dimensions (omit to use all 5)
      dimensions: ["accuracy", "completeness", "reasoning"]

      accuracy:
        description: "Correctness of root cause identification and technical details"
        must_have:
          - "Identifies the actual failing test(s) or job(s)"
          - "Extracts real error messages from logs or test output"
          - "Correctly interprets exit codes and job states"
        penalties:
          - "Misidentifies the root cause"
          - "Provides incorrect technical details"
          - "Confuses symptoms with underlying issues"

      completeness:
        description: "Thoroughness of investigation and remediation guidance"
        must_have:
          - "Examines multiple data sources (jobs, logs, annotations, test results)"
          - "Identifies all failed jobs, not just the first one"
          - "Provides specific remediation steps with code examples"
        nice_to_have:
          - "Distinguishes between hard failures and soft failures"
          - "Explains the context of why the failure occurred"
          - "Suggests preventive measures"

      reasoning:
        description: "Quality of logical deduction and evidence-based conclusions"
        must_have:
          - "Connects error messages to likely root causes"
          - "Identifies patterns (e.g., same test failing in parallel jobs)"
          - "Uses evidence from tool calls to support conclusions"
        nice_to_have:
          - "Distinguishes between environmental issues vs code issues"
          - "Explains reasoning process explicitly"

      # Optional: Define minimum acceptable scores
      minimum_scores:
        accuracy: 4
        completeness: 3
        reasoning: 3
```

### Integration with Grading System

Modify [mcp_evals.go:437-535](mcp_evals.go) `gradeWithTrace()` function to incorporate rubric:

```go
func (ec *EvalClient) gradeWithTrace(ctx context.Context, eval Eval, evalResult *EvalResult, execTrace *EvalTrace) (*GradeResult, *GradingTrace, error) {
    // ... existing code ...

    // Build grading prompt with rubric guidance
    gradingPrompt := ec.buildGradingPrompt(eval, evalResult, execTrace)

    // ... execute grading ...
}

func (ec *EvalClient) buildGradingPrompt(eval Eval, evalResult *EvalResult, execTrace *EvalTrace) string {
    var prompt strings.Builder

    // Standard context
    prompt.WriteString(fmt.Sprintf("Here is the user input: %s\n", evalResult.Prompt))
    prompt.WriteString(fmt.Sprintf("Here is the LLM's answer: %s\n", evalResult.RawResponse))

    // Add tool execution context
    if execTrace != nil && execTrace.ToolCallCount > 0 {
        prompt.WriteString("\n\nTool Execution Context:\n")
        // ... existing tool summary code ...
    }

    // NEW: Add rubric criteria if provided
    if eval.GradingRubric != nil {
        prompt.WriteString("\n\n## Custom Grading Criteria\n\n")
        prompt.WriteString("Use the following specific criteria when scoring this response:\n\n")

        if eval.GradingRubric.Accuracy != nil {
            prompt.WriteString(ec.formatDimensionCriteria("Accuracy", eval.GradingRubric.Accuracy))
        }
        if eval.GradingRubric.Completeness != nil {
            prompt.WriteString(ec.formatDimensionCriteria("Completeness", eval.GradingRubric.Completeness))
        }
        // ... similar for other dimensions ...

        if len(eval.GradingRubric.MinimumScores) > 0 {
            prompt.WriteString("\n### Minimum Acceptable Scores:\n")
            for dim, score := range eval.GradingRubric.MinimumScores {
                prompt.WriteString(fmt.Sprintf("- %s: %d/5\n", dim, score))
            }
        }
    }

    return prompt.String()
}

func (ec *EvalClient) formatDimensionCriteria(dimension string, criteria *DimensionCriteria) string {
    var sb strings.Builder

    sb.WriteString(fmt.Sprintf("### %s\n", dimension))

    if criteria.Description != "" {
        sb.WriteString(fmt.Sprintf("%s\n\n", criteria.Description))
    }

    if len(criteria.MustHave) > 0 {
        sb.WriteString("**Must have for high scores (4-5):**\n")
        for _, item := range criteria.MustHave {
            sb.WriteString(fmt.Sprintf("- %s\n", item))
        }
        sb.WriteString("\n")
    }

    if len(criteria.NiceToHave) > 0 {
        sb.WriteString("**Nice to have:**\n")
        for _, item := range criteria.NiceToHave {
            sb.WriteString(fmt.Sprintf("- %s\n", item))
        }
        sb.WriteString("\n")
    }

    if len(criteria.Penalties) > 0 {
        sb.WriteString("**Score reductions:**\n")
        for _, item := range criteria.Penalties {
            sb.WriteString(fmt.Sprintf("- %s\n", item))
        }
        sb.WriteString("\n")
    }

    return sb.String()
}
```

### Enhanced System Prompt

Update `EvalSystemPrompt` constant to acknowledge rubric criteria:

```go
const EvalSystemPrompt = `You are an expert evaluator assessing how well an LLM answers a given question. Review the provided answer and score it from 1 to 5 in each of the following categories:

- Accuracy: Does the answer contain factual errors or hallucinations?
- Completeness: Does the answer fully address all parts of the question?
- Relevance: Is the information directly related to the question?
- Clarity: Is the explanation easy to understand and well-structured?
- Reasoning: Does the answer show logical thinking or provide evidence or rationale?

If custom grading criteria are provided below, use those specific requirements to inform your scoring. The custom criteria define what "complete", "accurate", etc. mean for this particular evaluation.

CRITICAL: Return ONLY a valid JSON object with no markdown formatting, no code blocks, and no explanation. Your entire response must be valid JSON starting with { and ending with }.

Use this exact format:
{
    "accuracy": 1-5,
    "completeness": 1-5,
    "relevance": 1-5,
    "clarity": 1-5,
    "reasoning": 1-5,
    "overall_comments": "A short paragraph summarizing the strengths and weaknesses of the answer, specifically noting which rubric criteria were met or missed if custom criteria were provided."
}`
```

## Implementation Plan

### Phase 1: Core Data Structures (Breaking Change)
1. Add `GradingRubric`, `DimensionCriteria` structs to [mcp_evals.go](mcp_evals.go)
2. Add `GradingRubric *GradingRubric` field to `Eval` struct
3. Update JSON schema generation in [mcp_eval_config.go](mcp_eval_config.go)
4. Document breaking change in commit message and release notes

**Estimated complexity**: Medium
**Testing needs**: Unit tests for struct marshaling/unmarshaling

### Phase 2: Grading Integration
1. Implement `buildGradingPrompt()` and `formatDimensionCriteria()` helper functions
2. Modify `gradeWithTrace()` to use new prompt builder
3. Update `EvalSystemPrompt` constant
4. Ensure backward compatibility (rubric is optional)

**Estimated complexity**: Medium
**Testing needs**: Unit tests for prompt formatting, integration tests comparing rubric vs non-rubric grading

### Phase 3: Validation & User Experience
1. Add validation for rubric structure (scores must be 1-5, etc.)
2. Add warning if `minimum_scores` reference undefined dimensions
3. Update CLI to show rubric criteria when running evals with `--verbose`
4. Add JSON schema validation for rubric fields

**Estimated complexity**: Low
**Testing needs**: Unit tests for validation logic

### Phase 4: Documentation & Examples
1. Create example rubrics for common eval types (troubleshooting, data retrieval, analysis)
2. Update README with rubric usage examples
3. Add rubric section to [specs/grading_rubric.md](specs/grading_rubric.md) (this file)
4. Create migration guide for existing eval configs

**Estimated complexity**: Low
**Testing needs**: Verify examples work with real eval runs

## LLM-Assisted Rubric Drafting Guide

Manually writing detailed rubrics for every evaluation is time-consuming. Use an LLM to draft initial rubrics, then refine them based on actual evaluation results.

### Method 1: Generate Rubric from Eval Description

**Prompt Template:**

```
I'm creating an evaluation for an MCP server that tests: {EVAL_DESCRIPTION}

The evaluation prompt is: "{EVAL_PROMPT}"
The expected result is: "{EXPECTED_RESULT}"

Help me create a detailed grading rubric in YAML format. The rubric should specify:

1. **Accuracy criteria**: What technical details must be correct? What would count as factual errors?
2. **Completeness criteria**: What specific elements must the response include? What data sources should be checked?
3. **Reasoning criteria**: What logical deductions should be made? What patterns should be identified?

Format the rubric as YAML following this structure:

```yaml
grading_rubric:
  accuracy:
    description: "..."
    must_have:
      - "..."
    penalties:
      - "..."
  completeness:
    description: "..."
    must_have:
      - "..."
    nice_to_have:
      - "..."
  reasoning:
    description: "..."
    must_have:
      - "..."
```

Focus on specific, measurable criteria rather than vague requirements.
```

**Example Usage:**

```bash
# Save your eval description to a file
cat > eval_context.txt <<EOF
Eval: troubleshoot_build
Description: Test the MCP server's ability to diagnose a failed CI/CD build
Prompt: "troubleshoot the https://buildkite.com/buildkite/buildkite/builds/161061 build"
Expected: Should identify root cause of test failures and provide actionable remediation steps
EOF

# Use Claude or another LLM to generate rubric
claude "$(cat eval_context.txt)" "Generate a detailed grading rubric in YAML format for this evaluation"
```

### Method 2: Refine Rubric from Actual Results

After running an eval and reviewing the results, use the LLM response to improve the rubric:

**Prompt Template:**

```
I ran an evaluation and got this response:

{ACTUAL_LLM_RESPONSE}

The evaluation was graded with these scores:
- Accuracy: {SCORE}/5
- Completeness: {SCORE}/5
- Reasoning: {SCORE}/5

Help me create a grading rubric that captures what made this response good/bad. Specifically:

1. What specific elements should I look for to verify accuracy?
2. What data sources or steps are required for completeness?
3. What logical connections demonstrate good reasoning?

Format as YAML for the grading_rubric field.
```

### Method 3: Extract Rubric from Grade Comments

If you've already run evals without rubrics, extract criteria from the grading LLM's `overall_comments`:

**Prompt Template:**

```
I have grading comments from several evaluation runs. Extract common themes and convert them into a structured rubric.

Eval 1 Comments: {COMMENTS_1}
Eval 2 Comments: {COMMENTS_2}
Eval 3 Comments: {COMMENTS_3}

Create a YAML grading rubric that codifies:
- What elements the grader praised (should be "must_have")
- What elements the grader criticized (should be "penalties")
- What the grader said was missing (should be "must_have")

Format as YAML following the grading_rubric structure.
```

### Best Practices for LLM-Assisted Rubric Creation

1. **Start generic, refine iteratively**: Generate a first draft rubric, run evals, review scores, then refine
2. **Use actual data**: Include real tool outputs and responses in prompts for more specific criteria
3. **Focus on measurability**: Ask the LLM to make criteria objective ("must identify failing job ID" not "must understand the problem")
4. **Validate with multiple runs**: Generate rubric, run eval 3-5 times, check score consistency
5. **Combine methods**: Generate initial rubric (Method 1), refine from results (Method 2), extract from comments (Method 3)

### Example Workflow

```bash
# 1. Generate initial rubric from eval definition
claude "Create grading rubric for: troubleshoot CI build failures" > rubric_draft.yaml

# 2. Add rubric to eval config
cat rubric_draft.yaml >> mcp-test-evals.yaml

# 3. Run eval
go run ./cmd/mcp-evals run --config mcp-test-evals.yaml --output results.json

# 4. Review results and refine rubric
claude "Refine this rubric based on eval results: $(cat results.json | jq '.results[0]')"

# 5. Iterate until scores are consistent and meaningful
```

## Examples

### Example 1: Troubleshooting Eval

```yaml
- name: "troubleshoot_build"
  prompt: "troubleshoot build https://buildkite.com/org/pipeline/builds/123"
  expected_result: "Identify root cause and provide remediation"

  grading_rubric:
    dimensions: ["accuracy", "completeness", "reasoning"]

    accuracy:
      must_have:
        - "Identifies actual failing job(s) by name or ID"
        - "Extracts real error messages from logs"
        - "Correctly interprets exit codes"
      penalties:
        - "Misidentifies root cause"
        - "Fabricates error messages not in logs"

    completeness:
      must_have:
        - "Examines job logs"
        - "Checks build annotations"
        - "Provides code fix examples"
      nice_to_have:
        - "Analyzes test analytics data"
        - "Suggests preventive measures"

    reasoning:
      must_have:
        - "Connects error to likely code issue"
        - "Identifies failure patterns across parallel jobs"
      nice_to_have:
        - "Distinguishes environmental vs code issues"

    minimum_scores:
      accuracy: 4
      completeness: 3
```

### Example 2: Data Retrieval Eval

```yaml
- name: "get_user_info"
  prompt: "what user is associated with this token?"
  expected_result: "Return user name, email, and organization"

  grading_rubric:
    accuracy:
      must_have:
        - "Returns correct user name from API"
        - "Returns correct email address from API"
        - "Returns correct organization name from API"
      penalties:
        - "Returns incorrect field values"
        - "Fabricates data not returned by API"

    completeness:
      must_have:
        - "Includes user name"
        - "Includes email address"
        - "Includes organization name"
      nice_to_have:
        - "Includes additional user metadata (ID, avatar, etc.)"

    clarity:
      must_have:
        - "Formats response as structured data (table or list)"
        - "Labels each field clearly"
      nice_to_have:
        - "Groups related information together"

    minimum_scores:
      accuracy: 5
      completeness: 4
```

### Example 3: Analysis Eval

```yaml
- name: "identify_flaky_tests"
  prompt: "which tests are flaky in the last 100 builds?"
  expected_result: "Identify tests with inconsistent pass/fail patterns"

  grading_rubric:
    accuracy:
      must_have:
        - "Calculates correct pass/fail ratio for each test"
        - "Only flags tests that actually exist in test results"
      penalties:
        - "Includes tests that always pass or always fail"

    completeness:
      must_have:
        - "Analyzes test results from multiple builds"
        - "Provides specific test names and flake rates"
        - "Ranks tests by flakiness severity"
      nice_to_have:
        - "Shows trend over time"
        - "Groups by test suite or category"

    reasoning:
      must_have:
        - "Explains what criteria defines 'flaky'"
        - "Shows calculation method for flake rate"
      nice_to_have:
        - "Suggests potential causes of flakiness"
        - "Recommends tests to investigate first"

    minimum_scores:
      accuracy: 5
      completeness: 4
      reasoning: 3
```

## Testing Strategy

### Unit Tests

```go
func TestGradingRubricParsing(t *testing.T) {
    assert := require.New(t)

    yaml := `
name: test_eval
prompt: test prompt
grading_rubric:
  accuracy:
    must_have:
      - "item 1"
      - "item 2"
`

    var eval Eval
    err := yaml.Unmarshal([]byte(yaml), &eval)
    assert.NoError(err)
    assert.NotNil(eval.GradingRubric)
    assert.NotNil(eval.GradingRubric.Accuracy)
    assert.Len(eval.GradingRubric.Accuracy.MustHave, 2)
}

func TestGradingPromptWithRubric(t *testing.T) {
    assert := require.New(t)

    eval := Eval{
        GradingRubric: &GradingRubric{
            Accuracy: &DimensionCriteria{
                MustHave: []string{"criterion 1", "criterion 2"},
            },
        },
    }

    prompt := buildGradingPrompt(eval, &EvalResult{}, nil)
    assert.Contains(prompt, "criterion 1")
    assert.Contains(prompt, "criterion 2")
}
```

### Integration Tests

```go
func TestGradingWithRubric(t *testing.T) {
    // Run same eval with and without rubric
    // Verify rubric version provides more specific feedback
}

func TestRubricMinimumScores(t *testing.T) {
    // Verify minimum score thresholds are reflected in grading
}
```

## Migration Guide

Existing eval configs continue to work without modification. To add rubrics:

1. **Identify high-value evals**: Start with evals where grading consistency matters most
2. **Generate draft rubrics**: Use LLM-assisted drafting (see guide above)
3. **Run comparison**: Run eval with and without rubric, compare grade comments
4. **Refine iteratively**: Adjust rubric based on whether scores match expectations
5. **Document learnings**: Add comments in YAML explaining non-obvious criteria

## Future Enhancements

1. **Rubric templates**: Provide pre-built rubrics for common eval types
2. **Rubric validation**: CLI command to check rubric quality before running
3. **Score explanations**: Have grading LLM explain which criteria were met/missed
4. **Rubric evolution**: Track rubric changes over time and their impact on scores
5. **Weighted dimensions**: Allow specifying relative importance of dimensions

## References

- Current grading implementation: [mcp_evals.go:437-535](mcp_evals.go)
- Eval struct definition: [mcp_evals.go:552-557](mcp_evals.go)
- Config parsing: [mcp_eval_config.go](mcp_eval_config.go)
- Example config: [mcp-test-evals.yaml](mcp-test-evals.yaml)
