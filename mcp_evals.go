package evaluations

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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
	}
}

func (ec *EvalClient) Run(ctx context.Context) error {
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)
	cmd := exec.Command(ec.config.Command, ec.config.Args...)

	cmd.Env = ec.config.Env

	transport := &mcp.CommandTransport{
		Command: cmd,
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to create MCP client: %w", err)
	}
	defer func() { _ = session.Close() }()

	return nil
}
