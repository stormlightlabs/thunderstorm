// Package config reads a repository's tstorm settings.
//
// Nothing about one repository may be compiled in. The loop's checks run in
// whichever repository installed the payload, and where that repository keeps
// its documents is its own business, so the setting lives in a file beside its
// code rather than in a default that only one repository is right about.
//
// The same holds for the board. Project 13 under stormlightlabs carries
// several repositories at once and separates them with a Track field, which is
// one arrangement among many; a repository installing the loop is as likely to
// want a project of its own, or status options under different names.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/stormlightlabs/thunderstorm/internal/check"
)

// Name is the file tstorm reads, found at the repository root or any directory
// above the one a command runs in.
const Name = ".tstorm.json"

// Config is what a repository tells tstorm about itself.
type Config struct {
	// Documents is the tree the frontmatter check walks, relative to the file
	// that names it. A repository that keeps no documents leaves it empty and
	// the check has nothing to do.
	Documents string `json:"documents"`

	// Board is the GitHub Projects board the loop writes. A repository that
	// names none gets an error from the board commands rather than a guess.
	Board Board `json:"board"`

	// Prose is what the prose gate runs with. A repository that names
	// nothing gets tropius under its own defaults.
	Prose Prose `json:"prose"`

	// dir is the directory the file was read from, so a relative setting
	// resolves against the file rather than the caller's working directory.
	dir string
}

// Board names the project the loop reads and writes, and what this repository
// calls each state the loop uses.
type Board struct {
	// Owner is the user or organization the project belongs to, and Number is
	// the project's number in that owner's list.
	Owner  string `json:"owner"`
	Number int    `json:"number"`

	// Repository filters every read to one repository's issues, as
	// "owner/name". Left empty it is read from the origin remote, which is
	// the repository the command is running in.
	Repository string `json:"repository"`

	// StatusField is the name of the single-select field carrying status.
	StatusField string `json:"statusField"`

	// Status maps each state the loop uses to the option name this board
	// gives it.
	Status Status `json:"status"`

	// GroupField and GroupValue narrow a board that carries more than one
	// repository's work. A board that carries one leaves both empty.
	GroupField string `json:"groupField"`
	GroupValue string `json:"groupValue"`
}

// Prose configures the gate that runs tropius over a repository's writing.
// The detection is tropius's; what a repository decides here is which of its
// rules it mutes.
type Prose struct {
	// Dictionary is the project dictionary, relative to this file. Left
	// empty, tropius searches for its own from the working directory.
	Dictionary string `json:"dictionary"`

	// Mute drops a rule by id.
	Mute []MutedRule `json:"mute"`

	// Paths are the trees the gate reads when no path is given, relative to
	// this file.
	Paths []string `json:"paths"`
}

// MutedRule is one rule the gate drops, and why.
type MutedRule struct {
	Rule string `json:"rule"`
	Why  string `json:"why"`
}

// Rules is what the check package needs out of the prose settings, with the
// dictionary resolved against the file that named it.
func (c Config) Rules() check.ProseRules {
	rules := check.ProseRules{}
	if d := c.Prose.Dictionary; d != "" {
		rules.Dictionary = d
		if !filepath.IsAbs(d) {
			rules.Dictionary = filepath.Join(c.dir, d)
		}
	}
	for _, muted := range c.Prose.Mute {
		rules.Mute = append(rules.Mute, muted.Rule)
	}
	return rules
}

// ProsePaths are the configured trees as paths the caller can open.
func (c Config) ProsePaths() []string {
	var out []string
	for _, path := range c.Prose.Paths {
		if filepath.IsAbs(path) {
			out = append(out, path)
			continue
		}
		out = append(out, filepath.Join(c.dir, path))
	}
	return out
}

// Status is the option name for each of the three states the loop moves an
// issue between.
type Status struct {
	Todo       string `json:"todo"`
	InProgress string `json:"inProgress"`
	Done       string `json:"done"`
}

// Load reads the settings that apply to dir, walking up until it finds a file
// or runs out of parents. A repository that has configured nothing gets an
// empty Config and no error: a missing file is a repository that has not asked
// for these checks, which is not the same as a broken one.
func Load(dir string) (Config, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return Config{}, err
	}
	for {
		path := filepath.Join(dir, Name)
		body, err := os.ReadFile(path)
		if err == nil {
			var c Config
			if err := json.Unmarshal(body, &c); err != nil {
				return Config{}, fmt.Errorf("%s: %w", path, err)
			}
			c.dir = dir
			return c, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return Config{}, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return Config{}, nil
		}
		dir = parent
	}
}

// DocumentsDir is the configured tree as a path the caller can open, or an
// empty string when no file named one.
func (c Config) DocumentsDir() string {
	if c.Documents == "" {
		return ""
	}
	if filepath.IsAbs(c.Documents) {
		return c.Documents
	}
	return filepath.Join(c.dir, c.Documents)
}

// Validate reports every board setting that is missing, in one error. A
// repository configuring the board for the first time has usually left out
// more than one, and learning about them one run at a time is three runs.
func (b Board) Validate() error {
	var missing []string
	for _, setting := range []struct {
		name  string
		empty bool
	}{
		{"owner", b.Owner == ""},
		{"number", b.Number == 0},
		{"statusField", b.StatusField == ""},
		{"status.todo", b.Status.Todo == ""},
		{"status.inProgress", b.Status.InProgress == ""},
		{"status.done", b.Status.Done == ""},
		{"groupValue", b.GroupField != "" && b.GroupValue == ""},
	} {
		if setting.empty {
			missing = append(missing, "board."+setting.name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("%s names no %s", Name, strings.Join(missing, ", "))
}
