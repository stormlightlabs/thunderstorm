package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stormlightlabs/thunderstorm/internal/check"
	"github.com/stormlightlabs/thunderstorm/internal/config"
	"github.com/stormlightlabs/thunderstorm/internal/render"
	"github.com/stormlightlabs/thunderstorm/internal/ui"
)

func checkCmd(printer func(*cobra.Command) *ui.Printer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check <name>",
		Short: "Run one of the loop's gates",
		Long: "check runs one gate over what it is given.\n\n" +
			"Exit codes are the interface: 0 when the gate found nothing, 1 when\n" +
			"it found something, and 2 when the gate itself could not run. A hook\n" +
			"reading only \"non-zero\" cannot tell a bad commit message from an\n" +
			"unreadable file, and it has to treat the two differently.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	cmd.AddCommand(commitMessageCmd(printer), frontmatterCmd(printer), isolationCmd(printer),
		policyCmd(printer), proseCmd(printer), payloadVersionCmd(printer))
	return cmd
}

func payloadVersionCmd(printer func(*cobra.Command) *ui.Printer) *cobra.Command {
	var (
		source   string
		payloads string
		tag      string
	)

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Check that the payload's version moved with the payload",
		Long: "version compares the version in the workflow manifest against the\n" +
			"last tag and against the tree the payload was rendered from.\n\n" +
			"A harness caches an installed plugin by version and reports it\n" +
			"current while that string is unchanged, so a payload that changed\n" +
			"without a bump never reaches a machine that already installed it.\n\n" +
			"--tag names the tag being built, which has to match the manifest.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := printer(cmd)
			findings, err := check.Version(check.Release{Source: source, Payloads: payloads, Tag: tag})
			if err != nil {
				return failed("%v", err)
			}
			if err := report(cmd.ErrOrStderr(), findings); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s the payload's version answers for what it carries\n",
				p.OK.Render("ok"))
			return nil
		},
	}

	cmd.Flags().StringVar(&source, "source", "workflow", "workflow source directory holding the manifest")
	cmd.Flags().StringVar(&payloads, "payloads", "payloads", "directory holding the rendered payloads")
	cmd.Flags().StringVar(&tag, "tag", "", "tag being built, which has to match the manifest version")
	return cmd
}

func policyCmd(printer func(*cobra.Command) *ui.Printer) *cobra.Command {
	var (
		source   string
		expected string
	)

	cmd := &cobra.Command{
		Use:   "policy [settings.json]",
		Short: "Check that a repository's settings carry the workflow's denied commands",
		Long: "policy compares a repository's permissions against the commands\n" +
			"the workflow reserves for a person. No plugin mechanism carries a\n" +
			"permission, so the rules are merged by hand and nothing else\n" +
			"notices when the workflow gains one.\n\n" +
			"The expected list comes from the workflow manifest, or from a\n" +
			"rendered payload's settings.json with --expected, which is what an\n" +
			"installed repository has.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := printer(cmd)
			settings := filepath.Join(".claude", "settings.json")
			if len(args) == 1 {
				settings = args[0]
			}

			var denied []string
			var err error
			if expected != "" {
				denied, err = check.PolicyOf(expected)
			} else {
				var manifest render.Manifest
				if manifest, err = render.Load(render.Dir(source)); err == nil {
					denied = manifest.Policy.Deny
				}
			}
			if err != nil {
				return failed("%v", err)
			}
			if len(denied) == 0 {
				return failed("no denied commands to check for")
			}

			missing, err := check.Policy(settings, denied)
			if err != nil {
				return failed("%v", err)
			}
			if len(missing) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s denies all %d commands the workflow reserves\n",
					p.OK.Render("ok"), settings, len(denied))
				return nil
			}
			for _, prefix := range missing {
				fmt.Fprintf(cmd.ErrOrStderr(), "  %s is not denied; add %q\n", prefix, check.DenyRule(prefix))
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "\n%s is where the install page asks for that merge. "+
				"Until it holds them, a session here runs what the workflow reserves for a person.\n", settings)
			return findings("%s is missing %d deny %s", settings, len(missing), plural("rule", len(missing)))
		},
	}

	cmd.Flags().StringVar(&source, "source", "workflow", "workflow source directory holding the manifest")
	cmd.Flags().StringVar(&expected, "expected", "", "rendered payload settings.json to read the deny list from instead")
	return cmd
}

