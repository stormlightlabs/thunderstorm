package check

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

// The worktree skill makes every worktree the loop uses, outside the repository
// root so a build tool cannot reach the parent's configuration and build into
// the parent's output directory. A harness makes them inside the root, and
// takes the instruction two ways:
//
//   - An isolation key in a definition's frontmatter, which the harness reads
//     before any skill, so the key wins over every sentence written under it.
//   - An isolation setting on a dispatch, as far as a file can carry one. The
//     argument is a tool call, so what is checkable is the text it gets copied
//     from: a fenced code block naming the setting.
//
// The key is rejected everywhere rather than allowed for named files. No
// definition here could justify an allowlist entry: every worktree the loop
// wants is outside the root, which is the one place the harness will not put
// it. What no file can hold, the worktree skill's "Who gets one" covers.
const isolationKey = "isolation"

var (
	// A key line in the opening block, at any depth.
	isolationField = regexp.MustCompile(`^(\s*)([A-Za-z][A-Za-z0-9_-]*)\s*:(.*)$`)
	// The setting as a dispatch carries it. The bare word is prose and is left
	// alone.
	isolationSetting = regexp.MustCompile(`(\bisolation\b\s*[:=]|--isolation\b|["']isolation["'])`)
	// ``` or ~~~, three or more, because a longer fence is how a block nests
	// one.
	codeFence = regexp.MustCompile("^\\s*(`{3,}|~{3,})")
)

// skipped names a directory whose contents this check does not own. A worktree
// landing under the tree carries a second copy of it, whose findings would be
// duplicates against paths nothing tracks.
var skipped = []string{"worktrees"}

// Isolation checks every definition under root, returning how many it read and
// what failed. One run reports the whole tree, and a file that cannot be read
// is that file's failure rather than the run's.
func Isolation(root string) (int, []string, error) {
	found, err := documents(root)
	if err != nil {
		return 0, nil, err
	}

	var failures []string
	checked := 0
	for _, path := range found {
		relative := relativeTo(root, path)
		if underSkipped(relative) {
			continue
		}
		checked++

		body, err := os.ReadFile(path)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: cannot be read (%s)", relative, errorReason(err)))
			continue
		}
		if !utf8.Valid(body) {
			failures = append(failures, fmt.Sprintf("%s: is not valid UTF-8 (invalid byte at byte %d)",
				relative, firstInvalidByte(body)))
			continue
		}

		// A byte order mark would otherwise sit in front of the fence, and an
		// editor that writes one is how the key stays unreported.
		text := strings.TrimPrefix(string(body), "\ufeff")
		for _, problem := range isolationProblems(lines(text)) {
			failures = append(failures, fmt.Sprintf("%s: %s", relative, problem))
		}
	}

	return checked, failures, nil
}

func underSkipped(relative string) bool {
	parts := strings.Split(relative, "/")
	for _, part := range parts[:len(parts)-1] {
		for _, skip := range skipped {
			if part == skip {
				return true
			}
		}
	}
	return false
}

func isolationProblems(body []string) []string {
	return append(frontmatterDeclarations(body), codeBlockSettings(body)...)
}

// frontmatterDeclarations reads the opening block, if there is one, and reports
// the key.
//
// An unclosed block is left to the frontmatter check, which owns block shape.
// The block also has to open the file, as the harness requires of it: reading
// past either would make a --- in the body look like frontmatter.
func frontmatterDeclarations(body []string) []string {
	if len(body) == 0 || body[0] != fence {
		return nil
	}
	end := -1
	for i, line := range body[1:] {
		if line == fence {
			end = i + 1
			break
		}
	}
	if end < 0 {
		return nil
	}

	var problems []string
	for i, line := range body[1:end] {
		number := i + 2
		match := isolationField.FindStringSubmatch(line)
		switch {
		case match != nil && match[2] == isolationKey:
			value := strings.TrimSpace(match[3])
			if value == "" {
				value = "(empty)"
			}
			problems = append(problems, declared(number, value))
		case isolationSetting.MatchString(line):
			// A flow mapping, or the key nested in another key's value. The
			// line's own key is ruled out above, so prose in a value does not
			// reach here.
			problems = append(problems, declared(number, "(inside this line)"))
		}
	}
	return problems
}

// declared is the one message every frontmatter declaration reports, however it
// is written.
func declared(number int, value string) string {
	return fmt.Sprintf("line %d: frontmatter declares %s: %s. "+
		"The harness provisions from this key before any skill is read; "+
		"the `worktree` skill makes the worktree instead.", number, isolationKey, value)
}

// codeBlockSettings reports the setting inside a fenced block, which is text
// somebody runs.
//
// A fence closes on a run of the same character at least as long as the one
// that opened it, so a block quoting a fence does not end the block early.
func codeBlockSettings(body []string) []string {
	var problems []string
	opening := ""

	for i, line := range body {
		number := i + 1
		match := codeFence.FindStringSubmatch(line)
		if opening == "" {
			if match != nil {
				opening = match[1]
			}
			continue
		}

		if match != nil && match[1][0] == opening[0] && len(match[1]) >= len(opening) {
			opening = ""
			continue
		}

		if isolationSetting.MatchString(line) {
			problems = append(problems, fmt.Sprintf("line %d: code block sets %s. "+
				"A dispatch carrying it gets a worktree inside the repository root; "+
				"make one through the `worktree` skill instead.", number, isolationKey))
		}
	}

	return problems
}
