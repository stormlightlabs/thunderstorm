package cli

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stormlightlabs/thunderstorm"
	"github.com/stormlightlabs/thunderstorm/internal/check"
	"github.com/stormlightlabs/thunderstorm/internal/config"
	"github.com/stormlightlabs/thunderstorm/internal/render"
	"github.com/stormlightlabs/thunderstorm/internal/settings"
	"github.com/stormlightlabs/thunderstorm/internal/ui"
)

// install is what one run of the command was told to do. The fields are the
// flags, kept together so the steps below read as steps rather than as a
// signature each.
type install struct {
	target string
	dir    string
	source string
	check  bool
	adopt  bool

	noSettings bool
	noConfig   bool
	force      bool

	documents  string
	board      string
	statusFld  string
	track      string
	dictionary string
}

// workflowSource is the tree to read: the one the binary carries, unless a
// --source names a directory, which is how the loop itself is worked on.
func workflowSource(dir string) render.Source {
	if dir != "" {
		return render.Dir(dir)
	}
	return render.Embedded(thunderstorm.Workflow())
}

func installCmd(printer func(*cobra.Command) *ui.Printer) *cobra.Command {
	var in install

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the workflow into a repository",
		Long: "install puts the loop in a repository and leaves it able to run.\n\n" +
			"The payload is rendered from the workflow this binary carries, so\n" +
			"no checkout has to be anywhere. --source names a directory instead,\n" +
			"which is how the loop itself is worked on.\n\n" +
			"Three more things a payload needs and cannot carry: the commands\n" +
			"the workflow reserves for a person, denied in the repository's own\n" +
			"settings; the gate, registered against the same file, because\n" +
			"hooks.json is read by a plugin loader that a rendered payload never\n" +
			"reaches; and a config, written only from what the flags say, since\n" +
			"a board nobody named cannot be guessed.\n\n" +
			"--check reports all of it and writes nothing.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return in.run(cmd, printer(cmd))
		},
	}

	cmd.Flags().StringVar(&in.target, "target", "claude", "harness to install for: "+render.TargetNames())
	cmd.Flags().StringVar(&in.dir, "dir", ".", "repository to install into")
	cmd.Flags().StringVar(&in.source, "source", "", "workflow source directory (default: the one this binary carries)")
	cmd.Flags().BoolVar(&in.check, "check", false, "report what installing would do, and write nothing")
	cmd.Flags().BoolVar(&in.adopt, "adopt", false, "take over a payload directory carrying no marker")
	cmd.Flags().BoolVar(&in.noSettings, "no-settings", false, "leave the repository's settings file alone")
	cmd.Flags().BoolVar(&in.noConfig, "no-config", false, "write no .tstorm.toml")
	cmd.Flags().BoolVar(&in.force, "force", false, "replace a .tstorm.toml that is already there")
	cmd.Flags().StringVar(&in.documents, "documents", "", "tree the frontmatter check walks, created when it is not there")
	cmd.Flags().StringVar(&in.board, "board", "", "GitHub Projects board as owner/number")
	cmd.Flags().StringVar(&in.statusFld, "status-field", "Status", "field on that board carrying status")
	cmd.Flags().StringVar(&in.track, "track", "", "value separating this repository's work on a shared board")
	cmd.Flags().StringVar(&in.dictionary, "prose", "", "project dictionary the prose gate reads")
	return cmd
}

// merging reports whether this run owns the repository's settings file. Only
// Claude Code keeps its permissions and hooks there; Codex reads an execution
// policy and Pi its extension.
func (in install) merging(t *render.Target) bool {
	return !in.noSettings && t.Name == "claude"
}

func (in install) run(cmd *cobra.Command, p *ui.Printer) error {
	t, err := render.Lookup(in.target)
	if err != nil {
		return err
	}
	from := workflowSource(in.source)
	manifest, err := render.Load(from)
	if err != nil {
		return err
	}
	payload, err := render.Plan(manifest, t, from)
	if err != nil {
		return err
	}

	// The settings file is the repository's when install merges into it, so
	// the payload stops seeding one and the marker never claims the path.
	if in.merging(t) {
		payload.DropSeeds()
	}

	pending, err := in.payload(cmd, p, payload, filepath.Join(in.dir, t.Root))
	if err != nil {
		return err
	}
	waiting, err := in.settings(cmd, p, manifest, t)
	if err != nil {
		return err
	}
	pending += waiting
	waiting, err = in.config(cmd, p)
	if err != nil {
		return err
	}
	// --check answers a caller that wanted to know, so what it found has to
	// reach an exit code as well as the report.
	if pending += waiting; in.check && pending > 0 {
		return findings("%d %s waiting to be installed", pending, plural("change", pending))
	}
	return nil
}

