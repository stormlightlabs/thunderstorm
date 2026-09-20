package render

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture writes a source tree small enough to read in one screen and shaped
// like the real one: a skill with a reference, a command with an alias, an
// agent, a hook, and a script.
func fixture(t *testing.T, artifacts string) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("skills/review/SKILL.md",
		"---\nname: review\n---\n\nRun `{{PLUGIN}}/scripts/check.py`; a diff touching `{{ROOT}}/` is a change.\n")
	write("skills/review/references/diff.md", "How to read a diff.\n")
	write("commands/revise.md", "Use the `revise` skill.\n")
	write("agents/reviewer.md", "You review.\n")
	write("hooks/session-start.sh", "#!/bin/sh\necho hello\n")
	write("scripts/check.py", "print('ok')\n")
	write("manifest.json", `{
  "name": "thunderstorm",
  "version": "0.1.0",
  "description": "A test workflow.",
  "author": {"name": "Stormlight Labs"},
  "policy": {"deny": ["Bash(git merge:*)"]},
  "artifacts": [`+artifacts+`]
}`)
	return root
}

const allArtifacts = `
    {"kind": "skill", "name": "review", "source": "skills/review", "requires": ["skills"]},
    {"kind": "command", "name": "revise", "source": "commands/revise.md", "aliases": ["edit"], "requires": ["commands"]},
    {"kind": "agent", "name": "reviewer", "source": "agents/reviewer.md", "requires": ["subagents"]},
    {"kind": "hook", "name": "session-start.sh", "source": "hooks/session-start.sh", "requires": ["hooks"],
     "event": "SessionStart", "matcher": "startup", "timeout": 1200},
    {"kind": "script", "name": "check.py", "source": "scripts/check.py", "requires": ["scripts"]}`

func planFor(t *testing.T, target, artifacts string) (*Payload, string, error) {
	t.Helper()
	root := fixture(t, artifacts)
	m, err := Load(root)
	if err != nil {
		return nil, root, err
	}
	tgt, err := Lookup(target)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Plan(m, tgt, root)
	return p, root, err
}

func paths(p *Payload) []string {
	out := make([]string, 0, len(p.Files))
	for _, f := range p.Files {
		out = append(out, f.Path)
	}
	return out
}

func body(t *testing.T, p *Payload, want string) []byte {
	t.Helper()
	for _, f := range p.Files {
		if f.Path == want {
			return f.Body
		}
	}
	t.Fatalf("payload has no %s, only %v", want, paths(p))
	return nil
}

