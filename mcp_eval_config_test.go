package evaluations

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfig_YAML(t *testing.T) {
	assert := require.New(t)

	config, err := LoadConfig("testdata/mcp-test-evals.yaml")
	assert.NoError(err)

	// Verify basic fields
	assert.Equal("claude-3-5-sonnet-20241022", config.Model)
	assert.Equal("2m", config.Timeout)
	assert.EqualValues(10, config.MaxSteps)
	assert.EqualValues(4096, config.MaxTokens)

	// Verify MCP server config
	assert.Equal("go", config.MCPServer.Command)
	assert.Len(config.MCPServer.Args, 2)
	assert.Len(config.MCPServer.Env, 1)

	// Verify evals
	assert.Len(config.Evals, 3)

	firstEval := config.Evals[0]
	assert.Equal("add", firstEval.Name)
	assert.Equal("What is 5 plus 3?", firstEval.Prompt)
	assert.Equal("Should return 8", firstEval.ExpectedResult)
}

func TestLoadConfig_InvalidFile(t *testing.T) {
	assert := require.New(t)

	_, err := LoadConfig("testdata/nonexistent.yaml")
	assert.Error(err)
}

func TestLoadConfig_InvalidExtension(t *testing.T) {
	assert := require.New(t)

	_, err := LoadConfig("testdata/test.txt")
	assert.Error(err)
	assert.Contains(err.Error(), "unsupported file extension")
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
			assert := require.New(t)

			client := NewEvalClient(tt.config)

			assert.Equal(tt.expectedSteps, client.config.MaxSteps)
			assert.Equal(tt.expectedTokens, client.config.MaxTokens)
		})
	}
}

func TestSchemaForEvalConfig(t *testing.T) {
	assert := require.New(t)

	schema, err := SchemaForEvalConfig()
	assert.NoError(err)
	assert.NotEmpty(schema)

	t.Log("Generated JSON Schema:\n", schema)

	// Verify it's valid JSON
	var schemaMap map[string]any
	err = json.Unmarshal([]byte(schema), &schemaMap)
	assert.NoError(err)

	// Verify top-level metadata fields
	schemaURL, ok := schemaMap["$schema"].(string)
	assert.True(ok)
	assert.Equal("https://json-schema.org/draft/2020-12/schema", schemaURL)

	title, ok := schemaMap["title"].(string)
	assert.True(ok)
	assert.Equal("MCP Evaluation Configuration", title)

	desc, ok := schemaMap["description"].(string)
	assert.True(ok)
	assert.NotEmpty(desc)

	// Verify it has expected JSON schema fields
	_, ok = schemaMap["properties"]
	assert.True(ok)

	// Verify it contains expected EvalConfig properties
	properties, ok := schemaMap["properties"].(map[string]any)
	assert.True(ok)

	expectedProperties := []string{"model", "grading_model", "timeout", "max_steps", "max_tokens", "mcp_server", "evals"}
	for _, prop := range expectedProperties {
		assert.Contains(properties, prop)
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
			assert.True(ok, "path %v: expected map at level %d, got %T", tc.path, i, current)
			current, ok = m[key]
			assert.True(ok, "path %v: key %q not found", tc.path, key)
		}
		if desc, ok := current.(string); ok {
			assert.Contains(desc, tc.description)
		}
	}
}

func TestValidateConfigFile_Valid(t *testing.T) {
	assert := require.New(t)

	result, err := ValidateConfigFile("testdata/mcp-test-evals.yaml")
	assert.NoError(err)
	assert.True(result.Valid)
	assert.Empty(result.Errors)
}

func TestValidateConfigFile_MissingModel(t *testing.T) {
	assert := require.New(t)

	configContent := `
mcp_server:
  command: echo
  args: ["test"]

evals:
  - name: test
    prompt: "test prompt"
`

	tmpFile, err := os.CreateTemp("", "invalid-missing-model-*.yaml")
	assert.NoError(err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configContent)
	assert.NoError(err)
	tmpFile.Close()

	result, err := ValidateConfigFile(tmpFile.Name())
	assert.NoError(err)
	assert.False(result.Valid)
	assert.NotEmpty(result.Errors)

	t.Logf("Got %d validation errors (expected)", len(result.Errors))
	for _, verr := range result.Errors {
		t.Logf("  - [%s] %s", verr.Path, verr.Message)
	}
}

