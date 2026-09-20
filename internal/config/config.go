// Package config reads a repository's tstorm settings.
//
// Nothing about one repository may be compiled in. The loop's checks run in
// whichever repository installed the payload, and where that repository keeps
// its documents is its own business, so the setting lives in a file beside its
// code rather than in a default that only one repository is right about.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

	// dir is the directory the file was read from, so a relative setting
	// resolves against the file rather than the caller's working directory.
	dir string
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
