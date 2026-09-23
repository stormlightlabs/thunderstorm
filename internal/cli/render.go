package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stormlightlabs/thunderstorm/internal/render"
	"github.com/stormlightlabs/thunderstorm/internal/ui"
)

// printLimits says what the payload does not carry to this harness. It is not
// a failure, so it goes to stdout under the summary rather than to stderr.
func printLimits(cmd *cobra.Command, p *ui.Printer, payload *render.Payload) {
	for _, limit := range payload.Limits {
		fmt.Fprintln(cmd.OutOrStdout(), p.Subtle.Render("  "+limit))
	}
}

// leftInPlace says what the render did not write.
func leftInPlace(kept []string) string {
	switch len(kept) {
	case 0:
		return ""
	case 1:
		return "left " + kept[0] + ", which the repository owns"
	default:
		return fmt.Sprintf("left %d files the repository owns, %s first", len(kept), kept[0])
	}
}

// printReplaced says what --replace overwrote, by path: a count alone is
// exactly what an overwrite with nothing named already reported.
func printReplaced(cmd *cobra.Command, p *ui.Printer, replaced []string) {
	if len(replaced) == 0 {
		return
	}
	w := cmd.OutOrStdout()
	header := fmt.Sprintf("  replaced %d %s the repository held:", len(replaced), plural("path", len(replaced)))
	fmt.Fprintln(w, p.Subtle.Render(header))
	for _, path := range replaced {
		fmt.Fprintln(w, p.Subtle.Render("    "+path))
	}
}

func renderCmd(printer func(*cobra.Command) *ui.Printer) *cobra.Command {
	var (
		target  string
		source  string
		out     string
		check   bool
		replace bool
	)

	cmd := &cobra.Command{
		Use:   "render --target " + render.TargetNames(),
		Short: "Build one harness's payload from the workflow source",
		Long: "render builds a harness's payload from the canonical workflow source.\n\n" +
			"One source, one manifest, one payload per harness. An artifact that\n" +
			"needs something the target does not provide stops the render and\n" +
			"names the gap, because a payload that installs and then skips the\n" +
			"review fan-out is worse than no payload.\n\n" +
			"The destination defaults to the harness's own directory in this\n" +
			"repository: .claude, .codex, .pi. Building the payload this\n" +
			"repository commits is the other case, and it names --out.\n\n" +
			"A file the destination already holds is left alone, whether or\n" +
			"not tstorm wrote it, unless the payload also writes that path:\n" +
			"that collision is refused and named. --replace overwrites\n" +
			"exactly the paths that collided and leaves the rest of the\n" +
			"directory as it was.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := printer(cmd)
			t, err := render.Lookup(target)
			if err != nil {
				return err
			}
			from := render.Dir(source)
			manifest, err := render.Load(from)
			if err != nil {
				return err
			}
			payload, err := render.Plan(manifest, t, from)
			if err != nil {
				return err
			}
			if out == "" {
				out = t.Root
			}

			if check {
				diff, left, err := payload.Diff(out)
				if err != nil {
					return err
				}
				if len(diff) > 0 {
					rerun := fmt.Sprintf("tstorm render --target %s", t.Name)
					if out != t.Root {
						rerun += " --out " + out
					}
					if source != "workflow" {
						rerun += " --source " + source
					}
					return fmt.Errorf("%s is not what the source renders:\n  %s\n\nRun %s",
						out, strings.Join(diff, "\n  "), rerun)
				}
				fmt.Fprintln(cmd.OutOrStdout(), p.OK.Render("up to date"), payload.Summary())
				if line := leftInPlace(left); line != "" {
					fmt.Fprintln(cmd.OutOrStdout(), p.Subtle.Render("  "+line))
				}
				printLimits(cmd, p, payload)
				return nil
			}

			kept, replaced, err := payload.Write(out, replace)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), p.OK.Render("rendered"), payload.Summary(), p.Subtle.Render("→ "+out))
			if left := leftInPlace(kept); left != "" {
				fmt.Fprintln(cmd.OutOrStdout(), p.Subtle.Render("  "+left))
			}
			printReplaced(cmd, p, replaced)
			printLimits(cmd, p, payload)
			return nil
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "harness to build for: "+render.TargetNames())
	cmd.Flags().StringVar(&source, "source", "workflow", "canonical workflow source directory")
	cmd.Flags().StringVar(&out, "out", "", "where to write the payload (default: the harness directory, .claude for claude)")
	cmd.Flags().BoolVar(&check, "check", false, "report whether the payload on disk matches the source, and write nothing")
	cmd.Flags().BoolVar(&replace, "replace", false, "overwrite paths that collide with the payload, and leave the rest of the directory alone")
	if err := cmd.MarkFlagRequired("target"); err != nil {
		panic(err)
	}
	return cmd
}
