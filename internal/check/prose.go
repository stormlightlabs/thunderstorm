package check

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// Tropius is the binary this gate runs. Detection is its work, not tstorm's:
// it carries the catalogue, the project dictionary, and the structural
// detectors, and this gate runs it, drops the rules a repository has muted,
// and prints what is left.
const Tropius = "trps"

// TropiusHome is where a caller that does not have it goes to get it.
const TropiusHome = "https://github.com/stormlightlabs/trps"

// ProseNotInstalled is what a caller gets when the binary is not there. The
// gate reports rather than blocks, and a session with no tropius installed is
// a session with no prose findings, not a stopped one.
var ProseNotInstalled = errors.New(Tropius + " is not installed, so prose was not checked")

// ProseRules is what a repository tells the gate about the rules themselves.
type ProseRules struct {
	// Dictionary is the project dictionary tropius applies, or empty to let
	// it search from the working directory as it does on its own.
	Dictionary string
	// Mute drops a rule by id. A muted rule is one this repository cannot
	// read yet, not one it disagrees with, so each id is worth a reason
	// beside it in the file that names it.
	Mute []string
}

// ProseFinding is one thing tropius reported, at the place it found it.
type ProseFinding struct {
	Path     string
	Line     int
	Column   int
	Rule     string
	Severity string
	Matched  string
}

func (f ProseFinding) String() string {
	// A structural finding quotes the whole run it matched, which is three
	// bullets or a paragraph. One line of it locates the passage; the file
	// holds the rest.
	matched := f.Matched
	if cut := strings.IndexAny(matched, "\n\r"); cut >= 0 {
		matched = matched[:cut] + " ..."
	}
	return fmt.Sprintf("%s:%d:%d  %s %s: %s", f.Path, f.Line, f.Column, f.Severity, f.Rule, matched)
}

// report is the shape tropius writes with --json, narrowed to the fields this
// gate reads. Its version field is raised when a consumer would have to
// change, so a version this gate has not seen is worth saying out loud.
type proseReport struct {
	Version  int `json:"version"`
	Findings []struct {
		RuleID   string `json:"rule_id"`
		RuleName string `json:"rule_name"`
		Severity string `json:"severity"`
		Matched  string `json:"matched"`
		Path     string `json:"path"`
		Line     int    `json:"line"`
		Column   int    `json:"column"`
	} `json:"findings"`
}

// proseReportVersion is the report shape this gate was written against.
const proseReportVersion = 1

// ProseFiles are the extensions this gate reads. Tropius grades prose, and a
// repository's prose is its Markdown: the site pages, the skills, the
// changelog and the roadmap.
var ProseFiles = []string{".md", ".mdx"}

// proseInputs expands a directory into the files under it this gate reads,
// because tropius takes files. A path named directly is passed through
// whatever it is called, so a file with an unusual extension can still be
// checked by naming it.
func proseInputs(paths []string) ([]string, error) {
	var out []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			out = append(out, path)
			continue
		}
		err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := d.Name()
			// A dot directory is tooling rather than writing, and node_modules
			// is somebody else's prose in any case.
			if d.IsDir() {
				if p != path && (strings.HasPrefix(name, ".") || name == "node_modules") {
					return filepath.SkipDir
				}
				return nil
			}
			if slices.Contains(ProseFiles, strings.ToLower(filepath.Ext(name))) {
				out = append(out, p)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Prose runs tropius over paths and returns what it found, minus the muted
// rules. A directory is walked for the files this gate reads. A file tropius
// excludes through the project dictionary produces nothing, which is not an
// error.
func Prose(paths []string, rules ProseRules) ([]ProseFinding, error) {
	paths, err := proseInputs(paths)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil
	}
	binary := Tropius
	if named := os.Getenv("TRPS_BIN"); named != "" {
		binary = named
	}
	args := []string{"--json", "--quiet"}
	if rules.Dictionary != "" {
		args = append(args, "--dictionary", rules.Dictionary)
	}
	cmd := exec.Command(binary, append(args, paths...)...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		switch {
		case errors.Is(err, exec.ErrNotFound), errors.Is(err, fs.ErrNotExist):
			// Not on PATH, and TRPS_BIN naming a file that is not there.
			return nil, ProseNotInstalled
		case errors.As(err, &exit) && exit.ExitCode() == 1:
			// Findings are what exit 1 means, and the report is on stdout.
		default:
			return nil, fmt.Errorf("%s: %w: %s", binary, err, strings.TrimSpace(stderr.String()))
		}
	}

	var report proseReport
	if err := json.Unmarshal(out, &report); err != nil {
		return nil, fmt.Errorf("%s wrote a report this gate cannot read: %w", binary, err)
	}
	if report.Version != proseReportVersion {
		return nil, fmt.Errorf("%s writes report version %d, and this gate reads %d",
			binary, report.Version, proseReportVersion)
	}

	muted := map[string]bool{}
	for _, id := range rules.Mute {
		muted[id] = true
	}
	var found []ProseFinding
	for _, f := range report.Findings {
		if muted[f.RuleID] {
			continue
		}
		found = append(found, ProseFinding{
			Path:     f.Path,
			Line:     f.Line,
			Column:   f.Column,
			Rule:     f.RuleID,
			Severity: f.Severity,
			Matched:  f.Matched,
		})
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].Path != found[j].Path {
			return found[i].Path < found[j].Path
		}
		return found[i].Line < found[j].Line
	})
	return found, nil
}