func proseCmd(printer func(*cobra.Command) *ui.Printer) *cobra.Command {
	var warn bool

	cmd := &cobra.Command{
		Use:   "prose [path...]",
		Short: "Report the writing tells tropius finds in a file or a tree",
		Long: "prose runs tropius over what it is given and reports what comes\n" +
			"back, minus the rules " + config.Name + " mutes.\n\n" +
			"It covers part of the writing-docs catalogue: phrase patterns,\n" +
			"bold-first leads, tricolons, negative parallelism, and the words\n" +
			"that name a judgment. A clean run means those rules matched\n" +
			"nothing.\n\n" +
			"Tropius not being installed is a warning and exit 0.",
		RunE: func(cmd *cobra.Command, args []string) error {
			p := printer(cmd)
			settings, err := config.Load(".")
			if err != nil {
				return failed("%v", err)
			}
			paths := args
			if len(paths) == 0 {
				paths = settings.ProsePaths()
			}
			if len(paths) == 0 {
				return failed("name a file or a tree, or set \"prose.paths\" in %s", settings.File())
			}

			found, err := check.Prose(paths, settings.Rules())
			switch {
			case errors.Is(err, check.ProseNotInstalled):
				fmt.Fprintf(cmd.ErrOrStderr(), "%s: install it from %s\n", err, check.TropiusHome)
				return nil
			case err != nil:
				return failed("%v", err)
			}

			stream := cmd.ErrOrStderr()
			if warn {
				stream = cmd.OutOrStdout()
			}
			if len(found) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s nothing in the catalogue matched\n", p.OK.Render("ok"))
				return nil
			}
			for _, finding := range found {
				fmt.Fprintln(stream, "  "+here(finding).String())
			}
			if warn && os.Getenv("GITHUB_ACTIONS") == "true" {
				for _, finding := range found {
					fmt.Fprintf(cmd.OutOrStdout(), "::notice file=%s,line=%d,title=%s::%s\n",
						finding.Path, finding.Line, finding.Rule, finding.Matched)
				}
			}
			if warn {
				return nil
			}
			return findings("%d prose %s", len(found), plural("finding", len(found)))
		},
	}

	cmd.Flags().BoolVar(&warn, "warn", false, "report everything and exit 0")
	return cmd
}

// here shortens a finding's path against the working directory, because a
// gate run from the repository root reports paths a reader can open.
func here(f check.ProseFinding) check.ProseFinding {
	cwd, err := os.Getwd()
	if err != nil {
		return f
	}
	if rel, err := filepath.Rel(cwd, f.Path); err == nil && !strings.HasPrefix(rel, "..") {
		f.Path = rel
	}
	return f
}

func plural(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

func commitMessageCmd(printer func(*cobra.Command) *ui.Printer) *cobra.Command {
	var (
		warn      bool
		pr        bool
		titleFile string
		bodyFile  string
	)

	cmd := &cobra.Command{
		Use:   "commit-message <file>",
		Short: "Check a commit message, or a pull request's title and body, against the commits-and-prs skill",
		Long: "commit-message grades the shape of a message and the length of a body.\n\n" +
			"Shape fails the run: a missing type, a subject over the column git log\n" +
			"gives it, a body glued to its subject. Length never does. A body over\n" +
			"its target is usually padding, but sometimes a change earns the room,\n" +
			"and rejecting a message for length teaches authors to reach for\n" +
			"--no-verify, which skips the shape checks too.\n\n" +
			"The title and body arrive as files rather than as arguments. Both are\n" +
			"written by whoever opened the pull request, and a workflow that\n" +
			"interpolates a title into a shell command runs that title.\n\n" +
			"With --warn everything is reported and the status stays 0, which is\n" +
			"what CI wants: a pull request's title and body are editable until the\n" +
			"merge, so naming a problem is worth more than blocking on it.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			message, err := commitText(pr, titleFile, bodyFile, args)
			if err != nil {
				return err
			}

			problems, advice := check.Commit(message)
			subject, _, _ := strings.Cut(message.Text, "\n")
			what := "commit message"
			if message.FromPullRequest {
				what = "pull request text"
			}

			// Warnings are results rather than errors, so they belong on
			// stdout where a job summary or a pipe can pick them up.
			stream := cmd.ErrOrStderr()
			if warn {
				stream = cmd.OutOrStdout()
			}

			if len(problems) == 0 && len(advice) == 0 {
				fmt.Fprintf(stream, "%s checked, clean: %s\n", what, subject)
				return nil
			}

			fmt.Fprintln(stream, subject)
			for _, problem := range problems {
				fmt.Fprintf(stream, "  error:  %s\n", problem)
			}
			for _, note := range advice {
				fmt.Fprintf(stream, "  length: %s\n", note)
			}
			if warn && os.Getenv("GITHUB_ACTIONS") == "true" {
				for _, problem := range problems {
					fmt.Fprintf(cmd.OutOrStdout(), "::warning title=Commit message::%s\n", problem)
				}
				for _, note := range advice {
					fmt.Fprintf(cmd.OutOrStdout(), "::notice title=Commit length::%s\n", note)
				}
			}
			if len(problems) > 0 {
				fmt.Fprintf(stream, "\nThe %s needs a shape fix. The rules live in the "+
					"`commits-and-prs` skill.\n", what)
			}
			if len(advice) > 0 {
				// Said plainly so nobody goes looking for the exit code that
				// did not happen. Length is reported to be read.
				fmt.Fprintln(stream, "\nLength advice fails nothing. A pull request's title and body "+
					"stay editable until the merge, so this is worth reading rather than worth "+
					"blocking on.")
			}

			if warn || len(problems) == 0 {
				return nil
			}
			return findings("%s needs a shape fix", what)
		},
	}

	cmd.Flags().BoolVar(&warn, "warn", false, "report everything and exit 0")
	cmd.Flags().BoolVar(&pr, "pr", false, "grade the title and body a squash merge will join")
	cmd.Flags().StringVar(&titleFile, "title-file", "", "file holding the pull request title")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "file holding the pull request body")
	return cmd
}

