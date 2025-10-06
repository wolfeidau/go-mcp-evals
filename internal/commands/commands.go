package commands

import evaluations "github.com/wolfeidau/go-mcp-evals"

// Globals contains flags shared across all commands
type Globals struct {
}

func createClient(config *evaluations.EvalConfig, apiKey, baseURL string) *evaluations.EvalClient {
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
