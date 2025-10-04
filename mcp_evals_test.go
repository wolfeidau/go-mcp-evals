package evaluations

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
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
				"get_user",
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
			assert := require.New(t)

			client := NewEvalClient(EvalClientConfig{
				Command: tt.command,
				Args:    tt.args,
			})

			ctx := context.Background()
			session, toolsResp, err := client.loadMCPSession(ctx)

			if tt.expectError {
				assert.Error(err)
				return
			}

			assert.NoError(err)
			defer func() { _ = session.Close() }()

			assert.NotNil(toolsResp)

			// Verify expected tools are present
			toolMap := make(map[string]bool)
			for _, tool := range toolsResp.Tools {
				toolMap[tool.Name] = true
			}

			for _, expectedTool := range tt.expectedTools {
				assert.True(toolMap[expectedTool], "expected tool %q not found in response", expectedTool)
			}

			// Verify we got the correct number of tools
			assert.Len(toolsResp.Tools, len(tt.expectedTools))
		})
	}
}

func TestEvalClient_loadMCPSession_ToolExecution(t *testing.T) {
	assert := require.New(t)

	// Set environment variable for the test
	t.Setenv("TEST_VAR", "test_value")

	client := NewEvalClient(EvalClientConfig{
		Command: "go",
		Args:    []string{"run", "testdata/mcp-test-server/main.go"},
	})

	ctx := context.Background()
	session, _, err := client.loadMCPSession(ctx)
	assert.NoError(err)
	defer func() { _ = session.Close() }()

	// Test calling the get_env tool
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_env",
		Arguments: map[string]any{
			"name": "TEST_VAR",
		},
	})
	assert.NoError(err)
	assert.NotEmpty(result.Content)

	// Parse the response
	textContent, ok := result.Content[0].(*mcp.TextContent)
	assert.True(ok, "expected text content but got %T", result.Content[0])

	var output struct {
		Name  string `json:"name"`
		Value string `json:"value"`
		Set   bool   `json:"set"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &output)
	assert.NoError(err)

	// Verify the response
	assert.Equal("TEST_VAR", output.Name)
	assert.Equal("test_value", output.Value)
	assert.True(output.Set)
}

func TestEvalClient_loadMCPSession_CustomEnv(t *testing.T) {
	assert := require.New(t)

	// Test that custom environment variables are added while preserving parent env
	client := NewEvalClient(EvalClientConfig{
		Command: "go",
		Args:    []string{"run", "testdata/mcp-test-server/main.go"},
		Env:     []string{"CUSTOM_TEST_VAR=custom_value"},
	})

	ctx := context.Background()
	session, _, err := client.loadMCPSession(ctx)
	assert.NoError(err)
	defer func() { _ = session.Close() }()

	// Test that custom env var works
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_env",
		Arguments: map[string]any{
			"name": "CUSTOM_TEST_VAR",
		},
	})
	assert.NoError(err)

	textContent, ok := result.Content[0].(*mcp.TextContent)
	assert.True(ok, "expected text content but got %T", result.Content[0])

	var output struct {
		Name  string `json:"name"`
		Value string `json:"value"`
		Set   bool   `json:"set"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &output)
	assert.NoError(err)

	assert.Equal("custom_value", output.Value)
	assert.True(output.Set)

	// Test that parent env vars are still available (e.g., PATH)
	result2, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_env",
		Arguments: map[string]any{
			"name": "PATH",
		},
	})
	assert.NoError(err)

	textContent2, ok := result2.Content[0].(*mcp.TextContent)
	assert.True(ok, "expected text content but got %T", result2.Content[0])

	var output2 struct {
		Name  string `json:"name"`
		Value string `json:"value"`
		Set   bool   `json:"set"`
	}
	err = json.Unmarshal([]byte(textContent2.Text), &output2)
	assert.NoError(err)

	// PATH should be inherited from parent environment
	assert.True(output2.Set)
	assert.NotEmpty(output2.Value)
}
