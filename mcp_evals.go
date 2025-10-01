package evaluations

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"
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
	APIKey    string
	Command   string
	Args      []string
	Env       []string
	Model     string
	MaxSteps  int
	MaxTokens int
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

	// Apply defaults for optional fields
	if config.MaxSteps <= 0 {
		config.MaxSteps = 10
	}
	if config.MaxTokens <= 0 {
		config.MaxTokens = 4096
	}

	return &EvalClient{
		client: anthropic.NewClient(opts...), // uses ANTHROPIC_API_KEY from env
		config: config,
	}
}

// loadMCPSession creates an MCP client, connects to the server, and retrieves available tools
func (ec *EvalClient) loadMCPSession(ctx context.Context) (*mcp.ClientSession, *mcp.ListToolsResult, error) {
	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)
	// #nosec G204 - Command and args are provided by the library caller as part of EvalClientConfig
	cmd := exec.Command(ec.config.Command, ec.config.Args...)

	// If custom env vars are provided, append them to the parent environment
	if len(ec.config.Env) > 0 {
		cmd.Env = append(os.Environ(), ec.config.Env...)
	}

	transport := &mcp.CommandTransport{
		Command: cmd,
	}

	session, err := mcpClient.Connect(ctx, transport, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create MCP client: %w", err)
	}

	// get all the tools
	toolsResp, err := session.ListTools(ctx, nil)
	if err != nil {
		_ = session.Close()
		return nil, nil, fmt.Errorf("failed to list tools: %w", err)
	}

	return session, toolsResp, nil
}

// executeAndTraceToolCall executes a single MCP tool call and captures complete trace data
func (ec *EvalClient) executeAndTraceToolCall(
	ctx context.Context,
	toolUseBlock anthropic.ToolUseBlock,
	session *mcp.ClientSession,
) ToolCall {
	toolCall := ToolCall{
		ToolID:    toolUseBlock.ID,
		ToolName:  toolUseBlock.Name,
		StartTime: time.Now(),
	}

	// Capture input
	if inputJSON, err := json.Marshal(toolUseBlock.Input); err == nil {
		toolCall.Input = inputJSON
	}

	// Execute MCP tool call
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolUseBlock.Name,
		Arguments: toolUseBlock.Input,
	})

	toolCall.EndTime = time.Now()
	toolCall.Duration = toolCall.EndTime.Sub(toolCall.StartTime)

	if err != nil {
		toolCall.Success = false
		toolCall.Error = err.Error()
		// Create error output in JSON format for consistency
		errorOutput := map[string]string{"error": err.Error()}
		if outputJSON, marshalErr := json.Marshal(errorOutput); marshalErr == nil {
			toolCall.Output = outputJSON
		}
	} else {
		toolCall.Success = true
		// Convert MCP result to structured output
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
		resultContent := strings.Join(contentParts, "\n")

		// Store as JSON string for trace output
		outputData := map[string]string{"result": resultContent}
		if outputJSON, marshalErr := json.Marshal(outputData); marshalErr == nil {
			toolCall.Output = outputJSON
		}
	}

	return toolCall
}

