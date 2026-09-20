package check

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// The convention lives in the specify skill: a document opens with a name, a
// date, and a ULID that never changes, and an issue cites the document it came
// from by that identifier. A document missing one cannot be cited, and two
// documents sharing one cite each other's work.
const fence = "---"

var required = []string{"name", "last_updated", "id"}

var (
	// A plan that a milestone tracks names it, so a reader of the document
	// reaches the work and a reader of the milestone reaches the reasoning.
	milestonePattern = regexp.MustCompile(`^https://github\.com/[\w.-]+/[\w.-]+/milestone/\d+$`)
	fieldPattern     = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_-]*):(.*)$`)
	datePattern      = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	// Crockford base32 without I, L, O, and U, which the alphabet omits so a
	// transcribed identifier cannot turn into a different one.
	ulidPattern = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)
)

// documentSuffix is matched case-insensitively when finding documents, so that
// capitalising it cannot walk one past every check below, and exactly when
// reporting, so the tree stays one spelling.
const documentSuffix = ".md"

// unnamed holds per-feature plans and task lists back from the naming rule
// until their scheme is decided: five files named "plan" and five named
// "tasks" would collide under the rule that a name matches its filename.
// Tracked in issue #3, which drops this list rather than narrowing it.
//
// The waiver is as narrow as it can be. These files carry no block at all, so
// presence cannot be required of them until #3 gives them names to carry, and
// a file without one is passed over. A file that has a block is checked like
// any other except for its name, so the day #3 adds the blocks, a duplicate
// identifier is caught without anyone remembering to come back here.
//
// Each shape is matched against the whole relative path, component by
// component, so the exemption cannot be inherited by a features/ directory
// somewhere else in the tree.
var unnamed = [][]string{
	{"features", "*", "plan.md"},
	{"features", "*", "tasks.md"},
}

// Frontmatter checks every document under root, returning how many it read and
// what failed. One run reports the whole tree, and a document that cannot be
// read is that document's failure rather than the run's.
func Frontmatter(root string) (int, []string, error) {
	documents, err := documents(root)
	if err != nil {
		return 0, nil, err
	}

	var failures []string
	seen := map[string]string{}
	checked := 0

	for _, path := range documents {
		relative := relativeTo(root, path)

		if filepath.Ext(path) != documentSuffix {
			failures = append(failures, fmt.Sprintf("%s: extension is %q, expected %q",
				relative, filepath.Ext(path), documentSuffix))
		}

		fields, problems := readFrontmatter(path)
		waived := isUnnamed(relative)
		if waived && len(fields) == 0 {
			continue
		}

		checked++
		for _, problem := range problems {
			failures = append(failures, fmt.Sprintf("%s: %s", relative, problem))
		}

		name, ok := fields["name"]
		expected := expectedName(relative, root)
		if ok && name != "" && !waived && name != expected {
			failures = append(failures, fmt.Sprintf("%s: name is %q, expected %q", relative, name, expected))
		}

		if id, ok := fields["id"]; ok && ulidPattern.MatchString(id) {
			if first, taken := seen[id]; taken {
				failures = append(failures, fmt.Sprintf("%s: id is also on %s", relative, first))
			} else {
				seen[id] = relative
			}
		}
	}

	return checked, failures, nil
}

// FrontmatterSince returns a failure for every identifier that differs from the
// one at ref.
//
// Uniqueness within one tree is not immutability across time: an identifier
// edited in place leaves a tree that looks clean while every issue citing the
// old value points at nothing, and that is the one failure no snapshot of the
// tree can see.
func FrontmatterSince(root, ref string) ([]string, error) {
	repo, ok := git("-C", root, "rev-parse", "--show-toplevel")
	if !ok {
		return []string{fmt.Sprintf("cannot compare against %s: %s is not inside a git repository", ref, root)}, nil
	}
	repo = strings.TrimSpace(repo)

	// Every show below returns nothing for an unresolvable ref, which is
	// indistinguishable from a file that did not exist yet. Resolve the ref
	// once, so a shallow or single-branch clone fails loudly instead of
	// reporting that nothing changed.
	if _, ok := git("-C", repo, "rev-parse", "--verify", ref+"^{commit}"); !ok {
		return []string{fmt.Sprintf("cannot compare against %s: no such commit in %s", ref, repo)}, nil
	}

	documents, err := documents(root)
	if err != nil {
		return nil, err
	}

	var failures []string
	for _, path := range documents {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		tracked, err := filepath.Rel(repo, absolute)
		if err != nil {
			return nil, err
		}

		before, ok := git("-C", repo, "show", ref+":"+filepath.ToSlash(tracked))
		if !ok {
			continue
		}

		was, _ := parseBlock(lines(before))
		now, _ := readFrontmatter(path)
		old, new := was["id"], now["id"]
		if old != "" && new != "" && old != new {
			failures = append(failures, fmt.Sprintf(
				"%s: id was %s at %s and is now %s; an identifier never changes once assigned",
				relativeTo(root, path), old, ref, new))
		}
	}
	return failures, nil
}

// documents returns every markdown file in the tree, however its extension is
// capitalised, in one order whatever the filesystem's is.
func documents(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(path), documentSuffix) {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func relativeTo(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(relative)
}

// isUnnamed matches the whole path against a shape, so the waiver is not a
// suffix rule.
func isUnnamed(relative string) bool {
	parts := strings.Split(relative, "/")
	for _, shape := range unnamed {
		if len(parts) != len(shape) {
			continue
		}
		match := true
		for i, want := range shape {
			if want != "*" && want != parts[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// readFrontmatter parses the opening block. A file that cannot be read fails on
// its own line.
func readFrontmatter(path string) (map[string]string, []string) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{fmt.Sprintf("cannot be read (%s)", errorReason(err))}
	}
	if !utf8.Valid(body) {
		return nil, []string{fmt.Sprintf("is not valid UTF-8 (invalid byte at byte %d)", firstInvalidByte(body))}
	}
	return parseBlock(lines(string(body)))
}

// parseBlock reads the top level of the opening block and says what is wrong
// with it.
//
// Blank lines and comments are skipped, an indented line belongs to the key
// above it and is not inspected, and a key may carry any value the convention
// does not constrain. The three keys it does constrain are top-level scalars,
// so nothing deeper has to be understood to know whether they are right. This
// is not a YAML parser and does not try to be one.
func parseBlock(body []string) (map[string]string, []string) {
	if len(body) == 0 || body[0] != fence {
		return nil, []string{"does not open with frontmatter"}
	}
	end := -1
	for i, line := range body[1:] {
		if line == fence {
			end = i + 1
			break
		}
	}
	if end < 0 {
		return nil, []string{"frontmatter is never closed"}
	}

	fields := map[string]string{}
	var problems []string
	for _, line := range body[1:end] {
		stripped := strings.TrimSpace(line)
		if stripped == "" || strings.HasPrefix(stripped, "#") {
			continue
		}
		// Indented, so it continues the key above: a list entry or a nested
		// mapping. The convention constrains no key that can hold one.
		if line != strings.TrimLeft(line, " \t") {
			continue
		}

		match := fieldPattern.FindStringSubmatch(line)
		if match == nil {
			problems = append(problems, fmt.Sprintf("cannot parse %q", stripped))
			continue
		}
		// A key repeated inside one block is ambiguous rather than untidy.
		if _, taken := fields[match[1]]; taken {
			problems = append(problems, fmt.Sprintf("%s appears twice", match[1]))
		}
		fields[match[1]] = scalar(strings.TrimSpace(match[2]))
	}

	for _, key := range required {
		if value, ok := fields[key]; !ok {
			problems = append(problems, "no "+key)
		} else if value == "" {
			problems = append(problems, key+" is empty")
		}
	}

	return fields, append(problems, valueProblems(fields)...)
}

// valueProblems checks the keys whose form the convention constrains.
func valueProblems(fields map[string]string) []string {
	var problems []string

	if date := fields["last_updated"]; date != "" {
		if !datePattern.MatchString(date) {
			problems = append(problems, fmt.Sprintf("last_updated is %q, expected YYYY-MM-DD", date))
		} else if _, err := time.Parse("2006-01-02", date); err != nil {
			// The shape is right, which does not make it a date: a check that
			// lets 2026-13-45 through is not checking the thing it exists to
			// check.
			problems = append(problems, fmt.Sprintf("last_updated is %q, which is not a real date", date))
		}
	}

	if id := fields["id"]; id != "" && !ulidPattern.MatchString(id) {
		problems = append(problems, fmt.Sprintf("id is %q, expected a 26-character ULID", id))
	}

	if milestone := fields["milestone"]; milestone != "" && !milestonePattern.MatchString(milestone) {
		problems = append(problems, fmt.Sprintf("milestone is %q, expected a milestone URL such as "+
			"https://github.com/owner/repo/milestone/1", milestone))
	}

	return problems
}

// scalar is the value a line carries: unquoted, with any trailing comment
// removed.
//
// A # opens a comment only when a space precedes it, and never inside quotes.
// Escapes within a quoted scalar are not interpreted, which the convention's
// three keys never need.
func scalar(value string) string {
	if strings.HasPrefix(value, "'") || strings.HasPrefix(value, `"`) {
		quote := value[:1]
		// An unterminated quote is left whole so it fails loudly downstream
		// rather than being silently repaired into something plausible.
		if closing := strings.Index(value[1:], quote); closing >= 0 {
			return value[1 : closing+1]
		}
		return value
	}
	if strings.HasPrefix(value, "#") {
		return ""
	}
	if before, _, found := strings.Cut(value, " #"); found {
		return strings.TrimRight(before, " \t")
	}
	return value
}

