package evaluations

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestEvalClient_loadMCPSession(t *testing.T) {
	tests := []struct {
		name          string
		command       string
		args          []string
		expectedTools []string
		expectError   bool
	}{
		{
			name:    "successfully loads test MCP server",
			command: "go",
			args:    []string{"run", "testdata/mcp-test-server/main.go"},
			expectedTools: []string{
				"add",
				"echo",
				"get_current_time",
				"get_env",
			},
			expectError: false,
		},
		{
			name:        "invalid command",
			command:     "nonexistent-command",
			args:        []string{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewEvalClient(EvalClientConfig{
				Command: tt.command,
				Args:    tt.args,
			})

			ctx := context.Background()
			session, toolsResp, err := client.loadMCPSession(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer func() { _ = session.Close() }()

			if toolsResp == nil {
				t.Fatal("expected tools response but got nil")
			}

			// Verify expected tools are present
			toolMap := make(map[string]bool)
			for _, tool := range toolsResp.Tools {
				toolMap[tool.Name] = true
			}

			for _, expectedTool := range tt.expectedTools {
				if !toolMap[expectedTool] {
					t.Errorf("expected tool %q not found in response", expectedTool)
				}
			}

			// Verify we got the correct number of tools
			if len(toolsResp.Tools) != len(tt.expectedTools) {
				t.Errorf("expected %d tools but got %d", len(tt.expectedTools), len(toolsResp.Tools))
			}
		})
	}
}

func TestEvalClient_loadMCPSession_ToolExecution(t *testing.T) {
	// Set environment variable for the test
	t.Setenv("TEST_VAR", "test_value")

	client := NewEvalClient(EvalClientConfig{
		Command: "go",
		Args:    []string{"run", "testdata/mcp-test-server/main.go"},
	})

	ctx := context.Background()
	session, _, err := client.loadMCPSession(ctx)
	if err != nil {
		t.Fatalf("failed to load MCP session: %v", err)
	}
	defer func() { _ = session.Close() }()

	// Test calling the get_env tool
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_env",
		Arguments: map[string]any{
			"name": "TEST_VAR",
		},
	})
	if err != nil {
		t.Fatalf("failed to call get_env tool: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected tool result content but got none")
	}

	// Parse the response
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content but got %T", result.Content[0])
	}

	var output struct {
		Name  string `json:"name"`
		Value string `json:"value"`
		Set   bool   `json:"set"`
	}
	if err := json.Unmarshal([]byte(textContent.Text), &output); err != nil {
		t.Fatalf("failed to parse tool output: %v", err)
	}

	// Verify the response
	if output.Name != "TEST_VAR" {
		t.Errorf("expected name 'TEST_VAR' but got %q", output.Name)
	}
	if output.Value != "test_value" {
		t.Errorf("expected value 'test_value' but got %q", output.Value)
	}
	if !output.Set {
		t.Error("expected Set to be true but got false")
	}
}

func TestEvalClient_loadMCPSession_CustomEnv(t *testing.T) {
	// Test that custom environment variables are added while preserving parent env
	client := NewEvalClient(EvalClientConfig{
		Command: "go",
		Args:    []string{"run", "testdata/mcp-test-server/main.go"},
		Env:     []string{"CUSTOM_TEST_VAR=custom_value"},
	})

	ctx := context.Background()
	session, _, err := client.loadMCPSession(ctx)
	if err != nil {
		t.Fatalf("failed to load MCP session: %v", err)
	}
	defer func() { _ = session.Close() }()

	// Test that custom env var works
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_env",
		Arguments: map[string]any{
			"name": "CUSTOM_TEST_VAR",
		},
	})
	if err != nil {
		t.Fatalf("failed to call get_env tool: %v", err)
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content but got %T", result.Content[0])
	}

	var output struct {
		Name  string `json:"name"`
		Value string `json:"value"`
		Set   bool   `json:"set"`
	}
	if err := json.Unmarshal([]byte(textContent.Text), &output); err != nil {
		t.Fatalf("failed to parse tool output: %v", err)
	}

	if output.Value != "custom_value" {
		t.Errorf("expected value 'custom_value' but got %q", output.Value)
	}
	if !output.Set {
		t.Error("expected Set to be true but got false")
	}

	// Test that parent env vars are still available (e.g., PATH)
	result2, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_env",
		Arguments: map[string]any{
			"name": "PATH",
		},
	})
	if err != nil {
		t.Fatalf("failed to call get_env tool for PATH: %v", err)
	}

	textContent2, ok := result2.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content but got %T", result2.Content[0])
	}

	var output2 struct {
		Name  string `json:"name"`
		Value string `json:"value"`
		Set   bool   `json:"set"`
	}
	if err := json.Unmarshal([]byte(textContent2.Text), &output2); err != nil {
		t.Fatalf("failed to parse tool output: %v", err)
	}

	// PATH should be inherited from parent environment
	if !output2.Set {
		t.Error("expected PATH to be set from parent environment but it wasn't")
	}
	if output2.Value == "" {
		t.Error("expected PATH value to be non-empty but it was empty")
	}
}
