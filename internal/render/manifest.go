// Package render builds one harness payload from the canonical workflow source.
//
// Three harnesses read skills from three directories, name commands three
// ways, and mean different things by a subagent. The source under workflow/
// is written once; a manifest says what each artifact is and which harness
// capabilities it needs; a target says what its harness provides and where the
// files go. A requirement no target can meet stops the render, because a
// payload that installs and then silently skips the review fan-out is worse
// than no payload at all.
package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
)

// Kind is what an artifact is, which decides where a target puts it.
type Kind string

const (
	KindSkill   Kind = "skill"
	KindCommand Kind = "command"
	KindAgent   Kind = "agent"
	KindHook    Kind = "hook"
	KindScript  Kind = "script"
)

var kinds = []Kind{KindSkill, KindCommand, KindAgent, KindHook, KindScript}

// Capabilities a manifest may require of a harness.
const (
	CapSkills    = "skills"
	CapCommands  = "commands"
	CapSubagents = "subagents"
	CapHooks     = "hooks"
	CapScripts   = "scripts"
	// CapPermissions is a mechanism that refuses a command. Each harness
	// spells it differently, and Pi has none.
	CapPermissions = "permissions"
)

var capabilities = []string{CapSkills, CapCommands, CapSubagents, CapHooks, CapScripts, CapPermissions}

// Manifest is the canonical description of the workflow: one entry per
// artifact, plus the permission policy a harness is asked to enforce.
type Manifest struct {
	Name        string     `json:"name"`
	Version     string     `json:"version"`
	Description string     `json:"description"`
	Author      Author     `json:"author"`
	Policy      Policy     `json:"policy"`
	Artifacts   []Artifact `json:"artifacts"`
}

// Author identifies who publishes the payload. Every harness manifest wants it.
type Author struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// Policy is the permission rules the workflow depends on rather than merely
// prefers: the ones that keep an agent from approving or merging its own work.
// See docs/internal/thunderstorm.md.
//
// Deny lists command prefixes as a person would type them. A target
// translates them into what its own harness reads; one with no permission
// mechanism says so instead.
//
// Deny only. An allow rule is a literal command prefix, and where an installed
// payload puts its scripts is not knowable when the manifest is written, so a
// rendered allow list would be wrong on every machine that installed it.
type Policy struct {
	Deny []string `json:"deny"`
}

// Artifact is one file or directory of the source, with the capabilities a
// harness must provide before it means anything there.
type Artifact struct {
	Kind     Kind     `json:"kind"`
	Name     string   `json:"name"`
	Source   string   `json:"source"`
	Requires []string `json:"requires"`

	// Executable is the execute bit the payload writes. It is a manifest
	// field rather than the source file's own mode because a binary carries
	// the workflow through embed, which reports every file read-only, and
	// because a checkout's modes are whatever git and the umask agreed on.
	Executable bool `json:"executable,omitempty"`

	// Hooks only: the events that run this one, and how the harness is told
	// to wait for each. A harness registers hooks in its own settings file,
	// so the manifest has to carry what that registration needs. One script
	// answers several events, because tstorm picks the gate by event name.
	Events []HookEvent `json:"events,omitempty"`
}

// HookEvent is one registration: the event, the tools it covers, and the
// seconds the harness waits for an answer.
type HookEvent struct {
	Event   string `json:"event"`
	Matcher string `json:"matcher,omitempty"`
	Timeout int    `json:"timeout,omitempty"`
}

// mode is the permission the payload writes this artifact with.
func (a Artifact) mode() fs.FileMode {
	if a.Executable {
		return 0o755
	}
	return 0o644
}

