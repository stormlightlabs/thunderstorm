package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stormlightlabs/thunderstorm/internal/render"
	"github.com/stormlightlabs/thunderstorm/internal/ui"
)

func renderCmd(printer func(*cobra.Command) *ui.Printer) *cobra.Command {
	var (
		target string
		source string
		out    string
		check  bool
	)

	cmd := &cobra.Command{
		Use:   "render --target " + render.TargetNames(),
		Short: "Build one harness's payload from the workflow source",
		Long: "render builds a harness's payload from the canonical workflow source.\n\n" +
			"One source, one manifest, one payload per harness. An artifact that\n" +
			"needs something the target does not provide stops the render and\n" +
			"names the gap, because a payload that installs and then skips the\n" +
			"review fan-out is worse than no payload.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := printer(cmd)
			t, err := render.Lookup(target)
			if err != nil {
				return err
			}
			manifest, err := render.Load(source)
			if err != nil {
				return err
			}
			payload, err := render.Plan(manifest, t, source)
			if err != nil {
				return err
			}
			if out == "" {
				out = filepath.Join("payloads", t.Name)
			}

			if check {
				diff, err := payload.Diff(out)
				if err != nil {
					return err
				}
				if len(diff) > 0 {
					return fmt.Errorf("%s is not what the source renders:\n  %s\n\nRun tstorm render --target %s",
						out, strings.Join(diff, "\n  "), t.Name)
				}
				fmt.Fprintln(cmd.OutOrStdout(), p.OK.Render("up to date"), payload.Summary())
				return nil
			}

			if err := payload.Write(out); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), p.OK.Render("rendered"), payload.Summary(), p.Subtle.Render("→ "+out))
			return nil
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "harness to build for: "+render.TargetNames())
	cmd.Flags().StringVar(&source, "source", "workflow", "canonical workflow source directory")
	cmd.Flags().StringVar(&out, "out", "", "where to write the payload (default payloads/<target>)")
	cmd.Flags().BoolVar(&check, "check", false, "report whether the payload on disk matches the source, and write nothing")
	if err := cmd.MarkFlagRequired("target"); err != nil {
		panic(err)
	}
	return cmd
}
