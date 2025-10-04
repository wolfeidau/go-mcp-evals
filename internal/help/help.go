package help

import (
	"fmt"
	"io"
	"strings"

	"github.com/alecthomas/kong"
)

// Printer creates a custom help printer with lipgloss styling
func Printer(styles Styles) kong.HelpPrinter {
	return func(options kong.HelpOptions, ctx *kong.Context) error {
		return printHelp(ctx.Stdout, options, ctx, styles)
	}
}

func printHelp(w io.Writer, options kong.HelpOptions, ctx *kong.Context, styles Styles) error {
	// Get the selected command or root
	selected := ctx.Selected()
	if selected == nil {
		selected = ctx.Model.Node
	}

	// Print usage
	if err := printUsage(w, selected, ctx, styles); err != nil {
		return err
	}

	// Print description if available
	if selected.Help != "" {
		fmt.Fprintf(w, "\n%s\n", selected.Help)
	}

	// Print commands if available
	nodes := selected.Leaves(true)
	if len(nodes) > 0 {
		if err := printCommands(w, nodes, styles, options); err != nil {
			return err
		}
	}

	// Print flags
	allFlags := selected.AllFlags(true)
	var flatFlags []*kong.Flag
	for _, flagGroup := range allFlags {
		flatFlags = append(flatFlags, flagGroup...)
	}
	if err := printFlags(w, flatFlags, styles, options); err != nil {
		return err
	}

	return nil
}

func printUsage(w io.Writer, node *kong.Node, ctx *kong.Context, styles Styles) error {
	usage := styles.Section.Render("Usage:") + " "

	// Add program name
	usage += styles.Title.Render(node.Name)

	// Add flags placeholder
	flags := node.AllFlags(true)
	if len(flags) > 0 {
		usage += " " + styles.Flag.Render("[flags]")
	}

	// Add command placeholder if commands exist
	nodes := node.Leaves(true)
	if len(nodes) > 0 {
		usage += " " + styles.Command.Render("<command>")
	}

	fmt.Fprintf(w, "%s\n", usage)
	return nil
}

func printCommands(w io.Writer, nodes []*kong.Node, styles Styles, options kong.HelpOptions) error {
	fmt.Fprintf(w, "\n%s\n", styles.Section.Render("Commands:"))

	maxLen := 0
	for _, node := range nodes {
		if !node.Hidden {
			if len(node.Name) > maxLen {
				maxLen = len(node.Name)
			}
		}
	}

	for _, node := range nodes {
		if node.Hidden {
			continue
		}

		cmdName := styles.Command.Render(node.Name)
		padding := strings.Repeat(" ", maxLen-len(node.Name)+2)

		help := node.Help

		fmt.Fprintf(w, "  %s%s%s\n", cmdName, padding, styles.Description.Render(help))
	}

	return nil
}

func printFlags(w io.Writer, flags []*kong.Flag, styles Styles, options kong.HelpOptions) error {
	if len(flags) == 0 {
		return nil
	}

	fmt.Fprintf(w, "\n%s\n", styles.Section.Render("Flags:"))

	maxLen := 0
	for _, flag := range flags {
		if !flag.Hidden {
			flagStr := formatFlagName(flag)
			if len(flagStr) > maxLen {
				maxLen = len(flagStr)
			}
		}
	}

	for _, flag := range flags {
		if flag.Hidden {
			continue
		}

		flagStr := formatFlagName(flag)
		styledFlag := styles.Flag.Render(flagStr)
		padding := strings.Repeat(" ", maxLen-len(flagStr)+2)

		helpText := flag.Help
		if flag.Default != "" {
			helpText += " " + styles.Default.Render(fmt.Sprintf("(default: %s)", flag.Default))
		}

		fmt.Fprintf(w, "  %s%s%s\n", styledFlag, padding, styles.Description.Render(helpText))
	}

	return nil
}

func formatFlagName(flag *kong.Flag) string {
	parts := []string{}

	if flag.Short != 0 {
		parts = append(parts, fmt.Sprintf("-%c", flag.Short))
	}

	parts = append(parts, fmt.Sprintf("--%s", flag.Name))

	result := strings.Join(parts, ", ")

	// Add type hint for non-boolean flags
	if flag.IsBool() {
		return result
	}

	return result + "=" + strings.ToUpper(flag.Name)
}
