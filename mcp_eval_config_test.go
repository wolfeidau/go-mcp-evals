package evaluations

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLoadConfig_YAML(t *testing.T) {
	config, err := LoadConfig("testdata/mcp-test-evals.yaml")
	if err != nil {
		t.Fatalf("failed to load YAML config: %v", err)
	}

	// Verify basic fields
	if config.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("expected model 'claude-3-5-sonnet-20241022', got %q", config.Model)
	}
	if config.Timeout != "2m" {
		t.Errorf("expected timeout '2m', got %q", config.Timeout)
	}
	if config.MaxSteps != 10 {
		t.Errorf("expected max_steps 10, got %d", config.MaxSteps)
	}
	if config.MaxTokens != 4096 {
		t.Errorf("expected max_tokens 4096, got %d", config.MaxTokens)
	}

	// Verify MCP server config
	if config.MCPServer.Command != "go" {
		t.Errorf("expected command 'go', got %q", config.MCPServer.Command)
	}
	if len(config.MCPServer.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(config.MCPServer.Args))
	}
	if len(config.MCPServer.Env) != 1 {
		t.Errorf("expected 1 env var, got %d", len(config.MCPServer.Env))
	}

	// Verify evals
	if len(config.Evals) != 3 {
		t.Fatalf("expected 3 evals, got %d", len(config.Evals))
	}

	firstEval := config.Evals[0]
	if firstEval.Name != "add" {
		t.Errorf("expected first eval name 'add', got %q", firstEval.Name)
	}
	if firstEval.Prompt != "What is 5 plus 3?" {
		t.Errorf("expected first eval prompt 'What is 5 plus 3?', got %q", firstEval.Prompt)
	}
	if firstEval.ExpectedResult != "Should return 8" {
		t.Errorf("expected first eval expected_result 'Should return 8', got %q", firstEval.ExpectedResult)
	}
}

func TestLoadConfig_InvalidFile(t *testing.T) {
	_, err := LoadConfig("testdata/nonexistent.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadConfig_InvalidExtension(t *testing.T) {
	_, err := LoadConfig("testdata/test.txt")
	if err == nil {
		t.Error("expected error for invalid extension")
	}
	if !strings.Contains(err.Error(), "unsupported file extension") {
		t.Errorf("expected 'unsupported file extension' error, got: %v", err)
	}
}

func TestEvalClientConfig_Defaults(t *testing.T) {
	tests := []struct {
		name           string
		config         EvalClientConfig
		expectedSteps  int
		expectedTokens int
	}{
		{
			name: "applies defaults when not set",
			config: EvalClientConfig{
				Command: "echo",
				Model:   "test-model",
			},
			expectedSteps:  10,
			expectedTokens: 4096,
		},
		{
			name: "applies defaults when zero",
			config: EvalClientConfig{
				Command:   "echo",
				Model:     "test-model",
				MaxSteps:  0,
				MaxTokens: 0,
			},
			expectedSteps:  10,
			expectedTokens: 4096,
		},
		{
			name: "respects custom values",
			config: EvalClientConfig{
				Command:   "echo",
				Model:     "test-model",
				MaxSteps:  5,
				MaxTokens: 2048,
			},
			expectedSteps:  5,
			expectedTokens: 2048,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewEvalClient(tt.config)

			if client.config.MaxSteps != tt.expectedSteps {
				t.Errorf("expected MaxSteps %d, got %d", tt.expectedSteps, client.config.MaxSteps)
			}
			if client.config.MaxTokens != tt.expectedTokens {
				t.Errorf("expected MaxTokens %d, got %d", tt.expectedTokens, client.config.MaxTokens)
			}
		})
	}
}

func TestSchemaForEvalConfig(t *testing.T) {
	schema, err := SchemaForEvalConfig()
	if err != nil {
		t.Fatalf("SchemaForEvalConfig() returned error: %v", err)
	}

	if schema == "" {
		t.Fatal("SchemaForEvalConfig() returned empty schema")
	}

	t.Log("Generated JSON Schema:\n", schema)

	// Verify it's valid JSON
	var schemaMap map[string]any
	if err := json.Unmarshal([]byte(schema), &schemaMap); err != nil {
		t.Fatalf("SchemaForEvalConfig() returned invalid JSON: %v", err)
	}

	// Verify top-level metadata fields
	if schemaURL, ok := schemaMap["$schema"].(string); !ok || schemaURL != "http://json-schema.org/draft-07/schema#" {
		t.Errorf("expected $schema to be 'http://json-schema.org/draft-07/schema#', got %v", schemaMap["$schema"])
	}
	if title, ok := schemaMap["title"].(string); !ok || title != "MCP Evaluation Configuration" {
		t.Errorf("expected title to be 'MCP Evaluation Configuration', got %v", schemaMap["title"])
	}
	if desc, ok := schemaMap["description"].(string); !ok || desc == "" {
		t.Error("expected non-empty description field")
	}

	// Verify it has expected JSON schema fields
	if _, ok := schemaMap["properties"]; !ok {
		t.Error("schema missing 'properties' field")
	}

	// Verify it contains expected EvalConfig properties
	properties, ok := schemaMap["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties field is not a map")
	}

	expectedProperties := []string{"model", "grading_model", "timeout", "max_steps", "max_tokens", "mcp_server", "evals"}
	for _, prop := range expectedProperties {
		if _, ok := properties[prop]; !ok {
			t.Errorf("schema missing expected property: %s", prop)
		}
	}

	// Verify descriptions are present on key fields
	testCases := []struct {
		path        []string
		description string
	}{
		{[]string{"model", "description"}, "Anthropic model ID"},
		{[]string{"timeout", "description"}, "Timeout duration"},
		{[]string{"mcp_server", "description"}, "Configuration for the MCP server"},
		{[]string{"evals", "description"}, "List of evaluation test cases"},
	}

	for _, tc := range testCases {
		var current any = properties
		for i, key := range tc.path {
			m, ok := current.(map[string]any)
			if !ok {
				t.Errorf("path %v: expected map at level %d, got %T", tc.path, i, current)
				break
			}
			current, ok = m[key]
			if !ok {
				t.Errorf("path %v: key %q not found", tc.path, key)
				break
			}
		}
		if desc, ok := current.(string); ok {
			if !strings.Contains(desc, tc.description) {
				t.Errorf("path %v: expected description to contain %q, got %q", tc.path, tc.description, desc)
			}
		}
	}
}
