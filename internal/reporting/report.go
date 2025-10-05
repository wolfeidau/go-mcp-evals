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
	evaluations "github.com/wolfeidau/go-mcp-evals"
	"github.com/wolfeidau/go-mcp-evals/internal/help"
)

// PrintStyledReport generates a colorized, styled report from evaluation results
func PrintStyledReport(results []evaluations.EvalRunResult, verbose bool) error {
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

func printReportHeader(styles help.Styles) {
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(help.Charple).
		Padding(0, 1).
		Render("EVALUATION SUMMARY")

	border := strings.Repeat("═", 80)
	fmt.Println(border)
	fmt.Println(header)
	fmt.Println(border)
	fmt.Println()
}

func printSummaryTable(results []evaluations.EvalRunResult, styles help.Styles) {
	// Print table header
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(help.Charple)
	fmt.Printf("%s  %s  %s  %s  %s  %s  %s\n",
		padRight(headerStyle.Render("Name"), 20),
		padRight(headerStyle.Render("Status"), 10),
		padRight(headerStyle.Render("Avg"), 6),
		padRight(headerStyle.Render("Steps"), 6),
		padRight(headerStyle.Render("Tools"), 6),
		padRight(headerStyle.Render("Success%"), 9),
		padRight(headerStyle.Render("Tokens (I→O)"), 18),
	)

	fmt.Println(strings.Repeat("─", 90))

	// Print each result
	for _, result := range results {
		printResultRow(result, styles)
	}

	fmt.Println()
}

func printResultRow(result evaluations.EvalRunResult, styles help.Styles) {
	name := result.Eval.Name
	if len(name) > 20 {
		name = name[:17] + "..."
	}

	// Handle error case
	if result.Error != nil {
		status := styles.Error.Render("ERROR")
		fmt.Printf("%s  %s  %s\n",
			padRight(name, 20),
			padRight(status, 18), // Account for ANSI codes
			result.Error.Error())
		return
	}

	// Handle no trace case
	if result.Trace == nil {
		status := lipgloss.NewStyle().Foreground(help.Squid).Render("NO TRACE")
		fmt.Printf("%s  %s  %s\n",
			padRight(name, 20),
			padRight(status, 18),
			"-")
		return
	}

	// Calculate metrics
	avgScoreVal := 0.0
	statusStr := lipgloss.NewStyle().Foreground(help.Squid).Render("NO GRADE")
	if result.Grade != nil {
		avgScoreVal = avgScore(result.Grade)
		if avgScoreVal >= 3.0 {
			statusStr = lipgloss.NewStyle().Foreground(help.Guac).Render("PASS")
		} else {
			statusStr = styles.Error.Render("FAIL")
		}
	}

	trace := result.Trace
	successRate := calculateToolSuccessRate(trace)

	// Format token counts
	tokenStr := formatTokenCounts(trace.TotalInputTokens, trace.TotalOutputTokens)

	// Print row
	avgStr := "-"
	if result.Grade != nil {
		avgStr = fmt.Sprintf("%.1f", avgScoreVal)
	}

	fmt.Printf("%s  %s  %s  %s  %s  %s     %s\n",
		padRight(name, 20),
		padRight(statusStr, 18), // Account for ANSI codes
		padRight(avgStr, 6),
		padRight(fmt.Sprintf("%d", trace.StepCount), 6),
		padRight(fmt.Sprintf("%d", trace.ToolCallCount), 6),
		padRight(fmt.Sprintf("%d%%", int(successRate)), 9),
		tokenStr,
	)
}

func printOverallStats(results []evaluations.EvalRunResult, styles help.Styles) {
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

	// Print statistics box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(help.Charple).
		Padding(1, 2)

	var statsBuilder strings.Builder

	// Section header
	sectionHeader := styles.Section.Render("Overall Statistics")
	statsBuilder.WriteString(sectionHeader + "\n\n")

	// Total evaluations
	statsBuilder.WriteString(fmt.Sprintf("Total Evaluations: %d\n", totalEvals))

	// Pass/Fail/Error breakdown
	if passCount > 0 {
		passStr := lipgloss.NewStyle().Foreground(help.Guac).Render(fmt.Sprintf("✓ Pass:   %d (%.0f%%)", passCount, float64(passCount)/float64(totalEvals)*100))
		statsBuilder.WriteString(fmt.Sprintf("  %s\n", passStr))
	}
	if failCount > 0 {
		failStr := styles.Error.Render(fmt.Sprintf("✗ Fail:   %d (%.0f%%)", failCount, float64(failCount)/float64(totalEvals)*100))
		statsBuilder.WriteString(fmt.Sprintf("  %s\n", failStr))
	}
	if errorCount > 0 {
		errorStr := styles.Error.Render(fmt.Sprintf("⚠ Error:  %d (%.0f%%)", errorCount, float64(errorCount)/float64(totalEvals)*100))
		statsBuilder.WriteString(fmt.Sprintf("  %s\n", errorStr))
	}
	if noGradeCount > 0 {
		noGradeStr := lipgloss.NewStyle().Foreground(help.Squid).Render(fmt.Sprintf("○ No Grade: %d", noGradeCount))
		statsBuilder.WriteString(fmt.Sprintf("  %s\n", noGradeStr))
	}

	// Performance metrics
	if totalInputTokens > 0 || totalDuration > 0 {
		statsBuilder.WriteString("\n" + styles.Section.Render("Performance Metrics") + "\n")

		if totalDuration > 0 {
			statsBuilder.WriteString(fmt.Sprintf("  Total Duration:     %s\n", formatDuration(totalDuration)))
		}

		if totalInputTokens > 0 {
			statsBuilder.WriteString(fmt.Sprintf("  Total Tokens:       %s (I) → %s (O)\n",
				formatTokens(totalInputTokens),
				formatTokens(totalOutputTokens)))

			avgInput := totalInputTokens / totalEvals
			avgOutput := totalOutputTokens / totalEvals
			statsBuilder.WriteString(fmt.Sprintf("  Avg Tokens/Eval:    %s (I) → %s (O)\n",
				formatTokens(avgInput),
				formatTokens(avgOutput)))
		}
	}

	// Tool execution stats
	if totalToolCalls > 0 {
		statsBuilder.WriteString("\n" + styles.Section.Render("Tool Execution") + "\n")
		statsBuilder.WriteString(fmt.Sprintf("  Total Tool Calls:   %d\n", totalToolCalls))

		successRateOverall := float64(successfulToolCalls) / float64(totalToolCalls) * 100
		successRateStr := fmt.Sprintf("%.0f%% (%d/%d)", successRateOverall, successfulToolCalls, totalToolCalls)
		if successRateOverall >= 80 {
			successRateStr = lipgloss.NewStyle().Foreground(help.Guac).Render(successRateStr)
		} else if successRateOverall < 50 {
			successRateStr = styles.Error.Render(successRateStr)
		}
		statsBuilder.WriteString(fmt.Sprintf("  Success Rate:       %s\n", successRateStr))

		if totalToolCalls > successfulToolCalls {
			failedCalls := totalToolCalls - successfulToolCalls
			statsBuilder.WriteString(fmt.Sprintf("  Failed Calls:       %s\n",
				styles.Error.Render(fmt.Sprintf("%d", failedCalls))))
		}
	}

	fmt.Println(boxStyle.Render(statsBuilder.String()))
	fmt.Println()
}

func printDetailedBreakdown(results []evaluations.EvalRunResult, styles help.Styles) {
	fmt.Println(styles.Section.Render("Detailed Breakdown"))
	fmt.Println()

	for _, result := range results {
		printEvalDetail(result, styles)
	}
}

func printEvalDetail(result evaluations.EvalRunResult, styles help.Styles) {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(help.Charple).
		Padding(1, 2).
		Width(80)

	var content strings.Builder

	// Header
	header := lipgloss.NewStyle().Bold(true).Foreground(help.Charple).Render(fmt.Sprintf("Eval: %s", result.Eval.Name))
	content.WriteString(header + "\n")

	if result.Eval.Description != "" {
		content.WriteString(lipgloss.NewStyle().Foreground(help.Squid).Render(result.Eval.Description) + "\n")
	}
	content.WriteString("\n")

	// Status
	switch {
	case result.Error != nil:
		content.WriteString(fmt.Sprintf("Status: %s\n", styles.Error.Render("ERROR")))
		content.WriteString(fmt.Sprintf("Error: %s\n", result.Error.Error()))
	case result.Grade != nil:
		avg := avgScore(result.Grade)
		statusText := "PASS"
		statusStyle := lipgloss.NewStyle().Foreground(help.Guac)
		if avg < 3.0 {
			statusText = "FAIL"
			statusStyle = styles.Error
		}
		content.WriteString(fmt.Sprintf("Status: %s (%.1f/5)\n", statusStyle.Render(statusText), avg))
	default:
		content.WriteString(fmt.Sprintf("Status: %s\n", lipgloss.NewStyle().Foreground(help.Squid).Render("NO GRADE")))
	}
	content.WriteString("\n")

	// Execution trace
	if result.Trace != nil && len(result.Trace.Steps) > 0 {
		content.WriteString(styles.Section.Render("Execution Trace:") + "\n")

		for _, step := range result.Trace.Steps {
			content.WriteString(fmt.Sprintf("  Step %d: (%s, %s→%s tokens)\n",
				step.StepNumber,
				formatDuration(step.Duration),
				formatTokens(step.InputTokens),
				formatTokens(step.OutputTokens)))

			// Show tool calls
			for _, tool := range step.ToolCalls {
				if tool.Success {
					content.WriteString(fmt.Sprintf("    Tool: %s\n", tool.ToolName))
					content.WriteString(fmt.Sprintf("      %s (%s)\n",
						lipgloss.NewStyle().Foreground(help.Guac).Render("✓ Success"),
						formatDuration(tool.Duration)))
				} else {
					content.WriteString(fmt.Sprintf("    Tool: %s\n", tool.ToolName))
					content.WriteString(fmt.Sprintf("      %s (%s)\n",
						styles.Error.Render("✗ Failed"),
						formatDuration(tool.Duration)))
					if tool.Error != "" {
						content.WriteString(fmt.Sprintf("      Error: %s\n", tool.Error))
					}
				}
			}

			// Mark final answer step
			if step.StopReason == "end_turn" {
				content.WriteString("    " + lipgloss.NewStyle().Foreground(help.Guac).Render("→ Final answer") + "\n")
			}
		}
		content.WriteString("\n")
	}

	// Grading details
	if result.Grade != nil {
		content.WriteString(styles.Section.Render("Grading Details:") + "\n")

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
			scoreColor := getScoreColor(g.value)
			bar := makeScoreBar(g.value)
			scoredBar := lipgloss.NewStyle().Foreground(scoreColor).Render(bar)
			content.WriteString(fmt.Sprintf("  %-13s %d  %s\n", g.name+":", g.value, scoredBar))
		}

		if result.Grade.OverallComment != "" {
			content.WriteString(fmt.Sprintf("\n  Comments: %s\n",
				wrapText(result.Grade.OverallComment, 70)))
		}
	}

	fmt.Println(boxStyle.Render(content.String()))
	fmt.Println()
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

func getScoreColor(score int) color.Color {
	switch {
	case score >= 4:
		return help.Guac // Green
	case score == 3:
		return help.Squid // Gray
	default:
		return help.Cardinal // Red
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

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
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
