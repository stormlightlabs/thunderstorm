package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

var gate = Hook{Event: "PreToolUse", Matcher: "Bash", Command: "$CLAUDE_PROJECT_DIR/.claude/hooks/gate.sh", Timeout: 10}

var posttool = Hook{Event: "PostToolUse", Matcher: "Write|Edit", Command: gate.Command, Timeout: 10}

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

func remove(t *testing.T, path string, deny []string, hooks []Hook) Removal {
	t.Helper()
	r, err := PlanRemoval(path, deny, hooks)
	if err != nil {
		t.Fatalf("plan removal: %v", err)
	}
	if err := r.Apply(); err != nil {
		t.Fatalf("apply removal: %v", err)
	}
	return r
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

// A hand-edited settings file carries the comments encoding/json cannot read.
// The merge has to work rather than abort install with the payload already
// landed.
func TestACommentInSettingsMergesRatherThanAborting(t *testing.T) {
	path := file(t, `{
  // kept from a rebuild that broke the deploy
  "permissions": {"deny": ["Bash(git push:*)"]}
}`)
	merge(t, path, []string{"Bash(git push:*)", "Bash(git merge:*)"}, []Hook{gate})

	deny := read(t, path)["permissions"].(map[string]any)["deny"].([]any)
	if len(deny) != 2 {
		t.Errorf("deny holds %v, want the held rule and the missing one", deny)
	}
}

// A repository may have set settings.json to 0600, since it can carry `env`
// values, and a merge that rewrites the file through a fresh temporary file
// must not widen that to 0644 on its way past. A file this package creates
// rather than merges has no previous mode to keep.
func TestAMergeKeepsTheFilesMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits")
	}
	for name, tc := range map[string]struct {
		existing bool
		mode     os.FileMode
		want     os.FileMode
	}{
		"a file already at 0600 keeps 0600":         {existing: true, mode: 0o600, want: 0o600},
		"a file this package creates lands at 0644": {want: 0o644},
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if tc.existing {
				if err := os.WriteFile(path, []byte(`{"permissions": {}}`), tc.mode); err != nil {
					t.Fatal(err)
				}
			}
			merge(t, path, []string{"Bash(git push:*)"}, nil)

			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != tc.want {
				t.Errorf("mode is %o, want %o", got, tc.want)
			}
		})
	}
}

// An uninstall has to give a repository back what it had before the loop was
// installed, not a settings file merely missing the payload's rules.
func TestARemovalUndoesTheMerge(t *testing.T) {
	path := file(t, `{"model": "opus"}`)
	deny := []string{"Bash(git push:*)", "Bash(git merge:*)", "Bash(git reset:*)", "Bash(git clean:*)"}
	hooks := []Hook{gate, posttool}
	before := read(t, path)

	merge(t, path, deny, hooks)
	remove(t, path, deny, hooks)

	if after := read(t, path); !reflect.DeepEqual(before, after) {
		t.Errorf("removal left %v, want %v", after, before)
	}
}

// A repository's own deny rule, its own hook on the same event as the gate,
// and an unrelated key are not the payload's to take out.
func TestARemovalLeavesWhatTheRepositoryPutThere(t *testing.T) {
	path := file(t, `{
  "permissions": {"deny": ["Bash(git push:*)"]},
  "model": "opus",
  "hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "./audit.sh"}]}]}
}`)
	deny := []string{"Bash(git merge:*)"}
	merge(t, path, deny, []Hook{gate})
	remove(t, path, deny, []Hook{gate})

	body := read(t, path)
	if body["model"] != "opus" {
		t.Errorf("an unrelated key did not survive: %v", body["model"])
	}
	held := body["permissions"].(map[string]any)["deny"].([]any)
	if len(held) != 1 || held[0] != "Bash(git push:*)" {
		t.Errorf("the repository's own deny rule did not survive: %v", held)
	}
	entries := body["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(entries) != 1 {
		t.Fatalf("the matcher entry was dropped rather than emptied of the gate: %v", entries)
	}
	inner := entries[0].(map[string]any)["hooks"].([]any)
	if len(inner) != 1 {
		t.Fatalf("the event runs %d commands, want only the repository's", len(inner))
	}
	if command, _ := inner[0].(map[string]any)["command"].(string); command != "./audit.sh" {
		t.Errorf("the repository's own hook did not survive: %v", inner[0])
	}
}

// An event whose last registration was the gate's must not be left as an
// empty list, and permissions or hooks must not be left behind empty either.
func TestARemovalLeavesNoEmptyObjects(t *testing.T) {
	path := file(t, "")
	deny := []string{"Bash(git push:*)"}
	hooks := []Hook{gate}
	merge(t, path, deny, hooks)
	remove(t, path, deny, hooks)

	body := read(t, path)
	if _, ok := body["permissions"]; ok {
		t.Errorf("permissions was left behind: %v", body["permissions"])
	}
	if _, ok := body["hooks"]; ok {
		t.Errorf("hooks was left behind: %v", body["hooks"])
	}
}

// An uninstall may run twice - the second pass has to find nothing rather
// than erroring on rules already gone, and must not rewrite the file.
func TestASecondRemovalFindsNothingToDo(t *testing.T) {
	path := file(t, "")
	deny := []string{"Bash(git push:*)"}
	hooks := []Hook{gate}
	merge(t, path, deny, hooks)
	remove(t, path, deny, hooks)

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	second, err := PlanRemoval(path, deny, hooks)
	if err != nil {
		t.Fatalf("plan removal: %v", err)
	}
	if !second.Empty() {
		t.Errorf("a second removal would remove %v", second.Summary())
	}
	if err := second.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("a removal with nothing to remove rewrote the file")
	}
}

// A repository that never installed the loop, or already deleted the
// settings file, must not fail an uninstall.
func TestARemovalOfAMissingFileIsNotAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	r, err := PlanRemoval(path, []string{"Bash(git push:*)"}, []Hook{gate})
	if err != nil {
		t.Fatalf("plan removal: %v", err)
	}
	if !r.Empty() {
		t.Errorf("a missing file has nothing to remove, got %v", r.Summary())
	}
	if err := r.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("a removal on a missing file created one")
	}
}
