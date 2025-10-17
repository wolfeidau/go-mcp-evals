package reporting

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	evaluations "github.com/wolfeidau/mcp-evals"
	"github.com/wolfeidau/mcp-evals/internal/help"
)

// stripANSI removes ANSI escape codes from a string
func stripANSI(str string) string {
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[mGKH]`)
	return ansiRegex.ReplaceAllString(str, "")
}

// captureOutput captures stdout during test execution
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

// loadTestFixtures loads evaluation results from testdata JSON files
func loadTestFixtures(t *testing.T) []evaluations.EvalRunResult {
	t.Helper()

	fixtures := []string{
		"weather-forecast.json",
		"database-query.json",
		"api-integration-test.json",
		"connection-timeout.json",
		"simple-echo-test.json",
	}

	results := make([]evaluations.EvalRunResult, 0, len(fixtures))

	for _, fixture := range fixtures {
		path := filepath.Join("testdata", fixture)
		result, err := LoadTraceFile(path)
		if err != nil {
			t.Fatalf("failed to load fixture %s: %v", fixture, err)
		}

		// Handle error case for connection-timeout
		if result.Eval.Name == "connection-timeout" {
			result.Error = fmt.Errorf("connection timeout after 30s")
		}

		results = append(results, result)
	}

	return results
}

func TestPrintStyledReport(t *testing.T) {
	assert := require.New(t)

	results := loadTestFixtures(t)

	// Test non-verbose output
	t.Run("non-verbose output", func(t *testing.T) {
		output := captureOutput(func() {
			err := PrintStyledReport(results, false)
			assert.NoError(err)
		})

		// Strip ANSI codes for easier testing
		plainOutput := stripANSI(output)

		// Check for header
		assert.Contains(plainOutput, "# Evaluation Summary")

		// Check for table headers
		assert.Contains(plainOutput, "Name")
		assert.Contains(plainOutput, "Status")
		assert.Contains(plainOutput, "Avg")
		assert.Contains(plainOutput, "Steps")
		assert.Contains(plainOutput, "Tools")
		assert.Contains(plainOutput, "Success%")
		assert.Contains(plainOutput, "Tokens")

		// Check for eval names
		assert.Contains(plainOutput, "weather-forecast")
		assert.Contains(plainOutput, "database-query")
		assert.Contains(plainOutput, "api-integration-test")
		assert.Contains(plainOutput, "connection-timeout")
		assert.Contains(plainOutput, "simple-echo-test")

		// Check for status indicators
		assert.Contains(plainOutput, "PASS")
		assert.Contains(plainOutput, "FAIL")
		assert.Contains(plainOutput, "ERROR")
		assert.Contains(plainOutput, "NO GRADE")

		// Check for overall statistics section
		assert.Contains(plainOutput, "## Overall Statistics")
		assert.Contains(plainOutput, "Total Evaluations: 5")
		assert.Contains(plainOutput, "### Performance Metrics")
		assert.Contains(plainOutput, "### Tool Execution")

		// Should NOT contain detailed breakdown
		assert.NotContains(plainOutput, "## Detailed Breakdown")
		assert.NotContains(plainOutput, "#### Execution Trace")
		assert.NotContains(plainOutput, "#### Grading Details")
	})

	// Test verbose output
	t.Run("verbose output", func(t *testing.T) {
		output := captureOutput(func() {
			err := PrintStyledReport(results, true)
			assert.NoError(err)
		})

		// Strip ANSI codes for easier testing
		plainOutput := stripANSI(output)

		// Should contain everything from non-verbose
		assert.Contains(plainOutput, "# Evaluation Summary")
		assert.Contains(plainOutput, "## Overall Statistics")

		// Should contain detailed breakdown
		assert.Contains(plainOutput, "## Detailed Breakdown")

		// Check for execution trace details
		assert.Contains(plainOutput, "#### Execution Trace")
		assert.Contains(plainOutput, "Step 1:")
		assert.Contains(plainOutput, "Step 2:")
		assert.Contains(plainOutput, "Step 3:")

		// Check for tool call details
		assert.Contains(plainOutput, "Tool: get_location_coords")
		assert.Contains(plainOutput, "Tool: get_forecast")
		assert.Contains(plainOutput, "✓ Success")
		assert.Contains(plainOutput, "✗ Failed")

		// Check for grading details
		assert.Contains(plainOutput, "#### Grading Details")
		assert.Contains(plainOutput, "Accuracy")
		assert.Contains(plainOutput, "Completeness")
		assert.Contains(plainOutput, "Relevance")
		assert.Contains(plainOutput, "Clarity")
		assert.Contains(plainOutput, "Reasoning")

		// Check for comments (wrapped text may be on different lines)
		assert.Contains(plainOutput, "weather API tools")
		assert.Contains(plainOutput, "Failed to properly authenticate")
	})
}

func TestBuildResultRow(t *testing.T) {
	assert := require.New(t)

	results := loadTestFixtures(t)
	styles := help.DefaultStyles()

	t.Run("successful eval with high score", func(t *testing.T) {
		row := buildResultRow(results[0], styles)

		assert.Len(row, 7)
		assert.Equal("weather-forecast", row[0])
		assert.Contains(row[1], "PASS")
		assert.Equal("4.8", row[2])     // Average of 5,5,5,4,5
		assert.Equal("3", row[3])       // Steps
		assert.Equal("2", row[4])       // Tools
		assert.Equal("100%", row[5])    // Success rate
		assert.Contains(row[6], "1.2k") // Input tokens
		assert.Contains(row[6], "552")  // Output tokens
	})

	t.Run("failed eval with low score", func(t *testing.T) {
		row := buildResultRow(results[2], styles)

		assert.Len(row, 7)
		assert.Equal("api-integration-test", row[0])
		assert.Contains(row[1], "FAIL")
		assert.Equal("1.6", row[2]) // Average of 1,2,2,2,1
	})

	t.Run("error case", func(t *testing.T) {
		row := buildResultRow(results[3], styles)

		assert.Len(row, 7)
		assert.Equal("connection-timeout", row[0])
		assert.Contains(row[1], "ERROR")
		assert.Equal("-", row[2])
		assert.Equal("-", row[3])
		assert.Equal("-", row[4])
	})

	t.Run("no grade case", func(t *testing.T) {
		row := buildResultRow(results[4], styles)

		assert.Len(row, 7)
		assert.Equal("simple-echo-test", row[0])
		assert.Contains(row[1], "NO GRADE")
		assert.Equal("-", row[2])
		assert.Equal("1", row[3])  // Has steps
		assert.Equal("0", row[4])  // No tools
		assert.Equal("0%", row[5]) // No tools = 0% success
	})

	t.Run("truncates long names", func(t *testing.T) {
		longNameResult := evaluations.EvalRunResult{
			Eval: evaluations.Eval{
				Name: "this-is-a-very-long-evaluation-name-that-should-be-truncated",
			},
			Trace: &evaluations.EvalTrace{
				StepCount:     1,
				ToolCallCount: 0,
			},
		}

		row := buildResultRow(longNameResult, styles)
		assert.Len(row[0], 25) // Should be truncated to 22 chars + "..."
		assert.Contains(row[0], "...")
	})
}

func TestCalculateToolSuccessRate(t *testing.T) {
	assert := require.New(t)

	t.Run("100% success rate", func(t *testing.T) {
		trace := &evaluations.EvalTrace{
			ToolCallCount: 2,
			Steps: []evaluations.AgenticStep{
				{
					ToolCalls: []evaluations.ToolCall{
						{Success: true},
						{Success: true},
					},
				},
			},
		}

		rate := calculateToolSuccessRate(trace)
		assert.InDelta(100.0, rate, 0.01)
	})

	t.Run("50% success rate", func(t *testing.T) {
		trace := &evaluations.EvalTrace{
			ToolCallCount: 4,
			Steps: []evaluations.AgenticStep{
				{
					ToolCalls: []evaluations.ToolCall{
						{Success: true},
						{Success: false},
					},
				},
				{
					ToolCalls: []evaluations.ToolCall{
						{Success: true},
						{Success: false},
					},
				},
			},
		}

		rate := calculateToolSuccessRate(trace)
		assert.InDelta(50.0, rate, 0.01)
	})

	t.Run("no tool calls", func(t *testing.T) {
		trace := &evaluations.EvalTrace{
			ToolCallCount: 0,
			Steps:         []evaluations.AgenticStep{},
		}

		rate := calculateToolSuccessRate(trace)
		assert.InDelta(0.0, rate, 0.01)
	})
}

func TestFormatHelpers(t *testing.T) {
	assert := require.New(t)

	t.Run("formatDuration", func(t *testing.T) {
		assert.Equal("500ms", formatDuration(500*time.Millisecond))
		assert.Equal("1.5s", formatDuration(1500*time.Millisecond))
		assert.Equal("2.3s", formatDuration(2300*time.Millisecond))
	})

	t.Run("formatTokens", func(t *testing.T) {
		assert.Equal("123", formatTokens(123))
		assert.Equal("1.2k", formatTokens(1234))
		assert.Equal("12.3k", formatTokens(12345))
		assert.Equal("1.2M", formatTokens(1234567))
	})

	t.Run("formatTokenCounts", func(t *testing.T) {
		assert.Equal("1.2k → 500", formatTokenCounts(1234, 500))
		assert.Equal("100 → 50", formatTokenCounts(100, 50))
	})

	t.Run("avgScore", func(t *testing.T) {
		grade := &evaluations.GradeResult{
			Accuracy:     5,
			Completeness: 4,
			Relevance:    5,
			Clarity:      4,
			Reasoning:    5,
		}
		assert.InDelta(4.6, avgScore(grade), 0.01)
	})
}

func TestWrapText(t *testing.T) {
	assert := require.New(t)

	t.Run("short text no wrap", func(t *testing.T) {
		text := "Short text"
		wrapped := wrapText(text, 50)
		assert.Equal("Short text", wrapped)
		assert.NotContains(wrapped, "\n")
	})

	t.Run("long text wraps", func(t *testing.T) {
		text := "This is a very long piece of text that should definitely wrap when we apply the width constraint to it for proper formatting"
		wrapped := wrapText(text, 40)
		assert.Contains(wrapped, "\n")
		lines := strings.Split(wrapped, "\n")
		assert.Greater(len(lines), 1)
	})
}
