package dispatch

import (
	"fmt"
	"sort"
	"strings"
)

// Pi is the harness this package drives.
const Pi = "pi"

// elsewhere names every other harness the workflow mentions and says what
// drives a role there, so asking tstorm to do it is answered rather than
// attempted. A harness in neither this map nor Pi is unknown.
//
// The reasons come from docs/internal/hosts.md and docs/internal/models.md.
var elsewhere = map[string]string{
	"claude": "Claude Code provisions a subagent from the definition under agents/; " +
		"the run dispatches the role rather than starting a session for it",
	"codex": "Codex spawns its own agent with spawn_agent, and forks with " +
		"fork_turns set so the model and reasoning level can be chosen",
	"cursor": "Cursor is unsupported: nothing here has run it, and whether it " +
		"can carry a role at all is stormlightlabs/thunderstorm#2",
	"opencode": "OpenCode Go is unsupported: it has not been assessed against " +
		"what models.md asks of a harness, which is stormlightlabs/thunderstorm#2",
}

// CheckHarness reports whether tstorm dispatches on this harness.
func CheckHarness(name string) error {
	if name == Pi {
		return nil
	}
	if why, ok := elsewhere[name]; ok {
		return fmt.Errorf("tstorm does not dispatch on %s. %s", name, why)
	}
	known := make([]string, 0, len(elsewhere)+1)
	known = append(known, Pi)
	for h := range elsewhere {
		known = append(known, h)
	}
	sort.Strings(known)
	return fmt.Errorf("unknown harness %q; the workflow names %s", name, strings.Join(known, ", "))
}
