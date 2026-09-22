package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func settingsOf(t *testing.T, dir string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read the settings: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("the settings are not JSON: %v", err)
	}
	return body
}

// The whole point of the command: a repository with nothing in it ends up able
// to run the loop, from a binary carrying its own workflow.
func TestInstallLeavesARepositoryAbleToRunTheLoop(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := run(t, "install", "--dir", dir)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !strings.Contains(stdout, "installed") {
		t.Errorf("install said %q", stdout)
	}

	for _, want := range []string{
		filepath.Join(".claude", "skills", "implement", "SKILL.md"),
		filepath.Join(".claude", "commands", "thunderstorm.md"),
		filepath.Join(".claude", "hooks", "gate.sh"),
		filepath.Join(".claude", ".tstorm-payload"),
	} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("%s: %v", want, err)
		}
	}

	gate, err := os.Stat(filepath.Join(dir, ".claude", "hooks", "gate.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if gate.Mode()&0o111 == 0 {
		t.Error("the gate is not executable, so the hook cannot run it")
	}

	body := settingsOf(t, dir)
	deny := body["permissions"].(map[string]any)["deny"].([]any)
	if len(deny) != 4 {
		t.Errorf("the settings deny %v", deny)
	}
	hooks := body["hooks"].(map[string]any)
	if len(hooks) != 2 {
		t.Errorf("the settings register %d events, want PreToolUse and PostToolUse", len(hooks))
	}
}

// The payload must not claim the settings file: install merges into it, and a
// marker listing a path the merge edits reports it stale forever after.
func TestTheSettingsFileStaysTheRepositorys(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := run(t, "install", "--dir", dir); err != nil {
		t.Fatalf("install: %v", err)
	}
	marker, err := os.ReadFile(filepath.Join(dir, ".claude", ".tstorm-payload"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(marker), "settings.json") {
		t.Error("the payload claims the settings file")
	}

	stdout, _, err := run(t, "install", "--dir", dir, "--check")
	if err != nil {
		t.Fatalf("a second install reports work to do: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "up to date") {
		t.Errorf("check said %q", stdout)
	}
}

// --no-settings is for a repository that merges its own, so the payload seeds
// a settings file again rather than leaving it with none.
func TestWithoutTheMergeThePayloadSeedsTheSettings(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := run(t, "install", "--dir", dir, "--no-settings"); err != nil {
		t.Fatalf("install: %v", err)
	}
	body := settingsOf(t, dir)
	if _, held := body["hooks"]; held {
		t.Error("the seed registered a hook, which only the merge does")
	}
	if body["permissions"] == nil {
		t.Error("the seed carries no deny rules")
	}
}

// A config is written from what the flags say and from nothing else.
func TestTheConfigHoldsWhatTheFlagsSaid(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := run(t, "install", "--dir", dir, "--board", "stormlightlabs/13",
		"--documents", "docs/internal", "--track", "Tropius"); err != nil {
		t.Fatalf("install: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, ".tstorm.toml"))
	if err != nil {
		t.Fatalf("read the config: %v", err)
	}
	for _, want := range []string{`owner = "stormlightlabs"`, "number = 13", `groupValue = "Tropius"`, `documents = "docs/internal"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the config does not hold %s:\n%s", want, raw)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "docs", "internal")); err != nil {
		t.Errorf("the documents tree was not created: %v", err)
	}
}

func TestAMalformedBoardIsRefused(t *testing.T) {
	dir := t.TempDir()
	_, _, err := run(t, "install", "--dir", dir, "--board", "stormlightlabs")
	if err == nil {
		t.Fatal("a board with no number was accepted")
	}
	if !strings.Contains(err.Error(), "owner/number") {
		t.Errorf("the error does not say the shape: %v", err)
	}
}

// A repository that never configured a board is not one whose config install
// should invent.
func TestNoFlagsWriteNoConfig(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := run(t, "install", "--dir", dir)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".tstorm.toml")); !os.IsNotExist(err) {
		t.Errorf("a config was written with nothing to put in it: %v", err)
	}
	if !strings.Contains(stdout, "no config written") {
		t.Errorf("install did not say why: %q", stdout)
	}
}

// --check on a repository holding nothing has everything still to do, and says
// so through its exit code as well as its report.
func TestCheckOnAnEmptyRepositoryReportsWork(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := run(t, "install", "--dir", dir, "--check")
	if err == nil {
		t.Fatal("check reported nothing to install")
	}
	if !strings.Contains(stdout, "would install") {
		t.Errorf("check said %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude")); !os.IsNotExist(err) {
		t.Errorf("check wrote a payload: %v", err)
	}
}
