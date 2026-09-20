// Package dispatch runs one thunderstorm role as a session of its own.
//
// Claude Code provisions a subagent from a definition under agents/ and Codex
// spawns one, so on those harnesses a run hands over a prompt and receives a
// report. Pi ships no subagents, so the loop starts the session: a pi process
// in a detached tmux pane, in the directory the role owns, writing its
// transcript to disk.
//
// The definitions are the ones Claude Code reads. A definition's tools line is
// a Claude Code allowlist, so what a role may reach on Pi is that list
// translated in tools.go, with every name that has no counterpart reported.
package dispatch

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	thunderstorm "github.com/stormlightlabs/thunderstorm"
)

// Role is one definition: what the session is told to be, and what Claude Code
// would let it reach.
type Role struct {
	Name        string
	Description string
	// Prompt is the definition's body, which becomes the text appended to the
	// session's system prompt.
	Prompt string
	// ClaudeTools is the definition's tools line, in the order it was written.
	ClaudeTools []string
}

// Roles returns every role definition the binary carries, by name.
func Roles() ([]Role, error) {
	entries, err := fs.ReadDir(thunderstorm.Roles, "workflow/agents")
	if err != nil {
		return nil, err
	}
	var out []Role
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		body, err := fs.ReadFile(thunderstorm.Roles, path.Join("workflow/agents", e.Name()))
		if err != nil {
			return nil, err
		}
		role, err := parseRole(string(body))
		if err != nil {
			return nil, fmt.Errorf("role %s: %w", e.Name(), err)
		}
		out = append(out, role)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// LookupRole returns the role named name.
func LookupRole(name string) (Role, error) {
	roles, err := Roles()
	if err != nil {
		return Role{}, err
	}
	for _, r := range roles {
		if r.Name == name {
			return r, nil
		}
	}
	return Role{}, fmt.Errorf("unknown role %q; try one of %s", name, RoleNames())
}

// RoleNames lists every role for help text and error messages.
func RoleNames() string {
	roles, err := Roles()
	if err != nil {
		return ""
	}
	names := make([]string, 0, len(roles))
	for _, r := range roles {
		names = append(names, r.Name)
	}
	return strings.Join(names, ", ")
}

// parseRole reads the frontmatter a harness reads and keeps the rest as the
// prompt. The frontmatter here is three flat string keys, so a YAML parser
// would buy nothing; a key it cannot see is an error rather than a default,
// because a role dispatched without its tools line would reach everything.
func parseRole(text string) (Role, error) {
	const fence = "---\n"
	rest, ok := strings.CutPrefix(text, fence)
	if !ok {
		return Role{}, fmt.Errorf("no frontmatter")
	}
	head, body, ok := strings.Cut(rest, "\n"+fence)
	if !ok {
		return Role{}, fmt.Errorf("frontmatter is not closed")
	}

	var r Role
	for _, line := range strings.Split(head, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return Role{}, fmt.Errorf("line %q is not a key", line)
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "name":
			r.Name = value
		case "description":
			r.Description = value
		case "tools":
			for _, t := range strings.Split(value, ",") {
				if t = strings.TrimSpace(t); t != "" {
					r.ClaudeTools = append(r.ClaudeTools, t)
				}
			}
		}
	}
	switch {
	case r.Name == "":
		return Role{}, fmt.Errorf("no name")
	case r.Description == "":
		return Role{}, fmt.Errorf("no description")
	case len(r.ClaudeTools) == 0:
		return Role{}, fmt.Errorf("no tools line")
	}
	r.Prompt = strings.TrimSpace(body)
	if r.Prompt == "" {
		return Role{}, fmt.Errorf("no body")
	}
	return r, nil
}
