package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	evaluations "github.com/wolfeidau/go-mcp-evals"
)

// RunCmd handles the run command
type RunCmd struct {
	Config  string `help:"Path to evaluation configuration file (YAML or JSON)" required:"" type:"path"`
	APIKey  string `help:"Anthropic API key (overrides ANTHROPIC_API_KEY env var)"`
	BaseURL string `help:"Base URL for Anthropic API (overrides ANTHROPIC_BASE_URL env var)"`
}

// Run executes the run command
func (r *RunCmd) Run(globals *Globals) error {
	// Load configuration
	config, err := evaluations.LoadConfig(r.Config)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Parse timeout if specified
	var timeout time.Duration
	if config.Timeout != "" {
		timeout, err = time.ParseDuration(config.Timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout: %w", err)
		}
	}

	// Create context with timeout
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Resolve base URL: flag takes precedence, then env var
	resolvedBaseURL := r.BaseURL
	if resolvedBaseURL == "" {
		resolvedBaseURL = os.Getenv("ANTHROPIC_BASE_URL")
	}

	// Create client
	client := createClient(config, r.APIKey, resolvedBaseURL)

	// Run evaluations
	if !globals.Quiet {
		fmt.Printf("Running %d evaluation(s)...\n\n", len(config.Evals))
	}

	results, err := runEvals(ctx, client, config.Evals, globals.Quiet)
	if err != nil {
		return err
	}

	// Write traces if directory specified
	if globals.TraceDir != "" {
		if err := writeTraces(results, globals.TraceDir); err != nil {
			log.Error().Err(err).Msg("failed to write traces")
			return fmt.Errorf("failed to write traces: %w", err)
		}
	}

	// Print summary and determine exit code
	exitCode := printSummary(results)
	if exitCode != 0 {
		return fmt.Errorf("evaluations failed")
	}

	return nil
}

func runEvals(ctx context.Context, client *evaluations.EvalClient, evals []evaluations.Eval, quiet bool) ([]evaluations.EvalRunResult, error) {
	results := make([]evaluations.EvalRunResult, len(evals))

	for i, eval := range evals {
		if !quiet {
			fmt.Printf("[%d/%d] Running eval: %s\n", i+1, len(evals), eval.Name)
			if eval.Description != "" {
				fmt.Printf("        %s\n", eval.Description)
			}
		}

		result, err := client.RunEval(ctx, eval)
		if err != nil {
			results[i] = evaluations.EvalRunResult{
				Eval:  eval,
				Error: err,
			}
			if !quiet {
				fmt.Printf("        ❌ Error: %v\n\n", err)
			}
			continue
		}

		results[i] = *result

		if !quiet {
			if result.Grade != nil {
				fmt.Printf("        ✓ Completed (avg score: %.1f/5)\n\n", avgScore(result.Grade))
			} else {
				fmt.Printf("        ✓ Completed\n\n")
			}
		}
	}

	return results, nil
}

func writeTraces(results []evaluations.EvalRunResult, traceDir string) error {
	// Create trace directory if it doesn't exist
	if err := os.MkdirAll(traceDir, 0755); err != nil {
		return fmt.Errorf("failed to create trace directory: %w", err)
	}

	for _, result := range results {
		if result.Trace == nil {
			continue
		}

		filename := filepath.Join(traceDir, fmt.Sprintf("%s.json", result.Eval.Name))
		data, err := json.MarshalIndent(result.Trace, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal trace for %s: %w", result.Eval.Name, err)
		}

		// #nosec G306 - Trace files are meant to be readable by others for debugging
		if err := os.WriteFile(filename, data, 0644); err != nil {
			return fmt.Errorf("failed to write trace for %s: %w", result.Eval.Name, err)
		}
	}

	return nil
}

func printSummary(results []evaluations.EvalRunResult) int {
	fmt.Println("=" + repeatString("=", 79))
	fmt.Println("EVALUATION SUMMARY")
	fmt.Println("=" + repeatString("=", 79))
	fmt.Println()

	// Print header
	fmt.Printf("%-20s %-10s %-8s %-8s %-8s %-8s %-8s\n",
		"Name", "Status", "Acc", "Comp", "Rel", "Clar", "Reas")
	fmt.Println(repeatString("-", 80))

	hasFailures := false
	for _, result := range results {
		name := result.Eval.Name
		if len(name) > 20 {
			name = name[:17] + "..."
		}

		if result.Error != nil {
			fmt.Printf("%-20s %-10s %s\n", name, "ERROR", result.Error.Error())
			hasFailures = true
			continue
		}

		if result.Grade == nil {
			fmt.Printf("%-20s %-10s %s\n", name, "NO GRADE", "-")
			continue
		}

		grade := result.Grade
		status := "PASS"
		avg := avgScore(grade)
		if avg < 3.0 {
			status = "FAIL"
			hasFailures = true
		}

		fmt.Printf("%-20s %-10s %-8d %-8d %-8d %-8d %-8d\n",
			name, status,
			grade.Accuracy, grade.Completeness, grade.Relevance,
			grade.Clarity, grade.Reasoning)
	}

	fmt.Println()

	// Calculate overall statistics
	totalEvals := len(results)
	errorCount := 0
	passCount := 0
	failCount := 0

	for _, result := range results {
		if result.Error != nil {
			errorCount++
		} else if result.Grade != nil {
			if avgScore(result.Grade) >= 3.0 {
				passCount++
			} else {
				failCount++
			}
		}
	}

	fmt.Printf("Total: %d | Pass: %d | Fail: %d | Error: %d\n",
		totalEvals, passCount, failCount, errorCount)
	fmt.Println()

	if hasFailures {
		return 1
	}
	return 0
}

func avgScore(grade *evaluations.GradeResult) float64 {
	sum := grade.Accuracy + grade.Completeness + grade.Relevance + grade.Clarity + grade.Reasoning
	return float64(sum) / 5.0
}

func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