func TestValidateConfigFile_MissingMCPServerCommand(t *testing.T) {
	assert := require.New(t)

	configContent := `
model: claude-3-5-sonnet-20241022

mcp_server:
  args: ["test"]

evals:
  - name: test
    prompt: "test prompt"
`

	tmpFile, err := os.CreateTemp("", "invalid-missing-command-*.yaml")
	assert.NoError(err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configContent)
	assert.NoError(err)
	tmpFile.Close()

	result, err := ValidateConfigFile(tmpFile.Name())
	assert.NoError(err)
	assert.False(result.Valid)
	assert.NotEmpty(result.Errors)

	t.Logf("Got %d validation errors (expected)", len(result.Errors))
	for _, verr := range result.Errors {
		t.Logf("  - [%s] %s", verr.Path, verr.Message)
	}
}

func TestValidateConfigFile_MissingEvals(t *testing.T) {
	assert := require.New(t)

	configContent := `
model: claude-3-5-sonnet-20241022

mcp_server:
  command: echo
  args: ["test"]
`

	tmpFile, err := os.CreateTemp("", "invalid-missing-evals-*.yaml")
	assert.NoError(err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configContent)
	assert.NoError(err)
	tmpFile.Close()

	result, err := ValidateConfigFile(tmpFile.Name())
	assert.NoError(err)
	assert.False(result.Valid)
	assert.NotEmpty(result.Errors)

	t.Logf("Got %d validation errors (expected)", len(result.Errors))
	for _, verr := range result.Errors {
		t.Logf("  - [%s] %s", verr.Path, verr.Message)
	}
}

func TestValidateConfigFile_InvalidEval(t *testing.T) {
	assert := require.New(t)

	configContent := `
model: claude-3-5-sonnet-20241022

mcp_server:
  command: echo
  args: ["test"]

evals:
  - name: test
    # missing prompt
`

	tmpFile, err := os.CreateTemp("", "invalid-eval-*.yaml")
	assert.NoError(err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configContent)
	assert.NoError(err)
	tmpFile.Close()

	result, err := ValidateConfigFile(tmpFile.Name())
	assert.NoError(err)
	assert.False(result.Valid)
	assert.NotEmpty(result.Errors)

	t.Logf("Got %d validation errors (expected)", len(result.Errors))
	for _, verr := range result.Errors {
		t.Logf("  - [%s] %s", verr.Path, verr.Message)
	}
}

func TestValidateConfigFile_InvalidFileExtension(t *testing.T) {
	assert := require.New(t)

	tmpFile, err := os.CreateTemp("", "config-*.txt")
	assert.NoError(err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString("test")
	assert.NoError(err)
	tmpFile.Close()

	_, err = ValidateConfigFile(tmpFile.Name())
	assert.Error(err)
	assert.Contains(err.Error(), "unsupported file extension")
}

func TestValidateConfigFile_NonExistentFile(t *testing.T) {
	assert := require.New(t)

	_, err := ValidateConfigFile("nonexistent-file.yaml")
	assert.Error(err)
}

func TestValidateConfigFile_InvalidYAML(t *testing.T) {
	assert := require.New(t)

	invalidYAML := `
model: claude
  invalid: indentation
    wrong: level
`

	tmpFile, err := os.CreateTemp("", "invalid-*.yaml")
	assert.NoError(err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(invalidYAML)
	assert.NoError(err)
	tmpFile.Close()

	_, err = ValidateConfigFile(tmpFile.Name())
	assert.Error(err)
}

func TestValidateConfigFile_JSONFormat(t *testing.T) {
	assert := require.New(t)

	configContent := `{
  "model": "claude-3-5-sonnet-20241022",
  "mcp_server": {
    "command": "echo",
    "args": ["test"]
  },
  "evals": [
    {
      "name": "test",
      "prompt": "test prompt"
    }
  ]
}`

	tmpFile, err := os.CreateTemp("", "valid-*.json")
	assert.NoError(err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configContent)
	assert.NoError(err)
	tmpFile.Close()

	result, err := ValidateConfigFile(tmpFile.Name())
	assert.NoError(err)
	assert.True(result.Valid)
}
