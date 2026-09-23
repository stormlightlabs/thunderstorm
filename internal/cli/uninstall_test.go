package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// held writes a file the repository owns, under a directory the payload also
// uses, so the test asserts survival rather than absence.
func heldFile(t *testing.T, dir, rel, body string) string {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The whole point: a repository ends up with what it had before the loop was
// installed, and nothing of the loop's.
func TestUninstallLeavesTheRepositoryWhatWasIts(t *testing.T) {
	dir := t.TempDir()
	heldFile(t, dir, ".claude/settings.json", `{"model": "opus", "permissions": {"allow": ["Bash(cargo test:*)"]}}`)
	ours := heldFile(t, dir, ".claude/commands/deploy.md", "ours")
	if _, _, err := run(t, "install", "--dir", dir); err != nil {
		t.Fatalf("install: %v", err)
	}

	stdout, _, err := run(t, "uninstall", "--dir", dir)
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if !strings.Contains(stdout, "removed") {
		t.Errorf("uninstall said %q", stdout)
	}

	if _, err := os.Stat(ours); err != nil {
		t.Errorf("the repository's own command went with the payload: %v", err)
	}
	for _, gone := range []string{"skills", "agents", "hooks", ".tstorm-payload"} {
		if _, err := os.Stat(filepath.Join(dir, ".claude", gone)); !os.IsNotExist(err) {
			t.Errorf(".claude/%s survived the uninstall: %v", gone, err)
		}
	}

	raw, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body["model"] != "opus" {
		t.Errorf("an unrelated key went with the loop: %v", body["model"])
	}
	permissions := body["permissions"].(map[string]any)
	if permissions["allow"] == nil {
		t.Error("the repository's allow list went with the loop")
	}
	if permissions["deny"] != nil {
		t.Errorf("the loop's deny rules stayed: %v", permissions["deny"])
	}
	if body["hooks"] != nil {
		t.Errorf("the gate registration stayed: %v", body["hooks"])
	}
}

// A repository working two harnesses carries two payloads, and one command
// has to reach both or leave a repository half uninstalled.
func TestUninstallTakesEveryPayloadItFinds(t *testing.T) {
	dir := t.TempDir()
	for _, target := range []string{"claude", "pi"} {
		if _, _, err := run(t, "install", "--dir", dir, "--target", target); err != nil {
			t.Fatalf("install %s: %v", target, err)
		}
	}
	if _, _, err := run(t, "uninstall", "--dir", dir); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	for _, root := range []string{".claude", ".pi"} {
		if _, err := os.Stat(filepath.Join(dir, root, ".tstorm-payload")); !os.IsNotExist(err) {
			t.Errorf("%s still holds a payload: %v", root, err)
		}
	}
	// .pi holds nothing of the repository's, so the directory goes with the
	// payload. .claude keeps the settings file the merge wrote there, which
	// is a file to leave rather than one to delete.
	if _, err := os.Stat(filepath.Join(dir, ".pi")); !os.IsNotExist(err) {
		t.Errorf(".pi survived with nothing in it: %v", err)
	}
}

// The same repository is what update has to reach, for the same reason.
func TestUpdateReachesEveryPayload(t *testing.T) {
	dir := t.TempDir()
	for _, target := range []string{"claude", "pi"} {
		if _, _, err := run(t, "install", "--dir", dir, "--target", target); err != nil {
			t.Fatalf("install %s: %v", target, err)
		}
	}
	stdout, _, err := run(t, "update", "--dir", dir, "--offline")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	for _, root := range []string{".claude", ".pi"} {
		if !strings.Contains(stdout, root) {
			t.Errorf("update never mentioned %s:\n%s", root, stdout)
		}
	}
}

func TestUninstallChecksWithoutRemoving(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := run(t, "install", "--dir", dir); err != nil {
		t.Fatalf("install: %v", err)
	}
	stdout, _, err := run(t, "uninstall", "--dir", dir, "--check")
	if err != nil {
		t.Fatalf("uninstall --check: %v", err)
	}
	if !strings.Contains(stdout, "would remove") {
		t.Errorf("check said %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", ".tstorm-payload")); err != nil {
		t.Errorf("check removed the payload: %v", err)
	}
}

// A repository that keeps the deny rules is keeping a decision, so the files
// go and the settings stay.
func TestKeepSettingsLeavesTheRules(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := run(t, "install", "--dir", dir); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, _, err := run(t, "uninstall", "--dir", dir, "--keep-settings"); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "gh pr merge") {
		t.Errorf("the deny rules went anyway:\n%s", raw)
	}
}

func TestUninstallWhereNothingIsInstalledSaysSo(t *testing.T) {
	_, _, err := run(t, "uninstall", "--dir", t.TempDir())
	if err == nil {
		t.Fatal("uninstall found a payload in an empty directory")
	}
	if !strings.Contains(err.Error(), "install") {
		t.Errorf("the error does not say what to run: %v", err)
	}
}