// payload renders the loop, or says how the rendered one differs, and reports
// how many changes are waiting.
func (in install) payload(cmd *cobra.Command, p *ui.Printer, payload *render.Payload, out string) (int, error) {
	w := cmd.OutOrStdout()
	if in.check {
		diff, left, err := payload.Diff(out)
		if err != nil {
			return 0, err
		}
		if len(diff) > 0 {
			fmt.Fprintln(w, p.Bold.Render("would install"), payload.Summary(), p.Subtle.Render("→ "+out))
			for _, line := range diff {
				fmt.Fprintln(w, p.Subtle.Render("  "+line))
			}
		} else {
			fmt.Fprintln(w, p.OK.Render("up to date"), payload.Summary())
			if line := leftInPlace(left); line != "" {
				fmt.Fprintln(w, p.Subtle.Render("  "+line))
			}
		}
		printLimits(cmd, p, payload)
		return len(diff), nil
	}

	if in.adopt {
		if err := reportAdoption(cmd, p, payload, out); err != nil {
			return 0, err
		}
	}
	kept, err := payload.Write(out, in.adopt)
	if err != nil {
		return 0, err
	}
	fmt.Fprintln(w, p.OK.Render("installed"), payload.Summary(), p.Subtle.Render("→ "+out))
	if line := leftInPlace(kept); line != "" {
		fmt.Fprintln(w, p.Subtle.Render("  "+line))
	}
	printLimits(cmd, p, payload)
	return 0, nil
}

// settings merges the deny rules and the gate into the repository's own
// settings file.
func (in install) settings(cmd *cobra.Command, p *ui.Printer, m render.Manifest, t *render.Target) (int, error) {
	w := cmd.OutOrStdout()
	if !in.merging(t) {
		if !in.noSettings {
			fmt.Fprintln(w, p.Subtle.Render("  no settings to merge: "+t.Name+" carries its own policy"))
		}
		return 0, nil
	}

	file := filepath.Join(in.dir, t.Root, "settings.json")
	change, err := settings.Plan(file, denyRules(m), gateHooks(m, t))
	if err != nil {
		return 0, err
	}
	if change.Empty() {
		fmt.Fprintln(w, p.OK.Render("settings"), p.Subtle.Render(file+" already carries the deny rules and the gate"))
		return 0, nil
	}
	lines := change.Summary()
	verb := "merged into"
	if in.check {
		verb = "would merge into"
	}
	fmt.Fprintln(w, p.OK.Render("settings"), p.Subtle.Render(verb+" "+file))
	for _, line := range lines {
		fmt.Fprintln(w, p.Subtle.Render("  "+line))
	}
	if in.check {
		return len(lines), nil
	}
	return 0, change.Apply()
}

// config writes the settings a repository cannot be asked to guess at.
func (in install) config(cmd *cobra.Command, p *ui.Printer) (int, error) {
	w := cmd.OutOrStdout()
	if in.noConfig {
		return 0, nil
	}
	c, err := in.starter()
	if err != nil {
		return 0, err
	}
	file := filepath.Join(in.dir, config.Name)
	if c == nil {
		if held, err := os.Stat(file); err == nil && !held.IsDir() {
			fmt.Fprintln(w, p.OK.Render("config"), p.Subtle.Render(file+" is already there"))
			return 0, nil
		}
		fmt.Fprintln(w, p.Subtle.Render("  no config written: --board owner/number and --documents say what it would hold"))
		return 0, nil
	}
	if in.check {
		fmt.Fprintln(w, p.Bold.Render("would write"), p.Subtle.Render(file))
		return 1, nil
	}
	written, err := config.Create(in.dir, *c, in.force)
	if err != nil {
		return 0, err
	}
	fmt.Fprintln(w, p.OK.Render("config"), p.Subtle.Render(written))
	return 0, nil
}

// starter reads the config flags, and returns nil when none of them was given.
func (in install) starter() (*config.Config, error) {
	if in.documents == "" && in.board == "" && in.dictionary == "" {
		return nil, nil
	}
	c := config.Config{Documents: in.documents}
	c.Prose.Dictionary = in.dictionary
	if in.board != "" {
		owner, number, ok := strings.Cut(in.board, "/")
		if !ok {
			return nil, fmt.Errorf("--board takes owner/number, not %q", in.board)
		}
		n, err := strconv.Atoi(number)
		if err != nil {
			return nil, fmt.Errorf("--board takes owner/number, and %q is not a number", number)
		}
		c.Board.Owner, c.Board.Number = owner, n
		c.Board.StatusField = in.statusFld
		c.Board.Status = config.Status{Todo: "Todo", InProgress: "In Progress", Done: "Done"}
		if in.track != "" {
			c.Board.GroupField, c.Board.GroupValue = "Track", in.track
		}
	}
	return &c, nil
}

// denyRules is the workflow's reserved commands as the harness spells a rule.
func denyRules(m render.Manifest) []string {
	var out []string
	for _, prefix := range m.Policy.Deny {
		out = append(out, check.DenyRule(prefix))
	}
	return out
}

// gateHooks is every registration the manifest's hooks ask for, against the
// path the payload was installed at rather than the one a plugin loader would
// expand.
func gateHooks(m render.Manifest, t *render.Target) []settings.Hook {
	var out []settings.Hook
	for _, a := range m.Artifacts {
		if a.Kind != render.KindHook {
			continue
		}
		command := "$CLAUDE_PROJECT_DIR/" + path.Join(t.Root, "hooks", a.Name)
		for _, e := range a.Events {
			out = append(out, settings.Hook{
				Event:   e.Event,
				Matcher: e.Matcher,
				Command: command,
				Timeout: e.Timeout,
			})
		}
	}
	return out
}
