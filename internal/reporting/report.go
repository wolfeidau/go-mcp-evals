package reporting

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/lipgloss/v2/table"
	evaluations "github.com/wolfeidau/go-mcp-evals"
	"github.com/wolfeidau/go-mcp-evals/internal/help"
)

// PrintStyledReport generates a colorized, styled report from evaluation results
func PrintStyledReport(results []evaluations.EvalRunResult, verbose bool) error {
	styles := help.DefaultStyles()

	// Build the complete report content
	var content strings.Builder

	// Capture output for each section
	content.WriteString(captureReportHeader(styles))
	content.WriteString(captureSummaryTable(results, styles))
	content.WriteString(captureOverallStats(results, styles))

	// Print detailed view if verbose
	if verbose {
		content.WriteString(captureDetailedBreakdown(results, styles))
	}

	// Wrap the entire output with top/bottom margins only
	marginStyle := lipgloss.NewStyle().
		MarginTop(1).
		MarginBottom(1)

	fmt.Println(marginStyle.Render(content.String()))

	return nil
}

// Heading helpers for consistent spacing
func h1(styles help.Styles, text string) string {
	return styles.Heading.Render("# "+text) + "\n\n"
}

func h2(styles help.Styles, text string) string {
	return styles.Heading.Render("## "+text) + "\n\n"
}

func h3(styles help.Styles, text string) string {
	return styles.Heading.Render("### "+text) + "\n\n"
}

func h4(styles help.Styles, text string) string {
	return styles.Heading.Render("#### "+text) + "\n\n"
}

func captureReportHeader(styles help.Styles) string {
	return h1(styles, "Evaluation Summary")
}

func captureSummaryTable(results []evaluations.EvalRunResult, styles help.Styles) string {
	var output strings.Builder

	// Build table rows
	rows := make([][]string, 0, len(results))
	for _, result := range results {
		rows = append(rows, buildResultRow(result, styles))
	}

	// Create table with lipgloss
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(styles.Heading).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				// Header row
				return lipgloss.NewStyle().
					Bold(true).
					Foreground(styles.Heading.GetForeground()).
					Align(lipgloss.Left).Padding(0, 2)
			}
			return lipgloss.NewStyle().Align(lipgloss.Left).Padding(0, 2)
		}).
		Headers("Name", "Status", "Avg", "Steps", "Tools", "Success%", "Tokens (I→O)").
		Rows(rows...)

	output.WriteString(t.String() + "\n")
	output.WriteString("\n")
	return output.String()
}

func buildResultRow(result evaluations.EvalRunResult, styles help.Styles) []string {
	name := result.Eval.Name
	if len(name) > 25 {
		name = name[:22] + "..."
	}

	// Handle error case
	if result.Error != nil {
		status := styles.Error.Render("ERROR")
		return []string{name, status, "-", "-", "-", "-", "-"}
	}

	// Handle no trace case
	if result.Trace == nil {
		status := styles.Muted.Render("NO TRACE")
		return []string{name, status, "-", "-", "-", "-", "-"}
	}

	// Calculate metrics
	avgScoreVal := 0.0
	statusStr := styles.Muted.Render("NO GRADE")
	if result.Grade != nil {
		avgScoreVal = avgScore(result.Grade)
		if avgScoreVal >= 3.0 {
			statusStr = styles.Success.Render("PASS")
		} else {
			statusStr = styles.Error.Render("FAIL")
		}
	}

	trace := result.Trace
	successRate := calculateToolSuccessRate(trace)

	// Format values
	avgStr := "-"
	if result.Grade != nil {
		avgStr = fmt.Sprintf("%.1f", avgScoreVal)
	}

	stepsStr := fmt.Sprintf("%d", trace.StepCount)
	toolsStr := fmt.Sprintf("%d", trace.ToolCallCount)
	successStr := fmt.Sprintf("%d%%", int(successRate))
	tokenStr := formatTokenCounts(trace.TotalInputTokens, trace.TotalOutputTokens)

	return []string{name, statusStr, avgStr, stepsStr, toolsStr, successStr, tokenStr}
}

