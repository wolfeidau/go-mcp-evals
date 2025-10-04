package commands

import (
	"fmt"

	evaluations "github.com/wolfeidau/go-mcp-evals"
)

// ValidateCmd handles the validate command
type ValidateCmd struct {
	Config string `help:"Path to evaluation configuration file (YAML or JSON)" required:"" type:"path"`
}

// Run executes the validate command
func (v *ValidateCmd) Run(globals *Globals) error {
	// Validate the config file
	result, err := evaluations.ValidateConfigFile(v.Config)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	if result.Valid {
		fmt.Printf("✓ Configuration is valid: %s\n", v.Config)
		return nil
	}

	// Print validation errors
	fmt.Printf("✗ Configuration has %d error(s):\n\n", len(result.Errors))
	for i, verr := range result.Errors {
		if verr.Path != "" {
			fmt.Printf("%d. [%s] %s\n", i+1, verr.Path, verr.Message)
		} else {
			fmt.Printf("%d. %s\n", i+1, verr.Message)
		}
	}
	fmt.Println()

	return fmt.Errorf("validation failed")
}