// commitText assembles the message to grade, refusing an invocation that names
// neither a file nor both halves of a pull request.
func commitText(pr bool, titleFile, bodyFile string, args []string) (check.Message, error) {
	if !pr {
		if len(args) != 1 {
			return check.Message{}, failed("name the file holding the message, or pass --pr with " +
				"--title-file and --body-file")
		}
		text, err := readMessage(args[0])
		if err != nil {
			return check.Message{}, err
		}
		return check.Message{Text: text, FromFile: true}, nil
	}

	if titleFile == "" || bodyFile == "" {
		return check.Message{}, failed("--pr needs both --title-file and --body-file")
	}
	title, err := readMessage(titleFile)
	if err != nil {
		return check.Message{}, err
	}
	body, err := readMessage(bodyFile)
	if err != nil {
		return check.Message{}, err
	}
	return check.Message{Text: check.PullRequestMessage(title, body), FromPullRequest: true}, nil
}

// readMessage tolerates bytes that are not UTF-8. Git stores whatever the
// author's editor wrote, and refusing to decode them would replace a verdict
// with a crash, which in CI fails a job documented as never failing.
func readMessage(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", failed("%v", err)
	}
	return strings.ToValidUTF8(string(body), "�"), nil
}

func frontmatterCmd(printer func(*cobra.Command) *ui.Printer) *cobra.Command {
	var since string

	cmd := &cobra.Command{
		Use:   "frontmatter [dir]",
		Short: "Check that every document in a tree opens with its frontmatter block",
		Long: "frontmatter checks that every document opens with a name, a date, and\n" +
			"a ULID that never changes, because an issue cites the document it came\n" +
			"from by that identifier.\n\n" +
			"The tree is named rather than guessed. Installed, the gate runs outside\n" +
			"the repository being checked, and which directory holds a repository's\n" +
			"documents is the repository's business: name it here, or name it once\n" +
			"as \"documents\" in " + config.Name + ". A repository starting out puts\n" +
			"them in docs/internal and names that; one that names nothing is\n" +
			"skipped.\n\n" +
			"--since compares each identifier against the same file at a git ref.\n" +
			"Uniqueness within one tree is not immutability across time: an\n" +
			"identifier edited in place leaves a tree that looks clean while every\n" +
			"issue citing the old value points at nothing.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := printer(cmd)
			root, settingsFile, err := documentsRoot(args)
			if err != nil {
				return err
			}
			if root == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "no documents tree to check: name one, or set "+
					"\"documents\" in %s\n", settingsFile)
				return nil
			}
			if info, err := os.Stat(root); err != nil || !info.IsDir() {
				return failed("%s is not a directory", root)
			}

			checked, failures, err := check.Frontmatter(root)
			if err != nil {
				return failed("%v", err)
			}
			if since != "" {
				since, err := check.FrontmatterSince(root, since)
				if err != nil {
					return failed("%v", err)
				}
				failures = append(failures, since...)
			}
			// Nothing found and everything correct produce the same empty
			// list, and only one of them means the gate did its job.
			if checked == 0 {
				failures = append(failures, fmt.Sprintf("no documents found under %s", root))
			}

			if err := report(cmd.ErrOrStderr(), failures); err != nil {
				return err
			}
			noun, verb := "documents", "carry"
			if checked == 1 {
				noun, verb = "document", "carries"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %d %s under %s/ %s their frontmatter\n",
				p.OK.Render("ok"), checked, noun, filepath.Base(root), verb)
			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "git ref to compare identifiers against")
	return cmd
}

