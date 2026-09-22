package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// settingsHolding writes a permissions file carrying the rules it is given.
func settingsHolding(t *testing.T, deny ...string) string {
	t.Helper()
	quoted := make([]string, 0, len(deny))
	for _, rule := range deny {
		quoted = append(quoted, `"`+rule+`"`)
	}
	body := `{"permissions": {"deny": [` + strings.Join(quoted, ", ") + `]}}`
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The rendered payload is the list every install merges by hand.
const payloadSettings = "../../payloads/claude/settings.json"

func TestSettingsCarryingEveryDeniedCommandPass(t *testing.T) {
	file := settingsHolding(t,
		"Bash(gh pr merge:*)", "Bash(gh pr review:*)", "Bash(git push:*)", "Bash(git merge:*)")
	stdout, _, err := run(t, "check", "policy", "--expected", payloadSettings, file)
	if err != nil {
		t.Fatalf("check returned %v", err)
	}
	if !strings.Contains(stdout, "denies all 4 commands") {
		t.Errorf("the gate said %q", stdout)
	}
}

// A rule added to the workflow reaches no repository's settings on its own.
func TestAMissingDenyRuleIsNamedWithWhatToAdd(t *testing.T) {
	file := settingsHolding(t, "Bash(gh pr merge:*)", "Bash(gh pr review:*)", "Bash(git merge:*)")
	_, stderr, err := run(t, "check", "policy", "--expected", payloadSettings, file)
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit %d, want 1: %v", code, err)
	}
	if !strings.Contains(stderr, `add "Bash(git push:*)"`) {
		t.Errorf("the gate did not say what to add: %q", stderr)
	}
}

// No permissions block reads the same as a merge that was undone.
func TestSettingsWithNoPermissionsBlockFail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"enabledPlugins": {}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, err := run(t, "check", "policy", "--expected", payloadSettings, path)
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit %d, want 1: %v", code, err)
	}
	if strings.Count(stderr, "is not denied") != 4 {
		t.Errorf("the gate named %q, want all four", stderr)
	}
}

// A file it cannot read is the gate failing, not the settings.
func TestAnUnreadableSettingsFileIsTheGateFailing(t *testing.T) {
	_, _, err := run(t, "check", "policy", "--expected", payloadSettings,
		filepath.Join(t.TempDir(), "absent.json"))
	if code := ExitCode(err); code != 2 {
		t.Errorf("exit %d, want 2: %v", code, err)
	}
}

// The check CI runs, over the settings this repository committed.
func TestThisRepositoryCarriesTheDenyRules(t *testing.T) {
	_, _, err := run(t, "check", "policy", "--source", "../../workflow", "../../.claude/settings.json")
	if err != nil {
		t.Errorf("this repository's settings do not carry the deny rules: %v", err)
	}
}
