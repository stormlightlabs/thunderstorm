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

	"github.com/BurntSushi/toml"
	"github.com/stormlightlabs/thunderstorm/internal/check"
	"github.com/stormlightlabs/thunderstorm/internal/jsonc"
)

// Name is the file a repository writes its settings in, and the one a command
// names when a setting it needed is missing.
const Name = ".tstorm.toml"

// names are the files tstorm reads, in the order it looks for them in each
// directory. A repository that installed the loop before TOML already has a
// .tstorm.json, which keeps being read under the same setting names.
var names = []string{Name, ".tstorm.json"}

// Config is what a repository tells tstorm about itself.
type Config struct {
	// Documents is the tree the frontmatter check walks and the tree the
	// skills write plans and ideas into, relative to the file that names it.
	// A repository starting out sets it to docs/internal; one that keeps no
	// documents leaves it empty and the check has nothing to do.
	Documents string `json:"documents" toml:"documents"`

	// Board is the GitHub Projects board the loop writes. A repository that
	// names none gets an error from the board commands rather than a guess.
	Board Board `json:"board" toml:"board"`

	// Prose is what the prose gate runs with. A repository that names
	// nothing gets tropius under its own defaults.
	Prose Prose `json:"prose" toml:"prose"`

	// dir is the directory the file was read from, so a relative setting
	// resolves against the file rather than the caller's working directory.
	dir string

	// file is the name it was read under, so a diagnostic names the file the
	// repository has rather than the one it would have written today.
	file string
}

// File is the settings file that was read, or the file a repository should
// write when none was found. A message telling somebody to set a value says
// where to set it, and a repository still on .tstorm.json is not helped by
// the name of a file it does not have.
func (c Config) File() string {
	if c.file == "" {
		return Name
	}
	return c.file
}

// Board names the project the loop reads and writes, and what this repository
// calls each state the loop uses.
type Board struct {
	// Owner is the user or organization the project belongs to, and Number is
	// the project's number in that owner's list.
	Owner  string `json:"owner" toml:"owner"`
	Number int    `json:"number" toml:"number"`

	// Repository filters every read to one repository's issues, as
	// "owner/name". Left empty it is read from the origin remote, which is
	// the repository the command is running in.
	Repository string `json:"repository" toml:"repository"`

	// StatusField is the name of the single-select field carrying status.
	StatusField string `json:"statusField" toml:"statusField"`

	// Status maps each state the loop uses to the option name this board
	// gives it.
	Status Status `json:"status" toml:"status"`

	// GroupField and GroupValue narrow a board that carries more than one
	// repository's work. A board that carries one leaves both empty.
	GroupField string `json:"groupField" toml:"groupField"`
	GroupValue string `json:"groupValue" toml:"groupValue"`

	// file is the settings file this board was read from. A Board travels to
	// the board client on its own, so it carries the name its error needs.
	file string
}

// Prose configures the gate that runs tropius over a repository's writing.
// The detection is tropius's; what a repository decides here is which of its
// rules it mutes.
type Prose struct {
	// Dictionary is the project dictionary, relative to this file. Left
	// empty, tropius searches for its own from the working directory.
	Dictionary string `json:"dictionary" toml:"dictionary"`

	// Mute drops a rule by id.
	Mute []MutedRule `json:"mute" toml:"mute"`

	// Paths are the trees the gate reads when no path is given, relative to
	// this file.
	Paths []string `json:"paths" toml:"paths"`
}

// MutedRule is one rule the gate drops, and why. A .tstorm.toml writes the
// rule id on its own and the reason in a comment above it; a .tstorm.json has
// nowhere to put a comment, so it names the reason in a key beside the rule.
type MutedRule struct {
	Rule string `json:"rule"`
	Why  string `json:"why"`
}

// UnmarshalTOML reads the bare rule id TOML gives it. Why stays empty: the
// reason is a comment, which no parser reports.
func (m *MutedRule) UnmarshalTOML(value any) error {
	rule, ok := value.(string)
	if !ok {
		return fmt.Errorf("prose.mute takes rule ids as strings, not %T", value)
	}
	m.Rule = rule
	return nil
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
	Todo       string `json:"todo" toml:"todo"`
	InProgress string `json:"inProgress" toml:"inProgress"`
	Done       string `json:"done" toml:"done"`
}

// Load reads the settings that apply to dir, walking up until it finds a file
// or runs out of parents. The nearest directory holding one of names wins, and
// within a directory the order of names decides. A repository that has
// configured nothing gets an empty Config and no error: a missing file is a
// repository that has not asked for these checks, which is not the same as a
// broken one.
func Load(dir string) (Config, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return Config{}, err
	}
	for {
		for _, name := range names {
			path := filepath.Join(dir, name)
			body, err := os.ReadFile(path)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return Config{}, err
			}
			c, err := parse(name, body)
			if err != nil {
				return Config{}, fmt.Errorf("%s: %w", path, err)
			}
			c.dir = dir
			c.file = name
			c.Board.file = name
			return c, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return Config{}, nil
		}
		dir = parent
	}
}

// parse decodes one settings file by its extension. Which format a repository
// chose stops here: everything above reads the same Config.
//
// A file holding no settings is refused rather than returned empty. The first
// file found is the only one read, so an empty .tstorm.toml left by a
// conversion that got as far as touch would otherwise hide a .tstorm.json
// beside it and take every setting with it. An empty .tstorm.json has always
// been an error, and this is the same answer.
func parse(name string, body []byte) (Config, error) {
	var c Config
	switch ext := filepath.Ext(name); ext {
	case ".json":
		return c, json.Unmarshal(jsonc.Strip(body), &c)
	case ".toml":
		meta, err := toml.Decode(string(body), &c)
		if err != nil {
			return Config{}, err
		}
		if len(meta.Keys()) == 0 {
			return Config{}, errors.New("names no settings: fill it in, or remove it")
		}
		return c, nil
	default:
		return Config{}, fmt.Errorf("no decoder for %s", ext)
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
//
// b.file is empty only when Load found no settings file at all: Load sets it
// on every Config it reads from disk, and leaves it unset on the zero Config
// it returns for a directory with nothing in it. That is the signal used
// here to tell "nothing to edit yet" from "this file needs more in it".
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
	if b.file == "" {
		return errors.New("no config found; run tstorm install --board <owner>/<number> --documents <dir> to write one")
	}
	return fmt.Errorf("%s names no %s", b.file, strings.Join(missing, ", "))
}
