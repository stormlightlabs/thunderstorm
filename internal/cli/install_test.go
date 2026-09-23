package cli

import (
	"encoding/json"
	"fmt"
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

// Any repository already using Claude Code has a .claude before install ever
// runs. That alone used to refuse the whole install with "holds files tstorm
// did not render".
func TestInstallAcceptsAnAlreadyConfiguredClaudeDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".claude", "commands"), 0o755); err != nil {
		t.Fatal(err)
	}
	mine := `{"permissions":{"allow":["Bash(cargo test:*)"]}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	deployFile := filepath.Join(dir, ".claude", "commands", "deploy.md")
	if err := os.WriteFile(deployFile, []byte("our own command\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := run(t, "install", "--dir", dir)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !strings.Contains(stdout, "installed") {
		t.Errorf("install said %q", stdout)
	}

	if got, err := os.ReadFile(deployFile); err != nil || string(got) != "our own command\n" {
		t.Errorf("the repository's own command was touched: %q (%v)", got, err)
	}
	allow := settingsOf(t, dir)["permissions"].(map[string]any)["allow"].([]any)
	if len(allow) != 1 || allow[0] != "Bash(cargo test:*)" {
		t.Errorf("the repository's own permission did not survive the merge: %v", allow)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "commands", "revise.md")); err != nil {
		t.Errorf("the payload did not write beside the repository's own command: %v", err)
	}
}

// A hand-copied .claude carries the payload's own files with no marker to say
// so. Without --replace that is refused and named; with it, only the
// colliding path is overwritten.
func TestInstallRefusesACollidingSkillThenReplacesIt(t *testing.T) {
	dir := t.TempDir()
	skill := filepath.Join(dir, ".claude", "skills", "implement")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	mine := "our own skill\n"
	skillFile := filepath.Join(skill, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	neighbour := filepath.Join(dir, ".claude", "skills", "notes.md")
	if err := os.WriteFile(neighbour, []byte("also mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := run(t, "install", "--dir", dir); err == nil {
		t.Fatal("install overwrote a repository's own skill")
	} else if !strings.Contains(err.Error(), "skills/implement/SKILL.md") {
		t.Errorf("the refusal does not name the file: %v", err)
	}
	if got, err := os.ReadFile(skillFile); err != nil || string(got) != mine {
		t.Errorf("the refusal touched the colliding file anyway: %q (%v)", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "commands", "revise.md")); err == nil {
		t.Error("the refusal still wrote other payload files")
	}

	if _, _, err := run(t, "install", "--dir", dir, "--replace"); err != nil {
		t.Fatalf("install --replace: %v", err)
	}
	got, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == mine {
		t.Error("--replace did not overwrite the colliding file")
	}
	if got, err := os.ReadFile(neighbour); err != nil || string(got) != "also mine\n" {
		t.Errorf("--replace touched a neighbouring file the repository owns: %q (%v)", got, err)
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

// --track writes groupField as Track unless --group-field names another
// field, so install and the docs agree on what a shared board's key is
// called.
func TestTrackWritesTheGroupField(t *testing.T) {
	for name, tc := range map[string][]string{
		"default":      nil,
		"custom field": {"--group-field", "Repository"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			args := append([]string{"install", "--dir", dir, "--board", "stormlightlabs/13",
				"--track", "Tropius"}, tc...)
			if _, _, err := run(t, args...); err != nil {
				t.Fatalf("install: %v", err)
			}

			raw, err := os.ReadFile(filepath.Join(dir, ".tstorm.toml"))
			if err != nil {
				t.Fatalf("read the config: %v", err)
			}
			field := "Track"
			if len(tc) > 0 {
				field = tc[1]
			}
			want := fmt.Sprintf("groupField = %q", field)
			if !strings.Contains(string(raw), want) {
				t.Errorf("the config does not hold %s:\n%s", want, raw)
			}
		})
	}
}

// A --dir that is not there is refused rather than created, so a typo in the
// path does not become the directory install installs into.
func TestDirMustAlreadyExist(t *testing.T) {
	parent := t.TempDir()
	missing := filepath.Join(parent, "nope")
	_, _, err := run(t, "install", "--dir", missing)
	if err == nil {
		t.Fatal("a --dir that does not exist was accepted")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("the error does not name the path: %v", err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Errorf("install created the directory it was refusing: %v", err)
	}
}

// A malformed --board has to be caught before install writes anything: a
// caller reading the non-zero exit has to find the repository untouched, not
// half-configured by a payload and a settings merge that already landed.
// --check reads the same flags, so it has to refuse the same way.
func TestAMalformedBoardIsRefused(t *testing.T) {
	for name, extra := range map[string][]string{
		"install": nil,
		"check":   {"--check"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			args := append([]string{"install", "--dir", dir, "--board", "stormlightlabs"}, extra...)
			_, _, err := run(t, args...)
			if err == nil {
				t.Fatal("a board with no number was accepted")
			}
			if !strings.Contains(err.Error(), "owner/number") {
				t.Errorf("the error does not say the shape: %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, ".claude")); !os.IsNotExist(err) {
				t.Errorf("install wrote a payload before validating --board: %v", err)
			}
		})
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

// cursor is advertised nowhere to install, since every render for it fails on
// an unmet capability; render keeps naming the gap, which is how someone
// closing #13 sees what is missing.
func TestInstallOffersNoUnsupportedTarget(t *testing.T) {
	stdout, _, err := run(t, "install", "--help")
	if err != nil {
		t.Fatalf("install --help: %v", err)
	}
	if strings.Contains(stdout, "cursor") {
		t.Errorf("install --help offers cursor:\n%s", stdout)
	}

	_, _, err = run(t, "render", "--target", "cursor", "--source", "../../workflow", "--out", t.TempDir())
	if err == nil {
		t.Fatal("render --target cursor did not report its gap")
	}
	if !strings.Contains(err.Error(), "#13") {
		t.Errorf("render --target cursor error does not cite the issue: %v", err)
	}
}
