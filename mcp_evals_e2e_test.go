//go:build e2e

package evaluations

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestE2E_BasicEvaluation(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping e2e test")
	}

	// Build test server
	serverPath := buildTestServer(t)

	// Configure eval client
	config := EvalClientConfig{
		APIKey:  apiKey,
		Command: serverPath,
		Args:    []string{},
		Env:     []string{},
		Model:   "claude-3-5-sonnet-20241022",
	}

	client := NewEvalClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Run evaluation
	evalRunResult, err := client.RunEval(ctx, Eval{
		Name:           "basic_addition",
		Description:    "Test basic addition",
		Prompt:         "What is 5 plus 3?",
		ExpectedResult: "Should return 8",
	})
	if err != nil {
		t.Fatalf("RunEval failed: %v", err)
	}

	// Check for errors in the result
	if evalRunResult.Error != nil {
		t.Fatalf("Eval execution error: %v", evalRunResult.Error)
	}

	// Verify result
	if evalRunResult.Result == nil || evalRunResult.Result.RawResponse == "" {
		t.Fatal("Expected non-empty result")
	}

	// Check if answer contains expected value
	if !strings.Contains(evalRunResult.Result.RawResponse, "8") {
		t.Errorf("Expected answer to contain '8', got: %s", evalRunResult.Result.RawResponse)
	}

	t.Logf("Evaluation result: %s", evalRunResult.Result.RawResponse)

	// Validate grade structure (auto-graded)
	if evalRunResult.Grade == nil {
		t.Fatal("Expected grade to be auto-generated")
	}
	validateGrade(t, evalRunResult.Grade)
	t.Logf("Grade: Accuracy=%d, Completeness=%d, Relevance=%d, Clarity=%d, Reasoning=%d",
		evalRunResult.Grade.Accuracy, evalRunResult.Grade.Completeness, evalRunResult.Grade.Relevance,
		evalRunResult.Grade.Clarity, evalRunResult.Grade.Reasoning)
}

func TestE2E_MultipleTools(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping e2e test")
	}

	// Build test server
	serverPath := buildTestServer(t)

	// Configure eval client
	config := EvalClientConfig{
		APIKey:  apiKey,
		Command: serverPath,
		Args:    []string{},
		Env:     []string{},
		Model:   "claude-3-5-sonnet-20241022",
	}

	client := NewEvalClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Run evaluation with multiple tools
	evalRunResult, err := client.RunEval(ctx, Eval{
		Name:           "multiple_tools",
		Description:    "Test using multiple tools in sequence",
		Prompt:         "Echo the message 'hello world' and tell me what time it is",
		ExpectedResult: "Should echo 'hello world' and provide current time",
	})
	if err != nil {
		t.Fatalf("RunEval failed: %v", err)
	}

	// Check for errors in the result
	if evalRunResult.Error != nil {
		t.Fatalf("Eval execution error: %v", evalRunResult.Error)
	}

	// Verify result
	if evalRunResult.Result == nil || evalRunResult.Result.RawResponse == "" {
		t.Fatal("Expected non-empty result")
	}

	// Check if answer contains expected content
	if !strings.Contains(strings.ToLower(evalRunResult.Result.RawResponse), "hello world") {
		t.Errorf("Expected answer to contain 'hello world', got: %s", evalRunResult.Result.RawResponse)
	}

	t.Logf("Evaluation result: %s", evalRunResult.Result.RawResponse)

	// Validate grade structure (auto-graded)
	if evalRunResult.Grade == nil {
		t.Fatal("Expected grade to be auto-generated")
	}
	validateGrade(t, evalRunResult.Grade)
	t.Logf("Grade: Accuracy=%d, Completeness=%d, Relevance=%d, Clarity=%d, Reasoning=%d",
		evalRunResult.Grade.Accuracy, evalRunResult.Grade.Completeness, evalRunResult.Grade.Relevance,
		evalRunResult.Grade.Clarity, evalRunResult.Grade.Reasoning)
}

