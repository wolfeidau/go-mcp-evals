package commands

import (
	"fmt"
	"path/filepath"

	evaluations "github.com/wolfeidau/go-mcp-evals"
	"github.com/wolfeidau/go-mcp-evals/internal/reporting"
)

// ReportCmd handles the report command
type ReportCmd struct {
	TraceFiles []string `help:"Path(s) to trace JSON file(s)" required:"" type:"existingfile"`
	Verbose    bool     `help:"Show detailed per-eval breakdown" short:"v"`
}

// Run executes the report command
func (r *ReportCmd) Run(globals *Globals) error {
	// Load trace files
	results := make([]evaluations.EvalRunResult, 0, len(r.TraceFiles))

	for _, path := range r.TraceFiles {
		result, err := reporting.LoadTraceFile(path)
		if err != nil {
			return fmt.Errorf("failed to load trace file %s: %w", filepath.Base(path), err)
		}
		results = append(results, result)
	}

	// Generate styled report
	return reporting.PrintStyledReport(results, r.Verbose)
}
