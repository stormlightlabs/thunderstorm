package dispatch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestRolesAreTheFourARunDispatches(t *testing.T) {
	roles, err := Roles()
	if err != nil {
		t.Fatalf("Roles returned %v", err)
	}
	var names []string
	for _, r := range roles {
		names = append(names, r.Name)
		if r.Prompt == "" || r.Description == "" || len(r.ClaudeTools) == 0 {
			t.Errorf("role %s is missing a prompt, a description, or its tools", r.Name)
		}
	}
	want := []string{"adversarial-reviewer", "implementer", "reviewer", "reviser"}
	if !slices.Equal(names, want) {
		t.Errorf("roles are %v, want %v", names, want)
	}
}

func TestLookupRoleNamesTheOnesItHas(t *testing.T) {
	if _, err := LookupRole("implementer"); err != nil {
		t.Fatalf("LookupRole(implementer) returned %v", err)
	}
	_, err := LookupRole("orchestrator")
	if err == nil {
		t.Fatal("LookupRole accepted a role that does not exist")
	}
	if !strings.Contains(err.Error(), "reviewer") {
		t.Errorf("the error does not list the roles: %v", err)
	}
}

func TestParseRoleRefusesADefinitionItCannotTrust(t *testing.T) {
	for name, text := range map[string]string{
		"no frontmatter": "Use the review skill.\n",
		"unclosed":       "---\nname: reviewer\n",
		"no tools":       "---\nname: reviewer\ndescription: reviews\n---\nUse the review skill.\n",
		"no body":        "---\nname: reviewer\ndescription: reviews\ntools: Read\n---\n\n",
	} {
		if _, err := parseRole(text); err == nil {
			t.Errorf("%s: parseRole accepted it", name)
		}
	}
}

func TestPiToolsDropsWhatPiHasNoToolFor(t *testing.T) {
	reviewer, err := LookupRole("reviewer")
	if err != nil {
		t.Fatal(err)
	}
	allow, untranslated := PiTools(reviewer.ClaudeTools)
	if slices.Contains(allow, "write") || slices.Contains(allow, "edit") {
		t.Errorf("the reviewer may write: %v", allow)
	}
	for _, want := range []string{"bash", "find", "grep", "ls", "read"} {
		if !slices.Contains(allow, want) {
			t.Errorf("the reviewer cannot %s: %v", want, allow)
		}
	}
	if !slices.Contains(untranslated, "the MCP tools") {
		t.Errorf("the MCP tools went unreported: %v", untranslated)
	}
	if n := strings.Count(strings.Join(untranslated, " "), "MCP"); n != 1 {
		t.Errorf("the MCP tools are reported %d times, want once: %v", n, untranslated)
	}
}

func TestImplementerKeepsTheToolsItWritesWith(t *testing.T) {
	implementer, err := LookupRole("implementer")
	if err != nil {
		t.Fatal(err)
	}
	allow, _ := PiTools(implementer.ClaudeTools)
	for _, want := range []string{"write", "edit", "bash"} {
		if !slices.Contains(allow, want) {
			t.Errorf("the implementer cannot %s: %v", want, allow)
		}
	}
}

func TestShellQuoteSurvivesAQuote(t *testing.T) {
	quoted := shellQuote(`it's "fine"; rm -rf /`)
	out, err := exec.Command("/bin/sh", "-c", "printf '%s' "+quoted).Output()
	if err != nil {
		t.Fatalf("sh returned %v", err)
	}
	if string(out) != `it's "fine"; rm -rf /` {
		t.Errorf("sh read %q", out)
	}
}

func TestRunScriptRecordsTheExitStatus(t *testing.T) {
	dir := t.TempDir()
	script := runScript([]string{"/bin/sh", "-c", "exit 3"}, dir)
	path := filepath.Join(dir, "run.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("/bin/sh", path).Run(); err != nil {
		t.Fatalf("the script returned %v", err)
	}
	status, err := readStatus(filepath.Join(dir, "status"))
	if err != nil {
		t.Fatal(err)
	}
	if status != 3 {
		t.Errorf("status is %d, want 3", status)
	}
}

const transcript = `{"type":"agent_start"}
{"type":"message_end","message":{"role":"user","content":[{"type":"text","text":"review it"}]}}
{"type":"message_end","message":{"role":"assistant","model":"gpt-5.6-terra","provider":"openai-codex","content":[{"type":"text","text":"working"}]}}
not json
{"type":"message_end","message":{"role":"assistant","model":"gpt-5.6-terra","provider":"openai-codex","content":[{"type":"text","text":"No findings"}]}}
{"type":"agent_settled"}
`

