package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Target is one harness: what it provides, where its payload puts each kind of
// artifact, and what the prose should call its resource directory.
//
// Everything here comes from docs/internal/hosts.md, verified on 2026-09-19
// against Claude Code, pi 0.85.1, and codex-cli 0.146.0. A harness the fixture
// did not cover provides nothing, which is not a claim that it cannot.
//
// Commands render for every harness that has somewhere to read one. Where a
// harness reads them from a user directory rather than from the payload,
// docs/internal/using-thunderstorm.md carries the one copy that install takes.
type Target struct {
	Name string
	// Root replaces {{ROOT}} in rendered prose: the harness's own directory in
	// the repository being worked on. Plugin replaces {{PLUGIN}}: where the
	// installed payload sits, which is not inside that repository at all once
	// the payload is installed rather than copied. Plugin is empty for a
	// harness whose install layout has not been verified, and a source naming
	// {{PLUGIN}} then stops that target's render rather than guessing.
	Root   string
	Plugin string
	// pluginFor overrides Plugin when an artifact's location inside a package
	// changes the relative path back to the package root.
	pluginFor func(Artifact) string

	provides map[string]bool
	dirs     map[Kind]string
	// why explains an unmet capability or an unplaced kind, and names the
	// issue that would close it. Its text is what an operator reads.
	why    map[string]string
	extras func(Manifest, []Artifact) ([]File, error)
	// limits reports what the payload carries here without working, where the
	// render goes ahead anyway. An artifact the target cannot carry stops the
	// render; one it carries and cannot fire is a line in the report.
	limits func([]Artifact) []string
	// transform rewrites a source artifact when the harness reads a different
	// file format. Most targets copy source bytes unchanged.
	transform func(Artifact, []byte) ([]byte, error)
}

// Targets returns the harnesses render knows, in the order they are supported.
func Targets() []*Target {
	return []*Target{claudeTarget(), codexTarget(), piTarget(), cursorTarget()}
}

// Lookup returns the target named name.
func Lookup(name string) (*Target, error) {
	for _, t := range Targets() {
		if t.Name == name {
			return t, nil
		}
	}
	return nil, fmt.Errorf("unknown target %q; try one of %s", name, TargetNames())
}

// TargetNames lists every target name for help text and error messages.
func TargetNames() string {
	out := ""
	for i, t := range Targets() {
		if i > 0 {
			out += ", "
		}
		out += t.Name
	}
	return out
}

// claudePluginRoot is what Claude Code expands to the installed payload's
// directory, in a hook command and in rendered prose alike.
const claudePluginRoot = "${CLAUDE_PLUGIN_ROOT}"

// codexPluginRoot is what Codex expands to the installed plugin's directory in
// a hook command. It is not the "\.\." the skills resolve against, which is a
// path relative to the file naming it rather than a value the runtime expands.
const codexPluginRoot = "${PLUGIN_ROOT}"

func claudeTarget() *Target {
	return &Target{
		Name:   "claude",
		Root:   ".claude",
		Plugin: claudePluginRoot,
		provides: map[string]bool{
			CapSkills: true, CapCommands: true, CapSubagents: true,
			CapHooks: true, CapScripts: true, CapPermissions: true,
		},
		dirs: map[Kind]string{
			KindSkill: "skills", KindCommand: "commands", KindAgent: "agents",
			KindHook: "hooks", KindScript: "scripts",
		},
		extras: claudeExtras,
	}
}

func codexTarget() *Target {
	return &Target{
		Name:   "codex",
		Root:   ".codex",
		Plugin: "..",
		provides: map[string]bool{
			CapSkills: true, CapCommands: true, CapSubagents: true,
			CapHooks: true, CapScripts: true, CapPermissions: true,
		},
		dirs: map[Kind]string{
			KindSkill: "skills", KindCommand: "prompts", KindAgent: "agents",
			KindHook: "hooks", KindScript: "scripts",
		},
		extras:    codexExtras,
		limits:    codexLimits,
		transform: codexTransform,
		pluginFor: func(a Artifact) string {
			if a.Kind == KindSkill {
				return "../.."
			}
			return ".."
		},
	}
}

func piTarget() *Target {
	return &Target{
		Name:   "pi",
		Root:   ".pi",
		Plugin: "${THUNDERSTORM_PLUGIN_ROOT}",
		provides: map[string]bool{
			CapSkills: true, CapCommands: true, CapSubagents: true,
			CapHooks: true, CapScripts: true, CapPermissions: true,
		},
		dirs: map[Kind]string{
			KindSkill: "skills", KindCommand: "prompts", KindAgent: "roles",
			KindHook: "hooks", KindScript: "scripts",
		},
		extras: piExtras,
	}
}