func TestClaudePayloadPlacesEveryKind(t *testing.T) {
	p, _, err := planFor(t, "claude", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for _, want := range []string{
		"skills/review/SKILL.md",
		"skills/review/references/diff.md",
		"commands/revise.md",
		"agents/reviewer.md",
		"hooks/session-start.sh",
		"hooks/hooks.json",
		"scripts/check.py",
		"settings.json",
		".claude-plugin/plugin.json",
	} {
		body(t, p, want)
	}
}

// An alias is a second file, because no harness here resolves one itself.
func TestAnAliasBecomesItsOwnFile(t *testing.T) {
	p, _, err := planFor(t, "claude", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	canonical := body(t, p, "commands/revise.md")
	alias := body(t, p, "commands/edit.md")
	if string(canonical) != string(alias) {
		t.Errorf("alias body differs from the command it aliases:\n%s\n%s", canonical, alias)
	}
}

// The two roots are different places once a payload is installed rather than
// copied: the scripts sit where the plugin landed, and the worktrees sit in the
// repository being worked on.
func TestProseSeparatesThePluginFromTheRepository(t *testing.T) {
	p, _, err := planFor(t, "claude", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	skill := string(body(t, p, "skills/review/SKILL.md"))
	if strings.Contains(skill, rootToken) || strings.Contains(skill, pluginToken) {
		t.Errorf("a token survived the render: %q", skill)
	}
	if !strings.Contains(skill, "${CLAUDE_PLUGIN_ROOT}/scripts/check.py") {
		t.Errorf("the script does not resolve to the installed payload: %q", skill)
	}
	if !strings.Contains(skill, "`.claude/` is a change") {
		t.Errorf("the repository directory did not survive as itself: %q", skill)
	}
}

// A target with no verified install layout has no value for {{PLUGIN}}. Leaving
// the braces in the prose, or quietly dropping them, both reach a reader.
func TestAnUnvaluedTokenStopsTheRender(t *testing.T) {
	root := fixture(t, allArtifacts)
	m, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	claude, _ := Lookup("claude")
	claude.Plugin = ""
	if _, err := Plan(m, claude, root); err == nil {
		t.Fatal("rendered prose carrying a token the target cannot resolve")
	} else if !strings.Contains(err.Error(), pluginToken) {
		t.Errorf("the failure does not name the token: %v", err)
	}
}

// The first real install reported no hooks: a settings.json in a payload is
// read by nothing, and hooks/hooks.json is what a plugin carries.
func TestThePayloadRegistersItsHookWhereAPluginIsRead(t *testing.T) {
	p, _, err := planFor(t, "claude", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	var hooks struct {
		Hooks map[string][]struct {
			Matcher string
			Hooks   []struct{ Command string }
		}
	}
	if err := json.Unmarshal(body(t, p, "hooks/hooks.json"), &hooks); err != nil {
		t.Fatal(err)
	}
	start := hooks.Hooks["SessionStart"]
	if len(start) != 1 {
		t.Fatalf("SessionStart has %d registrations, want 1", len(start))
	}
	if got := start[0].Hooks[0].Command; got != "${CLAUDE_PLUGIN_ROOT}/hooks/session-start.sh" {
		t.Errorf("hook command is %q, which is not where the install puts it", got)
	}
}

// No plugin mechanism carries a permission rule, so the deny rules ride along
// as a file a repository merges into its own settings.
func TestTheDenyRulesShipForARepositoryToMerge(t *testing.T) {
	p, _, err := planFor(t, "claude", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	var settings struct {
		Permissions struct{ Deny []string }
	}
	if err := json.Unmarshal(body(t, p, "settings.json"), &settings); err != nil {
		t.Fatal(err)
	}
	if len(settings.Permissions.Deny) != 1 || settings.Permissions.Deny[0] != "Bash(git merge:*)" {
		t.Errorf("deny rules are %v", settings.Permissions.Deny)
	}
}

// The issue's rule: a payload that installs and then skips part of the workflow
// is worse than no payload, so an unmet requirement stops the whole render.
func TestAMissingCapabilityStopsTheRender(t *testing.T) {
	p, _, err := planFor(t, "pi", allArtifacts)
	if p != nil {
		t.Fatal("pi produced a payload despite having no subagents")
	}
	var unmet *Unmet
	if !errors.As(err, &unmet) {
		t.Fatalf("error is %T (%v), want *Unmet", err, err)
	}
	if unmet.Target != "pi" {
		t.Errorf("unmet names %q", unmet.Target)
	}
	if !strings.Contains(err.Error(), "#8") {
		t.Errorf("the failure does not name the issue that would close it:\n%s", err)
	}
	if !strings.Contains(err.Error(), "agent reviewer") {
		t.Errorf("the failure does not name the blocked artifact:\n%s", err)
	}
}

// Cursor is in the table so that asking for it answers, rather than looking
// like an option nobody thought about.
func TestCursorReportsAnUnverifiedContract(t *testing.T) {
	_, _, err := planFor(t, "cursor", allArtifacts)
	if err == nil {
		t.Fatal("cursor rendered a payload from an unverified contract")
	}
	if !strings.Contains(err.Error(), "unverified") || !strings.Contains(err.Error(), "#13") {
		t.Errorf("cursor's refusal does not say why:\n%s", err)
	}
}

func TestWriteThenPruneRemovesWhatTheSourceDropped(t *testing.T) {
	p, _, err := planFor(t, "claude", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	out := filepath.Join(t.TempDir(), "claude")
	if err := p.Write(out); err != nil {
		t.Fatalf("write: %v", err)
	}
	stale := filepath.Join(out, "commands", "gone.md")
	if err := os.WriteFile(stale, []byte("left over\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := p.Write(out); err != nil {
		t.Fatalf("second write: %v", err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("a file the source no longer produces survived the render")
	}
	if diff, err := p.Diff(out); err != nil || len(diff) > 0 {
		t.Errorf("payload differs from what was written: %v (%v)", diff, err)
	}
}

// --out is a path a person types, so a render refuses to prune a directory it
// cannot show it wrote.
func TestWriteRefusesADirectoryItDidNotRender(t *testing.T) {
	p, _, err := planFor(t, "claude", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	out := t.TempDir()
	if err := os.WriteFile(filepath.Join(out, "notes.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := p.Write(out); err == nil {
		t.Fatal("rendered over a directory that was not a payload")
	}
	if _, err := os.Stat(filepath.Join(out, "notes.md")); err != nil {
		t.Errorf("the refusal still removed a file: %v", err)
	}
}

func TestDiffNamesStaleAndUnexpectedFiles(t *testing.T) {
	p, _, err := planFor(t, "claude", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	out := filepath.Join(t.TempDir(), "claude")
	if err := p.Write(out); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, "agents", "reviewer.md"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "agents", "extra.md"), []byte("added\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	diff, err := p.Diff(out)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(diff, "\n")
	if !strings.Contains(joined, "stale: agents/reviewer.md") || !strings.Contains(joined, "unexpected: agents/extra.md") {
		t.Errorf("diff missed an edit or an addition:\n%s", joined)
	}
}

func TestManifestRejectsWhatWouldRenderWrong(t *testing.T) {
	cases := map[string]struct{ artifacts, want string }{
		"unknown capability": {
			`{"kind": "skill", "name": "review", "source": "skills/review", "requires": ["telepathy"]}`,
			"unknown capability",
		},
		"alias on a skill": {
			`{"kind": "skill", "name": "review", "source": "skills/review", "aliases": ["r"], "requires": ["skills"]}`,
			"only a command carries aliases",
		},
		"missing source": {
			`{"kind": "command", "name": "ghost", "source": "commands/ghost.md", "requires": ["commands"]}`,
			"no such file",
		},
		"hook without an event": {
			`{"kind": "hook", "name": "session-start.sh", "source": "hooks/session-start.sh", "requires": ["hooks"]}`,
			"has no event",
		},
		"two artifacts claiming one name": {
			`{"kind": "command", "name": "revise", "source": "commands/revise.md", "requires": ["commands"]},
			 {"kind": "command", "name": "revise", "source": "commands/revise.md", "requires": ["commands"]}`,
			"claimed by both",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Load(fixture(t, tc.artifacts))
			if err == nil {
				t.Fatal("manifest loaded, want an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error is %q, want it to mention %q", err, tc.want)
			}
		})
	}
}
