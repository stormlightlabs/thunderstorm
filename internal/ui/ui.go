// Package ui renders tstorm's terminal output.
//
// Color is a convenience for a person reading a terminal and noise for
// everything else, so it is off unless the output really is a terminal. Three
// things turn it off: NO_COLOR set to anything, --no-color, and a destination
// that is not a TTY, which covers a pipe, a file, a hook's captured output and
// CI all at once. https://clig.dev and https://no-color.org both ask for this.
package ui

import (
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
)

// Printer carries the styles for one output stream.
type Printer struct {
	Error   lipgloss.Style
	Warn    lipgloss.Style
	OK      lipgloss.Style
	Subtle  lipgloss.Style
	Bold    lipgloss.Style
	colored bool
}

// New returns a Printer writing to w. Color is enabled only when w is a
// terminal and nothing has asked for it to be off.
func New(w io.Writer, forceNoColor bool) *Printer {
	renderer := lipgloss.NewRenderer(w)
	colored := !forceNoColor && !noColorEnv() && isTerminal(w)
	if !colored {
		renderer.SetColorProfile(termenv.Ascii)
	}
	return &Printer{
		Error:   renderer.NewStyle().Foreground(lipgloss.Color("1")).Bold(true),
		Warn:    renderer.NewStyle().Foreground(lipgloss.Color("3")),
		OK:      renderer.NewStyle().Foreground(lipgloss.Color("2")),
		Subtle:  renderer.NewStyle().Foreground(lipgloss.Color("8")),
		Bold:    renderer.NewStyle().Bold(true),
		colored: colored,
	}
}

// Colored reports whether this Printer emits escape sequences.
func (p *Printer) Colored() bool { return p.colored }

// noColorEnv reports whether NO_COLOR is set. Per no-color.org any value counts,
// including the empty string, so presence is the whole test.
func noColorEnv() bool {
	_, ok := os.LookupEnv("NO_COLOR")
	return ok
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}
