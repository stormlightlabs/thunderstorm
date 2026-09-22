package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func installed(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, _, err := run(t, "install", "--dir", dir); err != nil {
		t.Fatalf("install: %v", err)
	}
	return dir
}

func marker(t *testing.T, dir string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, ".claude", ".tstorm-payload"))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// The marker says which version wrote the payload, which is what an update has
// to report and what a check compares.
func TestThePayloadRecordsTheVersionThatWroteIt(t *testing.T) {
	dir := installed(t)
	if !strings.Contains(marker(t, dir), "\nversion ") {
		t.Errorf("the marker names no version:\n%s", marker(t, dir))
	}

	stdout, _, err := run(t, "update", "--dir", dir, "--offline")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !strings.Contains(stdout, "up to date at") {
		t.Errorf("update said %q", stdout)
	}
}

// Every payload installed before the version line carries none. Reading one is
// the upgrade path, so it reports an update rather than an error.
func TestAPayloadFromBeforeTheVersionLineStillUpdates(t *testing.T) {
	dir := installed(t)
	file := filepath.Join(dir, ".claude", ".tstorm-payload")
	held := marker(t, dir)
	lines := strings.Split(held, "\n")
	older := append(lines[:1], lines[2:]...)
	if err := os.WriteFile(file, []byte(strings.Join(older, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := run(t, "update", "--dir", dir, "--offline")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !strings.Contains(stdout, "updating to") {
		t.Errorf("update said %q", stdout)
	}
	if !strings.Contains(marker(t, dir), "\nversion ") {
		t.Error("the update did not record a version")
	}
}

// An update is for a repository that has the loop. One that does not is told
// what to run instead.
func TestUpdatingWhereNothingIsInstalledSaysSo(t *testing.T) {
	_, _, err := run(t, "update", "--dir", t.TempDir(), "--offline")
	if err == nil {
		t.Fatal("update found a payload in an empty directory")
	}
	if !strings.Contains(err.Error(), "install") {
		t.Errorf("the error does not say what to run: %v", err)
	}
}

// A repository's board and documents tree are its own, whatever the update
// carries.
func TestUpdateLeavesTheConfigAlone(t *testing.T) {
	dir := installed(t)
	file := filepath.Join(dir, ".tstorm.toml")
	if err := os.WriteFile(file, []byte("documents = \"notes\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := run(t, "update", "--dir", dir, "--offline"); err != nil {
		t.Fatalf("update: %v", err)
	}
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "documents = \"notes\"\n" {
		t.Errorf("the config was rewritten:\n%s", body)
	}
}

// An update has to reach a repository whose settings lost a rule, because that
// is the case the merge exists for.
func TestUpdateRestoresADenyRuleThatWentMissing(t *testing.T) {
	dir := installed(t)
	file := filepath.Join(dir, ".claude", "settings.json")
	if err := os.WriteFile(file, []byte(`{"permissions": {"deny": ["Bash(git push:*)"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := run(t, "update", "--dir", dir, "--offline")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !strings.Contains(stdout, "merged into") {
		t.Errorf("update said %q", stdout)
	}
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"gh pr merge", "gh pr review", "git merge", "gate.sh"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the settings do not carry %s:\n%s", want, body)
		}
	}
}
