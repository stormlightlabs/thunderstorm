package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repo builds a git repository holding a workflow manifest at version, with
// one tag cut at that state, and returns its path.
func repo(t *testing.T, version string) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v %s", strings.Join(args, " "), err, out)
		}
	}
	writeUnder(t, root, "workflow/manifest.json", `{"version": "`+version+`"}`)
	writeUnder(t, root, "payloads/claude/skills/review/SKILL.md", "a skill\n")
	run("init", "-q", "-b", "main")
	run("config", "user.email", "gate@test")
	run("config", "user.name", "gate")
	run("add", "-A")
	run("commit", "-qm", "chore: first")
	run("tag", "-a", "v"+version, "-m", "v"+version)
	return root
}

func writeUnder(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// in runs the check from dir, since the gate reads the repository it is in.
func in(t *testing.T, dir string, args ...string) (string, string, error) {
	t.Helper()
	was, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(was) })
	return run(t, args...)
}

func commit(t *testing.T, root, rel, body string) {
	t.Helper()
	writeUnder(t, root, rel, body)
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", "chore: change"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
}

// A payload that changed without a bump installs as the version a machine
// already has, so the harness reports it current and never replaces it.
func TestAChangedPayloadAtTheSameVersionFails(t *testing.T) {
	root := repo(t, "0.1.0")
	commit(t, root, "payloads/claude/skills/review/SKILL.md", "a changed skill\n")
	_, stderr, err := in(t, root, "check", "version")
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit %d, want 1: %v", code, err)
	}
	if !strings.Contains(stderr, "bump version") {
		t.Errorf("the gate said %q", stderr)
	}
}

func TestAChangedPayloadWithABumpPasses(t *testing.T) {
	root := repo(t, "0.1.0")
	commit(t, root, "payloads/claude/skills/review/SKILL.md", "a changed skill\n")
	commit(t, root, "workflow/manifest.json", `{"version": "0.1.1"}`)
	if _, _, err := in(t, root, "check", "version"); err != nil {
		t.Errorf("a bumped payload failed: %v", err)
	}
}

func TestAnUnchangedPayloadPasses(t *testing.T) {
	root := repo(t, "0.1.0")
	if _, _, err := in(t, root, "check", "version"); err != nil {
		t.Errorf("an unchanged payload failed: %v", err)
	}
}

// The release direction: a tag publishes a plugin that says what the tag says.
func TestATagThatDisagreesWithTheManifestFails(t *testing.T) {
	root := repo(t, "0.1.0")
	_, stderr, err := in(t, root, "check", "version", "--tag", "v0.2.0")
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit %d, want 1: %v", code, err)
	}
	if !strings.Contains(stderr, "v0.2.0") {
		t.Errorf("the gate said %q", stderr)
	}
}
