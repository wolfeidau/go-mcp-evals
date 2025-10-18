package commands

import (
	"fmt"
	"os"

	evaluations "github.com/wolfeidau/mcp-evals"
	"github.com/wolfeidau/mcp-evals/internal/help"
)

// Globals contains flags shared across all commands
type Globals struct {
}

func createClient(config *evaluations.EvalConfig, apiKey, baseURL string, quiet bool) *evaluations.EvalClient {
	styles := help.DefaultStyles()

	clientConfig := evaluations.EvalClientConfig{
		APIKey:       apiKey,
		BaseURL:      baseURL,
		Command:      config.MCPServer.Command,
		Args:         config.MCPServer.Args,
		Env:          config.MCPServer.Env,
		Model:        config.Model,
		GradingModel: config.GradingModel,
		MaxSteps:     int(config.MaxSteps),
		MaxTokens:    int(config.MaxTokens),
		StderrCallback: func(line string) {
			if !quiet {
				fmt.Fprintln(os.Stderr, styles.FormatMCPStderr(line))
			}
		},
	}

	// Map caching configuration from YAML to client config
	if config.EnablePromptCaching != nil {
		clientConfig.EnablePromptCaching = config.EnablePromptCaching
	}
	if config.CacheTTL != "" {
		clientConfig.CacheTTL = config.CacheTTL
	}

	return evaluations.NewEvalClient(clientConfig)
}
