// Package settings merges what a payload needs into a repository's Claude Code
// settings file.
//
// A plugin install carries its own permissions and hook registrations. A
// rendered payload carries neither: the deny rules live in a settings file the
// repository owns, and hooks/hooks.json is read by a plugin loader that a
// rendered payload never reaches. Both were a paragraph on the install page
// asking a person to edit JSON by hand, which is the kind of step that is
// skipped once and then missing for a year.
//
// Everything here is idempotent: an install and an update run the same merge,
// and a second run adds nothing.
package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// schema is what an editor reads the file against. It is written only into a
// file this package creates.
const schema = "https://json.schemastore.org/claude-code-settings.json"

// Hook is one registration: which event runs the command, which tools it
// covers, and how long the harness waits for an answer.
type Hook struct {
	Event   string
	Matcher string
	Command string
	Timeout int
}

// Change is what a merge would do to one settings file. Reading and writing
// are separate so that a caller can report the change without making it.
type Change struct {
	File  string
	Deny  []string
	Hooks []Hook

	body    map[string]any
	created bool
}

// Plan reads a settings file and works out what the payload still needs from
// it. A file that is not there is planned as one this package creates.
func Plan(file string, deny []string, hooks []Hook) (Change, error) {
	c := Change{File: file}

	raw, err := os.ReadFile(file)
	switch {
	case os.IsNotExist(err):
		c.created = true
		c.body = map[string]any{"$schema": schema}
	case err != nil:
		return c, fmt.Errorf("read %s: %w", file, err)
	default:
		if err := json.Unmarshal(raw, &c.body); err != nil {
			return c, fmt.Errorf("parse %s: %w", file, err)
		}
		if c.body == nil {
			c.body = map[string]any{}
		}
	}

	held := heldDeny(c.body)
	for _, rule := range deny {
		if !slices.Contains(held, rule) {
			c.Deny = append(c.Deny, rule)
		}
	}
	for _, h := range hooks {
		if !registered(c.body, h) {
			c.Hooks = append(c.Hooks, h)
		}
	}
	return c, nil
}

// Empty reports whether the file already carries everything the payload needs.
func (c Change) Empty() bool { return len(c.Deny) == 0 && len(c.Hooks) == 0 }

// Summary says what the change adds, one line each.
func (c Change) Summary() []string {
	var out []string
	for _, rule := range c.Deny {
		out = append(out, "denies "+rule)
	}
	for _, h := range c.Hooks {
		out = append(out, fmt.Sprintf("runs the gate on %s %s", h.Event, h.Matcher))
	}
	return out
}

// Created reports whether Apply would write a file where there is none.
func (c Change) Created() bool { return c.created }

// Apply writes the merged file, leaving every key it did not come for as it
// found it.
//
// The file is rewritten rather than patched, so a key's position may move. Its
// content does not: what a repository put there is decoded and re-encoded, and
// only the deny list and the hook registrations gain entries.
func (c Change) Apply() error {
	if c.Empty() {
		return nil
	}
	body := c.body
	if len(c.Deny) > 0 {
		permissions := object(body, "permissions")
		deny := append(heldDeny(body), c.Deny...)
		permissions["deny"] = deny
		body["permissions"] = permissions
	}
	for _, h := range c.Hooks {
		register(body, h)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(body); err != nil {
		return fmt.Errorf("encode %s: %w", c.File, err)
	}
	if err := os.MkdirAll(filepath.Dir(c.File), 0o755); err != nil {
		return err
	}
	return write(c.File, buf.Bytes())
}

// write puts the body in place through a temporary file beside it, so an
// interrupted write cannot leave a repository with half a settings file.
func write(file string, body []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(file), "."+filepath.Base(file)+".tstorm-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		return err
	}
	return os.Rename(name, file)
}

// heldDeny reads permissions.deny, tolerating every shape a hand-edited file
// can be in: a missing block, a null, or a list holding something that is not
// a string.
func heldDeny(body map[string]any) []string {
	permissions, ok := body["permissions"].(map[string]any)
	if !ok {
		return nil
	}
	held, ok := permissions["deny"].([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, rule := range held {
		if s, ok := rule.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// registered reports whether the file already runs this command on this event
// and matcher.
func registered(body map[string]any, h Hook) bool {
	hooks, ok := body["hooks"].(map[string]any)
	if !ok {
		return false
	}
	entries, ok := hooks[h.Event].([]any)
	if !ok {
		return false
	}
	for _, entry := range entries {
		e, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if matcher, _ := e["matcher"].(string); matcher != h.Matcher {
			continue
		}
		inner, ok := e["hooks"].([]any)
		if !ok {
			continue
		}
		for _, one := range inner {
			o, ok := one.(map[string]any)
			if !ok {
				continue
			}
			if command, _ := o["command"].(string); command == h.Command {
				return true
			}
		}
	}
	return false
}

// register adds one hook, keeping any other registration on the same event.
func register(body map[string]any, h Hook) {
	hooks := object(body, "hooks")
	entry := map[string]any{"type": "command", "command": h.Command}
	if h.Timeout > 0 {
		entry["timeout"] = h.Timeout
	}
	entries, _ := hooks[h.Event].([]any)
	for i, held := range entries {
		e, ok := held.(map[string]any)
		if !ok {
			continue
		}
		if matcher, _ := e["matcher"].(string); matcher != h.Matcher {
			continue
		}
		inner, _ := e["hooks"].([]any)
		e["hooks"] = append(inner, entry)
		entries[i] = e
		hooks[h.Event] = entries
		body["hooks"] = hooks
		return
	}
	held := map[string]any{"hooks": []any{entry}}
	if h.Matcher != "" {
		held["matcher"] = h.Matcher
	}
	hooks[h.Event] = append(entries, held)
	body["hooks"] = hooks
}

// object returns the map at key, replacing whatever is there when it is not
// one. A settings file holding a string where the schema wants an object is
// already broken; the merge does not preserve the break.
func object(body map[string]any, key string) map[string]any {
	held, ok := body[key].(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return held
}