func cursorTarget() *Target {
	unverified := "Cursor's host contract is unverified: no Cursor agent was installed when hosts.md was " +
		"written, so nothing is claimed about where it reads anything; see #13"
	why := map[string]string{}
	for _, c := range capabilities {
		why[c] = unverified
	}
	for _, k := range kinds {
		why[string(k)] = unverified
	}
	return &Target{Name: "cursor", Root: ".cursor", provides: map[string]bool{}, why: why}
}

// unmet returns why the target cannot carry the artifact, or an empty slice.
// A missing capability and a missing place for the kind are usually the same
// gap said twice, so each reason is reported once.
func (t *Target) unmet(a Artifact) []string {
	var out []string
	seen := map[string]bool{}
	add := func(reason string) {
		if !seen[reason] {
			seen[reason] = true
			out = append(out, reason)
		}
	}
	for _, c := range a.Requires {
		if !t.provides[c] {
			add(t.reason(c))
		}
	}
	if _, ok := t.dirs[a.Kind]; !ok {
		add(t.reason(string(a.Kind)))
	}
	return out
}

func (t *Target) reason(key string) string {
	if why, ok := t.why[key]; ok {
		return why
	}
	return fmt.Sprintf("target table records no reason for %s", key)
}

func (t *Target) pluginRoot(a Artifact) string {
	if t.pluginFor != nil {
		return t.pluginFor(a)
	}
	return t.Plugin
}

// claudeExtras writes the three files Claude Code needs that are not copied
// from the source.
//
// hooks/hooks.json is what a plugin install reads; a settings.json in a
// payload is read by nothing, which is why the first install reported no
// hooks at all. It is written only when the workflow has a hook to register,
// which it does not today. settings.json stays for the deny rules, which no
// plugin mechanism can carry: a repository merges them into its own settings,
// and docs/src/content/docs/start/install.md says so.
func claudeExtras(m Manifest, present []Artifact) ([]File, error) {
	plugin, err := marshal(map[string]any{
		"name":        m.Name,
		"version":     m.Version,
		"description": m.Description,
		"author":      m.Author,
	})
	if err != nil {
		return nil, err
	}

	events := hookEvents(present, claudePluginRoot+"/hooks/")

	deny := make([]string, 0, len(m.Policy.Deny))
	for _, cmd := range m.Policy.Deny {
		deny = append(deny, fmt.Sprintf("Bash(%s:*)", cmd))
	}
	settings, err := marshal(map[string]any{
		"$schema":     "https://json.schemastore.org/claude-code-settings.json",
		"permissions": map[string]any{"deny": deny},
	})
	if err != nil {
		return nil, err
	}

	files := []File{
		{Path: ".claude-plugin/plugin.json", Body: plugin},
		{Path: "settings.json", Body: settings, Seed: true},
	}
	if len(events) > 0 {
		hooks, err := marshal(map[string]any{"hooks": events})
		if err != nil {
			return nil, err
		}
		files = append(files, File{Path: "hooks/hooks.json", Body: hooks})
	}
	return files, nil
}

// hookCommand is one command a harness runs for an event. Claude Code and
// Codex read the same three fields.
type hookCommand struct {
	Type          string `json:"type"`
	Command       string `json:"command"`
	Timeout       int    `json:"timeout,omitempty"`
	StatusMessage string `json:"statusMessage,omitempty"`
}

// hookRegistration groups the commands one matcher runs.
type hookRegistration struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []hookCommand `json:"hooks"`
}

// hookEvents turns the manifest's hook artifacts into the registration both
// harnesses read, with each command written against the directory prefix that
// harness expands to the installed payload.
func hookEvents(present []Artifact, prefix string) map[string][]hookRegistration {
	events := map[string][]hookRegistration{}
	for _, a := range present {
		if a.Kind != KindHook {
			continue
		}
		for _, e := range a.Events {
			events[e.Event] = append(events[e.Event], hookRegistration{
				Matcher: e.Matcher,
				Hooks: []hookCommand{{
					Type:    "command",
					Command: prefix + a.Name,
					Timeout: e.Timeout,
				}},
			})
		}
	}
	return events
}

