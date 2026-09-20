package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/stormlightlabs/thunderstorm/internal/board"
	"github.com/stormlightlabs/thunderstorm/internal/config"
	"github.com/stormlightlabs/thunderstorm/internal/ui"
)

func boardCmd(printer func(*cobra.Command) *ui.Printer) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "board",
		Short: "Read and write the loop's GitHub Projects board",
		Long: "board performs the loop's board operations against the project.\n\n" +
			"Status is a single select on a Projects V2 board, which is GraphQL\n" +
			"only, and issue dependencies are REST. No transport a coding agent\n" +
			"has performs either, so what a skill could offer was a description of\n" +
			"a read, a write, and a race to notice afterwards. These commands do\n" +
			"the reading and the writing and report what changed.\n\n" +
			"Exit codes: 0 when the board ended up as asked, 1 when it did not,\n" +
			"and 2 when the command could not run. A claim somebody else won and\n" +
			"a missing token are different answers.\n\n" +
			"The project, the status field, the option names and the field that\n" +
			"separates one repository's work from another's come from " + config.Name + ".",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}

	cmd.PersistentFlags().BoolVar(&asJSON, "json", false, "write the result as JSON")
	cmd.AddCommand(
		boardListCmd(printer, &asJSON),
		boardShowCmd(printer, &asJSON),
		boardClaimCmd(printer, &asJSON),
		boardMoveCmd(printer, &asJSON),
		boardFileCmd(printer, &asJSON),
		boardAddCmd(printer, &asJSON),
		boardSubCmd(printer, &asJSON),
		boardBlockedByCmd(printer, &asJSON),
	)
	return cmd
}

// client builds the board client from the repository the command runs in.
// Everything it needs that is not configuration is the token, which comes
// from the environment or from gh.
func client(cmd *cobra.Command) (*board.Client, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, failed("%s", err)
	}
	settings, err := config.Load(dir)
	if err != nil {
		return nil, failed("%s", err)
	}
	c, err := board.New(settings.Board, board.Options{Dir: dir})
	if err != nil {
		return nil, failed("%s", err)
	}
	return c, nil
}

func boardListCmd(printer func(*cobra.Command) *ui.Printer, asJSON *bool) *cobra.Command {
	var status string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List this repository's issues on the board",
		Long: "list reads the board, filtered to this repository and to the group\n" +
			"the configuration names.\n\n" +
			"An item with no status reads as todo, because that is what every\n" +
			"reader here makes of one.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}

			items, err := c.Items(cmd.Context())
			if err != nil {
				return failed("%s", err)
			}
			if status != "" {
				wanted, err := board.ParseStatus(status)
				if err != nil {
					return failed("%s", err)
				}
				items = keep(items, wanted)
			}

			if *asJSON {
				return write(cmd, items)
			}
			p := printer(cmd)
			out := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for _, item := range items {
				fmt.Fprintf(out, "#%d\t%s\t%s\t%s\n", item.Number,
					p.Bold.Render(string(item.Status)),
					strings.Join(item.Assignees, ","), item.Title)
			}
			if len(items) == 0 {
				fmt.Fprintln(out, p.Subtle.Render("nothing on the board for "+c.Repository()))
			}
			return out.Flush()
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "only this state: todo, in-progress or done")
	return cmd
}

func boardShowCmd(printer func(*cobra.Command) *ui.Printer, asJSON *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "show <issue>",
		Short: "Read one issue's status, assignees, children and blockers",
		Long: "show reads one issue from the board and from the issue itself.\n\n" +
			"Whether it is claimable takes both: the board carries the status,\n" +
			"and the issue carries who holds it and what it waits for.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			number, err := issueNumber(args[0])
			if err != nil {
				return err
			}
			c, err := client(cmd)
			if err != nil {
				return err
			}
			report, err := c.Show(cmd.Context(), number)
			if err != nil {
				return failed("%s", err)
			}
			if *asJSON {
				return write(cmd, report)
			}

			p := printer(cmd)
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s %s\n", p.Bold.Render(fmt.Sprintf("#%d", report.Issue)), report.Title)
			fmt.Fprintf(w, "%s  %s  %s\n", report.Status, strings.ToLower(report.State), report.URL)
			if len(report.Assignees) > 0 {
				fmt.Fprintf(w, "held by %s\n", strings.Join(report.Assignees, ", "))
			}
			printRefs(w, p, "sub-issues", report.SubIssues)
			printRefs(w, p, "blocked by", report.BlockedBy)
			printRefs(w, p, "blocking", report.Blocking)
			if report.Claimable {
				fmt.Fprintln(w, p.OK.Render("claimable"))
			} else {
				fmt.Fprintln(w, p.Subtle.Render("not claimable: "+report.Unclaimed))
			}
			return nil
		},
	}
}

