package evaluations

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	SystemPrompt = "You are an assistant responsible for evaluating the results of calling various tools. Given the user's query, use the tools available to you to answer the question."

	EvalSystemPrompt = `You are an expert evaluator assessing how well an LLM answers a given question. Review the provided answer and score it from 1 to 5 in each of the following categories:
        Accuracy - Does the answer contain factual errors or hallucinations?
        Completeness - Does the answer fully address all parts of the question?
        Relevance - Is the information directly related to the question?
        Clarity - Is the explanation easy to understand and well-structured?
        Reasoning - Does the answer show logical thinking or provide evidence or rationale?
        Return your evaluation as a JSON object in the format:
        {
            "accuracy": 1-5,
            "completeness": 1-5,
            "relevance": 1-5,
            "clarity": 1-5,
            "reasoning": 1-5,
            "overall_comments": "A short paragraph summarizing the strengths and weaknesses of the answer."
        }`
)

type EvalClientConfig struct {
	APIKey  string
	Command string
	Args    []string
	Env     []string
	Model   string
}

type EvalClient struct {
	client anthropic.Client
	config EvalClientConfig
}

func NewEvalClient(config EvalClientConfig) *EvalClient {
	opts := []option.RequestOption{}
	if config.APIKey != "" {
		opts = append(opts, option.WithAPIKey(config.APIKey))
	}

	return &EvalClient{
		client: anthropic.NewClient(opts...), // uses ANTHROPIC_API_KEY from env
		config: config,
	}
}

func (ec *EvalClient) RunEval(ctx context.Context, prompt string) (*EvalResult, error) {
	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)
	// #nosec G204 - Command and args are provided by the library caller as part of EvalClientConfig
	cmd := exec.Command(ec.config.Command, ec.config.Args...)

	cmd.Env = ec.config.Env

	transport := &mcp.CommandTransport{
		Command: cmd,
	}

	session, err := mcpClient.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %w", err)
	}
	defer func() { _ = session.Close() }()

	// get all the tools
	toolsResp, err := session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	// convert the tools to the format expected by the anthropic model
	toolParams := make([]anthropic.ToolParam, 0, len(toolsResp.Tools))
	for _, tool := range toolsResp.Tools {
		// Convert the MCP tool input schema to Anthropic format
		var properties map[string]any
		if tool.InputSchema != nil {
			// MCP uses JSON Schema, convert to map
			schemaBytes, _ := json.Marshal(tool.InputSchema)
			var schema map[string]any
			if err := json.Unmarshal(schemaBytes, &schema); err == nil {
				if props, ok := schema["properties"].(map[string]any); ok {
					properties = props
				}
			}
		}

		toolParam := anthropic.ToolParam{
			Name:        tool.Name,
			Description: anthropic.String(tool.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: properties,
			},
		}
		toolParams = append(toolParams, toolParam)
	}

	tools := make([]anthropic.ToolUnionParam, len(toolParams))
	for i, toolParam := range toolParams {
		tools[i] = anthropic.ToolUnionParam{OfTool: &toolParam}
	}

	// Initialize message history
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
	}

	const maxSteps = 10
	var finalText strings.Builder

	// Agentic loop
	for step := 0; step < maxSteps; step++ {
		stream := ec.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(ec.config.Model),
			MaxTokens: 4096,
			System: []anthropic.TextBlockParam{
				{Text: SystemPrompt},
			},
			Messages: messages,
			Tools:    tools,
		})

		message := anthropic.Message{}

		// Process the stream
		for stream.Next() {
			event := stream.Current()
			if err := message.Accumulate(event); err != nil {
				return nil, fmt.Errorf("failed to accumulate event: %w", err)
			}

			if evt, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
				finalText.WriteString(evt.Delta.Text)
			}
		}

		if err := stream.Err(); err != nil {
			return nil, fmt.Errorf("streaming error: %w", err)
		}

		// Add assistant message to history
		messages = append(messages, message.ToParam())

		// Check stop reason
		if message.StopReason == anthropic.StopReasonEndTurn {
			// Model finished without tool use
			break
		}

		if message.StopReason != anthropic.StopReasonToolUse {
			// Unexpected stop reason
			break
		}

		// Execute tools and collect results
		var toolResults []anthropic.ContentBlockParamUnion
		for _, block := range message.Content {
			if variant, ok := block.AsAny().(anthropic.ToolUseBlock); ok {
				// Call the MCP tool
				result, err := session.CallTool(ctx, &mcp.CallToolParams{
					Name:      block.Name,
					Arguments: variant.Input,
				})

				var resultContent string
				isError := false
				if err != nil {
					resultContent = fmt.Sprintf("Error calling tool: %v", err)
					isError = true
				} else {
					// Convert MCP result to string
					var contentParts []string
					for _, content := range result.Content {
						switch c := content.(type) {
						case *mcp.TextContent:
							contentParts = append(contentParts, c.Text)
						case *mcp.ImageContent:
							contentParts = append(contentParts, fmt.Sprintf("[Image: %s]", c.MIMEType))
						case *mcp.EmbeddedResource:
							contentParts = append(contentParts, fmt.Sprintf("[Resource: %s]", c.Resource.URI))
						}
					}
					resultContent = strings.Join(contentParts, "\n")
				}

				toolResults = append(toolResults, anthropic.NewToolResultBlock(
					block.ID,
					resultContent,
					isError,
				))
			}
		}

		// If no tool results, we're done
		if len(toolResults) == 0 {
			break
		}

		// Add tool results to message history
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	return &EvalResult{
		Prompt:      prompt,
		RawResponse: finalText.String(),
	}, nil
}

func (ec *EvalClient) Grade(ctx context.Context, evalResult *EvalResult) (*GradeResult, error) {

	// use a string template to create the grading prompt
	gradingPrompt := fmt.Sprintf(`Here is the user input: %s
Here is the LLM's answer: %s`, evalResult.Prompt, evalResult.RawResponse)

	resp, err := ec.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(ec.config.Model),
		MaxTokens: 1000,
		System: []anthropic.TextBlockParam{
			{Text: EvalSystemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(gradingPrompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get grading response: %w", err)
	}

	var gradeResult GradeResult
	if err := json.Unmarshal([]byte(resp.Content[0].AsAny().(anthropic.TextBlock).Text), &gradeResult); err != nil {
		return nil, fmt.Errorf("failed to parse grading response: %w", err)
	}

	return &gradeResult, nil
}

type EvalResult struct {
	Prompt      string
	RawResponse string
}

type GradeResult struct {
	Accuracy       int    `json:"accuracy"`
	Completeness   int    `json:"completeness"`
	Relevance      int    `json:"relevance"`
	Clarity        int    `json:"clarity"`
	Reasoning      int    `json:"reasoning"`
	OverallComment string `json:"overall_comments"`
}
