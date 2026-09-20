// Package hook answers a harness's tool hook with one of the loop's gates.
//
// Claude Code and Codex send the same event and read the same reply: the
// fields are `hook_event_name`, `tool_name`, `cwd` and `tool_input` going in,
// and `hookSpecificOutput` coming back. Pi sends neither, and its extension
// translates its own event into this shape before calling the binary. So the
// gate is written once and each harness's difference is absorbed where it
// arises. See docs/internal/hosts.md.
//
// Nothing here ever refuses a write. A gate that bricks an editor gets
// uninstalled, and a document with a broken block is worth saying once, not
// worth stopping work over.
package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/stormlightlabs/thunderstorm/internal/check"
	"github.com/stormlightlabs/thunderstorm/internal/config"
)

// Event is one hook call.
type Event struct {
	HookEventName string         `json:"hook_event_name"`
	ToolName      string         `json:"tool_name"`
	CWD           string         `json:"cwd"`
	ToolInput     map[string]any `json:"tool_input"`
}

// Answer is what the harness reads back. An empty Answer says nothing
// happened, which is what most writes deserve.
type Answer struct {
	HookSpecificOutput *Output `json:"hookSpecificOutput,omitempty"`
}

// Output carries the reply fields both harnesses read. Only additionalContext
// is used: the gate reports, and the session decides what to do about it.
type Output struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext,omitempty"`
}

// PostToolUse is the event a write arrives on.
const PostToolUse = "PostToolUse"

// Decide runs the gate the event calls for. It returns an empty Answer, and no
// error, for every event the gate has nothing to say about.
func Decide(e Event) (Answer, error) {
	if e.HookEventName != PostToolUse {
		return Answer{}, nil
	}
	path := writtenPath(e)
	if path == "" {
		return Answer{}, nil
	}

	cwd := e.CWD
	if cwd == "" {
		var err error
		if cwd, err = os.Getwd(); err != nil {
			return Answer{}, err
		}
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}

	settings, err := config.Load(cwd)
	if err != nil {
		return Answer{}, err
	}
	root := settings.DocumentsDir()
	if root == "" || !under(root, path) {
		return Answer{}, nil
	}

	findings, err := check.FrontmatterFile(root, path)
	if err != nil || len(findings) == 0 {
		return Answer{}, err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s does not carry the frontmatter the specify skill asks for:\n",
		filepath.Base(path))
	for _, finding := range findings {
		fmt.Fprintf(&b, "  %s\n", finding)
	}
	b.WriteString("An issue cites a document by its identifier, so a document without one cannot be cited.")
	return Answer{HookSpecificOutput: &Output{HookEventName: PostToolUse, AdditionalContext: b.String()}}, nil
}

// writtenPath is the file the tool wrote, under whichever name the harness
// gives it. Claude Code and Codex send file_path; Pi's extension sends the
// path its write and edit tools take.
func writtenPath(e Event) string {
	for _, key := range []string{"file_path", "path"} {
		if value, ok := e.ToolInput[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

// under reports whether path sits inside root, by path rather than by prefix,
// so a sibling directory whose name starts the same way is not inside it.
func under(root, path string) bool {
	root, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