func (ec *EvalClient) RunEval(ctx context.Context, eval Eval) (*EvalRunResult, error) {
	overallStart := time.Now()
	trace := &EvalTrace{
		Steps: make([]AgenticStep, 0, ec.config.MaxSteps),
	}

	result := &EvalRunResult{
		Eval:  eval,
		Trace: trace,
	}

	session, toolsResp, err := ec.loadMCPSession(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = session.Close() }()

	// convert the tools to the format expected by the anthropic model
	toolParams := make([]anthropic.ToolParam, 0, len(toolsResp.Tools))
	for _, tool := range toolsResp.Tools {
		// Convert the MCP tool input schema to Anthropic format
		var properties map[string]any
		if tool.InputSchema != nil {
			// MCP uses JSON Schema, convert to map
			schemaBytes, _ := json.Marshal(tool.InputSchema)
			var schema map[string]any
			if err = json.Unmarshal(schemaBytes, &schema); err == nil {
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
		anthropic.NewUserMessage(anthropic.NewTextBlock(eval.Prompt)),
	}

	var finalText strings.Builder

	// Agentic loop with tracing
	stepNumber := 0
	for range ec.config.MaxSteps {
		stepNumber++
		stepStart := time.Now()
		step := AgenticStep{
			StepNumber: stepNumber,
			StartTime:  stepStart,
			ToolCalls:  make([]ToolCall, 0),
		}

		stream := ec.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(ec.config.Model),
			MaxTokens: int64(ec.config.MaxTokens),
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
			if err = message.Accumulate(event); err != nil {
				step.Error = err.Error()
				trace.Steps = append(trace.Steps, step)
				return nil, fmt.Errorf("failed to accumulate event: %w", err)
			}

			if evt, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
				finalText.WriteString(evt.Delta.Text)
			}
		}

		if err = stream.Err(); err != nil {
			step.Error = err.Error()
			trace.Steps = append(trace.Steps, step)
			return nil, fmt.Errorf("streaming error: %w", err)
		}

		// Record step data from message
		step.StopReason = string(message.StopReason)
		step.InputTokens = int(message.Usage.InputTokens)
		step.OutputTokens = int(message.Usage.OutputTokens)

		// Extract text content
		for _, block := range message.Content {
			if textBlock, ok := block.AsAny().(anthropic.TextBlock); ok {
				step.ModelResponse += textBlock.Text
			}
		}

		// Add assistant message to history
		messages = append(messages, message.ToParam())

		// Check stop reason
		if message.StopReason == anthropic.StopReasonEndTurn {
			step.EndTime = time.Now()
			step.Duration = step.EndTime.Sub(stepStart)
			trace.Steps = append(trace.Steps, step)
			// Model finished without tool use
			break
		}

		if message.StopReason != anthropic.StopReasonToolUse {
			step.EndTime = time.Now()
			step.Duration = step.EndTime.Sub(stepStart)
			trace.Steps = append(trace.Steps, step)
			// Unexpected stop reason
			break
		}

		// Execute tools and collect results
		var toolResults []anthropic.ContentBlockParamUnion
		for _, block := range message.Content {
			if variant, ok := block.AsAny().(anthropic.ToolUseBlock); ok {
				// Execute and trace tool call
				toolCall := ec.executeAndTraceToolCall(ctx, variant, session)
				step.ToolCalls = append(step.ToolCalls, toolCall)

				// Build result block for message history
				var resultContent string
				if toolCall.Success {
					resultContent = string(toolCall.Output)
				} else {
					resultContent = fmt.Sprintf("Error calling tool: %s", toolCall.Error)
				}

				toolResults = append(toolResults, anthropic.NewToolResultBlock(
					block.ID,
					resultContent,
					!toolCall.Success,
				))
			}
		}

		step.EndTime = time.Now()
		step.Duration = step.EndTime.Sub(stepStart)
		trace.Steps = append(trace.Steps, step)

		// If no tool results, we're done
		if len(toolResults) == 0 {
			break
		}

		// Add tool results to message history
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	// Calculate trace metrics
	trace.StepCount = len(trace.Steps)
	for _, step := range trace.Steps {
		trace.TotalInputTokens += step.InputTokens
		trace.TotalOutputTokens += step.OutputTokens
		trace.ToolCallCount += len(step.ToolCalls)
	}

	evalResult := &EvalResult{
		Prompt:      eval.Prompt,
		RawResponse: finalText.String(),
	}
	result.Result = evalResult

	// Auto-grade the result with tracing
	grade, gradingTrace, err := ec.gradeWithTrace(ctx, eval, evalResult)
	if err != nil {
		// Don't fail the entire eval if grading fails, just log it
		result.Error = fmt.Errorf("grading failed: %w", err)
		trace.Grading = gradingTrace // Still include partial trace if available
	} else {
		result.Grade = grade
		trace.Grading = gradingTrace
	}

	// Finalize trace timing
	trace.TotalDuration = time.Since(overallStart)

	return result, nil
}

// RunEvals executes multiple evaluations and returns all results.
// Each eval reuses the same MCP session for efficiency.
// Individual eval failures are captured in EvalRunResult.Error and don't stop the batch.
func (ec *EvalClient) RunEvals(ctx context.Context, evals []Eval) ([]EvalRunResult, error) {
	results := make([]EvalRunResult, len(evals))

	for i, eval := range evals {
		result, err := ec.RunEval(ctx, eval)
		if err != nil {
			// Capture error but continue with other evals
			results[i] = EvalRunResult{
				Eval:  eval,
				Error: err,
			}
			continue
		}
		results[i] = *result
	}

	return results, nil
}

// gradeWithTrace grades an evaluation result and returns complete trace data
func (ec *EvalClient) gradeWithTrace(ctx context.Context, eval Eval, evalResult *EvalResult) (*GradeResult, *GradingTrace, error) {
	trace := &GradingTrace{
		UserPrompt:     eval.Prompt,
		ModelResponse:  evalResult.RawResponse,
		ExpectedResult: eval.ExpectedResult,
		StartTime:      time.Now(),
	}

	// Build grading prompt
	gradingPrompt := fmt.Sprintf(`Here is the user input: %s
Here is the LLM's answer: %s`, evalResult.Prompt, evalResult.RawResponse)

	trace.GradingPrompt = gradingPrompt

	// Execute grading
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

	trace.EndTime = time.Now()
	trace.Duration = trace.EndTime.Sub(trace.StartTime)

	if err != nil {
		trace.Error = err.Error()
		return nil, trace, fmt.Errorf("failed to get grading response: %w", err)
	}

	// Capture raw response and token usage
	rawResponse := resp.Content[0].AsAny().(anthropic.TextBlock).Text
	trace.RawGradingOutput = rawResponse
	trace.InputTokens = int(resp.Usage.InputTokens)
	trace.OutputTokens = int(resp.Usage.OutputTokens)

	// Parse grade result
	var gradeResult GradeResult
	if err := json.Unmarshal([]byte(rawResponse), &gradeResult); err != nil {
		trace.Error = err.Error()
		return nil, trace, fmt.Errorf("failed to parse grading response: %w", err)
	}

	return &gradeResult, trace, nil
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

// Eval represents a single evaluation test case
type Eval struct {
	Name           string `yaml:"name" json:"name"`
	Description    string `yaml:"description,omitempty" json:"description,omitempty"`
	Prompt         string `yaml:"prompt" json:"prompt"`
	ExpectedResult string `yaml:"expected_result,omitempty" json:"expected_result,omitempty"`
}

// EvalRunResult combines the eval configuration with its execution results
type EvalRunResult struct {
	Eval   Eval
	Result *EvalResult
	Grade  *GradeResult
	Error  error
	Trace  *EvalTrace // Complete execution trace for debugging and analysis
}

// EvalTrace captures complete execution history of an evaluation run
type EvalTrace struct {
	Steps             []AgenticStep `json:"steps"`               // Each step in the agentic loop
	Grading           *GradingTrace `json:"grading,omitempty"`   // Grading interaction details
	TotalDuration     time.Duration `json:"total_duration"`      // Total execution time
	TotalInputTokens  int           `json:"total_input_tokens"`  // Sum of input tokens across all steps
	TotalOutputTokens int           `json:"total_output_tokens"` // Sum of output tokens across all steps
	StepCount         int           `json:"step_count"`          // Number of agentic steps executed
	ToolCallCount     int           `json:"tool_call_count"`     // Total number of tool calls made
}

// AgenticStep records a single iteration of the agentic loop
type AgenticStep struct {
	StepNumber    int           `json:"step_number"`     // 1-indexed step number
	StartTime     time.Time     `json:"start_time"`      // When this step started
	EndTime       time.Time     `json:"end_time"`        // When this step completed
	Duration      time.Duration `json:"duration"`        // Step execution duration
	ModelResponse string        `json:"model_response"`  // Text content from assistant
	StopReason    string        `json:"stop_reason"`     // end_turn, tool_use, max_tokens, etc.
	ToolCalls     []ToolCall    `json:"tool_calls"`      // Tools executed in this step
	InputTokens   int           `json:"input_tokens"`    // Input tokens for this step
	OutputTokens  int           `json:"output_tokens"`   // Output tokens for this step
	Error         string        `json:"error,omitempty"` // Error message if step failed
}

// ToolCall captures details of a single tool invocation
type ToolCall struct {
	ToolID    string          `json:"tool_id"`         // Unique ID from content block
	ToolName  string          `json:"tool_name"`       // MCP tool name
	StartTime time.Time       `json:"start_time"`      // When tool execution started
	EndTime   time.Time       `json:"end_time"`        // When tool execution completed
	Duration  time.Duration   `json:"duration"`        // Tool execution duration
	Input     json.RawMessage `json:"input"`           // Tool arguments as JSON
	Output    json.RawMessage `json:"output"`          // Tool result as JSON
	Success   bool            `json:"success"`         // Whether tool executed successfully
	Error     string          `json:"error,omitempty"` // Error message if tool failed
}

// GradingTrace records the grading interaction with the LLM
type GradingTrace struct {
	UserPrompt       string        `json:"user_prompt"`        // Original eval prompt
	ModelResponse    string        `json:"model_response"`     // Model's answer being graded
	ExpectedResult   string        `json:"expected_result"`    // Expected result description
	GradingPrompt    string        `json:"grading_prompt"`     // Full prompt sent to grader
	RawGradingOutput string        `json:"raw_grading_output"` // Complete LLM response before parsing
	StartTime        time.Time     `json:"start_time"`         // When grading started
	EndTime          time.Time     `json:"end_time"`           // When grading completed
	Duration         time.Duration `json:"duration"`           // Grading duration
	InputTokens      int           `json:"input_tokens"`       // Input tokens for grading
	OutputTokens     int           `json:"output_tokens"`      // Output tokens for grading
	Error            string        `json:"error,omitempty"`    // Error message if grading failed
}

// MCPServerConfig defines how to start the MCP server
type MCPServerConfig struct {
	Command string   `yaml:"command" json:"command"`
	Args    []string `yaml:"args,omitempty" json:"args,omitempty"`
	Env     []string `yaml:"env,omitempty" json:"env,omitempty"`
}

// EvalConfig represents the top-level configuration for running evaluations
type EvalConfig struct {
	Model     string          `yaml:"model" json:"model"`
	Timeout   string          `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	MaxSteps  int             `yaml:"max_steps,omitempty" json:"max_steps,omitempty"`
	MaxTokens int             `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty"`
	MCPServer MCPServerConfig `yaml:"mcp_server" json:"mcp_server"`
	Evals     []Eval          `yaml:"evals" json:"evals"`
}

// LoadConfig loads an evaluation configuration from a YAML or JSON file.
// The file format is detected by the file extension (.yaml, .yml, or .json).
func LoadConfig(filePath string) (*EvalConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config EvalConfig
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &config); err != nil {
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

	return &config, nil
}
