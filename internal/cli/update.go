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
	current, err := release.Current()
	if err != nil {
		return failed("no release answered: %v", err)
	}
	if !release.Newer(running.Version, current.Tag) {
		fmt.Fprintln(w, p.OK.Render("up to date"), p.Subtle.Render(running.String()+" is not behind "+current.Tag))
		return nil
	}
	if check {
		fmt.Fprintln(w, p.Bold.Render("would replace"), p.Subtle.Render(running.String()+" with "+current.Tag))
		return nil
	}

	self, err := os.Executable()
	if err != nil {
		return failed("%v", err)
	}
	if self, err = filepath.EvalSymlinks(self); err != nil {
		return failed("%v", err)
	}
	installed, err := release.Install(self, running.Version)
	if err != nil {
		return failed("%v", err)
	}
	fmt.Fprintln(w, p.OK.Render("replaced"), p.Subtle.Render(self+" is now "+installed))
	return nil
}

// updatePayload re-renders whichever payload the repository holds.
func updatePayload(cmd *cobra.Command, p *ui.Printer, dir, source string, check bool) error {
	found, err := installedUnder(dir)
	if err != nil {
		return err
	}
	pending := 0
	for _, one := range found {
		waiting, err := updateOne(cmd, p, one, dir, source, check)
		if err != nil {
			return err
		}
		pending += waiting
	}
	if check && pending > 0 {
		return findings("%d %s waiting", pending, plural("change", pending))
	}
	return nil
}

// updateOne moves one payload to what this binary carries.
func updateOne(cmd *cobra.Command, p *ui.Printer, one held, dir, source string, check bool) (int, error) {
	in := install{target: one.Target, dir: dir, source: source, check: check, noConfig: true}
	t, err := render.Lookup(one.Target)
	if err != nil {
		return 0, fmt.Errorf("%s was installed for %s, which this binary does not know", one.Root, one.Target)
	}
	from := workflowSource(source)
	manifest, err := render.Load(from)
	if err != nil {
		return 0, err
	}
	payload, err := render.Plan(manifest, t, from)
	if err != nil {
		return 0, err
	}
	if in.merging(t) {
		payload.DropSeeds()
	}

	fmt.Fprintln(cmd.OutOrStdout(), p.Bold.Render(versionMove(one.Version, manifest.Version)), p.Subtle.Render(one.Root))

	pending, err := in.payload(cmd, p, payload, one.Root)
	if err != nil {
		return 0, err
	}
	waiting, err := in.settings(cmd, p, manifest, t)
	if err != nil {
		return 0, err
	}
	return pending + waiting, nil
}

// held is one payload a repository carries, and where it sits.
type held struct {
	render.Installed
	Root string
}

// installedUnder finds every payload in a repository. Each target roots its
// own directory, so a repository working two harnesses carries two, and an
// update or an uninstall that moved only the first would leave the other on a
// version nobody chose.
func installedUnder(dir string) ([]held, error) {
	var found []held
	for _, t := range render.Targets() {
		root := filepath.Join(dir, t.Root)
		in, ok, err := render.Read(root)
		if err != nil {
			return nil, err
		}
		if ok {
			found = append(found, held{Installed: in, Root: root})
		}
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("%s holds no payload; tstorm install puts one there", dir)
	}
	return found, nil
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
