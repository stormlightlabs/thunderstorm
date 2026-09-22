package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

var gate = Hook{Event: "PreToolUse", Matcher: "Bash", Command: "$CLAUDE_PROJECT_DIR/.claude/hooks/gate.sh", Timeout: 10}

func file(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	if body != "" {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func read(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("the merged file is not JSON: %v", err)
	}
	return body
}

func merge(t *testing.T, path string, deny []string, hooks []Hook) Change {
	t.Helper()
	c, err := Plan(path, deny, hooks)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if err := c.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	return c
}

// What a repository put in the file is what the merge has to give back. A
// permission it granted itself, an unrelated key, a hook of its own.
func TestTheMergeKeepsWhatTheRepositoryPutThere(t *testing.T) {
	path := file(t, `{
  "permissions": {"allow": ["Bash(cargo test:*)"], "deny": ["Bash(git push:*)"]},
  "model": "opus",
  "hooks": {"SessionStart": [{"matcher": "startup", "hooks": [{"type": "command", "command": "./warm.sh"}]}]}
}`)
	merge(t, path, []string{"Bash(git push:*)", "Bash(git merge:*)"}, []Hook{gate})

	body := read(t, path)
	if body["model"] != "opus" {
		t.Errorf("an unrelated key did not survive: %v", body["model"])
	}
	permissions := body["permissions"].(map[string]any)
	if allow := permissions["allow"].([]any); len(allow) != 1 || allow[0] != "Bash(cargo test:*)" {
		t.Errorf("the allow list did not survive: %v", allow)
	}
	deny := permissions["deny"].([]any)
	if len(deny) != 2 {
		t.Fatalf("deny holds %v, want the held rule and the missing one", deny)
	}
	sessionStart := body["hooks"].(map[string]any)["SessionStart"]
	if sessionStart == nil {
		t.Error("the repository's own hook did not survive")
	}
}

// An install and an update run the same merge, so the second one has to be a
// no-op rather than a second copy of every rule.
func TestASecondMergeAddsNothing(t *testing.T) {
	path := file(t, "")
	deny := []string{"Bash(git push:*)"}

	first := merge(t, path, deny, []Hook{gate})
	if first.Empty() {
		t.Fatal("the first merge found nothing to do")
	}
	if !first.Created() {
		t.Error("a file that was not there is reported as held")
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	second, err := Plan(path, deny, []Hook{gate})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !second.Empty() {
		t.Errorf("a second merge would add %v", second.Summary())
	}
	if err := second.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("a merge with nothing to add rewrote the file")
	}
}

// The same event with a different matcher is a different registration, and a
// harness runs both.
func TestTwoMatchersOnOneEventBothRegister(t *testing.T) {
	path := file(t, "")
	write := Hook{Event: "PostToolUse", Matcher: "Write|Edit", Command: gate.Command, Timeout: 10}
	merge(t, path, nil, []Hook{gate, write})

	hooks := read(t, path)["hooks"].(map[string]any)
	if len(hooks) != 2 {
		t.Fatalf("hooks holds %d events, want PreToolUse and PostToolUse", len(hooks))
	}
	entries := hooks["PostToolUse"].([]any)
	entry := entries[0].(map[string]any)
	if entry["matcher"] != "Write|Edit" {
		t.Errorf("the matcher is %v", entry["matcher"])
	}
	inner := entry["hooks"].([]any)[0].(map[string]any)
	if inner["timeout"] != float64(10) {
		t.Errorf("the timeout is %v", inner["timeout"])
	}
}

// A repository already running something on the event the gate wants keeps it.
func TestTheGateJoinsAHookAlreadyOnTheEvent(t *testing.T) {
	path := file(t, `{"hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "./audit.sh"}]}]}}`)
	merge(t, path, nil, []Hook{gate})

	entries := read(t, path)["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(entries) != 1 {
		t.Fatalf("the matcher was duplicated: %v", entries)
	}
	inner := entries[0].(map[string]any)["hooks"].([]any)
	if len(inner) != 2 {
		t.Fatalf("the event runs %d commands, want the repository's and the gate", len(inner))
	}
}

// A settings file nobody can parse is a settings file nobody should rewrite.
func TestUnreadableSettingsStopTheMerge(t *testing.T) {
	path := file(t, `{"permissions": {`)
	if _, err := Plan(path, []string{"Bash(git push:*)"}, nil); err == nil {
		t.Error("a truncated file was accepted")
	}
}
