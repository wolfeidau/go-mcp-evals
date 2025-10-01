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
	evalResult, err := client.RunEval(ctx, "What is 5 plus 3?")
	if err != nil {
		t.Fatalf("RunEval failed: %v", err)
	}

	// Verify result
	if evalResult.RawResponse == "" {
		t.Fatal("Expected non-empty result")
	}

	// Check if answer contains expected value
	if !strings.Contains(evalResult.RawResponse, "8") {
		t.Errorf("Expected answer to contain '8', got: %s", evalResult.RawResponse)
	}

	t.Logf("Evaluation result: %s", evalResult.RawResponse)

	// Grade the result
	grade, err := client.Grade(ctx, evalResult)
	if err != nil {
		t.Fatalf("Grade failed: %v", err)
	}

	// Validate grade structure
	validateGrade(t, grade)
	t.Logf("Grade: Accuracy=%d, Completeness=%d, Relevance=%d, Clarity=%d, Reasoning=%d",
		grade.Accuracy, grade.Completeness, grade.Relevance, grade.Clarity, grade.Reasoning)
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
	evalResult, err := client.RunEval(ctx, "Echo the message 'hello world' and tell me what time it is")
	if err != nil {
		t.Fatalf("RunEval failed: %v", err)
	}

	// Verify result
	if evalResult.RawResponse == "" {
		t.Fatal("Expected non-empty result")
	}

	// Check if answer contains expected content
	if !strings.Contains(strings.ToLower(evalResult.RawResponse), "hello world") {
		t.Errorf("Expected answer to contain 'hello world', got: %s", evalResult.RawResponse)
	}

	t.Logf("Evaluation result: %s", evalResult.RawResponse)

	// Grade the result
	grade, err := client.Grade(ctx, evalResult)
	if err != nil {
		t.Fatalf("Grade failed: %v", err)
	}

	// Validate grade structure
	validateGrade(t, grade)
	t.Logf("Grade: Accuracy=%d, Completeness=%d, Relevance=%d, Clarity=%d, Reasoning=%d",
		grade.Accuracy, grade.Completeness, grade.Relevance, grade.Clarity, grade.Reasoning)
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
	evalResult, err := client.RunEval(ctx, "What is the value of the TEST_API_TOKEN environment variable?")
	if err != nil {
		t.Fatalf("RunEval failed: %v", err)
	}

	// Verify result contains the token value
	if evalResult.RawResponse == "" {
		t.Fatal("Expected non-empty result")
	}

	// Check if answer contains the test token
	if !strings.Contains(evalResult.RawResponse, testToken) {
		t.Errorf("Expected answer to contain test token '%s', got: %s", testToken, evalResult.RawResponse)
	}

	t.Logf("Evaluation result: %s", evalResult.RawResponse)

	// Grade the result
	grade, err := client.Grade(ctx, evalResult)
	if err != nil {
		t.Fatalf("Grade failed: %v", err)
	}

	// Validate grade structure
	validateGrade(t, grade)
	t.Logf("Grade: Accuracy=%d, Completeness=%d, Relevance=%d, Clarity=%d, Reasoning=%d",
		grade.Accuracy, grade.Completeness, grade.Relevance, grade.Clarity, grade.Reasoning)
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
	evalResult, err := client.RunEval(ctx, "What is 10 plus 20?")
	if err != nil {
		t.Fatalf("RunEval failed: %v", err)
	}

	// Grade the result
	grade, err := client.Grade(ctx, evalResult)
	if err != nil {
		t.Fatalf("Grade failed: %v", err)
	}

	// Validate grade structure and scores
	validateGrade(t, grade)

	// Check that scores are reasonable for a correct answer
	// We expect high scores since the answer should be correct
	if grade.Accuracy < 3 {
		t.Errorf("Expected accuracy >= 3 for correct answer, got %d", grade.Accuracy)
	}

	t.Logf("Grade details:")
	t.Logf("  Accuracy: %d", grade.Accuracy)
	t.Logf("  Completeness: %d", grade.Completeness)
	t.Logf("  Relevance: %d", grade.Relevance)
	t.Logf("  Clarity: %d", grade.Clarity)
	t.Logf("  Reasoning: %d", grade.Reasoning)
	t.Logf("  Overall: %s", grade.OverallComment)
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
