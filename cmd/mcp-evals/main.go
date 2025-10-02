package main

import (
	"flag"
	"fmt"
	"os"

	evaluations "github.com/wolfeidau/go-mcp-evals"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "schema":
		if err := schemaCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func schemaCommand() error {
	fs := flag.NewFlagSet("schema", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: mcp-evals schema\n")
		fmt.Fprintf(os.Stderr, "\nGenerate JSON schema for evaluation configuration.\n")
	}
	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	schema, err := evaluations.SchemaForEvalConfig()
	if err != nil {
		return fmt.Errorf("failed to generate schema: %w", err)
	}

	fmt.Println(schema)
	return nil
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: mcp-evals <command>\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  schema    Generate JSON schema for evaluation configuration\n")
	fmt.Fprintf(os.Stderr, "  help      Show this help message\n")
}