// expectedName is the name a document's path asks for: a README is named for
// the directory holding it, any other file for itself.
//
// The convention asks for kebab-case, so BUGS.md is named "bugs": the filename
// decides the name, its capitalisation does not.
func expectedName(relative, root string) string {
	stem := strings.TrimSuffix(filepath.Base(relative), filepath.Ext(relative))
	if stem == "README" {
		stem = filepath.Base(filepath.Dir(relative))
		if stem == "." || stem == string(filepath.Separator) {
			stem = filepath.Base(root)
		}
	}
	return strings.ReplaceAll(strings.ToLower(stem), "_", "-")
}

func firstInvalidByte(body []byte) int {
	for i := 0; i < len(body); {
		r, size := utf8.DecodeRune(body[i:])
		if r == utf8.RuneError && size <= 1 {
			return i
		}
		i += size
	}
	return len(body)
}

// errorReason is what went wrong without the path repeated, which the caller
// has already printed.
func errorReason(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	return err.Error()
}

// FrontmatterFile checks one document and reports only what that document is
// answerable for.
//
// The tree is still walked. An identifier is unique or not against every other
// document, which one file on its own cannot say, and a hook that reported the
// whole tree on every write would be ignored by the second week.
func FrontmatterFile(root, path string) ([]string, error) {
	_, failures, err := Frontmatter(root)
	if err != nil {
		return nil, err
	}
	prefix := relativeTo(root, path) + ": "
	var mine []string
	for _, failure := range failures {
		if strings.HasPrefix(failure, prefix) {
			mine = append(mine, strings.TrimPrefix(failure, prefix))
		}
	}
	return mine, nil
}
