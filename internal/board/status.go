package board

import (
	"fmt"
	"strings"
)

// Status is one of the three states the loop moves an issue between. The name
// here is the loop's; what this board calls it is configuration.
type Status string

const (
	// Todo is nobody holding the issue. An item added to the board with no
	// status reads as this: every reader here treats a missing value and Todo
	// as the same thing.
	Todo Status = "todo"
	// InProgress is a run holding it, whether or not a pull request is open.
	InProgress Status = "in-progress"
	// Done is a merged change confirmed on the default branch.
	Done Status = "done"
)

// Statuses returns the three states in the order a piece of work passes
// through them.
func Statuses() []Status { return []Status{Todo, InProgress, Done} }

// transitions is the table. Which option names these move between is
// configuration; that Done is terminal and that nothing reaches In Progress
// except from Todo is not.
var transitions = map[Status][]Status{
	Todo:       {InProgress},
	InProgress: {Todo, Done},
	Done:       nil,
}

// Allowed reports whether the table lists a move from one state to another.
func Allowed(from, to Status) bool {
	for _, allowed := range transitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

// Next returns the states reachable from one, for an error message that says
// what the caller could have asked for.
func Next(from Status) []Status { return transitions[from] }

// ParseStatus reads a state from an argument. Spaces and case are accepted
// because "In Progress" is what the board shows a person.
func ParseStatus(value string) (Status, error) {
	normalized := Status(strings.ReplaceAll(strings.ToLower(strings.TrimSpace(value)), " ", "-"))
	for _, status := range Statuses() {
		if normalized == status {
			return status, nil
		}
	}
	return "", fmt.Errorf("no status %q: the loop uses todo, in-progress and done", value)
}

// OptionName is what this board calls a state.
func (c *Client) OptionName(status Status) string {
	switch status {
	case Todo:
		return c.settings.Status.Todo
	case InProgress:
		return c.settings.Status.InProgress
	case Done:
		return c.settings.Status.Done
	}
	return ""
}

// status reads an option name back as a state. An option the configuration
// does not name belongs to somebody else's workflow on the same board, and
// reads as unknown rather than as one of these three.
func (c *Client) status(option string) (Status, bool) {
	if option == "" {
		return Todo, true
	}
	for _, status := range Statuses() {
		if c.OptionName(status) == option {
			return status, true
		}
	}
	return "", false
}
