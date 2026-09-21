package hook

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// commitMessage is the message a shell command would commit, and whether it
// commits at all. What cannot be read here reaches the commit-msg hook
// instead.
func commitMessage(command, cwd string) (string, bool) {
	words, ok := split(command)
	if !ok {
		return "", false
	}
	rest, committing := after(words, "git", "commit")
	if !committing {
		return "", false
	}

	for i := 0; i < len(rest); i++ {
		word := rest[i]
		switch {
		case word == "-m" || word == "--message":
			if i+1 < len(rest) {
				return rest[i+1], true
			}
		case strings.HasPrefix(word, "--message="):
			return strings.TrimPrefix(word, "--message="), true
		case word == "-F" || word == "--file":
			if i+1 < len(rest) {
				return readMessage(rest[i+1], cwd)
			}
		case strings.HasPrefix(word, "--file="):
			return readMessage(strings.TrimPrefix(word, "--file="), cwd)
		}
	}
	// An editor commit, an amend with no message, a -C reusing another: there
	// is nothing to read.
	return "", false
}

func readMessage(path, cwd string) (string, bool) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(body), true
}

// separators end one command and start the next, so that `echo git commit -m
// ...` is not read as a commit.
var separators = []string{"&&", "||", ";", "|", "&", "(", ")", "{", "}", "\n", "then", "else", "do"}

// after finds a run of words at the start of a command and returns what
// follows it. `git add -A && git commit -m ...` is searched whole.
func after(words []string, prefix ...string) ([]string, bool) {
	for i := 0; i+len(prefix) <= len(words); i++ {
		if i > 0 && !slices.Contains(separators, words[i-1]) {
			continue
		}
		if slicesEqual(words[i:i+len(prefix)], prefix) {
			return words[i+len(prefix):], true
		}
	}
	return nil, false
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// split breaks a command into words the way a shell would, enough to read an
// argument out of one. It reports false for a command substitution, an
// unclosed quote or a heredoc, which the caller treats as nothing to check.
func split(command string) ([]string, bool) {
	if strings.Contains(command, "$(") || strings.Contains(command, "`") || strings.Contains(command, "<<") {
		return nil, false
	}
	var words []string
	var word strings.Builder
	held := false
	for i := 0; i < len(command); i++ {
		c := command[i]
		switch c {
		case ' ', '\t', '\n':
			if held {
				words = append(words, word.String())
				word.Reset()
				held = false
			}
			// A newline ends a command the way a semicolon does.
			if c == '\n' {
				words = append(words, "\n")
			}
		case '\'':
			end := strings.IndexByte(command[i+1:], '\'')
			if end < 0 {
				return nil, false
			}
			word.WriteString(command[i+1 : i+1+end])
			held = true
			i += end + 1
		case '"':
			text, width, ok := doubleQuoted(command[i:])
			if !ok {
				return nil, false
			}
			word.WriteString(text)
			held = true
			i += width - 1
		case '\\':
			if i+1 >= len(command) {
				return nil, false
			}
			word.WriteByte(command[i+1])
			held = true
			i++
		default:
			word.WriteByte(c)
			held = true
		}
	}
	if held {
		words = append(words, word.String())
	}
	return words, true
}

// doubleQuoted reads one double-quoted span with the escapes a shell honours
// inside one, and returns the text and how much input it consumed.
func doubleQuoted(s string) (string, int, bool) {
	var out strings.Builder
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '"':
			return out.String(), i + 1, true
		case '\\':
			if i+1 >= len(s) {
				return "", 0, false
			}
			i++
			if next := s[i]; next == '"' || next == '\\' || next == '$' || next == '`' {
				out.WriteByte(next)
				continue
			}
			out.WriteByte('\\')
			out.WriteByte(s[i])
		default:
			out.WriteByte(s[i])
		}
	}
	return "", 0, false
}