func boardClaimCmd(printer func(*cobra.Command) *ui.Printer, asJSON *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "claim <issue>",
		Short: "Take an issue: status first, then the assignment",
		Long: "claim writes the status, assigns the token's own login, and reads\n" +
			"the assignment back.\n\n" +
			"The order survives an interruption. A run killed between the two\n" +
			"leaves In Progress with nobody on it, which the stale-claim rule\n" +
			"returns to the queue; assigning first would leave an owned issue\n" +
			"reading Todo, which the next run takes as free.\n\n" +
			"GitHub's assignment endpoint adds rather than replaces, so two runs\n" +
			"claiming at once both find themselves on the issue. The claim held\n" +
			"only when the list is exactly one login and it is this one. A lost\n" +
			"claim gives back its own assignment, leaves the status to whoever\n" +
			"won it, and exits 1.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			number, err := issueNumber(args[0])
			if err != nil {
				return err
			}
			c, err := client(cmd)
			if err != nil {
				return err
			}
			claim, err := c.Claim(cmd.Context(), number)
			if err != nil {
				return failed("%s", err)
			}

			if *asJSON {
				if err := write(cmd, claim); err != nil {
					return err
				}
			} else {
				p := printer(cmd)
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "#%d %s\n", claim.Issue, claim.Title)
				printChanged(w, p, claim.Changed)
				if claim.Won {
					fmt.Fprintln(w, p.OK.Render("claimed by "+claim.Login))
				} else {
					fmt.Fprintln(w, p.Subtle.Render("not claimed: "+claim.Reason))
				}
			}
			if !claim.Won {
				return findings("issue %d was not claimed: %s", claim.Issue, claim.Reason)
			}
			return nil
		},
	}
}

func boardMoveCmd(printer func(*cobra.Command) *ui.Printer, asJSON *bool) *cobra.Command {
	var to, comment string

	cmd := &cobra.Command{
		Use:   "move <issue> --to <status>",
		Short: "Move an issue between the states the table lists",
		Long: "move writes one status and reads the board back.\n\n" +
			"Todo reaches in-progress, in-progress reaches todo or done, and done\n" +
			"is where an issue stops. A move the table does not list is refused\n" +
			"with exit 1 and nothing is written.\n\n" +
			"Moving back to todo is a run giving the issue up, so it also removes\n" +
			"this run's own assignment. Pass --comment to say what would unblock\n" +
			"it: an issue back in todo with no comment cannot be told from one\n" +
			"nobody has started, and the next run meets the same wall.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			number, err := issueNumber(args[0])
			if err != nil {
				return err
			}
			wanted, err := board.ParseStatus(to)
			if err != nil {
				return failed("%s", err)
			}
			c, err := client(cmd)
			if err != nil {
				return err
			}
			move, err := c.Transition(cmd.Context(), number, wanted, comment)
			if err != nil {
				return failed("%s", err)
			}

			if *asJSON {
				if err := write(cmd, move); err != nil {
					return err
				}
			} else {
				p := printer(cmd)
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "#%d %s\n", move.Issue, move.Title)
				printChanged(w, p, move.Changed)
				if move.Landed {
					fmt.Fprintln(w, p.OK.Render(fmt.Sprintf("%s to %s", move.From, move.Status)))
				} else {
					fmt.Fprintln(w, p.Subtle.Render("not moved: "+move.Reason))
				}
			}
			if !move.Landed {
				return findings("issue %d is %s: %s", move.Issue, move.Status, move.Reason)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "state to move to: todo, in-progress or done")
	cmd.Flags().StringVar(&comment, "comment", "", "comment to post before the move")
	if err := cmd.MarkFlagRequired("to"); err != nil {
		panic(err)
	}
	return cmd
}