func captureOverallStats(results []evaluations.EvalRunResult, styles help.Styles) string {
	var output strings.Builder

	// Calculate overall statistics
	totalEvals := len(results)
	errorCount := 0
	passCount := 0
	failCount := 0
	noGradeCount := 0

	var totalDuration time.Duration
	totalInputTokens := 0
	totalOutputTokens := 0
	totalToolCalls := 0
	successfulToolCalls := 0

	for _, result := range results {
		if result.Error != nil {
			errorCount++
			continue
		}

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

		if result.Grade != nil {
			if avgScore(result.Grade) >= 3.0 {
				passCount++
			} else {
				failCount++
			}
		} else {
			noGradeCount++
		}
	}

	// Build statistics output
	output.WriteString(h2(styles, "Overall Statistics"))

	// Total evaluations
	output.WriteString(fmt.Sprintf("Total Evaluations: %d\n", totalEvals))

	// Pass/Fail/Error breakdown
	if passCount > 0 {
		passStr := styles.Success.Render(fmt.Sprintf("✓ Pass:   %d (%.0f%%)", passCount, float64(passCount)/float64(totalEvals)*100))
		output.WriteString(fmt.Sprintf("  %s\n", passStr))
	}
	if failCount > 0 {
		failStr := styles.Error.Render(fmt.Sprintf("✗ Fail:   %d (%.0f%%)", failCount, float64(failCount)/float64(totalEvals)*100))
		output.WriteString(fmt.Sprintf("  %s\n", failStr))
	}
	if errorCount > 0 {
		errorStr := styles.Error.Render(fmt.Sprintf("⚠ Error:  %d (%.0f%%)", errorCount, float64(errorCount)/float64(totalEvals)*100))
		output.WriteString(fmt.Sprintf("  %s\n", errorStr))
	}
	if noGradeCount > 0 {
		noGradeStr := styles.Muted.Render(fmt.Sprintf("○ No Grade: %d", noGradeCount))
		output.WriteString(fmt.Sprintf("  %s\n", noGradeStr))
	}
	output.WriteString("\n")

	// Performance metrics
	if totalInputTokens > 0 || totalDuration > 0 {
		output.WriteString(h3(styles, "Performance Metrics"))

		if totalDuration > 0 {
			output.WriteString(fmt.Sprintf("Total Duration:     %s\n", formatDuration(totalDuration)))
		}

		if totalInputTokens > 0 {
			output.WriteString(fmt.Sprintf("Total Tokens:       %s (I) → %s (O)\n",
				formatTokens(totalInputTokens),
				formatTokens(totalOutputTokens)))

			avgInput := totalInputTokens / totalEvals
			avgOutput := totalOutputTokens / totalEvals
			output.WriteString(fmt.Sprintf("Avg Tokens/Eval:    %s (I) → %s (O)\n",
				formatTokens(avgInput),
				formatTokens(avgOutput)))
		}
		output.WriteString("\n")
	}

	// Tool execution stats
	if totalToolCalls > 0 {
		output.WriteString(h3(styles, "Tool Execution"))
		output.WriteString(fmt.Sprintf("Total Tool Calls:   %d\n", totalToolCalls))

		successRateOverall := float64(successfulToolCalls) / float64(totalToolCalls) * 100
		successRateStr := fmt.Sprintf("%.0f%% (%d/%d)", successRateOverall, successfulToolCalls, totalToolCalls)
		if successRateOverall >= 80 {
			successRateStr = styles.Success.Render(successRateStr)
		} else if successRateOverall < 50 {
			successRateStr = styles.Error.Render(successRateStr)
		}
		output.WriteString(fmt.Sprintf("Success Rate:       %s\n", successRateStr))

		if totalToolCalls > successfulToolCalls {
			failedCalls := totalToolCalls - successfulToolCalls
			output.WriteString(fmt.Sprintf("Failed Calls:       %s\n",
				styles.Error.Render(fmt.Sprintf("%d", failedCalls))))
		}
		output.WriteString("\n")
	}

	return output.String()
}

func captureDetailedBreakdown(results []evaluations.EvalRunResult, styles help.Styles) string {
	var output strings.Builder

	output.WriteString(h2(styles, "Detailed Breakdown"))

	for i, result := range results {
		output.WriteString(captureEvalDetail(result, styles))
		// Add separator between evals except after the last one
		if i < len(results)-1 {
			output.WriteString(strings.Repeat("─", 80) + "\n")
			output.WriteString("\n")
		}
	}

	return output.String()
}

