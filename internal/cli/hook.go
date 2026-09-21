package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stormlightlabs/thunderstorm/internal/hook"
	"github.com/stormlightlabs/thunderstorm/internal/ui"
)

func hookCmd(_ func(*cobra.Command) *ui.Printer) *cobra.Command {
	return &cobra.Command{
		Use:   "hook",
		Short: "Answer a harness's tool hook, reading the event on stdin",
		Long: "hook reads one hook event as JSON on stdin and writes the harness's\n" +
			"reply as JSON on stdout.\n\n" +
			"Claude Code and Codex send the same event and read the same reply, so\n" +
			"one registration covers both. Pi has no hooks; its package extension\n" +
			"translates its own event into this shape and calls the same command.\n\n" +
			"A write is read for the frontmatter an issue cites it by and for the\n" +
			"tells the catalogue lists, and both are reported. A commit message is\n" +
			"read for its shape, which is the one thing here that refuses: a\n" +
			"subject that will not read in git log cannot be fixed later.\n\n" +
			"It exits 0 whatever it finds. The answer is in the reply, and a hook\n" +
			"that fails on its own findings tells the harness nothing it can use.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var event hook.Event
			if err := json.NewDecoder(cmd.InOrStdin()).Decode(&event); err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "tstorm hook: cannot read the event:", err)
				return nil
			}
			answer, err := hook.Decide(event)
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "tstorm hook:", err)
				return nil
			}
			if answer.HookSpecificOutput == nil {
				return nil
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(answer)
		},
	}
}
