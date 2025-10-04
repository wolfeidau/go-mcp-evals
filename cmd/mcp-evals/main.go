package main

import (
	"github.com/alecthomas/kong"
	"github.com/wolfeidau/go-mcp-evals/internal/commands"
	"github.com/wolfeidau/go-mcp-evals/internal/help"
)

var (
	version = "dev"
)

// CLI represents the command-line interface
type CLI struct {
	commands.Globals

	Version kong.VersionFlag `short:"v" help:"Show version information"`

	Run      commands.RunCmd      `cmd:"" help:"Run evaluations against an MCP server (default)" default:"1"`
	Validate commands.ValidateCmd `cmd:"" help:"Validate configuration file against JSON schema"`
	Schema   commands.SchemaCmd   `cmd:"" help:"Generate JSON schema for evaluation configuration"`
}

func main() {
	cli := &CLI{}
	styles := help.DefaultStyles()
	ctx := kong.Parse(cli,
		kong.Name("mcp-evals"),
		kong.Description("Run evaluations against an MCP server"),
		kong.UsageOnError(),
		kong.Vars{"version": version},
		kong.Help(help.Printer(styles)),
	)

	err := ctx.Run(&cli.Globals)
	ctx.FatalIfErrorf(err)
}
