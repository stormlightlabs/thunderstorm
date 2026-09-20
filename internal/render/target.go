package render

import (
	"encoding/json"
	"fmt"
	"sort"
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

	provides map[string]bool
	dirs     map[Kind]string
	// why explains an unmet capability or an unplaced kind, and names the
	// issue that would close it. Its text is what an operator reads.
	why    map[string]string
	extras func(Manifest, []Artifact) ([]File, error)
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

func claudeTarget() *Target {
	return &Target{
		Name:   "claude",
		Root:   ".claude",
		Plugin: claudePluginRoot,
		provides: map[string]bool{
			CapSkills: true, CapCommands: true, CapSubagents: true,
			CapHooks: true, CapScripts: true, CapDenyRules: true,
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
		Name: "codex",
		Root: ".codex",
		provides: map[string]bool{
			CapSkills: true, CapCommands: true, CapSubagents: true, CapHooks: true, CapScripts: true,
		},
		dirs: map[Kind]string{
			KindSkill: "skills", KindCommand: "prompts", KindHook: "hooks", KindScript: "scripts",
		},
		why: map[string]string{
			string(KindAgent): "Codex dispatches a skill carrying agents/openai.yaml rather than an agent file; " +
				"writing that sidecar is #8",
			CapDenyRules: "merge denial is a Claude Code setting today; see #9",
		},
	}
}

func piTarget() *Target {
	return &Target{
		Name:     "pi",
		Root:     ".pi",
		provides: map[string]bool{CapSkills: true, CapCommands: true},
		dirs:     map[Kind]string{KindSkill: "skills", KindCommand: "prompts"},
		why: map[string]string{
			CapSubagents: "Pi has no subagent mechanism; a role runs there as a pi session in a tmux pane, " +
				"which tstorm dispatch starts from a copy of the definitions inside the binary",
			string(KindAgent): "Pi has no subagent mechanism; a role runs there as a pi session in a tmux pane, " +
				"which tstorm dispatch starts from a copy of the definitions inside the binary",
			CapHooks:           "Pi's only hook equivalent is a TypeScript extension; see #18",
			string(KindHook):   "Pi's only hook equivalent is a TypeScript extension; see #18",
			CapScripts:         "a pi package has no slot for the check scripts; they move into tstorm in #16",
			string(KindScript): "a pi package has no slot for the check scripts; they move into tstorm in #16",
			CapDenyRules:       "merge denial is a Claude Code setting today; see #9",
		},
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
	return "no reason recorded, which is itself a gap in the target table"
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

	type command struct {
		Type    string `json:"type"`
		Command string `json:"command"`
		Timeout int    `json:"timeout,omitempty"`
	}
	type registration struct {
		Matcher string    `json:"matcher,omitempty"`
		Hooks   []command `json:"hooks"`
	}
	events := map[string][]registration{}
	for _, a := range present {
		if a.Kind != KindHook {
			continue
		}
		events[a.Event] = append(events[a.Event], registration{
			Matcher: a.Matcher,
			Hooks: []command{{
				Type:    "command",
				Command: claudePluginRoot + "/hooks/" + a.Name,
				Timeout: a.Timeout,
			}},
		})
	}

	settings, err := marshal(map[string]any{
		"$schema":     "https://json.schemastore.org/claude-code-settings.json",
		"permissions": map[string]any{"deny": m.Policy.Deny},
	})
	if err != nil {
		return nil, err
	}

	files := []File{
		{Path: ".claude-plugin/plugin.json", Body: plugin},
		{Path: "settings.json", Body: settings},
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
