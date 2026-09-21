package hook

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTheMessageIsReadOutOfTheCommand(t *testing.T) {
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "msg.txt"), []byte("feat: from a file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		command string
		want    string
	}{
		"a quoted message":         {`git commit -m "feat: add a thing"`, "feat: add a thing"},
		"single quotes":            {`git commit -m 'feat: add a thing'`, "feat: add a thing"},
		"the long flag":            {`git commit --message="feat: add a thing"`, "feat: add a thing"},
		"a message file":           {`git commit -F msg.txt`, "feat: from a file\n"},
		"after another command":    {`git add -A && git commit -m "feat: add a thing"`, "feat: add a thing"},
		"flags before the message": {`git commit --no-verify -m "feat: add a thing"`, "feat: add a thing"},
		"an escaped quote":         {`git commit -m "feat: add \"a thing\""`, `feat: add "a thing"`},
	} {
		t.Run(name, func(t *testing.T) {
			got, ok := commitMessage(tc.command, cwd)
			if !ok || got != tc.want {
				t.Errorf("read %q (%v), want %q", got, ok, tc.want)
			}
		})
	}
}

// What the gate cannot read, it says nothing about.
func TestACommandWhoseMessageCannotBeReadIsLeftAlone(t *testing.T) {
	for name, command := range map[string]string{
		"an editor commit":       `git commit`,
		"an amend with no text":  `git commit --amend --no-edit`,
		"a command substitution": `git commit -m "$(cat msg.txt)"`,
		"a heredoc":              "git commit -F - <<'EOF'\nfeat: a thing\nEOF",
		"an unclosed quote":      `git commit -m "feat: a thing`,
		"a message file absent":  `git commit -F nowhere.txt`,
		"not a commit at all":    `git log --oneline -5`,
		"a commit elsewhere":     `echo git commit -m "feat: not this"`,
	} {
		t.Run(name, func(t *testing.T) {
			if got, ok := commitMessage(command, t.TempDir()); ok {
				t.Errorf("read %q out of %q", got, command)
			}
		})
	}
}
