package evaluations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"mvdan.cc/sh/v3/shell"
)

// MCPServerConfig defines how to start the MCP server
type MCPServerConfig struct {
	Command string   `yaml:"command" json:"command" jsonschema:"Command to start the MCP server"`
	Args    []string `yaml:"args,omitempty" json:"args,omitempty" jsonschema:"Arguments to pass to the command"`
	Env     []string `yaml:"env,omitempty" json:"env,omitempty" jsonschema:"Environment variables to set for the MCP server"`
}

type MaxTokens int
type MaxSteps int

// EvalConfig represents the top-level configuration for running evaluations
type EvalConfig struct {
	Model                string          `yaml:"model" json:"model" jsonschema:"Anthropic model ID to use for evaluations"`
	GradingModel         string          `yaml:"grading_model,omitempty" json:"grading_model,omitempty" jsonschema:"Anthropic model ID to use for grading (defaults to same as model)"`
	Timeout              string          `yaml:"timeout,omitempty" json:"timeout,omitempty" jsonschema:"Timeout duration for each evaluation (e.g., '2m', '30s')"`
	MaxSteps             MaxSteps        `yaml:"max_steps,omitempty" json:"max_steps,omitempty" jsonschema:"Maximum number of agentic loop iterations"`
	MaxTokens            MaxTokens       `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty" jsonschema:"Maximum tokens per LLM request"`
	EnablePromptCaching  *bool           `yaml:"enable_prompt_caching,omitempty" json:"enable_prompt_caching,omitempty" jsonschema:"Enable Anthropic prompt caching for tool definitions and system prompts (defaults to true for cost savings)"`
	CacheTTL             string          `yaml:"cache_ttl,omitempty" json:"cache_ttl,omitempty" jsonschema:"Cache time-to-live: '5m' (default, free) or '1h' (premium). Requires enable_prompt_caching=true"`
	EnforceMinimumScores *bool           `yaml:"enforce_minimum_scores,omitempty" json:"enforce_minimum_scores,omitempty" jsonschema:"Enforce minimum scores from grading rubrics (defaults to true; set to false to disable)"`
	MCPServer            MCPServerConfig `yaml:"mcp_server" json:"mcp_server" jsonschema:"Configuration for the MCP server to evaluate"`
	Evals                []Eval          `yaml:"evals" json:"evals" jsonschema:"List of evaluation test cases to run"`
}

// LoadConfig loads an evaluation configuration from a YAML or JSON file.
// The file format is detected by the file extension (.yaml, .yml, or .json).
// Environment variables in the config file are expanded using ${VAR} or $VAR syntax.
// Supports shell-style default values: ${VAR:-default}
func LoadConfig(filePath string) (*EvalConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	expandedStr, err := shell.Expand(string(data), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to expand environment variables: %w", err)
	}
	expandedData := []byte(expandedStr)

	var config EvalConfig
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(expandedData, &config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(expandedData, &config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported file extension: %s (expected .yaml, .yml, or .json)", ext)
	}

	// Validate required fields
	if config.Model == "" {
		return nil, fmt.Errorf("model is required in config")
	}
	if config.MCPServer.Command == "" {
		return nil, fmt.Errorf("mcp_server.command is required in config")
	}
	if len(config.Evals) == 0 {
		return nil, fmt.Errorf("at least one eval is required in config")
	}

	// Validate grading rubrics for each eval
	for i, eval := range config.Evals {
		if err := eval.GradingRubric.Validate(); err != nil {
			return nil, fmt.Errorf("eval[%d] '%s' has invalid rubric: %w", i, eval.Name, err)
		}
	}

	return &config, nil
}

// generateSchema creates a jsonschema.Schema for EvalConfig with custom metadata
func generateSchema() (*jsonschema.Schema, error) {
	customSchemas := map[reflect.Type]*jsonschema.Schema{
		reflect.TypeFor[MaxTokens](): {Type: "integer", Minimum: jsonschema.Ptr(1.0), Maximum: jsonschema.Ptr(20000.0), Default: json.RawMessage("4096")},
		reflect.TypeFor[MaxSteps]():  {Type: "integer", Minimum: jsonschema.Ptr(1.0), Maximum: jsonschema.Ptr(100.0), Default: json.RawMessage("10")},
	}

	opts := &jsonschema.ForOptions{TypeSchemas: customSchemas}

	schema, err := jsonschema.For[EvalConfig](opts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate JSON schema: %w", err)
	}

	schema.Title = "MCP Evaluation Configuration"
	schema.Description = "Configuration schema for running evaluations against Model Context Protocol (MCP) servers"
	schema.Schema = "https://json-schema.org/draft/2020-12/schema"

	return schema, nil
}

func SchemaForEvalConfig() (string, error) {
	schema, err := generateSchema()
	if err != nil {
		return "", err
	}

	// Marshal the complete schema
	schemaJSON, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal final schema: %w", err)
	}
	return string(schemaJSON), nil
}

// ValidationError represents a single validation error with location information
type ValidationError struct {
	Path    string // JSON path to the error (e.g., "mcp_server.command")
	Message string // Human-readable error message
}

// ValidationResult contains the results of validating a config file
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

// ValidateConfigFile validates a configuration file against the JSON schema.
// It reads the file, converts YAML to JSON if needed, and validates against the schema.
func ValidateConfigFile(filePath string) (*ValidationResult, error) {
	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Convert to JSON if needed
	var jsonData []byte
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".yaml", ".yml":
		// Parse YAML first
		var yamlData any
		if err := yaml.Unmarshal(data, &yamlData); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
		// Convert to JSON
		jsonData, err = json.Marshal(yamlData)
		if err != nil {
			return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
		}
	case ".json":
		jsonData = data
	default:
		return nil, fmt.Errorf("unsupported file extension: %s (expected .yaml, .yml, or .json)", ext)
	}

	// Generate schema
	schema, err := generateSchema()
	if err != nil {
		return nil, err
	}

	var configData any
	if err = json.Unmarshal(jsonData, &configData); err != nil {
		return nil, fmt.Errorf("failed to parse config as JSON: %w", err)
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve schema: %w", err)
	}

	validationErr := resolved.Validate(configData)

	result := &ValidationResult{
		Valid: validationErr == nil,
	}

	// If there's a validation error, parse it into our format
	if validationErr != nil {
		// The error from Validate is a detailed error message
		result.Errors = []ValidationError{
			{
				Path:    "",
				Message: validationErr.Error(),
			},
		}
	}

	return result, nil
}
