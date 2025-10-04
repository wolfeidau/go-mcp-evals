package commands

import (
	"fmt"

	evaluations "github.com/wolfeidau/go-mcp-evals"
)

// SchemaCmd handles the schema command
type SchemaCmd struct{}

// Run executes the schema command
func (s *SchemaCmd) Run(globals *Globals) error {
	schema, err := evaluations.SchemaForEvalConfig()
	if err != nil {
		return fmt.Errorf("failed to generate schema: %w", err)
	}

	fmt.Println(schema)
	return nil
}
