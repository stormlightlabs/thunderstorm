// Package cli dispatches tstorm's subcommands.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/stormlightlabs/tstorm/internal/buildinfo"
)

// ErrUsage reports that the command line was wrong and usage has already been
// written. main exits non-zero without printing it again.
var ErrUsage = errors.New("usage")

type command struct {
	summary string
	run     func(ctx context.Context, args []string, stdout, stderr io.Writer) error
}

var commands = map[string]command{
	"version": {
		summary: "print the version of tstorm that is running",
		run: func(_ context.Context, _ []string, stdout, _ io.Writer) error {
			_, err := fmt.Fprintln(stdout, buildinfo.String())
			return err
		},
	},
}

// Run dispatches args to a subcommand.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		usage(stderr)
		return ErrUsage
	}
	name := args[0]
	if name == "-h" || name == "--help" || name == "help" {
		usage(stdout)
		return nil
	}
	cmd, ok := commands[name]
	if !ok {
		fmt.Fprintf(stderr, "tstorm: unknown command %q\n\n", name)
		usage(stderr)
		return ErrUsage
	}
	return cmd.run(ctx, args[1:], stdout, stderr)
}

func usage(w io.Writer) {
	fmt.Fprint(w, "tstorm runs the thunderstorm workflow's checks and board operations.\n\nUsage:\n\n\ttstorm <command> [arguments]\n\nCommands:\n\n")
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)
	tw := tabwriter.NewWriter(w, 0, 0, 4, ' ', 0)
	for _, name := range names {
		fmt.Fprintf(tw, "\t%s\t%s\n", name, commands[name].summary)
	}
	tw.Flush()
	fmt.Fprintln(w)
}
