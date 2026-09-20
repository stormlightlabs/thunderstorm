package check

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Push pushes the current branch and proves the remote took it.
//
// `git push` exits 0 for a push that carried nothing. With a detached HEAD the
// branch has not moved, so git finds no ref to update, prints "Everything
// up-to-date" and succeeds. Every word of that is true and any claim of
// "pushed" built on it is false. A pre-push hook cannot catch it: with no ref
// to update git never runs one, so the check has to compare the end state.
//
// expected is the branch the caller claimed. Two implementers sharing a
// checkout share one HEAD, so the second to create a branch moves HEAD and
// carries the first's staged work onto it; the first then pushes a branch it
// never meant to touch. Naming the branch up front makes that loud. An empty
// expected skips the comparison, which is what a caller with one worktree and
// one branch wants.
//
// The returned string is the line to print on success.
func Push(remote, expected string, stdout, stderr io.Writer) (string, error) {
	branch, ok := git("symbolic-ref", "-q", "--short", "HEAD")
	if !ok {
		head, _ := git("rev-parse", "--short", "HEAD")
		return "", fmt.Errorf("HEAD is detached at %s.\n"+
			"Commits made here belong to no branch and no push will carry them.\n"+
			"Attach them first:\n"+
			"  git branch -f <branch> HEAD && git checkout <branch>", strings.TrimSpace(head))
	}
	branch = strings.TrimSpace(branch)

	// Before the push, so a branch that was stolen is reported while its
	// commits are still only local.
	if expected != "" && branch != expected {
		return "", fmt.Errorf("HEAD is on %s, not the %s this run claimed.\n"+
			"Commits made here landed on another worker's branch.\n"+
			"Check them out where they belong before pushing", branch, expected)
	}

	if err := run(stdout, stderr, "push", "-u", remote, branch); err != nil {
		return "", err
	}
	if err := run(stdout, stderr, "fetch", "-q", remote, branch); err != nil {
		return "", err
	}

	local, ok := git("rev-parse", "HEAD")
	if !ok {
		return "", fmt.Errorf("cannot read HEAD")
	}
	tracked, ok := git("rev-parse", "refs/remotes/"+remote+"/"+branch)
	if !ok {
		return "", fmt.Errorf("%s has no %s after the push", remote, branch)
	}
	local, tracked = strings.TrimSpace(local), strings.TrimSpace(tracked)
	if local != tracked {
		return "", fmt.Errorf("push did not land: %s is %s on %s, %s here",
			branch, short(tracked), remote, short(local))
	}

	return fmt.Sprintf("%s is %s on %s", branch, short(local), remote), nil
}

// run lets git write its own progress to the caller's streams, and leaves its
// stdin attached so a push can ask for a credential.
func run(stdout, stderr io.Writer, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, stdout, stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}