// codexExtras packages the workflow for Codex and writes its command policy
// twice. The plugin hook applies while the plugin is enabled. The execpolicy
// file remains available to repositories that also want the policy outside a
// Thunderstorm session.
//
// Verified with codex execpolicy check on codex-cli 0.146.0, 2026-09-19.
func codexExtras(m Manifest, present []Artifact) ([]File, error) {
	var b strings.Builder
	b.WriteString("# thunderstorm: a session does not merge or approve its own work.\n")
	b.WriteString("#\n")
	b.WriteString("# The plugin hook applies this policy while Thunderstorm is enabled.\n")
	b.WriteString("# Copy this file into a Codex rules directory only when the same policy\n")
	b.WriteString("# should apply outside Thunderstorm.\n")
	for _, cmd := range m.Policy.Deny {
		fields := strings.Fields(cmd)
		quoted := make([]string, 0, len(fields))
		for _, f := range fields {
			quoted = append(quoted, fmt.Sprintf("%q", f))
		}
		b.WriteString("\nprefix_rule(\n")
		fmt.Fprintf(&b, "    pattern = [%s],\n", strings.Join(quoted, ", "))
		b.WriteString("    decision = \"forbidden\",\n")
		fmt.Fprintf(&b, "    justification = %q,\n", "only a human merges or approves a thunderstorm run")
		b.WriteString(")\n")
	}
	compatibility, err := marshal(map[string]any{
		"name":        m.Name,
		"version":     m.Version,
		"description": m.Description,
		"author":      m.Author,
		"skills":      "./skills/",
		"interface": map[string]any{
			"displayName":      "Thunderstorm",
			"shortDescription": "Run issues through implementation and review.",
			"longDescription":  m.Description,
			"developerName":    m.Author.Name,
			"category":         "Developer Tools",
			"capabilities":     []string{"Read", "Write"},
			"defaultPrompt":    []string{"Run an issue and its sub-issues with Thunderstorm."},
		},
	})
	if err != nil {
		return nil, err
	}
	portable, err := marshal(map[string]any{
		"$schema":     "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		"name":        m.Name,
		"version":     m.Version,
		"description": m.Description,
		"author":      m.Author,
		"repository":  "https://github.com/stormlightlabs/thunderstorm",
		"license":     "Apache-2.0",
		"keywords":    []string{"development", "review", "workflow"},
	})
	if err != nil {
		return nil, err
	}
	denied, err := json.Marshal(m.Policy.Deny)
	if err != nil {
		return nil, err
	}
	// The workflow's own hooks register beside the command policy, which is
	// Codex's alone: the policy exists because Codex has no deny setting for a
	// repository to carry, and no other harness renders it.
	events := hookEvents(present, codexPluginRoot+"/hooks/")
	events["PreToolUse"] = append(events["PreToolUse"], hookRegistration{
		Matcher: "Bash",
		Hooks: []hookCommand{{
			Type:          "command",
			Command:       "python3 " + codexPluginRoot + "/hooks/deny-command.py",
			Timeout:       5,
			StatusMessage: "Checking Thunderstorm command policy",
		}},
	})
	hooks, err := marshal(map[string]any{"hooks": events})
	if err != nil {
		return nil, err
	}
	denyHook := `#!/usr/bin/env python3
import json
import re
import sys

DENIED = ` + string(denied) + `


def command_pattern(prefix):
    words = [r'''["']?''' + re.escape(word) + r'''["']?''' for word in prefix.split()]
    return re.compile(r'''(^|[;&|()\n]\s*)''' + r'''\s+'''.join(words) + r'''(?=\s|$|[;&|()])''')


try:
    event = json.load(sys.stdin)
    command = event.get("tool_input", {}).get("command", "")
except (AttributeError, json.JSONDecodeError):
    print("Thunderstorm could not inspect the shell command.", file=sys.stderr)
    raise SystemExit(2)
if not isinstance(command, str):
    print("Thunderstorm received a shell command in an unknown format.", file=sys.stderr)
    raise SystemExit(2)
for prefix in DENIED:
    if command_pattern(prefix).search(command):
        json.dump({
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "deny",
                "permissionDecisionReason": prefix + " is reserved for a human in a Thunderstorm run.",
            }
        }, sys.stdout)
        break
`
	return []File{
		{Path: ".codex-plugin/plugin.json", Body: compatibility},
		{Path: "hooks/deny-command.py", Body: []byte(denyHook), Mode: 0o755},
		{Path: "hooks/hooks.json", Body: hooks},
		{Path: "plugin.json", Body: portable},
		{Path: "rules/thunderstorm.rules", Body: []byte(b.String())},
	}, nil
}