func boardFileCmd(printer func(*cobra.Command) *ui.Printer, asJSON *bool) *cobra.Command {
	var (
		title    string
		bodyFile string
		parent   int
	)

	cmd := &cobra.Command{
		Use:   "file --title <title> --body-file <file>",
		Short: "File new work and put it on the board",
		Long: "file creates an issue, adds it to the project, and gives it the\n" +
			"group value and the starting status.\n\n" +
			"An issue that never reaches the board is invisible to every read\n" +
			"here, which is why one command does all three.\n\n" +
			"The body arrives as a file rather than as an argument, because it is\n" +
			"text somebody else may have written.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			body, err := os.ReadFile(bodyFile)
			if err != nil {
				return failed("%s", err)
			}
			c, err := client(cmd)
			if err != nil {
				return err
			}
			filed, err := c.File(cmd.Context(), title, string(body), parent)
			if err != nil {
				return failed("%s", err)
			}

			if *asJSON {
				return write(cmd, filed)
			}
			p := printer(cmd)
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "#%d %s\n%s\n", filed.Issue, filed.Title, filed.URL)
			if filed.Parent > 0 {
				fmt.Fprintf(w, "under #%d\n", filed.Parent)
			}
			fmt.Fprintln(w, p.OK.Render("on the board as "+string(filed.Status)))
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "issue title")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "file holding the issue body")
	cmd.Flags().IntVar(&parent, "parent", 0, "issue to file this one under")
	for _, required := range []string{"title", "body-file"} {
		if err := cmd.MarkFlagRequired(required); err != nil {
			panic(err)
		}
	}
	return cmd
}

func boardAddCmd(printer func(*cobra.Command) *ui.Printer, asJSON *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "add <issue>",
		Short: "Put an existing issue on the board",
		Long: "add puts an issue the project does not carry onto it, with the\n" +
			"group value and the starting status.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			number, err := issueNumber(args[0])
			if err != nil {
				return err
			}
			c, err := client(cmd)
			if err != nil {
				return err
			}
			issue, err := c.Issue(cmd.Context(), number)
			if err != nil {
				return failed("%s", err)
			}
			item, err := c.Add(cmd.Context(), issue)
			if err != nil {
				return failed("%s", err)
			}
			if *asJSON {
				return write(cmd, item)
			}
			p := printer(cmd)
			fmt.Fprintf(cmd.OutOrStdout(), "#%d %s\n%s\n", item.Number, item.Title,
				p.OK.Render("on the board as "+string(item.Status)))
			return nil
		},
	}
}

func boardSubCmd(printer func(*cobra.Command) *ui.Printer, asJSON *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sub",
		Short: "Read and write sub-issues",
		Long: "sub reads and writes the parent relation.\n\n" +
			"A sub-issue says what an issue is part of. What it has to wait for\n" +
			"is a dependency, which is blocked-by.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list <parent>",
		Short: "List an issue's children",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			number, err := issueNumber(args[0])
			if err != nil {
				return err
			}
			c, err := client(cmd)
			if err != nil {
				return err
			}
			children, err := c.SubIssues(cmd.Context(), number)
			if err != nil {
				return failed("%s", err)
			}
			if *asJSON {
				return write(cmd, children)
			}
			printRefs(cmd.OutOrStdout(), printer(cmd), "sub-issues", children)
			return nil
		},
	})

	cmd.AddCommand(relationCmd(printer, asJSON, relation{
		use:   "add <parent> <child>",
		short: "Attach a child to a parent",
		verb:  "under",
		write: func(cmd *cobra.Command, c *board.Client, parent int, child board.Issue) error {
			return c.AddSubIssue(cmd.Context(), parent, child.ID)
		},
	}))
	cmd.AddCommand(relationCmd(printer, asJSON, relation{
		use:   "remove <parent> <child>",
		short: "Detach a child from a parent",
		verb:  "out from under",
		write: func(cmd *cobra.Command, c *board.Client, parent int, child board.Issue) error {
			return c.RemoveSubIssue(cmd.Context(), parent, child.ID)
		},
	}))
	return cmd
}

