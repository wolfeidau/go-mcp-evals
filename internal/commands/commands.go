package commands

import evaluations "github.com/wolfeidau/go-mcp-evals"

// Globals contains flags shared across all commands
type Globals struct {
	Quiet    bool   `help:"Suppress progress output, only show summary" short:"q"`
	TraceDir string `help:"Directory to write trace files" type:"path"`
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

	return evaluations.NewEvalClient(clientConfig)
}
