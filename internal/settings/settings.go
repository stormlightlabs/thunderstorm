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
// and a second run adds nothing. Plan and Change add what a payload needs;
// PlanRemoval and Removal take exactly that back out, for a repository that
// uninstalls the loop.
package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/stormlightlabs/thunderstorm/internal/jsonc"
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
		if err := json.Unmarshal(jsonc.Strip(raw), &c.body); err != nil {
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
// only the deny list and the hook registrations gain entries. Plan reads a
// comment through jsonc.Strip, but the decoded body carries no memory of one,
// so a comment in the file does not survive this rewrite.
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

// Removal is what taking a payload's deny rules and hook registrations back
// out would do to one settings file. Reading and removing are separate for
// the same reason as Plan and Change: a caller can report the removal before
// making it.
type Removal struct {
	File  string
	Deny  []string
	Hooks []Hook

	body map[string]any
}

// PlanRemoval reads a settings file and works out which of the given deny
// rules and hook registrations it still holds. A file that is not there holds
// none of them, so PlanRemoval reports an empty Removal rather than an error:
// there is nothing to remove from a settings file that was never merged, or
// that a repository has already deleted.
func PlanRemoval(file string, deny []string, hooks []Hook) (Removal, error) {
	r := Removal{File: file}

	raw, err := os.ReadFile(file)
	switch {
	case os.IsNotExist(err):
		return r, nil
	case err != nil:
		return r, fmt.Errorf("read %s: %w", file, err)
	}
	if err := json.Unmarshal(jsonc.Strip(raw), &r.body); err != nil {
		return r, fmt.Errorf("parse %s: %w", file, err)
	}
	if r.body == nil {
		return r, nil
	}

	held := heldDeny(r.body)
	for _, rule := range deny {
		if slices.Contains(held, rule) {
			r.Deny = append(r.Deny, rule)
		}
	}
	for _, h := range hooks {
		if registered(r.body, h) {
			r.Hooks = append(r.Hooks, h)
		}
	}
	return r, nil
}

// Empty reports whether the file already holds none of what Removal was
// asked to take out.
func (r Removal) Empty() bool { return len(r.Deny) == 0 && len(r.Hooks) == 0 }

// Summary says what the removal takes out, one line each.
func (r Removal) Summary() []string {
	var out []string
	for _, rule := range r.Deny {
		out = append(out, "no longer denies "+rule)
	}
	for _, h := range r.Hooks {
		out = append(out, fmt.Sprintf("no longer runs the gate on %s %s", h.Event, h.Matcher))
	}
	return out
}

// Apply writes the file with exactly the deny rules and hook registrations
// Plan found taken out, leaving every other rule, registration and key as it
// found them. An event or matcher left with nothing registered is dropped
// rather than left as an empty list, and a permissions or hooks object left
// with nothing in it is dropped rather than left behind.
func (r Removal) Apply() error {
	if r.Empty() {
		return nil
	}
	body := r.body

	if len(r.Deny) > 0 {
		removeDeny(body, r.Deny)
	}
	for _, h := range r.Hooks {
		unregister(body, h)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(body); err != nil {
		return fmt.Errorf("encode %s: %w", r.File, err)
	}
	return write(r.File, buf.Bytes())
}

// write puts the body in place through a temporary file beside it, so an
// interrupted write cannot leave a repository with half a settings file.
//
// The temporary file lands at the mode the settings file already had, not at
// a fixed one: a repository may have set it to 0600, since it can carry `env`
// values, and a merge must not widen that on its way past. A file this
// package is creating rather than merging has no previous mode to keep, so it
// lands at 0644.
func write(file string, body []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(file); err == nil {
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}

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
	if err := os.Chmod(name, mode); err != nil {
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

// removeDeny takes the given rules out of permissions.deny, then drops deny
// and permissions themselves once nothing is left in them.
func removeDeny(body map[string]any, rules []string) {
	permissions, ok := body["permissions"].(map[string]any)
	if !ok {
		return
	}
	var kept []any
	for _, rule := range heldDeny(body) {
		if !slices.Contains(rules, rule) {
			kept = append(kept, rule)
		}
	}
	if len(kept) == 0 {
		delete(permissions, "deny")
	} else {
		permissions["deny"] = kept
	}
	if len(permissions) == 0 {
		delete(body, "permissions")
	} else {
		body["permissions"] = permissions
	}
}

// unregister takes one hook's command out of its matcher's entry, then drops
// the matcher, the event and hooks itself once nothing is left in them.
func unregister(body map[string]any, h Hook) {
	hooks, ok := body["hooks"].(map[string]any)
	if !ok {
		return
	}
	entries, ok := hooks[h.Event].([]any)
	if !ok {
		return
	}
	var kept []any
	for _, entry := range entries {
		e, ok := entry.(map[string]any)
		if !ok {
			kept = append(kept, entry)
			continue
		}
		if matcher, _ := e["matcher"].(string); matcher != h.Matcher {
			kept = append(kept, entry)
			continue
		}
		inner, _ := e["hooks"].([]any)
		var innerKept []any
		for _, one := range inner {
			o, ok := one.(map[string]any)
			if ok {
				if command, _ := o["command"].(string); command == h.Command {
					continue
				}
			}
			innerKept = append(innerKept, one)
		}
		if len(innerKept) == 0 {
			continue
		}
		e["hooks"] = innerKept
		kept = append(kept, e)
	}
	if len(kept) == 0 {
		delete(hooks, h.Event)
	} else {
		hooks[h.Event] = kept
	}
	if len(hooks) == 0 {
		delete(body, "hooks")
	} else {
		body["hooks"] = hooks
	}
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
