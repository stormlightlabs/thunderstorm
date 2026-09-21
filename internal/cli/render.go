package cli

import (
	"fmt"
	"path/filepath"
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

// leftInPlace says what the render did not write, so an operator reads that
// the repository's own configuration survived rather than assuming it.
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

// reportAdoption says what adoption takes over and what it leaves, before
// anything is written. What it leaves is where an older payload's files show
// up: a render removes what it wrote, and a directory with no marker has no
// record of that.
func reportAdoption(cmd *cobra.Command, p *ui.Printer, payload *render.Payload, out string) error {
	a, err := payload.Adopt(out)
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, p.OK.Render("adopting"), out)
	fmt.Fprintln(w, p.Subtle.Render(fmt.Sprintf("  replaces %d files the payload writes", len(a.Replace))))
	fmt.Fprintln(w, p.Subtle.Render(fmt.Sprintf("  leaves %d files it does not", len(a.Leave))))
	for _, path := range a.Leave {
		fmt.Fprintln(w, p.Subtle.Render("    "+path))
	}
	return nil
}

func renderCmd(printer func(*cobra.Command) *ui.Printer) *cobra.Command {
	var (
		target string
		source string
		out    string
		check  bool
		adopt  bool
	)

	cmd := &cobra.Command{
		Use:   "render --target " + render.TargetNames(),
		Short: "Build one harness's payload from the workflow source",
		Long: "render builds a harness's payload from the canonical workflow source.\n\n" +
			"One source, one manifest, one payload per harness. An artifact that\n" +
			"needs something the target does not provide stops the render and\n" +
			"names the gap, because a payload that installs and then skips the\n" +
			"review fan-out is worse than no payload.\n\n" +
			"A destination carrying no marker is refused, because a directory\n" +
			"with no record of what wrote it may be somebody's work. --adopt\n" +
			"says otherwise: the files this payload writes are taken over, the\n" +
			"rest are left, and the marker is written for the next render. With\n" +
			"--check it reports what adopting would do and writes nothing.",
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

			if check && adopt {
				return reportAdoption(cmd, p, payload, out)
			}

			if check {
				diff, left, err := payload.Diff(out)
				if err != nil {
					return err
				}
				if len(diff) > 0 {
					rerun := fmt.Sprintf("tstorm render --target %s", t.Name)
					if out != filepath.Join("payloads", t.Name) {
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

			if adopt {
				if err := reportAdoption(cmd, p, payload, out); err != nil {
					return err
				}
			}
			kept, err := payload.Write(out, adopt)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), p.OK.Render("rendered"), payload.Summary(), p.Subtle.Render("→ "+out))
			if left := leftInPlace(kept); left != "" {
				fmt.Fprintln(cmd.OutOrStdout(), p.Subtle.Render("  "+left))
			}
			printLimits(cmd, p, payload)
			return nil
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "harness to build for: "+render.TargetNames())
	cmd.Flags().StringVar(&source, "source", "workflow", "canonical workflow source directory")
	cmd.Flags().StringVar(&out, "out", "", "where to write the payload (default payloads/<target>)")
	cmd.Flags().BoolVar(&check, "check", false, "report whether the payload on disk matches the source, and write nothing")
	cmd.Flags().BoolVar(&adopt, "adopt", false, "take over a directory carrying no marker, replacing the payload's own files and leaving the rest")
	if err := cmd.MarkFlagRequired("target"); err != nil {
		panic(err)
	}
	return cmd
}
