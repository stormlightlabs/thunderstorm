package check

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Settings is the part of a Claude Code settings file this gate reads.
type Settings struct {
	Permissions struct {
		Deny []string `json:"deny"`
	} `json:"permissions"`
}

// DenyRule is how a command prefix reaches a settings file. Claude Code reads
// a tool call rather than a shell string, so the prefix arrives wrapped.
func DenyRule(prefix string) string { return fmt.Sprintf("Bash(%s:*)", prefix) }

// Policy reports the denied commands a repository's settings do not carry.
//
// The workflow states four commands a session may not run, and no plugin
// mechanism carries a permission, so a repository merges them into its own
// settings by hand. A rule added to the workflow reaches every payload and no
// repository's settings, and the gap is silent: the session that should have
// been refused runs the command instead.
func Policy(settingsFile string, denied []string) ([]string, error) {
	body, err := os.ReadFile(settingsFile)
	if err != nil {
		return nil, err
	}
	var settings Settings
	if err := json.Unmarshal(body, &settings); err != nil {
		return nil, fmt.Errorf("%s: %w", settingsFile, err)
	}

	carried := map[string]bool{}
	for _, rule := range settings.Permissions.Deny {
		carried[strings.TrimSpace(rule)] = true
	}
	var missing []string
	for _, prefix := range denied {
		if !carried[DenyRule(prefix)] {
			missing = append(missing, prefix)
		}
	}
	return missing, nil
}

// PolicyOf reads the deny list out of a rendered payload's settings file,
// which is what an installed repository has where it does not have the
// workflow source.
func PolicyOf(settingsFile string) ([]string, error) {
	body, err := os.ReadFile(settingsFile)
	if err != nil {
		return nil, err
	}
	var settings Settings
	if err := json.Unmarshal(body, &settings); err != nil {
		return nil, fmt.Errorf("%s: %w", settingsFile, err)
	}
	var prefixes []string
	for _, rule := range settings.Permissions.Deny {
		rule = strings.TrimSpace(rule)
		inner, ok := strings.CutPrefix(rule, "Bash(")
		if !ok {
			continue
		}
		if prefix, ok := strings.CutSuffix(inner, ":*)"); ok {
			prefixes = append(prefixes, prefix)
		}
	}
	return prefixes, nil
}