// documentsRoot is the tree to walk: the one named on the command line, else
// the one the repository configured, else nothing at all. It returns the
// settings file alongside, so a message asking for the setting names the file
// this repository has rather than the one it would write today.
func documentsRoot(args []string) (string, string, error) {
	if len(args) == 1 {
		return args[0], config.Name, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", failed("%v", err)
	}
	settings, err := config.Load(cwd)
	if err != nil {
		return "", "", failed("%v", err)
	}
	return settings.DocumentsDir(), settings.File(), nil
}

func isolationCmd(printer func(*cobra.Command) *ui.Printer) *cobra.Command {
	return &cobra.Command{
		Use:   "isolation [dir]",
		Short: "Check that no skill or role asks the harness for a worktree",
		Long: "isolation checks that nothing in a tree of definitions asks its harness\n" +
			"to provision a worktree.\n\n" +
			"The harness makes one inside the repository root, where a build tool\n" +
			"reaches the parent's configuration and builds into the parent's output\n" +
			"directory. The `worktree` skill makes them outside it instead, and this\n" +
			"gate is what keeps a frontmatter key from quietly overriding the skill.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := printer(cmd)
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			if info, err := os.Stat(root); err != nil || !info.IsDir() {
				return failed("%s is not a directory", root)
			}

			checked, failures, err := check.Isolation(root)
			if err != nil {
				return failed("%v", err)
			}
			if checked == 0 {
				failures = append(failures, fmt.Sprintf("no definitions found under %s", root))
			}

			if err := report(cmd.ErrOrStderr(), failures); err != nil {
				return err
			}
			noun, verb := "definitions", "ask"
			if checked == 1 {
				noun, verb = "definition", "asks"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %d %s under %s/ %s the harness for no worktree\n",
				p.OK.Render("ok"), checked, noun, filepath.Base(root), verb)
			return nil
		},
	}
}

// report prints every failure a tree produced, so one run covers the whole
// tree rather than stopping at the first file.
func report(stderr io.Writer, failures []string) error {
	if len(failures) == 0 {
		return nil
	}
	for _, failure := range failures {
		fmt.Fprintln(stderr, failure)
	}
	return findings("%d failed", len(failures))
}

func pushCmd(printer func(*cobra.Command) *ui.Printer) *cobra.Command {
	var remote, branch string

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push the current branch and prove the remote took it",
		Long: "push runs the push and then checks the end state.\n\n" +
			"`git push` exits 0 for a push that carried nothing. With a detached\n" +
			"HEAD there is no ref to update, so git prints \"Everything up-to-date\"\n" +
			"and succeeds, and no pre-push hook runs to catch it.\n\n" +
			"--branch names the branch the run claimed. Two workers sharing a\n" +
			"checkout share one HEAD, so the second to create a branch moves HEAD\n" +
			"and carries the first's staged work onto it. Naming the branch makes\n" +
			"that loud before the push rather than after the merge.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := printer(cmd)
			line, err := check.Push(remote, branch, cmd.OutOrStdout(), cmd.ErrOrStderr())
			if err != nil {
				return findings("%v", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), p.OK.Render("pushed"), line)
			return nil
		},
	}

	cmd.Flags().StringVar(&remote, "remote", "origin", "remote to push to")
	cmd.Flags().StringVar(&branch, "branch", "", "branch this run claimed; refuse to push from any other")
	return cmd
}

func ulidCmd(_ func(*cobra.Command) *ui.Printer) *cobra.Command {
	return &cobra.Command{
		Use:   "ulid [count]",
		Short: "Print a ULID for a new document",
		Long: "ulid prints a 48-bit millisecond timestamp followed by 80 random bits,\n" +
			"in Crockford base32.\n\n" +
			"A document carries one for the life of the repository, and an issue\n" +
			"cites the document by it, so it is generated once and never edited.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			count := 1
			if len(args) == 1 {
				parsed, err := strconv.Atoi(args[0])
				if err != nil || parsed < 1 {
					return failed("%q is not a count of identifiers to print", args[0])
				}
				count = parsed
			}
			for range count {
				id, err := check.ULID()
				if err != nil {
					return failed("%v", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), id)
			}
			return nil
		},
	}
}
