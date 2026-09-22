package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Create writes a starter config at dir and creates the documents tree it
// names. It returns the file it wrote.
//
// Nothing here is inferred. A repository that told install nothing gets no
// file, because a config naming a board that does not exist fails later and
// further from the mistake than a missing one does.
func Create(dir string, c Config, force bool) (string, error) {
	file := filepath.Join(dir, Name)
	if _, err := os.Stat(file); err == nil && !force {
		return "", fmt.Errorf("%s is already there; --force replaces it", file)
	}
	body := starter(c)
	if body == "" {
		return "", fmt.Errorf("nothing to write: name a documents tree or a board")
	}
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		return "", err
	}
	if c.Documents != "" {
		if err := os.MkdirAll(filepath.Join(dir, c.Documents), 0o755); err != nil {
			return file, err
		}
	}
	return file, nil
}

// starter renders the settings as TOML, with the comment beside each one that
// says what reads it. An empty section is left out rather than written blank:
// a key with no value is a setting a reader has to guess at.
func starter(c Config) string {
	var b strings.Builder
	if c.Documents != "" {
		b.WriteString("# Where specify writes plans and ideas, and what an issue cites by identifier.\n")
		fmt.Fprintf(&b, "documents = %q\n", c.Documents)
	}

	if c.Board.Owner != "" || c.Board.Number != 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("# The GitHub Projects board tstorm board reads and writes.\n")
		b.WriteString("[board]\n")
		fmt.Fprintf(&b, "owner = %q\n", c.Board.Owner)
		fmt.Fprintf(&b, "number = %d\n", c.Board.Number)
		fmt.Fprintf(&b, "statusField = %q\n", c.Board.StatusField)
		if c.Board.GroupValue != "" {
			b.WriteString("\n# A board carrying several repositories separates them here.\n")
			fmt.Fprintf(&b, "groupField = %q\n", c.Board.GroupField)
			fmt.Fprintf(&b, "groupValue = %q\n", c.Board.GroupValue)
		}
		b.WriteString("\n# What this board calls each state the loop moves an issue between.\n")
		b.WriteString("[board.status]\n")
		fmt.Fprintf(&b, "todo = %q\n", c.Board.Status.Todo)
		fmt.Fprintf(&b, "inProgress = %q\n", c.Board.Status.InProgress)
		fmt.Fprintf(&b, "done = %q\n", c.Board.Status.Done)
	}

	if c.Prose.Dictionary != "" || len(c.Prose.Paths) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("# What check prose reads, and the dictionary it reads with.\n")
		b.WriteString("[prose]\n")
		if c.Prose.Dictionary != "" {
			fmt.Fprintf(&b, "dictionary = %q\n", c.Prose.Dictionary)
		}
		if len(c.Prose.Paths) > 0 {
			quoted := make([]string, len(c.Prose.Paths))
			for i, p := range c.Prose.Paths {
				quoted[i] = fmt.Sprintf("%q", p)
			}
			fmt.Fprintf(&b, "paths = [%s]\n", strings.Join(quoted, ", "))
		}
	}
	return b.String()
}
