package render

import (
	"encoding/json"
	"errors"
	"io/fs"
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
		mode := fs.FileMode(0o644)
		if strings.HasSuffix(rel, ".py") || strings.HasSuffix(rel, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(full, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(full, mode); err != nil {
			t.Fatal(err)
		}
	}
	write("skills/review/SKILL.md",
		"---\nname: review\n---\n\nRun `{{PLUGIN}}/scripts/check.py`; a diff touching `{{ROOT}}/` is a change.\n")
	write("skills/review/references/diff.md", "How to read a diff.\n")
	write("commands/revise.md", "Use the `revise` skill.\n")
	write("agents/reviewer.md", "---\nname: reviewer\ndescription: Review the change.\ntools: Skill, Bash, Read\n---\n\nYou review.\n")
	write("hooks/session-start.sh", "#!/bin/sh\necho hello\n")
	write("scripts/check.py", "print('ok')\n")
	write("manifest.json", `{
  "name": "thunderstorm",
  "version": "0.1.0",
  "description": "A test workflow.",
  "author": {"name": "Stormlight Labs"},
  "policy": {"deny": ["git merge"]},
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

// commandOnly keeps tests about one generated file independent of the other
// artifact formats.
const commandOnly = `
    {"kind": "command", "name": "revise", "source": "commands/revise.md", "requires": ["commands"]}`

const piArtifacts = `
    {"kind": "skill", "name": "review", "source": "skills/review", "requires": ["skills"]},
    {"kind": "command", "name": "revise", "source": "commands/revise.md", "aliases": ["edit"], "requires": ["commands"]},
    {"kind": "agent", "name": "reviewer", "source": "agents/reviewer.md", "requires": ["subagents"]},
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
	unverified := *claude
	unverified.Plugin = ""
	if _, err := Plan(m, &unverified, root); err == nil {
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
// as a file a repository merges into its own settings. The manifest names a
// command; Claude Code's own spelling of it is the renderer's to write.
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

// Codex reads a command against execpolicy rules, so the same policy renders
// as one forbidden rule per command, with each command's words as the tokens
// the rule matches.
func TestCodexCarriesThePolicyAsExecpolicyRules(t *testing.T) {
	p, _, err := planFor(t, "codex", commandOnly)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	rules := string(body(t, p, "rules/thunderstorm.rules"))
	for _, want := range []string{
		`pattern = ["git", "merge"]`,
		`decision = "forbidden"`,
	} {
		if !strings.Contains(rules, want) {
			t.Errorf("the rule file does not carry %q:\n%s", want, rules)
		}
	}
}

func TestCodexCarriesCurrentPluginAndAgentFormats(t *testing.T) {
	p, _, err := planFor(t, "codex", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for _, want := range []string{
		"plugin.json",
		".codex-plugin/plugin.json",
		"agents/reviewer.toml",
		"scripts/check.py",
	} {
		body(t, p, want)
	}
	agent := string(body(t, p, "agents/reviewer.toml"))
	for _, want := range []string{
		`name = "reviewer"`,
		`sandbox_mode = "read-only"`,
		`developer_instructions = "You review."`,
	} {
		if !strings.Contains(agent, want) {
			t.Errorf("Codex agent does not carry %q:\n%s", want, agent)
		}
	}
}

// Pi's extension rejects the same command prefixes that the other targets
// write into their native policy files.
func TestPiEnforcesTheDeniedCommands(t *testing.T) {
	p, _, err := planFor(t, "pi", commandOnly)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(p.Limits) != 0 {
		t.Errorf("pi reports %v", p.Limits)
	}
	extension := string(body(t, p, "extensions/thunderstorm.ts"))
	for _, want := range []string{`"git merge"`, `event.toolName !== "bash"`, `block: true`} {
		if !strings.Contains(extension, want) {
			t.Errorf("Pi extension does not carry %q:\n%s", want, extension)
		}
	}
}

func TestPiCarriesTheWholeCurrentWorkflowShape(t *testing.T) {
	p, _, err := planFor(t, "pi", piArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for _, want := range []string{
		"package.json",
		"skills/review/SKILL.md",
		"prompts/revise.md",
		"roles/reviewer.md",
		"scripts/check.py",
	} {
		body(t, p, want)
	}
	skill := string(body(t, p, "skills/review/SKILL.md"))
	if !strings.Contains(skill, "${THUNDERSTORM_PLUGIN_ROOT}/scripts/check.py") {
		t.Errorf("Pi skill does not resolve its script through the package extension: %q", skill)
	}
	var pkg struct {
		Pi struct {
			Extensions []string
			Skills     []string
			Prompts    []string
		}
	}
	if err := json.Unmarshal(body(t, p, "package.json"), &pkg); err != nil {
		t.Fatal(err)
	}
	if len(pkg.Pi.Extensions) != 1 || len(pkg.Pi.Skills) != 1 || len(pkg.Pi.Prompts) != 1 {
		t.Errorf("Pi package resources are %+v", pkg.Pi)
	}
}

// Pi still has no generic hook format. A hook in the source must stop its
// render until the extension adapter in #18 exists.
func TestPiReportsAnUnsupportedHook(t *testing.T) {
	p, _, err := planFor(t, "pi", allArtifacts)
	if p != nil {
		t.Fatal("pi produced a payload despite having no hook adapter")
	}
	var unmet *Unmet
	if !errors.As(err, &unmet) {
		t.Fatalf("error is %T (%v), want *Unmet", err, err)
	}
	if unmet.Target != "pi" {
		t.Errorf("unmet names %q", unmet.Target)
	}
	if !strings.Contains(err.Error(), "#18") {
		t.Errorf("the failure does not name the issue that would close it:\n%s", err)
	}
	if !strings.Contains(err.Error(), "hook session-start.sh") {
		t.Errorf("the failure does not name the blocked artifact:\n%s", err)
	}
}

// Commands render wherever a harness has somewhere to read one.
func TestCommandsRenderForEveryHarnessThatReadsThem(t *testing.T) {
	const commandOnly = `
    {"kind": "command", "name": "revise", "source": "commands/revise.md", "aliases": ["edit"], "requires": ["commands"]}`

	want := map[string]string{
		"claude": "commands/revise.md",
		"codex":  "prompts/revise.md",
		"pi":     "prompts/revise.md",
	}
	for name, wantPath := range want {
		t.Run(name, func(t *testing.T) {
			p, _, err := planFor(t, name, commandOnly)
			if err != nil {
				t.Fatalf("%s cannot carry a command: %v", name, err)
			}
			body(t, p, wantPath)
			if alias := strings.Replace(wantPath, "revise", "edit", 1); len(body(t, p, alias)) == 0 {
				t.Errorf("%s got no alias file", name)
			}
		})
	}
}

// A name becomes a path inside the payload.
func TestANameThatEscapesThePayloadIsRejected(t *testing.T) {
	for _, bad := range []string{`"../../escape"`, `"sub/dir"`, `".."`} {
		artifact := `{"kind": "command", "name": ` + bad + `, "source": "commands/revise.md", "requires": ["commands"]}`
		if _, err := Load(fixture(t, artifact)); err == nil {
			t.Errorf("name %s was accepted", bad)
		}
	}
	aliased := `{"kind": "command", "name": "revise", "source": "commands/revise.md",
	             "aliases": ["../../escape"], "requires": ["commands"]}`
	if _, err := Load(fixture(t, aliased)); err == nil {
		t.Error("an alias escaping the payload was accepted")
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

func TestAnUnexplainedGapNamesTheMissingEntry(t *testing.T) {
	got := claudeTarget().reason("wombat")
	want := "target table records no reason for wombat"
	if got != want {
		t.Errorf("reason is %q, want %q", got, want)
	}
}

func TestWriteReplacesWhatTheSourceDropped(t *testing.T) {
	root := fixture(t, allArtifacts)
	m, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	claude, _ := Lookup("claude")
	full, err := Plan(m, claude, root)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	out := filepath.Join(t.TempDir(), "claude")
	if err := full.Write(out); err != nil {
		t.Fatalf("write: %v", err)
	}
	dropped := filepath.Join(out, "agents", "reviewer.md")
	if _, err := os.Stat(dropped); err != nil {
		t.Fatalf("the first render did not write the agent: %v", err)
	}

	// The same source with one artifact gone, which is what dropping a skill
	// or an agent from the manifest looks like.
	fewer := m
	fewer.Artifacts = nil
	for _, a := range m.Artifacts {
		if a.Kind != KindAgent {
			fewer.Artifacts = append(fewer.Artifacts, a)
		}
	}
	next, err := Plan(fewer, claude, root)
	if err != nil {
		t.Fatalf("plan without the agent: %v", err)
	}
	if err := next.Write(out); err != nil {
		t.Fatalf("second write: %v", err)
	}
	if _, err := os.Stat(dropped); err == nil {
		t.Error("an artifact the source dropped survived the render")
	}
	if diff, err := next.Diff(out); err != nil || len(diff) > 0 {
		t.Errorf("payload differs from what was written: %v (%v)", diff, err)
	}
}

// Anything the marker does not account for is somebody's work, whatever the
// directory is called.
func TestWriteRefusesADirectoryHoldingSomethingNoRenderWrote(t *testing.T) {
	p, _, err := planFor(t, "claude", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	out := filepath.Join(t.TempDir(), "claude")
	if err := p.Write(out); err != nil {
		t.Fatalf("write: %v", err)
	}
	notes := filepath.Join(out, "notes.md")
	if err := os.WriteFile(notes, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = p.Write(out)
	if err == nil {
		t.Fatal("rendered over a directory holding a file no render wrote")
	}
	if !strings.Contains(err.Error(), "notes.md") {
		t.Errorf("the refusal does not name the file: %v", err)
	}
	if _, err := os.Stat(notes); err != nil {
		t.Errorf("the refusal deleted the file anyway: %v", err)
	}
}

// The marker names a target. Trusting its presence alone let a render delete
// any directory that happened to hold a file by that name, including one
// carried along by a copy of the committed payload.
func TestWriteRefusesADirectoryHoldingAnotherTargetsPayload(t *testing.T) {
	p, _, err := planFor(t, "claude", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	out := t.TempDir()
	keep := filepath.Join(out, "thesis.txt")
	if err := os.WriteFile(keep, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, marker), []byte("pi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = p.Write(out)
	if err == nil {
		t.Fatal("rendered over a directory carrying another target's marker")
	}
	if !strings.Contains(err.Error(), "pi") {
		t.Errorf("the refusal does not say what the directory holds: %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("the refusal deleted a file anyway: %v", err)
	}
}

// --out is a path a person types, so a render refuses to replace a directory
// it cannot show it wrote.
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

// A render that cannot finish leaves the previous payload whole, rather than
// a tree half of one version and half of another.
func TestAFailedWriteLeavesThePreviousPayloadIntact(t *testing.T) {
	p, root, err := planFor(t, "claude", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	out := filepath.Join(t.TempDir(), "claude")
	if err := p.Write(out); err != nil {
		t.Fatalf("write: %v", err)
	}
	before := string(body(t, p, "agents/reviewer.md"))

	second, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	claude, _ := Lookup("claude")
	next, err := Plan(second, claude, root)
	if err != nil {
		t.Fatal(err)
	}
	// One file the staging directory cannot hold: a path under a name that is
	// already a file there.
	next.Files = append(next.Files, File{Path: "agents/reviewer.md/nested", Body: []byte("x")})
	if err := next.Write(out); err == nil {
		t.Fatal("a payload that cannot be staged was written anyway")
	}
	got, err := os.ReadFile(filepath.Join(out, "agents", "reviewer.md"))
	if err != nil {
		t.Fatalf("the previous payload did not survive: %v", err)
	}
	if string(got) != before {
		t.Error("the previous payload was replaced by a partial render")
	}
}

// A symlinked --out was unlinked and then reported missing, which destroyed
// the link and left the render half done.
func TestWriteFollowsASymlinkedOut(t *testing.T) {
	p, _, err := planFor(t, "claude", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := p.Write(link); err != nil {
		t.Fatalf("write through a symlink: %v", err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the symlink did not survive the render: %v", err)
	}
	if _, err := os.Stat(filepath.Join(real, "settings.json")); err != nil {
		t.Errorf("the payload did not land in the directory the link points at: %v", err)
	}
}

// The skills invoke the scripts by path, so an executable bit that drifted is
// a broken payload that a bytes-only check would certify as correct.
func TestCheckReportsAnExecutableBitThatDrifted(t *testing.T) {
	p, _, err := planFor(t, "claude", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	out := filepath.Join(t.TempDir(), "claude")
	if err := p.Write(out); err != nil {
		t.Fatalf("write: %v", err)
	}
	script := filepath.Join(out, "scripts", "check.py")
	if err := os.Chmod(script, 0o644); err != nil {
		t.Fatal(err)
	}
	diff, err := p.Diff(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff) != 1 || !strings.Contains(diff[0], "scripts/check.py") {
		t.Fatalf("a dropped executable bit went unreported: %v", diff)
	}
	if err := p.Write(out); err != nil {
		t.Fatalf("re-render: %v", err)
	}
	if diff, _ := p.Diff(out); len(diff) > 0 {
		t.Errorf("a re-render did not repair the mode: %v", diff)
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