func TestE2E_EnvironmentVariables(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping e2e test")
	}

	// Build test server
	serverPath := buildTestServer(t)

	// Configure eval client with custom environment variable
	testToken := "test-secret-token-12345"
	config := EvalClientConfig{
		APIKey:  apiKey,
		Command: serverPath,
		Args:    []string{},
		Env:     []string{"TEST_API_TOKEN=" + testToken},
		Model:   "claude-3-5-sonnet-20241022",
	}

	client := NewEvalClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Run evaluation that requires checking an environment variable
	evalRunResult, err := client.RunEval(ctx, Eval{
		Name:           "environment_variables",
		Description:    "Test accessing custom environment variables",
		Prompt:         "What is the value of the TEST_API_TOKEN environment variable?",
		ExpectedResult: "Should return '" + testToken + "'",
	})
	if err != nil {
		t.Fatalf("RunEval failed: %v", err)
	}

	// Check for errors in the result
	if evalRunResult.Error != nil {
		t.Fatalf("Eval execution error: %v", evalRunResult.Error)
	}

	// Verify result contains the token value
	if evalRunResult.Result == nil || evalRunResult.Result.RawResponse == "" {
		t.Fatal("Expected non-empty result")
	}

	// Check if answer contains the test token
	if !strings.Contains(evalRunResult.Result.RawResponse, testToken) {
		t.Errorf("Expected answer to contain test token '%s', got: %s", testToken, evalRunResult.Result.RawResponse)
	}

	t.Logf("Evaluation result: %s", evalRunResult.Result.RawResponse)

	// Validate grade structure (auto-graded)
	if evalRunResult.Grade == nil {
		t.Fatal("Expected grade to be auto-generated")
	}
	validateGrade(t, evalRunResult.Grade)
	t.Logf("Grade: Accuracy=%d, Completeness=%d, Relevance=%d, Clarity=%d, Reasoning=%d",
		evalRunResult.Grade.Accuracy, evalRunResult.Grade.Completeness, evalRunResult.Grade.Relevance,
		evalRunResult.Grade.Clarity, evalRunResult.Grade.Reasoning)
}

func TestE2E_GradingScores(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping e2e test")
	}

	// Build test server
	serverPath := buildTestServer(t)

	// Configure eval client
	config := EvalClientConfig{
		APIKey:  apiKey,
		Command: serverPath,
		Args:    []string{},
		Env:     []string{},
		Model:   "claude-3-5-sonnet-20241022",
	}

	client := NewEvalClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Run evaluation
	evalRunResult, err := client.RunEval(ctx, Eval{
		Name:           "grading_test",
		Description:    "Test grading scores for correct answer",
		Prompt:         "What is 10 plus 20?",
		ExpectedResult: "Should return 30",
	})
	if err != nil {
		t.Fatalf("RunEval failed: %v", err)
	}

	// Check for errors in the result
	if evalRunResult.Error != nil {
		t.Fatalf("Eval execution error: %v", evalRunResult.Error)
	}

	// Validate grade structure and scores (auto-graded)
	if evalRunResult.Grade == nil {
		t.Fatal("Expected grade to be auto-generated")
	}
	validateGrade(t, evalRunResult.Grade)

	// Check that scores are reasonable for a correct answer
	// We expect high scores since the answer should be correct
	if evalRunResult.Grade.Accuracy < 3 {
		t.Errorf("Expected accuracy >= 3 for correct answer, got %d", evalRunResult.Grade.Accuracy)
	}

	t.Logf("Grade details:")
	t.Logf("  Accuracy: %d", evalRunResult.Grade.Accuracy)
	t.Logf("  Completeness: %d", evalRunResult.Grade.Completeness)
	t.Logf("  Relevance: %d", evalRunResult.Grade.Relevance)
	t.Logf("  Clarity: %d", evalRunResult.Grade.Clarity)
	t.Logf("  Reasoning: %d", evalRunResult.Grade.Reasoning)
	t.Logf("  Overall: %s", evalRunResult.Grade.OverallComment)
}