func boardBlockedByCmd(printer func(*cobra.Command) *ui.Printer, asJSON *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blocked-by",
		Short: "Read and write issue dependencies",
		Long: "blocked-by reads and writes what an issue waits for.\n\n" +
			"GitHub refuses to close an issue whose blockers are open, so a\n" +
			"dependency is also what decides whether a closing keyword does\n" +
			"anything. The relation is stored once and projected both ways:\n" +
			"list reads one end and the other reads back as blocking.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list <issue>",
		Short: "List what an issue waits for",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			number, err := issueNumber(args[0])
			if err != nil {
				return err
			}
			c, err := client(cmd)
			if err != nil {
				return err
			}
			blockers, err := c.BlockedBy(cmd.Context(), number)
			if err != nil {
				return failed("%s", err)
			}
			if *asJSON {
				return write(cmd, blockers)
			}
			printRefs(cmd.OutOrStdout(), printer(cmd), "blocked by", blockers)
			return nil
		},
	})

	cmd.AddCommand(relationCmd(printer, asJSON, relation{
		use:   "add <issue> <blocker>",
		short: "Record that an issue waits for another",
		verb:  "blocked by",
		write: func(cmd *cobra.Command, c *board.Client, blocked int, blocker board.Issue) error {
			return c.Block(cmd.Context(), blocked, blocker.ID)
		},
	}))
	cmd.AddCommand(relationCmd(printer, asJSON, relation{
		use:   "remove <issue> <blocker>",
		short: "Remove a dependency",
		verb:  "no longer blocked by",
		write: func(cmd *cobra.Command, c *board.Client, blocked int, blocker board.Issue) error {
			return c.Unblock(cmd.Context(), blocked, blocker.ID)
		},
	}))
	return cmd
}

// relation is one write between two issues. All four take a number in the
// path and the other issue's ID in the body, which is why the second issue is
// read before the write: an issue's number is per repository where its ID is
// global, and a number sent as an ID is some other repository's issue.
type relation struct {
	use   string
	short string
	verb  string
	write func(*cobra.Command, *board.Client, int, board.Issue) error
}

func relationCmd(printer func(*cobra.Command) *ui.Printer, asJSON *bool, r relation) *cobra.Command {
	return &cobra.Command{
		Use:   r.use,
		Short: r.short,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			subject, err := issueNumber(args[0])
			if err != nil {
				return err
			}
			object, err := issueNumber(args[1])
			if err != nil {
				return err
			}
			c, err := client(cmd)
			if err != nil {
				return err
			}
			other, err := c.Issue(cmd.Context(), object)
			if err != nil {
				return failed("%s", err)
			}
			if err := r.write(cmd, c, subject, other); err != nil {
				return failed("%s", err)
			}

			result := map[string]any{"issue": subject, "other": other.Number, "relation": r.verb}
			if *asJSON {
				return write(cmd, result)
			}
			p := printer(cmd)
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n",
				p.OK.Render(fmt.Sprintf("#%d %s #%d", subject, r.verb, other.Number)))
			return nil
		},
	}
}

func keep(items []board.Item, status board.Status) []board.Item {
	var matching []board.Item
	for _, item := range items {
		if item.Status == status {
			matching = append(matching, item)
		}
	}
	return matching
}

func issueNumber(arg string) (int, error) {
	number, err := strconv.Atoi(strings.TrimPrefix(arg, "#"))
	if err != nil || number <= 0 {
		return 0, failed("%q is not an issue number", arg)
	}
	return number, nil
}

// write encodes a result for a hook or a workflow, which is a likelier caller
// than a person.
func write(cmd *cobra.Command, value any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func printRefs(w io.Writer, p *ui.Printer, label string, refs []board.Ref) {
	if len(refs) == 0 {
		return
	}
	numbers := make([]string, 0, len(refs))
	for _, ref := range refs {
		number := fmt.Sprintf("#%d", ref.Number)
		if !strings.EqualFold(ref.State, "open") {
			number = p.Subtle.Render(number)
		}
		numbers = append(numbers, number)
	}
	fmt.Fprintf(w, "%s %s\n", label, strings.Join(numbers, " "))
}

func printChanged(w io.Writer, p *ui.Printer, changed []string) {
	for _, change := range changed {
		fmt.Fprintln(w, p.Subtle.Render("  "+change))
	}
}
