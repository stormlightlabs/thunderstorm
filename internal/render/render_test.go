package render

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
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
		"---\nname: review\n---\n\nRead `{{PLUGIN}}/agents/reviewer.toml`, then run `{{PLUGIN}}/scripts/check.py`; a diff touching `{{ROOT}}/` is a change.\n")
	write("skills/review/references/diff.md", "How to read a diff.\n")
	write("commands/revise.md", "Use the `revise` skill.\n")
	write("agents/reviewer.md", "---\nname: reviewer\ndescription: Review the change.\ntools: Skill, Bash, Read\n---\n\nYou review.\n")
	write("hooks/session-start.sh", "#!/bin/sh\necho hello\n")
	write("hooks/check-documents.sh", "#!/bin/sh\nexec tstorm hook\n")
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
    {"kind": "hook", "name": "check-documents.sh", "source": "hooks/check-documents.sh", "requires": ["hooks"],
     "event": "PostToolUse", "matcher": "Write|Edit", "timeout": 10},
    {"kind": "script", "name": "check.py", "source": "scripts/check.py", "requires": ["scripts"]}`

// commandOnly keeps tests about one generated file independent of the other
// artifact formats.
const commandOnly = `
    {"kind": "command", "name": "revise", "source": "commands/revise.md", "requires": ["commands"]}`

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
		"hooks/deny-command.py",
		"hooks/hooks.json",
		"scripts/check.py",
	} {
		body(t, p, want)
	}
	agent := string(body(t, p, "agents/reviewer.toml"))
	for _, want := range []string{
		`name = "reviewer"`,
		`sandbox_mode = "read-only"`,
		"Resolve paths that start with `../` from the directory containing this role file.",
		`You review.`,
	} {
		if !strings.Contains(agent, want) {
			t.Errorf("Codex agent does not carry %q:\n%s", want, agent)
		}
	}
	skill := string(body(t, p, "skills/review/SKILL.md"))
	for _, want := range []string{
		"Resolve paths that start with `../../` from the directory containing this `SKILL.md`.",
		"`../../agents/reviewer.toml`",
		"`../../scripts/check.py`",
	} {
		if !strings.Contains(skill, want) {
			t.Errorf("Codex skill does not carry %q:\n%s", want, skill)
		}
	}
}

func TestCodexPluginHookDeniesReservedCommands(t *testing.T) {
	p, _, err := planFor(t, "codex", commandOnly)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	script := filepath.Join(t.TempDir(), "deny-command.py")
	if err := os.WriteFile(script, body(t, p, "hooks/deny-command.py"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", script)
	cmd.Stdin = strings.NewReader(`{"tool_input":{"command":"go test ./... && git merge main"}}`)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hook: %v", err)
	}
	var result struct {
		HookSpecificOutput struct {
			Decision string `json:"permissionDecision"`
			Reason   string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if result.HookSpecificOutput.Decision != "deny" {
		t.Errorf("decision is %q", result.HookSpecificOutput.Decision)
	}
	if !strings.Contains(result.HookSpecificOutput.Reason, "git merge") {
		t.Errorf("reason is %q", result.HookSpecificOutput.Reason)
	}
	bad := exec.Command("python3", script)
	bad.Stdin = strings.NewReader("{")
	if err := bad.Run(); err == nil {
		t.Fatal("hook accepted malformed input")
	} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 2 {
		t.Fatalf("malformed input exited with %v, want 2", err)
	}
}

func TestRepositoryCodexMarketplacePointsAtPayload(t *testing.T) {
	root := filepath.Join("..", "..")
	marketplaceBody, err := os.ReadFile(filepath.Join(root, ".agents", "plugins", "marketplace.json"))
	if err != nil {
		t.Fatal(err)
	}
	var marketplace struct {
		Name    string
		Plugins []struct {
			Name   string
			Source struct{ Path string }
		}
	}
	if err := json.Unmarshal(marketplaceBody, &marketplace); err != nil {
		t.Fatal(err)
	}
	if marketplace.Name != "stormlightlabs" || len(marketplace.Plugins) != 1 {
		t.Fatalf("marketplace is %+v", marketplace)
	}
	plugin := marketplace.Plugins[0]
	if plugin.Name != "thunderstorm" || plugin.Source.Path != "./payloads/codex" {
		t.Errorf("marketplace plugin is %+v", plugin)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(plugin.Source.Path, "./")), "plugin.json")); err != nil {
		t.Errorf("marketplace source has no plugin manifest: %v", err)
	}
	config, err := os.ReadFile(filepath.Join(root, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`[marketplaces.stormlightlabs]`,
		`source = "https://github.com/stormlightlabs/thunderstorm.git"`,
		`[plugins."thunderstorm@stormlightlabs"]`,
		`enabled = true`,
	} {
		if !strings.Contains(string(config), want) {
			t.Errorf("project Codex config does not carry %q:\n%s", want, config)
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
	p, _, err := planFor(t, "pi", allArtifacts)
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

// Pi has no hooks. The payload carries the same scripts the other two
// harnesses register, and the package extension is what runs them, so a gate
// written once reaches all three.
func TestPiRunsAHookThroughItsExtension(t *testing.T) {
	p, _, err := planFor(t, "pi", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	body(t, p, "hooks/check-documents.sh")

	extension := string(body(t, p, "extensions/thunderstorm.ts"))
	for _, want := range []string{
		`"name":"check-documents.sh"`,
		`"matcher":"Write|Edit"`,
		`"timeout":10`,
		`pi.on("tool_result"`,
	} {
		if !strings.Contains(extension, want) {
			t.Errorf("the extension does not carry %s:\n%s", want, extension)
		}
	}
	// Pi's tool names are lower case, so a matcher written in Claude Code's
	// names has to reach them.
	if !strings.Contains(extension, `"^(" + gate.matcher + ")$", "i"`) {
		t.Errorf("the extension matches Pi's tool names case-sensitively:\n%s", extension)
	}
}

// Both events register together on Codex: the workflow's own gate and the
// command policy that exists because Codex has no deny setting.
func TestCodexRegistersTheWorkflowHooksBesideItsPolicy(t *testing.T) {
	p, _, err := planFor(t, "codex", allArtifacts)
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
		t.Fatalf("hooks.json: %v", err)
	}
	post := hooks.Hooks["PostToolUse"]
	if len(post) != 1 || post[0].Matcher != "Write|Edit" {
		t.Fatalf("PostToolUse registrations are %+v", post)
	}
	if got := post[0].Hooks[0].Command; got != "${PLUGIN_ROOT}/hooks/check-documents.sh" {
		t.Errorf("the gate resolves to %q", got)
	}
	pre := hooks.Hooks["PreToolUse"]
	if len(pre) != 1 || pre[0].Matcher != "Bash" {
		t.Fatalf("the command policy registration is %+v", pre)
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
// like an option nobody thought about. It is also the one target that still
// refuses everything, which is what keeps the loud-failure path covered.
func TestCursorReportsAnUnverifiedContract(t *testing.T) {
	p, _, err := planFor(t, "cursor", allArtifacts)
	if p != nil {
		t.Fatal("cursor rendered a payload from an unverified contract")
	}
	var unmet *Unmet
	if !errors.As(err, &unmet) {
		t.Fatalf("error is %T (%v), want *Unmet", err, err)
	}
	if unmet.Target != "cursor" {
		t.Errorf("unmet names %q", unmet.Target)
	}
	if !strings.Contains(err.Error(), "unverified") || !strings.Contains(err.Error(), "#13") {
		t.Errorf("cursor's refusal does not say why:\n%s", err)
	}
	if !strings.Contains(err.Error(), "hook check-documents.sh") {
		t.Errorf("the refusal does not name every blocked artifact:\n%s", err)
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
	if _, err := full.Write(out, false); err != nil {
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
	if _, err := next.Write(out, false); err != nil {
		t.Fatalf("second write: %v", err)
	}
	if _, err := os.Stat(dropped); err == nil {
		t.Error("an artifact the source dropped survived the render")
	}
	if diff, _, err := next.Diff(out); err != nil || len(diff) > 0 {
		t.Errorf("payload differs from what was written: %v (%v)", diff, err)
	}
}

// A repository keeps its own settings, hooks and worktrees in the directory it
// installs the payload into. The marker decides what a render may replace, and
// everything else survives it.
func TestWriteLeavesWhatTheMarkerDoesNotList(t *testing.T) {
	p, _, err := planFor(t, "claude", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	out := filepath.Join(t.TempDir(), "claude")
	if _, err := p.Write(out, false); err != nil {
		t.Fatalf("write: %v", err)
	}
	theirs := map[string]string{
		"notes.md":            "mine\n",
		"hooks/repo-owned.sh": "#!/bin/sh\n",
		"worktrees/one/file":  "a checkout\n",
	}
	for rel, body := range theirs {
		full := filepath.Join(out, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	kept, err := p.Write(out, false)
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if len(kept) != len(theirs) {
		t.Errorf("kept %v, want the %d files the repository owns", kept, len(theirs))
	}
	for rel, body := range theirs {
		got, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("the render removed %s: %v", rel, err)
			continue
		}
		if string(got) != body {
			t.Errorf("%s reads %q, want %q", rel, got, body)
		}
	}
	diff, left, err := p.Diff(out)
	if err != nil || len(diff) > 0 {
		t.Errorf("payload differs from what was written: %v (%v)", diff, err)
	}
	if len(left) != len(theirs) {
		t.Errorf("the check reported %v as left in place, want %d files", left, len(theirs))
	}
}

// Picking a winner where both want the same path is how a settings file
// disappears.
func TestWriteRefusesAPathTheRepositoryAlsoOwns(t *testing.T) {
	p, _, err := planFor(t, "claude", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	out := filepath.Join(t.TempDir(), "claude")
	if _, err := p.Write(out, false); err != nil {
		t.Fatalf("write: %v", err)
	}
	// The marker is rewritten without one of the files the payload produces,
	// which is the state a repository reaches by writing a file the next
	// render also wants.
	held, err := os.ReadFile(filepath.Join(out, marker))
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(string(held)), "\n") {
		if line != "agents/reviewer.md" {
			lines = append(lines, line)
		}
	}
	if err := os.WriteFile(filepath.Join(out, marker), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	theirs := filepath.Join(out, "agents", "reviewer.md")
	if err := os.WriteFile(theirs, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = p.Write(out, false)
	if err == nil {
		t.Fatal("rendered over a file the repository owns")
	}
	if !strings.Contains(err.Error(), "agents/reviewer.md") {
		t.Errorf("the refusal does not name the file: %v", err)
	}
	body, err := os.ReadFile(theirs)
	if err != nil || !strings.Contains(string(body), "mine") {
		t.Errorf("the refusal replaced the file anyway: %q (%v)", body, err)
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
	_, err = p.Write(out, false)
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

// A repository that installed by copying has skills, commands and scripts from
// whatever version it copied, and no marker saying which files those are.
func TestAdoptTakesOverADirectoryCarryingNoMarker(t *testing.T) {
	p, _, err := planFor(t, "claude", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	out := filepath.Join(t.TempDir(), "claude")
	copied := map[string]string{
		"agents/reviewer.md": "an older copy\n",
		"scripts/legacy.py":  "#!/usr/bin/env python3\n",
		"settings.json":      "{\"mine\": true}\n",
		"worktrees/one/file": "a checkout\n",
	}
	for rel, body := range copied {
		full := filepath.Join(out, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := p.Write(out, false); err == nil {
		t.Fatal("rendered over a directory carrying no marker")
	}

	report, err := p.Adopt(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(report.Replace, "\n") != "agents/reviewer.md" {
		t.Errorf("adoption would replace %v, want the one file the payload writes", report.Replace)
	}
	// settings.json is a seed: the payload provides it once and the
	// repository owns it, permissions block and all.
	want := "scripts/legacy.py\nsettings.json\nworktrees/one/file"
	if strings.Join(report.Leave, "\n") != want {
		t.Errorf("adoption would leave %v, want the older script, the settings and the worktree", report.Leave)
	}

	kept, err := p.Write(out, true)
	if err != nil {
		t.Fatalf("adopt: %v", err)
	}
	if strings.Join(kept, "\n") != want {
		t.Errorf("kept %v, want what adoption said it would leave", kept)
	}
	if body := string(body(t, p, "agents/reviewer.md")); body == copied["agents/reviewer.md"] {
		t.Error("the older copy survived the adoption")
	}
	if got, err := os.ReadFile(filepath.Join(out, "scripts", "legacy.py")); err != nil || string(got) != copied["scripts/legacy.py"] {
		t.Errorf("a file the render does not write was removed: %q (%v)", got, err)
	}
	// The marker is what makes the next render ordinary.
	diff, left, err := p.Diff(out)
	if err != nil || len(diff) > 0 {
		t.Errorf("the adopted directory differs from the payload: %v (%v)", diff, err)
	}
	if strings.Join(left, "\n") != want {
		t.Errorf("the check reports %v as left in place, want the three files", left)
	}
	if got, err := os.ReadFile(filepath.Join(out, "settings.json")); err != nil || string(got) != copied["settings.json"] {
		t.Errorf("the repository's settings were replaced: %q (%v)", got, err)
	}
	if _, err := p.Write(out, false); err != nil {
		t.Errorf("a render after adoption needed the flag again: %v", err)
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
	if _, err := p.Write(out, false); err == nil {
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
	if _, err := p.Write(out, false); err != nil {
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
	if _, err := next.Write(out, false); err == nil {
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
	if _, err := p.Write(link, false); err != nil {
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
	if _, err := p.Write(out, false); err != nil {
		t.Fatalf("write: %v", err)
	}
	script := filepath.Join(out, "scripts", "check.py")
	if err := os.Chmod(script, 0o644); err != nil {
		t.Fatal(err)
	}
	diff, _, err := p.Diff(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff) != 1 || !strings.Contains(diff[0], "scripts/check.py") {
		t.Fatalf("a dropped executable bit went unreported: %v", diff)
	}
	if _, err := p.Write(out, false); err != nil {
		t.Fatalf("re-render: %v", err)
	}
	if diff, _, _ := p.Diff(out); len(diff) > 0 {
		t.Errorf("a re-render did not repair the mode: %v", diff)
	}
}

func TestDiffNamesStaleAndUnexpectedFiles(t *testing.T) {
	p, _, err := planFor(t, "claude", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	out := filepath.Join(t.TempDir(), "claude")
	if _, err := p.Write(out, false); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, "agents", "reviewer.md"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "agents", "extra.md"), []byte("added\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	diff, left, err := p.Diff(out)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(diff, "\n")
	if !strings.Contains(joined, "stale: agents/reviewer.md") {
		t.Errorf("diff missed an edit:\n%s", joined)
	}
	// The added file is not listed by the marker, so the check reports it as
	// left in place rather than as a difference from the source.
	if strings.Contains(joined, "agents/extra.md") {
		t.Errorf("an unlisted file was read as a difference:\n%s", joined)
	}
	if strings.Join(left, "\n") != "agents/extra.md" {
		t.Errorf("left in place is %v, want the added file", left)
	}
}

// A payload directory that lost a file still reports it, because the marker
// lists it and the render no longer writes it.
func TestDiffNamesAMissingPayloadFile(t *testing.T) {
	p, _, err := planFor(t, "claude", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	out := filepath.Join(t.TempDir(), "claude")
	if _, err := p.Write(out, false); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Remove(filepath.Join(out, "agents", "reviewer.md")); err != nil {
		t.Fatal(err)
	}
	diff, _, err := p.Diff(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(diff, "\n") != "missing: agents/reviewer.md" {
		t.Errorf("diff is %v, want the removed file", diff)
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

// The renderer's job is to refuse a payload that would underperform silently.
// A Codex hook matched on tool names Codex does not have installs and never
// fires, which is the same failure one step quieter, so the render says so.
func TestCodexReportsAHookThatWillNotFire(t *testing.T) {
	p, _, err := planFor(t, "codex", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	var found string
	for _, limit := range p.Limits {
		if strings.Contains(limit, "check-documents.sh") {
			found = limit
		}
	}
	if found == "" {
		t.Fatalf("the render claims a gate Codex will not run: %v", p.Limits)
	}
	if !strings.Contains(found, "exec") || !strings.Contains(found, "#18") {
		t.Errorf("the limit does not say why or where it is tracked: %q", found)
	}
}

// Claude Code has the tools the matcher names, so the same hook is not a limit
// there. A report that cried wolf on every target would be skipped.
func TestClaudeReportsNoHookLimit(t *testing.T) {
	p, _, err := planFor(t, "claude", allArtifacts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for _, limit := range p.Limits {
		if strings.Contains(limit, "check-documents.sh") {
			t.Errorf("claude reports a limit it does not have: %q", limit)
		}
	}
}