// codexLimits reports the hooks Codex registers and will not run.
//
// Codex has no Write or Edit tool. A session writes through `exec`, so a
// PostToolUse matcher written in Claude Code's tool names never fires there,
// and the registration installs and does nothing. Verified against a real
// install on 2026-09-20; docs/internal/hosts.md holds the session it came
// from, and #18 is where the extraction that fixes it goes.
func codexLimits(present []Artifact) []string {
	var out []string
	for _, a := range present {
		if a.Kind != KindHook {
			continue
		}
		for _, e := range a.Events {
			if e.Event != "PostToolUse" {
				continue
			}
			if strings.Contains(e.Matcher, "Write") || strings.Contains(e.Matcher, "Edit") {
				out = append(out, fmt.Sprintf(
					"%s answers %s on %s, which Codex never sends: it writes through exec; see #18",
					a.Name, e.Event, e.Matcher))
			}
		}
	}
	return out
}

// codexTransform gives plugin skills relative paths and turns each shared role
// into TOML the orchestrator can pass to a built-in Codex agent. The same TOML
// also works as a project or personal custom-agent file.
func codexTransform(a Artifact, body []byte) ([]byte, error) {
	if a.Kind == KindSkill && bytes.Contains(body, []byte("../../")) {
		return addCodexPathNote(body), nil
	}
	if a.Kind != KindAgent {
		return body, nil
	}
	fields, prompt, err := roleParts(string(body))
	if err != nil {
		return nil, fmt.Errorf("agent %q: %w", a.Name, err)
	}
	sandbox := "read-only"
	for _, tool := range strings.FieldsFunc(fields["tools"], func(r rune) bool { return r == ',' || r == ' ' }) {
		if tool == "Write" || tool == "Edit" {
			sandbox = "workspace-write"
			break
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "name = %s\n", strconv.Quote(fields["name"]))
	fmt.Fprintf(&b, "description = %s\n", strconv.Quote(fields["description"]))
	fmt.Fprintf(&b, "sandbox_mode = %s\n", strconv.Quote(sandbox))
	prompt = "Resolve paths that start with `../` from the directory containing this role file.\n\n" + prompt
	fmt.Fprintf(&b, "developer_instructions = %s\n", strconv.Quote(prompt))
	return []byte(b.String()), nil
}

func addCodexPathNote(body []byte) []byte {
	const note = "Resolve paths that start with `../../` from the directory containing this `SKILL.md`.\n"
	text := string(body)
	if strings.HasPrefix(text, "---\n") {
		if end := strings.Index(text[4:], "\n---\n"); end >= 0 {
			at := end + 9
			return []byte(text[:at] + note + text[at:])
		}
	}
	return append([]byte(note), body...)
}

func roleParts(text string) (map[string]string, string, error) {
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return nil, "", fmt.Errorf("no frontmatter")
	}
	head, prompt, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return nil, "", fmt.Errorf("frontmatter is not closed")
	}
	fields := map[string]string{}
	for _, line := range strings.Split(head, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok {
			fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	for _, key := range []string{"name", "description", "tools"} {
		if fields[key] == "" {
			return nil, "", fmt.Errorf("no %s", key)
		}
	}
	return fields, strings.TrimSpace(prompt), nil
}

// piExtras makes the payload a Pi package. The extension records its package
// root for the shared prose, refuses the command prefixes in the manifest, and
// runs the workflow's hooks, which Pi has no other way to reach.
func piExtras(m Manifest, present []Artifact) ([]File, error) {
	pkg, err := marshal(map[string]any{
		"name":        m.Name,
		"version":     m.Version,
		"description": m.Description,
		"keywords":    []string{"pi-package"},
		"type":        "module",
		"pi": map[string]any{
			"extensions": []string{"./extensions"},
			"skills":     []string{"./skills"},
			"prompts":    []string{"./prompts"},
		},
	})
	if err != nil {
		return nil, err
	}

	denied, err := json.Marshal(m.Policy.Deny)
	if err != nil {
		return nil, err
	}
	gates, err := json.Marshal(piGates(present))
	if err != nil {
		return nil, err
	}
	extension := `import { spawn } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const denied = ` + string(denied) + `;
const gates = ` + string(gates) + `;

function shellWord(word: string): string {
	const escaped = word.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
	return "[\"']?" + escaped + "[\"']?";
}

function commandPattern(prefix: string): RegExp {
	const words = prefix.split(/\s+/).map(shellWord).join("\\s+");
	return new RegExp("(^|[;&|()\\n]\\s*)" + words + "(?=\\s|$|[;&|()])");
}

// Pi has no hooks, so a write reaches the gate through the same script the
// other two harnesses register, with this handler translating Pi's event into
// the shape that script reads and its reply back into a tool result.
function runGate(root: string, gate: { name: string; timeout: number }, event: object): Promise<string> {
	return new Promise((done) => {
		const child = spawn(resolve(root, "hooks", gate.name), [], { stdio: ["pipe", "pipe", "inherit"] });
		const timer = setTimeout(() => child.kill("SIGKILL"), gate.timeout * 1000);
		let out = "";
		child.stdout.on("data", (chunk) => {
			out += chunk;
		});
		child.on("error", () => {
			clearTimeout(timer);
			done("");
		});
		child.on("close", () => {
			clearTimeout(timer);
			done(out);
		});
		child.stdin.on("error", () => {});
		child.stdin.end(JSON.stringify(event));
	});
}

export default function (pi) {
	const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
	process.env.THUNDERSTORM_PLUGIN_ROOT = root;
	const patterns = denied.map((prefix) => ({ prefix, pattern: commandPattern(prefix) }));
	const tools = (gate) => new RegExp("^(" + gate.matcher + ")$", "i");
	const writeGates = gates
		.filter((gate) => gate.event === "PostToolUse")
		.map((gate) => ({ ...gate, tools: tools(gate) }));
	const commandGates = gates
		.filter((gate) => gate.event === "PreToolUse")
		.map((gate) => ({ ...gate, tools: tools(gate) }));

	pi.on("tool_call", async (event) => {
		if (event.toolName !== "bash") return undefined;
		const command = event.input.command as string;
		const match = patterns.find(({ pattern }) => pattern.test(command));
		if (match) {
			return {
				block: true,
				reason: match.prefix + " is reserved for a human in a thunderstorm run.",
			};
		}
		// Pi blocks or allows, with nothing between, so a gate that asks is
		// read as an allow here and its finding is lost. Only a refusal
		// crosses. See docs/internal/hosts.md.
		const gate = commandGates.find((candidate) => candidate.tools.test("Bash"));
		if (!gate) return undefined;
		const reply = await runGate(root, gate, {
			hook_event_name: "PreToolUse",
			tool_name: "Bash",
			cwd: process.cwd(),
			tool_input: event.input,
		});
		if (!reply.trim()) return undefined;
		let answer;
		try {
			answer = JSON.parse(reply)?.hookSpecificOutput;
		} catch {
			return undefined;
		}
		if (answer?.permissionDecision !== "deny") return undefined;
		return { block: true, reason: answer.permissionDecisionReason };
	});

	pi.on("tool_result", async (event) => {
		const gate = writeGates.find((candidate) => candidate.tools.test(event.toolName));
		if (!gate) return undefined;
		const reply = await runGate(root, gate, {
			hook_event_name: "PostToolUse",
			tool_name: event.toolName,
			cwd: process.cwd(),
			tool_input: event.input,
		});
		if (!reply.trim()) return undefined;
		let context: string | undefined;
		try {
			context = JSON.parse(reply)?.hookSpecificOutput?.additionalContext;
		} catch {
			return undefined;
		}
		if (!context) return undefined;
		return { content: [...event.content, { type: "text", text: context }] };
	});
}
`
	return []File{
		{Path: "extensions/thunderstorm.ts", Body: []byte(extension)},
		{Path: "package.json", Body: pkg},
	}, nil
}

// piGates is what the extension needs to run each hook artifact: the file to
// spawn, the event it answers, the tools it covers, and how long to wait.
func piGates(present []Artifact) []map[string]any {
	out := []map[string]any{}
	for _, a := range present {
		if a.Kind != KindHook {
			continue
		}
		for _, e := range a.Events {
			out = append(out, map[string]any{
				"name": a.Name, "event": e.Event, "matcher": e.Matcher, "timeout": e.Timeout,
			})
		}
	}
	return out
}

func marshal(v any) ([]byte, error) {
	body, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

// sortedKinds keeps a render's report in one order whatever the manifest's is.
func sortedKinds(counts map[Kind]int) []Kind {
	out := make([]Kind, 0, len(counts))
	for k := range counts {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
