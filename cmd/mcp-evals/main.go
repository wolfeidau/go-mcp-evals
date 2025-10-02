package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	evaluations "github.com/wolfeidau/go-mcp-evals"
)

func main() {
	// If no args or starts with a flag, default to run command
	if len(os.Args) < 2 || (len(os.Args) >= 2 && os.Args[1][0] == '-') {
		if err := runCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	switch os.Args[1] {
	case "run":
		if err := runCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "validate":
		if err := validateCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "schema":
		if err := schemaCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func validateCommand() error {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to evaluation configuration file (YAML or JSON)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: mcp-evals validate --config <path>\n\n")
		fmt.Fprintf(os.Stderr, "Validate evaluation configuration file against JSON schema.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	if *configPath == "" {
		return fmt.Errorf("--config flag is required")
	}

	// Validate the config file
	result, err := evaluations.ValidateConfigFile(*configPath)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	if result.Valid {
		fmt.Printf("✓ Configuration is valid: %s\n", *configPath)
		return nil
	}

	// Print validation errors
	fmt.Printf("✗ Configuration has %d error(s):\n\n", len(result.Errors))
	for i, verr := range result.Errors {
		if verr.Path != "" {
			fmt.Printf("%d. [%s] %s\n", i+1, verr.Path, verr.Message)
		} else {
			fmt.Printf("%d. %s\n", i+1, verr.Message)
		}
	}
	fmt.Println()

	return fmt.Errorf("validation failed")
}

func schemaCommand() error {
	fs := flag.NewFlagSet("schema", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: mcp-evals schema\n")
		fmt.Fprintf(os.Stderr, "\nGenerate JSON schema for evaluation configuration.\n")
	}
	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	schema, err := evaluations.SchemaForEvalConfig()
	if err != nil {
		return fmt.Errorf("failed to generate schema: %w", err)
	}

	fmt.Println(schema)
	return nil
}

func runCommand() error {
	// Determine starting position for flag parsing
	args := os.Args[1:]
	if len(os.Args) >= 2 && os.Args[1] == "run" {
		args = os.Args[2:]
	}

	fs := flag.NewFlagSet("run", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to evaluation configuration file (YAML or JSON)")
	apiKey := fs.String("api-key", "", "Anthropic API key (overrides ANTHROPIC_API_KEY env var)")
	traceDir := fs.String("trace-dir", "", "Directory to write trace files (optional)")
	quiet := fs.Bool("quiet", false, "Suppress progress output, only show summary")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: mcp-evals run [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Run evaluations against an MCP server.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *configPath == "" {
		return fmt.Errorf("--config flag is required")
	}

	// Load configuration
	config, err := evaluations.LoadConfig(*configPath)
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

	// Create client
	client := createClient(config, *apiKey)

	// Run evaluations
	if !*quiet {
		fmt.Printf("Running %d evaluation(s)...\n\n", len(config.Evals))
	}

	results, err := runEvals(ctx, client, config.Evals, *quiet)
	if err != nil {
		return err
	}

	// Write traces if directory specified
	if *traceDir != "" {
		if err := writeTraces(results, *traceDir); err != nil {
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

func createClient(config *evaluations.EvalConfig, apiKey string) *evaluations.EvalClient {
	clientConfig := evaluations.EvalClientConfig{
		APIKey:       apiKey,
		Command:      config.MCPServer.Command,
		Args:         config.MCPServer.Args,
		Env:          config.MCPServer.Env,
		Model:        config.Model,
		GradingModel: config.GradingModel,
		MaxSteps:     int(config.MaxSteps),
		MaxTokens:    int(config.MaxTokens),
	}

	return evaluations.NewEvalClient(clientConfig)
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

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: mcp-evals <command> [flags]\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  run       Run evaluations against an MCP server (default)\n")
	fmt.Fprintf(os.Stderr, "  validate  Validate configuration file against JSON schema\n")
	fmt.Fprintf(os.Stderr, "  schema    Generate JSON schema for evaluation configuration\n")
	fmt.Fprintf(os.Stderr, "  help      Show this help message\n\n")
	fmt.Fprintf(os.Stderr, "Run 'mcp-evals <command> --help' for more information on a command.\n")
}
