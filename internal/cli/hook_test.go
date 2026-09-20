package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// documented builds a repository whose documents tree is configured, and
// returns its root.
func documented(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".tstorm.json"),
		[]byte(`{"documents": "docs/internal"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for relative, text := range files {
		path := filepath.Join(root, "docs", "internal", relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// event is one hook call in the shape Claude Code and Codex both send.
func event(t *testing.T, cwd, tool, path string) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"hook_event_name": "PostToolUse",
		"tool_name":       tool,
		"cwd":             cwd,
		"tool_input":      map[string]any{"file_path": path},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// advice is the additionalContext the hook answered with, empty when it said
// nothing.
func advice(t *testing.T, stdout string) string {
	t.Helper()
	if strings.TrimSpace(stdout) == "" {
		return ""
	}
	var answer struct {
		Out struct {
			Event   string `json:"hookEventName"`
			Context string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(stdout), &answer); err != nil {
		t.Fatalf("the reply is not JSON the harness can read: %v %q", err, stdout)
	}
	if answer.Out.Event != "PostToolUse" {
		t.Errorf("the reply names event %q, which the harness will not match", answer.Out.Event)
	}
	return answer.Out.Context
}

func TestAWrittenDocumentWithNoBlockIsReported(t *testing.T) {
	root := documented(t, map[string]string{"broken.md": "# no block at all\n"})
	stdout, _, err := runWith(t, event(t, root, "Write", "docs/internal/broken.md"), "hook")
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v", code, err)
	}
	if got := advice(t, stdout); !strings.Contains(got, "does not open with frontmatter") {
		t.Errorf("the gate said %q", got)
	}
}

// The gate reports the document that was written, not every document in the
// tree. A hook that reports its neighbours is ignored by the second week.
func TestTheGateReportsOnlyTheDocumentThatWasWritten(t *testing.T) {
	root := documented(t, map[string]string{
		"broken.md": "# no block at all\n",
		"other.md":  "# also no block\n",
	})
	stdout, _, err := runWith(t, event(t, root, "Write", "docs/internal/broken.md"), "hook")
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v", code, err)
	}
	got := advice(t, stdout)
	if !strings.Contains(got, "broken.md") || strings.Contains(got, "other.md") {
		t.Errorf("the gate reported a file the session did not write: %q", got)
	}
}

func TestTheGateIsSilentWhenItHasNothingToSay(t *testing.T) {
	good := "---\nname: ok\nlast_updated: 2026-09-20\nid: 01M2RFP6G4NBXT94SAAYZ1D1FH\n---\n\n# Body\n"
	root := documented(t, map[string]string{"ok.md": good})
	for name, path := range map[string]string{
		"a document that is right":     "docs/internal/ok.md",
		"a file outside the tree":      "README.md",
		"a file the tree does not own": "internal/check/frontmatter.go",
	} {
		t.Run(name, func(t *testing.T) {
			stdout, _, err := runWith(t, event(t, root, "Write", path), "hook")
			if code := ExitCode(err); code != 0 {
				t.Fatalf("exit %d, want 0: %v", code, err)
			}
			if strings.TrimSpace(stdout) != "" {
				t.Errorf("the gate spoke up: %q", stdout)
			}
		})
	}
}

// A repository that has configured no documents tree has not asked for this
// gate, and every write there has to pass in silence.
func TestAnUnconfiguredRepositoryHearsNothing(t *testing.T) {
	root := t.TempDir()
	stdout, _, err := runWith(t, event(t, root, "Write", "notes.md"), "hook")
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v", code, err)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("the gate spoke up: %q", stdout)
	}
}

// Every failure here is a failure to run a gate, and a gate that refuses a
// write gets uninstalled. Nothing below may reach a non-zero status.
func TestTheGateNeverBlocksAWrite(t *testing.T) {
	root := documented(t, map[string]string{"broken.md": "# no block\n"})
	for name, stdin := range map[string]string{
		"an event that is not JSON":   "not json at all",
		"an empty event":              "{}",
		"an event with no tool input": `{"hook_event_name":"PostToolUse","tool_name":"Write"}`,
		"an event for another moment": `{"hook_event_name":"SessionStart","cwd":"` + root + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			stdout, _, err := runWith(t, stdin, "hook")
			if code := ExitCode(err); code != 0 {
				t.Fatalf("exit %d, want 0: %v", code, err)
			}
			if strings.TrimSpace(stdout) != "" {
				t.Errorf("the gate answered anyway: %q", stdout)
			}
		})
	}
}

// Pi's write and edit tools name the file `path`. The adapter absorbs that
// rather than the gate.
func TestTheGateReadsPiSpellingOfTheFilePath(t *testing.T) {
	root := documented(t, map[string]string{"broken.md": "# no block\n"})
	stdin, err := json.Marshal(map[string]any{
		"hook_event_name": "PostToolUse",
		"tool_name":       "write",
		"cwd":             root,
		"tool_input":      map[string]any{"path": "docs/internal/broken.md"},
	})
	if err != nil {
		t.Fatal(err)
	}
	stdout, _, runErr := runWith(t, string(stdin), "hook")
	if code := ExitCode(runErr); code != 0 {
		t.Fatalf("exit %d, want 0: %v", code, runErr)
	}
	if got := advice(t, stdout); !strings.Contains(got, "does not open with frontmatter") {
		t.Errorf("the gate said %q", got)
	}
}
