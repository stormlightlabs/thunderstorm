package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/stormlightlabs/thunderstorm/internal/dispatch"
	"github.com/stormlightlabs/thunderstorm/internal/ui"
)

func dispatchCmd(printer func(*cobra.Command) *ui.Printer) *cobra.Command {
	var (
		harness     string
		role        string
		worktree    string
		provider    string
		model       string
		thinking    string
		multiplexer string
		out         string
		timeout     time.Duration
	)

	cmd := &cobra.Command{
		Use:   "dispatch --role <role> --worktree <dir> --model <id> -- <task>",
		Short: "Run one thunderstorm role as a session of its own on Pi",
		Long: "dispatch runs one role of a thunderstorm run as a separate pi session.\n\n" +
			"Claude Code provisions a subagent for a role and Codex spawns one. Pi\n" +
			"ships neither, so the loop opens a tmux window or Zellij tab in the\n" +
			"directory the role owns, running the provider and model the dispatch\n" +
			"names. The role's report comes back on stdout for the next pass to\n" +
			"read, and its transcript stays on disk, where the model that answered\n" +
			"is recorded.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := printer(cmd)
			if err := dispatch.CheckHarness(harness); err != nil {
				return err
			}
			r, err := dispatch.LookupRole(role)
			if err != nil {
				return err
			}
			res, runErr := dispatch.Run(cmd.Context(), dispatch.Request{
				Role:        r,
				Worktree:    worktree,
				Provider:    provider,
				Model:       model,
				Thinking:    thinking,
				Multiplexer: multiplexer,
				Task:        strings.Join(args, " "),
				Out:         out,
				Timeout:     timeout,
			})
			// The report is printed even when the session failed. A pass that
			// got most of the way through still wrote something the operator
			// needs, and the error says where the rest of it is.
			if res.Report != "" {
				w := cmd.OutOrStdout()
				fmt.Fprintln(w, p.Bold.Render(fmt.Sprintf("%s · %s · %s", res.Role, res.Model, res.Thinking)))
				if len(res.Untranslated) > 0 {
					fmt.Fprintln(w, p.Subtle.Render("pi has no "+strings.Join(res.Untranslated, ", ")))
				}
				fmt.Fprintln(w, p.Subtle.Render(res.Out))
				fmt.Fprintf(w, "\n%s\n", res.Report)
			}
			return runErr
		},
	}

	cmd.Flags().StringVar(&harness, "harness", dispatch.Pi, "harness to dispatch on")
	cmd.Flags().StringVar(&role, "role", "", "role to run: "+dispatch.RoleNames())
	cmd.Flags().StringVar(&worktree, "worktree", "", "directory the role works in, which the worktree skill creates")
	cmd.Flags().StringVar(&provider, "provider", "", "Pi provider (optional when --model includes it)")
	cmd.Flags().StringVar(&model, "model", "", "model the session runs, as Pi names it")
	cmd.Flags().StringVar(&thinking, "thinking", "medium", "reasoning level: "+dispatch.ThinkingLevels())
	cmd.Flags().StringVar(&multiplexer, "multiplexer", "auto", "terminal multiplexer: "+dispatch.Multiplexers())
	cmd.Flags().StringVar(&out, "out", "", "where the transcript goes (default a new directory under the temporary directory)")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "how long to wait before killing the session")
	for _, required := range []string{"role", "worktree", "model"} {
		if err := cmd.MarkFlagRequired(required); err != nil {
			panic(err)
		}
	}
	return cmd
}
