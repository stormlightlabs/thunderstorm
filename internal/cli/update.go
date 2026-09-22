package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/stormlightlabs/thunderstorm/internal/buildinfo"
	"github.com/stormlightlabs/thunderstorm/internal/release"
	"github.com/stormlightlabs/thunderstorm/internal/render"
	"github.com/stormlightlabs/thunderstorm/internal/ui"
)

func updateCmd(printer func(*cobra.Command) *ui.Printer) *cobra.Command {
	var (
		dir     string
		source  string
		check   bool
		offline bool
		self    bool
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Move an installed payload to what this binary carries",
		Long: "update re-renders the payload a repository already holds.\n\n" +
			"It finds the payload by its marker, says which version wrote it\n" +
			"and which one this binary carries, and merges the deny rules and\n" +
			"the gate again, so a command the workflow newly reserves reaches a\n" +
			"repository that installed months ago.\n\n" +
			"What it does not touch is the config: a repository's board and\n" +
			"documents tree are its own.\n\n" +
			"--self replaces the running binary from the latest release rather\n" +
			"than touching any repository.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := printer(cmd)
			if self {
				return updateSelf(cmd, p, check)
			}
			if err := updatePayload(cmd, p, dir, source, check); err != nil {
				return err
			}
			if !offline {
				reportNewerRelease(cmd, p)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", ".", "repository holding the payload")
	cmd.Flags().StringVar(&source, "source", "", "workflow source directory (default: the one this binary carries)")
	cmd.Flags().BoolVar(&check, "check", false, "report what updating would do, and write nothing")
	cmd.Flags().BoolVar(&offline, "offline", false, "ask GitHub nothing about newer releases")
	cmd.Flags().BoolVar(&self, "self", false, "replace this binary with the latest release")
	return cmd
}

// updateSelf replaces the running binary with the latest release.
func updateSelf(cmd *cobra.Command, p *ui.Printer, check bool) error {
	w := cmd.OutOrStdout()
	running := buildinfo.Release()
	latest, ok := release.Latest()
	if !ok {
		return failed("no release answered; try again, or take a build from GitHub by hand")
	}
	if !release.Newer(running.Version, latest.Tag) {
		fmt.Fprintln(w, p.OK.Render("up to date"), p.Subtle.Render(running.String()+" is not behind "+latest.Tag))
		return nil
	}
	if check {
		fmt.Fprintln(w, p.Bold.Render("would replace"), p.Subtle.Render(running.String()+" with "+latest.Tag))
		return nil
	}

	self, err := os.Executable()
	if err != nil {
		return failed("%v", err)
	}
	if self, err = filepath.EvalSymlinks(self); err != nil {
		return failed("%v", err)
	}
	if err := latest.Replace(self); err != nil {
		return failed("%v", err)
	}
	fmt.Fprintln(w, p.OK.Render("replaced"), p.Subtle.Render(self+" is now "+latest.Tag))
	return nil
}

// updatePayload re-renders whichever payload the repository holds.
func updatePayload(cmd *cobra.Command, p *ui.Printer, dir, source string, check bool) error {
	held, root, err := installedUnder(dir)
	if err != nil {
		return err
	}

	in := install{target: held.Target, dir: dir, source: source, check: check, noConfig: true}
	t, err := render.Lookup(held.Target)
	if err != nil {
		return fmt.Errorf("%s was installed for %s, which this binary does not know", root, held.Target)
	}
	from := workflowSource(source)
	manifest, err := render.Load(from)
	if err != nil {
		return err
	}
	payload, err := render.Plan(manifest, t, from)
	if err != nil {
		return err
	}
	if in.merging(t) {
		payload.DropSeeds()
	}

	fmt.Fprintln(cmd.OutOrStdout(), p.Bold.Render(versionMove(held.Version, manifest.Version)), p.Subtle.Render(root))

	pending, err := in.payload(cmd, p, payload, root)
	if err != nil {
		return err
	}
	waiting, err := in.settings(cmd, p, manifest, t)
	if err != nil {
		return err
	}
	if pending += waiting; check && pending > 0 {
		return findings("%d %s waiting", pending, plural("change", pending))
	}
	return nil
}

// installedUnder finds the payload in a repository. Every target roots its own
// directory, so the marker is what says which one is there rather than the
// caller having to name it.
func installedUnder(dir string) (render.Installed, string, error) {
	for _, t := range render.Targets() {
		root := filepath.Join(dir, t.Root)
		held, ok, err := render.Read(root)
		if err != nil {
			return held, root, err
		}
		if ok {
			return held, root, nil
		}
	}
	return render.Installed{}, "", fmt.Errorf("%s holds no payload; tstorm install puts one there", dir)
}

// versionMove says what the update is between. A payload installed before the
// marker carried a version names none, which is a fact rather than a failure.
func versionMove(from, to string) string {
	switch {
	case from == to:
		return "up to date at " + to
	case from == "":
		return "updating to " + to
	default:
		return "updating " + from + " → " + to
	}
}

// reportNewerRelease says when the binary is behind what GitHub publishes. It
// is a remark rather than a step, so a network that will not answer costs
// nothing and says nothing.
func reportNewerRelease(cmd *cobra.Command, p *ui.Printer) {
	latest, ok := release.Latest()
	if !ok {
		return
	}
	running := buildinfo.Release()
	if !release.Newer(running.Version, latest.Tag) {
		return
	}
	fmt.Fprintln(cmd.OutOrStdout(), p.Bold.Render("tstorm "+latest.Tag+" is out"),
		p.Subtle.Render("this is "+running.String()+"; tstorm update --self takes it"))
}