// Load reads the manifest at the source's root and checks it against the tree
// beside it, so a typo in a path is an error here rather than a file missing
// from a payload.
func Load(src Source) (Manifest, error) {
	var m Manifest
	raw, err := fs.ReadFile(src.FS, "manifest.json")
	if err != nil {
		return m, fmt.Errorf("read manifest from %s: %w", src.Name, err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return m, fmt.Errorf("parse manifest: %w", err)
	}
	// A second document after the first is a bad merge, not a manifest.
	if dec.More() {
		return m, fmt.Errorf("parse manifest: more than one JSON document")
	}
	if err := m.validate(src); err != nil {
		return m, err
	}
	return m, nil
}

func (m Manifest) validate(src Source) error {
	if m.Name == "" || m.Version == "" || m.Description == "" {
		return fmt.Errorf("manifest needs a name, a version, and a description")
	}
	if len(m.Artifacts) == 0 {
		return fmt.Errorf("manifest lists no artifacts")
	}
	seen := map[string]string{}
	for _, a := range m.Artifacts {
		switch {
		case !slices.Contains(kinds, a.Kind):
			return fmt.Errorf("artifact %q: unknown kind %q", a.Name, a.Kind)
		case a.Name == "":
			return fmt.Errorf("artifact with source %q has no name", a.Source)
		case a.Source == "":
			return fmt.Errorf("artifact %q has no source", a.Name)
		case len(a.Requires) == 0:
			return fmt.Errorf("artifact %q requires nothing; say which capability it needs", a.Name)
		case a.Executable && a.Kind != KindHook && a.Kind != KindScript:
			return fmt.Errorf("artifact %q is a %s; only a hook or a script is executable", a.Name, a.Kind)
		case a.Kind == KindHook && len(a.Events) == 0:
			return fmt.Errorf("hook %q has no events; a harness cannot register it", a.Name)
		case a.Kind != KindHook && len(a.Events) > 0:
			return fmt.Errorf("artifact %q is a %s, which registers no events", a.Name, a.Kind)
		}
		// The name becomes a path inside the payload, so a name carrying a
		// separator or a dot segment writes outside it.
		if n := a.Name; n != path.Base(n) || n == "." || n == ".." || strings.ContainsAny(n, `/\`+"\n") {
			return fmt.Errorf("artifact %q: %q is not a usable file name", a.Name, n)
		}
		for _, e := range a.Events {
			if e.Event == "" {
				return fmt.Errorf("hook %q lists a registration with no event", a.Name)
			}
		}
		for _, c := range a.Requires {
			if !slices.Contains(capabilities, c) {
				return fmt.Errorf("artifact %q requires unknown capability %q", a.Name, c)
			}
		}
		key := string(a.Kind) + "/" + a.Name
		if other, dup := seen[key]; dup {
			return fmt.Errorf("%s %q is claimed by both %s and %s", a.Kind, a.Name, other, a.Source)
		}
		seen[key] = a.Source
		if err := checkSource(src, a); err != nil {
			return err
		}
	}
	return nil
}

// checkSource rejects a source that escapes the tree, is missing, or is the
// wrong shape: a skill is a directory holding SKILL.md everywhere, and every
// other kind is a single file.
func checkSource(src Source, a Artifact) error {
	if a.Source != path.Clean(a.Source) || strings.HasPrefix(a.Source, "/") || strings.HasPrefix(a.Source, "..") {
		return fmt.Errorf("artifact %q: source %q must be a relative path inside the source tree", a.Name, a.Source)
	}
	info, err := fs.Stat(src.FS, a.Source)
	if err != nil {
		return fmt.Errorf("artifact %q: %w", a.Name, err)
	}
	if a.Kind == KindSkill {
		if !info.IsDir() {
			return fmt.Errorf("skill %q: %s is not a directory", a.Name, a.Source)
		}
		if _, err := fs.Stat(src.FS, path.Join(a.Source, "SKILL.md")); err != nil {
			return fmt.Errorf("skill %q: %w", a.Name, err)
		}
		return nil
	}
	if info.IsDir() {
		return fmt.Errorf("artifact %q: %s is a directory, but a %s is one file", a.Name, a.Source, a.Kind)
	}
	return nil
}

// files returns the source-relative paths an artifact carries, so a skill's
// references travel with its SKILL.md.
func (a Artifact) files(src Source) ([]string, error) {
	if a.Kind != KindSkill {
		return []string{a.Source}, nil
	}
	var out []string
	err := fs.WalkDir(src.FS, a.Source, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		out = append(out, p)
		return nil
	})
	slices.Sort(out)
	return out, err
}
