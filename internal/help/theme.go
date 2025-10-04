package help

import (
	"image/color"
	"os"

	"github.com/charmbracelet/lipgloss/v2"
)

var (
	Charple     = lipgloss.Color("#6B50FF")
	Pony        = lipgloss.Color("#FF4FBF")
	Cheeky      = lipgloss.Color("#FF79D0")
	Charcoal    = lipgloss.Color("#3A3943")
	Squid       = lipgloss.Color("#858392")
	Smoke       = lipgloss.Color("#BFBCC8")
	Guac        = lipgloss.Color("#12C78F")
	Ash         = lipgloss.Color("#DFDBDD")
	Cherry      = lipgloss.Color("#FF388B")
	BrightGreen = lipgloss.Color("#A6E22E")
	DarkGreen   = lipgloss.Color("#5F8700")
)

// ColorScheme defines colors for different help elements
type ColorScheme struct {
	Title       color.Color
	Command     color.Color
	Flag        color.Color
	Argument    color.Color
	Description color.Color
	Default     color.Color
	Section     color.Color
	Error       color.Color
}

// Styles contains all the lipgloss styles for help output
type Styles struct {
	Title       lipgloss.Style
	Command     lipgloss.Style
	Flag        lipgloss.Style
	Argument    lipgloss.Style
	Description lipgloss.Style
	Default     lipgloss.Style
	Section     lipgloss.Style
	Error       lipgloss.Style
}

// DefaultColorScheme returns a color scheme adapted from charm fang theme
func DefaultColorScheme(c lipgloss.LightDarkFunc) ColorScheme {
	return ColorScheme{
		Title:       Charple,
		Command:     c(Pony, Cheeky),
		Flag:        c(lipgloss.Color("#0CB37F"), Guac),
		Argument:    c(Charcoal, Ash),
		Description: c(Charcoal, Ash),
		Default:     c(Smoke, Squid),
		Section:     c(DarkGreen, BrightGreen),
		Error:       c(lipgloss.Color("#D70000"), lipgloss.Color("#FF5F87")),
	}
}

// NewStyles creates a new Styles instance from a color scheme
func NewStyles(scheme ColorScheme) Styles {
	return Styles{
		Title: lipgloss.NewStyle().
			Foreground(scheme.Title).
			Bold(true),
		Command: lipgloss.NewStyle().
			Foreground(scheme.Command).
			Bold(true),
		Flag: lipgloss.NewStyle().
			Foreground(scheme.Flag),
		Argument: lipgloss.NewStyle().
			Foreground(scheme.Argument),
		Description: lipgloss.NewStyle().
			Foreground(scheme.Description),
		Default: lipgloss.NewStyle().
			Foreground(scheme.Default).
			Faint(true),
		Section: lipgloss.NewStyle().
			Foreground(scheme.Section).
			Bold(true).
			Underline(true),
		Error: lipgloss.NewStyle().
			Foreground(scheme.Error).
			Bold(true),
	}
}

// DefaultStyles returns the default styled theme
func DefaultStyles() Styles {
	return NewStyles(DefaultColorScheme(lipgloss.LightDark(lipgloss.HasDarkBackground(os.Stdin, os.Stdout))))
}