func captureEvalDetail(result evaluations.EvalRunResult, styles help.Styles) string {
	var output strings.Builder

	// Header
	output.WriteString(h3(styles, result.Eval.Name))

	if result.Eval.Description != "" {
		output.WriteString(styles.Muted.Render(result.Eval.Description) + "\n")
		output.WriteString("\n")
	}

	// Status
	switch {
	case result.Error != nil:
		output.WriteString(fmt.Sprintf("Status: %s\n", styles.Error.Render("ERROR")))
		output.WriteString(fmt.Sprintf("Error: %s\n", result.Error.Error()))
	case result.Grade != nil:
		avg := avgScore(result.Grade)
		statusText := "PASS"
		statusStyle := styles.Success
		if avg < 3.0 {
			statusText = "FAIL"
			statusStyle = styles.Error
		}
		output.WriteString(fmt.Sprintf("Status: %s (%.1f/5)\n", statusStyle.Render(statusText), avg))
	default:
		output.WriteString(fmt.Sprintf("Status: %s\n", styles.Muted.Render("NO GRADE")))
	}
	output.WriteString("\n")

	// Execution trace
	if result.Trace != nil && len(result.Trace.Steps) > 0 {
		output.WriteString(h4(styles, "Execution Trace"))

		for _, step := range result.Trace.Steps {
			output.WriteString(fmt.Sprintf("Step %d: (%s, %s→%s tokens)\n",
				step.StepNumber,
				formatDuration(step.Duration),
				formatTokens(step.InputTokens),
				formatTokens(step.OutputTokens)))

			// Show tool calls
			for _, tool := range step.ToolCalls {
				if tool.Success {
					output.WriteString(fmt.Sprintf("  Tool: %s\n", tool.ToolName))
					output.WriteString(fmt.Sprintf("    %s (%s)\n",
						styles.Success.Render("✓ Success"),
						formatDuration(tool.Duration)))
				} else {
					output.WriteString(fmt.Sprintf("  Tool: %s\n", tool.ToolName))
					output.WriteString(fmt.Sprintf("    %s (%s)\n",
						styles.Error.Render("✗ Failed"),
						formatDuration(tool.Duration)))
					if tool.Error != "" {
						output.WriteString(fmt.Sprintf("    Error: %s\n", tool.Error))
					}
				}
			}

			// Mark final answer step
			if step.StopReason == "end_turn" {
				output.WriteString("  " + styles.Success.Render("→ Final answer") + "\n")
			}
		}
		output.WriteString("\n")
	}

	// Grading details
	if result.Grade != nil {
		output.WriteString(h4(styles, "Grading Details"))

		grades := []struct {
			name  string
			value int
		}{
			{"Accuracy", result.Grade.Accuracy},
			{"Completeness", result.Grade.Completeness},
			{"Relevance", result.Grade.Relevance},
			{"Clarity", result.Grade.Clarity},
			{"Reasoning", result.Grade.Reasoning},
		}

		for _, g := range grades {
			scoreColor := getScoreColor(g.value, styles)
			bar := makeScoreBar(g.value)
			scoredBar := lipgloss.NewStyle().Foreground(scoreColor).Render(bar)
			output.WriteString(fmt.Sprintf("%-13s %d  %s\n", g.name+":", g.value, scoredBar))
		}

		comments := lipgloss.NewStyle().
			Padding(1, 0, 0, 0).
			Render

		paragraph := lipgloss.NewStyle().
			Width(78).
			Padding(1, 0, 0, 2).
			Render

		if result.Grade.OverallComment != "" {
			output.WriteString(comments("Comments:\n"))
			output.WriteString(paragraph(result.Grade.OverallComment) + "\n")
		}
		output.WriteString("\n")
	}

	return output.String()
}

// LoadTraceFile loads a trace file and reconstructs an EvalRunResult
func LoadTraceFile(path string) (evaluations.EvalRunResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return evaluations.EvalRunResult{}, err
	}

	// Try to unmarshal as full EvalRunResult first (new format)
	var fullResult struct {
		Eval  evaluations.Eval         `json:"eval"`
		Grade *evaluations.GradeResult `json:"grade,omitempty"`
		Trace *evaluations.EvalTrace   `json:"trace"`
	}

	if err := json.Unmarshal(data, &fullResult); err == nil && fullResult.Eval.Name != "" {
		// New format with full result
		return evaluations.EvalRunResult{
			Eval:  fullResult.Eval,
			Grade: fullResult.Grade,
			Trace: fullResult.Trace,
		}, nil
	}

	// Fall back to old format (just trace)
	var trace evaluations.EvalTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		return evaluations.EvalRunResult{}, fmt.Errorf("failed to parse trace file: %w", err)
	}

	// Extract eval name from filename
	evalName := strings.TrimSuffix(filepath.Base(path), ".json")

	return evaluations.EvalRunResult{
		Eval: evaluations.Eval{
			Name: evalName,
		},
		Trace: &trace,
	}, nil
}

// Helper functions

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

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func formatTokens(count int) string {
	if count >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(count)/1000000)
	}
	if count >= 1000 {
		return fmt.Sprintf("%.1fk", float64(count)/1000)
	}
	return fmt.Sprintf("%d", count)
}

func formatTokenCounts(input, output int) string {
	return fmt.Sprintf("%s → %s", formatTokens(input), formatTokens(output))
}

func avgScore(grade *evaluations.GradeResult) float64 {
	sum := grade.Accuracy + grade.Completeness + grade.Relevance + grade.Clarity + grade.Reasoning
	return float64(sum) / 5.0
}

func getScoreColor(score int, styles help.Styles) color.Color {
	switch {
	case score >= 4:
		return styles.Success.GetForeground()
	case score == 3:
		return styles.Muted.GetForeground()
	default:
		return styles.Error.GetForeground()
	}
}

func makeScoreBar(score int) string {
	filled := "█"
	empty := "░"
	bar := ""
	for i := 1; i <= 5; i++ {
		if i <= score {
			bar += filled
		} else {
			bar += empty
		}
	}
	return bar
}

func wrapText(text string, width int) string {
	if len(text) <= width {
		return text
	}

	var wrapped strings.Builder
	words := strings.Fields(text)
	lineLen := 0

	for i, word := range words {
		wordLen := len(word)
		if lineLen+wordLen+1 > width && lineLen > 0 {
			wrapped.WriteString("\n               ")
			lineLen = 0
		}
		if i > 0 && lineLen > 0 {
			wrapped.WriteString(" ")
			lineLen++
		}
		wrapped.WriteString(word)
		lineLen += wordLen
	}

	return wrapped.String()
}