// buildTestServer builds the test MCP server and returns the path to the binary
func buildTestServer(t *testing.T) string {
	t.Helper()

	serverDir := filepath.Join("testdata", "mcp-test-server")
	outputPath := filepath.Join(t.TempDir(), "test-server")

	cmd := exec.Command("go", "build", "-o", outputPath, ".")
	cmd.Dir = serverDir

	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build test server: %v\n%s", err, output)
	}

	return outputPath
}

func TestE2E_LoadConfigAndRunEvals(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping e2e test")
	}

	// Build test server
	serverPath := buildTestServer(t)

	// Load config from YAML
	config, err := LoadConfig("testdata/mcp-test-evals.yaml")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Override the command to use the built test server
	config.MCPServer.Command = serverPath
	config.MCPServer.Args = []string{}

	// Create eval client from config
	evalConfig := EvalClientConfig{
		APIKey:  apiKey,
		Command: config.MCPServer.Command,
		Args:    config.MCPServer.Args,
		Env:     config.MCPServer.Env,
		Model:   config.Model,
	}
	client := NewEvalClient(evalConfig)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Run all evals from config
	results, err := client.RunEvals(ctx, config.Evals)
	if err != nil {
		t.Fatalf("RunEvals failed: %v", err)
	}

	// Verify we got results for all evals
	if len(results) != len(config.Evals) {
		t.Errorf("expected %d results, got %d", len(config.Evals), len(results))
	}

	// Check each result
	for i, result := range results {
		t.Logf("Eval %d: %s", i, result.Eval.Name)

		if result.Error != nil {
			t.Errorf("Eval %s failed: %v", result.Eval.Name, result.Error)
			continue
		}

		if result.Result == nil {
			t.Errorf("Eval %s has no result", result.Eval.Name)
			continue
		}

		if result.Result.RawResponse == "" {
			t.Errorf("Eval %s has empty response", result.Eval.Name)
			continue
		}

		if result.Grade == nil {
			t.Errorf("Eval %s has no grade", result.Eval.Name)
			continue
		}

		validateGrade(t, result.Grade)
		t.Logf("  Response: %s", result.Result.RawResponse)
		t.Logf("  Grade: Accuracy=%d, Completeness=%d, Relevance=%d, Clarity=%d, Reasoning=%d",
			result.Grade.Accuracy, result.Grade.Completeness, result.Grade.Relevance,
			result.Grade.Clarity, result.Grade.Reasoning)
	}

	// Verify specific results based on eval names
	evalsByName := make(map[string]EvalRunResult)
	for _, result := range results {
		evalsByName[result.Eval.Name] = result
	}

	// Check "add" eval
	if addResult, ok := evalsByName["add"]; ok {
		if !strings.Contains(addResult.Result.RawResponse, "8") {
			t.Errorf("Expected 'add' eval to contain '8', got: %s", addResult.Result.RawResponse)
		}
	} else {
		t.Error("Missing 'add' eval in results")
	}

	// Check "multiply" eval
	if multiplyResult, ok := evalsByName["multiply"]; ok {
		if !strings.Contains(multiplyResult.Result.RawResponse, "42") {
			t.Errorf("Expected 'multiply' eval to contain '42', got: %s", multiplyResult.Result.RawResponse)
		}
	} else {
		t.Error("Missing 'multiply' eval in results")
	}
}

// validateGrade validates that a GradeResult has all required fields and valid values
func validateGrade(t *testing.T, grade *GradeResult) {
	t.Helper()

	if grade == nil {
		t.Fatal("Expected non-nil grade")
	}

	dimensions := []struct {
		name  string
		score int
	}{
		{"accuracy", grade.Accuracy},
		{"completeness", grade.Completeness},
		{"relevance", grade.Relevance},
		{"clarity", grade.Clarity},
		{"reasoning", grade.Reasoning},
	}

	for _, dim := range dimensions {
		if dim.score < 0 || dim.score > 5 {
			t.Errorf("%s score %d out of valid range [0-5]", dim.name, dim.score)
		}
	}

	if grade.OverallComment == "" {
		t.Error("overall_comments is empty")
	}
}
