package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/stormlightlabs/thunderstorm/internal/config"
	"github.com/stormlightlabs/thunderstorm/internal/render"
	"github.com/stormlightlabs/thunderstorm/internal/settings"
	"github.com/stormlightlabs/thunderstorm/internal/ui"
)

func uninstallCmd(printer func(*cobra.Command) *ui.Printer) *cobra.Command {
	var (
		dir          string
		check        bool
		keepSettings bool
	)

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Take the workflow back out of a repository",
		Long: "uninstall removes what install put in a repository.\n\n" +
			"The marker is the record of what the payload owns, so a file the\n" +
			"repository put in the payload directory stays, and a directory\n" +
			"goes only once nothing is left in it.\n\n" +
			"The deny rules and the gate registrations come back out of the\n" +
			"settings file, and anything else in that file is left as it was,\n" +
			"a hook of the repository's own on the same event included.\n\n" +
			"What it does not touch is .tstorm.toml. A repository's board and\n" +
			"documents tree outlive the loop that read them.\n\n" +
			"--check reports all of it and removes nothing.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return uninstall(cmd, printer(cmd), dir, check, keepSettings)
		},
	}

	cmd.Flags().StringVar(&dir, "dir", ".", "repository to take the workflow out of")
	cmd.Flags().BoolVar(&check, "check", false, "report what removing would do, and remove nothing")
	cmd.Flags().BoolVar(&keepSettings, "keep-settings", false, "leave the deny rules and the gate in the settings file")
	return cmd
}

func uninstall(cmd *cobra.Command, p *ui.Printer, dir string, check, keepSettings bool) error {
	found, err := installedUnder(dir)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	for _, one := range found {
		if check {
			fmt.Fprintln(w, p.Bold.Render("would remove"), p.Subtle.Render(fmt.Sprintf(
				"%d %s from %s", len(one.Files), plural("file", len(one.Files)), one.Root)))
			continue
		}
		removed, err := render.Remove(one.Root)
		if err != nil {
			return err
		}
		fmt.Fprintln(w, p.OK.Render("removed"), p.Subtle.Render(fmt.Sprintf(
			"%d %s from %s", len(removed), plural("file", len(removed)), one.Root)))
	}

	if err := retireSettings(cmd, p, dir, found, check, keepSettings); err != nil {
		return err
	}

	// The config is the repository's answer about itself, not the loop's, so
	// saying where it is beats removing it and beats silence.
	file := filepath.Join(dir, config.Name)
	if info, err := os.Stat(file); err == nil && !info.IsDir() {
		fmt.Fprintln(w, p.Subtle.Render("  "+file+" is left where it is"))
	}
	return nil
}

// retireSettings takes the deny rules and the gate registrations back out of
// the settings file, for the one target that keeps them there.
func retireSettings(cmd *cobra.Command, p *ui.Printer, dir string, found []held, check, keep bool) error {
	if keep {
		return nil
	}
	w := cmd.OutOrStdout()
	for _, one := range found {
		t, err := render.Lookup(one.Target)
		if err != nil {
			continue
		}
		in := install{target: one.Target, dir: dir}
		if !in.merging(t) {
			continue
		}
		manifest, err := render.Load(workflowSource(""))
		if err != nil {
			return err
		}

		file := filepath.Join(one.Root, "settings.json")
		removal, err := settings.PlanRemoval(file, denyRules(manifest), gateHooks(manifest, t))
		if err != nil {
			return err
		}
		if removal.Empty() {
			fmt.Fprintln(w, p.Subtle.Render("  "+file+" carries none of the loop's rules"))
			continue
		}
		verb := "taken out of"
		if check {
			verb = "would come out of"
		}
		fmt.Fprintln(w, p.OK.Render("settings"), p.Subtle.Render(verb+" "+file))
		for _, line := range removal.Summary() {
			fmt.Fprintln(w, p.Subtle.Render("  "+line))
		}
		if check {
			continue
		}
		if err := removal.Apply(); err != nil {
			return err
		}
	}
	return nil
}
