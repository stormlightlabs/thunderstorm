package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// checkout builds a repository with one commit and a bare remote to push to,
// and makes it the working directory for the test.
func checkout(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	remote := filepath.Join(parent, "remote.git")
	work := filepath.Join(parent, "work")

	mustGit(t, parent, "init", "-q", "--bare", remote)
	mustGit(t, parent, "init", "-q", "-b", "main", work)
	if err := os.WriteFile(filepath.Join(work, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, work, "add", "-A")
	mustGit(t, work, "commit", "-qm", "first")
	mustGit(t, work, "remote", "add", "origin", remote)

	t.Chdir(work)
	return work
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir,
		"-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

func TestAPushOnTheBranchItClaimedLands(t *testing.T) {
	work := checkout(t)
	stdout, stderr, err := run(t, "push", "--branch", "main")
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v %q", code, err, stderr)
	}
	if !strings.Contains(stdout, "main is") {
		t.Errorf("did not name the branch it pushed: %q", stdout)
	}
	_ = work
}

// Two workers sharing a checkout share one HEAD, so the second to create a
// branch carries the first's work onto it. The claim is what makes that loud.
func TestAPushFromAnotherWorkersBranchIsRefused(t *testing.T) {
	work := checkout(t)
	mustGit(t, work, "checkout", "-q", "-b", "agent/2")
	_, _, err := run(t, "push", "--branch", "agent/1")
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit %d, want 1: %v", code, err)
	}
	for _, want := range []string{"agent/1", "agent/2"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %s: %v", want, err)
		}
	}
}

// Nothing was pushed, so the remote still has no such branch.
func TestARefusedPushSendsNothing(t *testing.T) {
	work := checkout(t)
	mustGit(t, work, "checkout", "-q", "-b", "agent/2")
	if _, _, err := run(t, "push", "--branch", "agent/1"); ExitCode(err) != 1 {
		t.Fatalf("the push was not refused: %v", err)
	}
	out, err := exec.Command("git", "-C", work, "ls-remote", "--heads", "origin").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "agent/2") {
		t.Errorf("a refused push reached the remote: %q", out)
	}
}

// `git push` exits 0 for a push that carried nothing, and with a detached HEAD
// there is no ref to update, so nothing downstream notices.
func TestADetachedHEADIsRefused(t *testing.T) {
	work := checkout(t)
	mustGit(t, work, "checkout", "-q", "--detach")
	_, _, err := run(t, "push")
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit %d, want 1: %v", code, err)
	}
	if !strings.Contains(err.Error(), "detached") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}

// Called with no claim, the behavior is unchanged, so existing callers keep
// working.
func TestAPushWithNoClaimStillPushes(t *testing.T) {
	checkout(t)
	if _, stderr, err := run(t, "push"); ExitCode(err) != 0 {
		t.Fatalf("a push with no --branch failed: %v %q", err, stderr)
	}
}
