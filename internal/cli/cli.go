// Package cli builds tstorm's command tree.
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/stormlightlabs/tstorm/internal/buildinfo"
	"github.com/stormlightlabs/tstorm/internal/ui"
)

// Root returns the tstorm command tree writing to stdout and stderr.
func Root(stdout, stderr io.Writer) *cobra.Command {
	var noColor bool

	root := &cobra.Command{
		Use:   "tstorm",
		Short: "Run the thunderstorm workflow's checks and board operations",
		Long: "tstorm runs the thunderstorm workflow's checks and board operations.\n\n" +
			"The skills a harness loads are prose, and prose cannot fail a run.\n" +
			"This is the other half: the checks that hold the loop to what the\n" +
			"prose says, and the board writes too racy to leave to a model.",
		SilenceUsage:      true,
		SilenceErrors:     true,
		DisableAutoGenTag: true,
		// clig.dev: a bare invocation shows help rather than doing something.
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable color even on a terminal")
	root.SetVersionTemplate("{{.Version}}\n")
	root.Version = buildinfo.String()

	printer := func(cmd *cobra.Command) *ui.Printer {
		return ui.New(cmd.OutOrStdout(), noColor)
	}

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
func Execute(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	root := Root(stdout, stderr)
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}
