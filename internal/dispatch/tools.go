package dispatch

import (
	"sort"
	"strings"
)

// piEquivalents maps each tool a role definition names to what pi calls the
// same capability. A name mapping to nothing has no counterpart there:
//
//   - Skill is not a tool on pi. It discovers skills itself, from
//     .agents/skills and .pi/skills, so a role reaches its skill whether or
//     not the allowlist says so.
//   - WebFetch has no built-in equivalent.
//   - The GitHub MCP tools are the cloud transport. A pane runs on the machine
//     the operator is sitting at, where the transport is gh through bash.
//
// Glob maps to two names because pi splits the one capability: find matches
// paths and ls lists a directory.
var piEquivalents = map[string][]string{
	"Bash":  {"bash"},
	"Edit":  {"edit"},
	"Glob":  {"find", "ls"},
	"Grep":  {"grep"},
	"Read":  {"read"},
	"Write": {"write"},
}

// PiTools translates a role's Claude Code allowlist into the list pi's --tools
// takes, and returns the names it could not translate.
//
// An allowlist that shrinks without saying so reads as a smaller permission
// than it is, which is what the second return value is for. It matters most
// for the reviewer roles: dropping Write and Edit leaves bash, which writes.
// What limits a role on Pi is the directory it was started in and whatever the
// operating system enforces around the process, not this list.
func PiTools(claude []string) (allow, untranslated []string) {
	translated, reported := map[string]bool{}, map[string]bool{}
	for _, name := range claude {
		equivalents, ok := piEquivalents[name]
		if !ok {
			// One entry for the whole MCP set. A role names half a dozen of
			// them and they are missing for one reason, which a reader needs
			// once.
			if strings.HasPrefix(name, "mcp__") {
				name = "the MCP tools"
			}
			if !reported[name] {
				reported[name] = true
				untranslated = append(untranslated, name)
			}
			continue
		}
		for _, e := range equivalents {
			if !translated[e] {
				translated[e] = true
				allow = append(allow, e)
			}
		}
	}
	sort.Strings(allow)
	return allow, untranslated
}
