// Package ui renders tstorm's terminal output.
//
// Escape sequences help a person at a terminal and corrupt anything that parses
// the output, so color stays off unless the destination is a terminal. It is
// also off whenever NO_COLOR is present or --no-color is passed. The TTY test
// covers a pipe, a file, a hook's captured output and a CI log together.
// See https://clig.dev and https://no-color.org.
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
