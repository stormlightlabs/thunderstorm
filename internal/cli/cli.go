// Package cli builds tstorm's command tree.
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/stormlightlabs/thunderstorm/internal/buildinfo"
	"github.com/stormlightlabs/thunderstorm/internal/ui"
)

// Root returns the tstorm command tree reading stdin and writing to stdout
// and stderr. Only the hook command reads stdin, and it is the reason the
// stream is passed in rather than taken from the process: a test has to be
// able to hand it an event.
func Root(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	var noColor bool

	root := &cobra.Command{
		Use:   "tstorm",
		Short: "Run the thunderstorm workflow's checks and board operations",
		Long: "tstorm runs the thunderstorm workflow's checks and board operations.\n\n" +
			"An agent reads the workflow as instructions and can skip one without\n" +
			"leaving a record. These checks exit non-zero instead. The board\n" +
			"commands write to GitHub Projects, where a lost race has to be\n" +
			"detected rather than hoped away.",
		SilenceUsage:      true,
		SilenceErrors:     true,
		DisableAutoGenTag: true,
		// clig.dev: a bare invocation shows help rather than doing something.
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable color even on a terminal")
	root.SetVersionTemplate("{{.Version}}\n")
	root.Version = buildinfo.String()

	printer := func(cmd *cobra.Command) *ui.Printer {
		return ui.New(cmd.OutOrStdout(), noColor)
	}

	root.AddCommand(boardCmd(printer))
	root.AddCommand(checkCmd(printer))
	root.AddCommand(dispatchCmd(printer))
	root.AddCommand(hookCmd(printer))
	root.AddCommand(installCmd(printer))
	root.AddCommand(pushCmd(printer))
	root.AddCommand(renderCmd(printer))
	root.AddCommand(ulidCmd(printer))
	root.AddCommand(uninstallCmd(printer))
	root.AddCommand(updateCmd(printer))
	root.AddCommand(versionCmd(printer))
	return root
}

func versionCmd(printer func(*cobra.Command) *ui.Printer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of tstorm that is running",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := printer(cmd)
			_, err := fmt.Fprintln(cmd.OutOrStdout(), p.Bold.Render(buildinfo.String()))
			return err
		},
	}
}

// Execute runs the command tree against args.
func Execute(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	root := Root(stdin, stdout, stderr)
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}