func TestReadTranscriptReturnsTheLastReportAndItsModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte(transcript), 0o644); err != nil {
		t.Fatal(err)
	}
	report, model, err := readTranscript(path)
	if err != nil {
		t.Fatalf("readTranscript returned %v", err)
	}
	if report != "No findings" {
		t.Errorf("report is %q", report)
	}
	if model != "openai-codex/gpt-5.6-terra" {
		t.Errorf("model is %q", model)
	}
}

func TestReadTranscriptSaysWhenThereIsNoReport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"agent_start"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readTranscript(path); err == nil {
		t.Fatal("readTranscript accepted a transcript with no report")
	}
}

func TestRunRefusesARequestItCannotRecord(t *testing.T) {
	role, err := LookupRole("reviewer")
	if err != nil {
		t.Fatal(err)
	}
	good := Request{Role: role, Worktree: t.TempDir(), Model: "m", Thinking: "high", Task: "go", Timeout: time.Minute}
	for name, change := range map[string]func(*Request){
		"no model":       func(r *Request) { r.Model = "" },
		"no task":        func(r *Request) { r.Task = "  " },
		"no timeout":     func(r *Request) { r.Timeout = 0 },
		"unknown level":  func(r *Request) { r.Thinking = "hard" },
		"no such tree":   func(r *Request) { r.Worktree = filepath.Join(r.Worktree, "gone") },
		"tree is a file": func(r *Request) { r.Worktree = writeFile(t, "tree") },
	} {
		req := good
		change(&req)
		if err := req.validate(); err == nil {
			t.Errorf("%s: validate accepted it", name)
		}
	}
	if err := good.validate(); err != nil {
		t.Errorf("validate rejected a whole request: %v", err)
	}
}

func TestCheckHarnessNamesWhatDrivesARoleElsewhere(t *testing.T) {
	if err := CheckHarness(Pi); err != nil {
		t.Errorf("CheckHarness(pi) returned %v", err)
	}
	for harness, want := range map[string]string{
		"claude":   "subagent",
		"codex":    "spawn_agent",
		"cursor":   "unsupported",
		"opencode": "unsupported",
	} {
		err := CheckHarness(harness)
		if err == nil {
			t.Errorf("CheckHarness(%s) returned nil", harness)
			continue
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("CheckHarness(%s) does not say why: %v", harness, err)
		}
	}
	if err := CheckHarness("aider"); err == nil {
		t.Error("CheckHarness accepted a harness the workflow does not name")
	}
}

// TestRunDrivesAPane runs the whole dispatch against a pi that answers from a
// fixture, so the tmux driving is exercised without a model.
func TestRunDrivesAPane(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	sockets := t.TempDir()
	t.Setenv("TMUX_TMPDIR", sockets)

	stub := t.TempDir()
	argv := filepath.Join(stub, "argv")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >" + shellQuote(argv) + "\ncat " + shellQuote(writeTranscript(t)) + "\n"
	if err := os.WriteFile(filepath.Join(stub, "pi"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", stub+string(os.PathListSeparator)+os.Getenv("PATH"))

	role, err := LookupRole("reviewer")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "pass")
	res, err := Run(context.Background(), Request{
		Role:     role,
		Worktree: t.TempDir(),
		Model:    "openai-codex/gpt-5.6-terra",
		Thinking: "high",
		Task:     "Review #8",
		Out:      out,
		Timeout:  30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Run returned %v", err)
	}
	if res.Report != "No findings" {
		t.Errorf("report is %q", res.Report)
	}
	if res.Model != "openai-codex/gpt-5.6-terra" {
		t.Errorf("model is %q", res.Model)
	}
	if res.Status != 0 {
		t.Errorf("status is %d", res.Status)
	}

	// tmux leaves a socket file behind when its server exits, and a run that
	// leaves one adds a file per dispatch to the socket directory.
	left, err := filepath.Glob(filepath.Join(sockets, "tmux-*", "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(left) > 0 {
		t.Errorf("the dispatch left %v", left)
	}

	passed, err := os.ReadFile(argv)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--thinking\nhigh", "--tools\nbash,find,grep,ls,read", "Review #8"} {
		if !strings.Contains(string(passed), want) {
			t.Errorf("pi was not given %q: %s", want, passed)
		}
	}
	system, err := os.ReadFile(filepath.Join(out, "system.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(system), "openai-codex/gpt-5.6-terra") {
		t.Error("the system prompt does not name the model the role has to report")
	}
}

// TestRunRefusesASecondTranscript keeps one run's evidence from being written
// over by the next.
func TestRunRefusesASecondTranscript(t *testing.T) {
	out := t.TempDir()
	if err := os.WriteFile(filepath.Join(out, "events.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (Request{Role: Role{Name: "reviewer"}, Out: out}).outDir(); err == nil {
		t.Error("outDir accepted a directory that already holds a transcript")
	}
}

func writeFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeTranscript(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.jsonl")
	if err := os.WriteFile(path, []byte(transcript), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
