package evaluations

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/mark3labs/mcp-go/client"
)

type EvalClientConfig struct {
	APIKey  string
	Command string
	Args    []string
	Env     []string
	Model   string
}

type EvalClient struct {
	anthropic.Client
	config EvalClientConfig
}

func NewEvalClient(config EvalClientConfig) *EvalClient {

	opts := []option.RequestOption{}
	if config.APIKey != "" {
		opts = append(opts, option.WithAPIKey(config.APIKey))
	}

	return &EvalClient{
		Client: anthropic.NewClient(opts...), // uses ANTHROPIC_API_KEY from env
	}
}

func (ec *EvalClient) Run(ctx context.Context) error {
	c, err := client.NewStdioMCPClient(
		ec.config.Command,
		ec.config.Env,
		ec.config.Args...,
	)
	if err != nil {
		return fmt.Errorf("failed to create MCP client: %w", err)
	}
	defer func() { _ = c.Close() }()

	return nil
}
