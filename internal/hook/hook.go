// Package hook answers a harness's tool hook with one of the loop's gates.
//
// Claude Code and Codex send the same event and read the same reply: the
// fields are `hook_event_name`, `tool_name`, `cwd` and `tool_input` going in,
// and `hookSpecificOutput` coming back. Pi sends neither, and its extension
// translates its own event into this shape before calling the binary. So the
// gate is written once and each harness's difference is absorbed where it
// arises. See docs/internal/hosts.md.
//
// A write is never refused; the gates report and the session decides. A
// commit is the exception: a message whose shape is wrong is denied, and what
// the prose gate finds in it is put to the person as an ask.
package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

// Output carries the reply fields both harnesses read. A write answers with
// additionalContext, a commit with permissionDecision.
type Output struct {
	HookEventName            string `json:"hookEventName"`
	AdditionalContext        string `json:"additionalContext,omitempty"`
	PermissionDecision       string `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
}

// The events the gates answer on, under the same names on Claude Code and
// Codex.
const (
	PostToolUse = "PostToolUse"
	PreToolUse  = "PreToolUse"
)

// What a PreToolUse answer can say. Deny stops the command, Ask puts it to
// the person.
const (
	Deny = "deny"
	Ask  = "ask"
)

// Decide runs the gate the event calls for. It returns an empty Answer, and no
// error, for every event the gate has nothing to say about.
func Decide(e Event) (Answer, error) {
	switch e.HookEventName {
	case PreToolUse:
		return decideCommand(e)
	case PostToolUse:
		return decideWrite(e)
	default:
		return Answer{}, nil
	}
}

// decideCommand answers for a commit, and says nothing about any other
// command. Shape denies; prose asks.
func decideCommand(e Event) (Answer, error) {
	command, _ := e.ToolInput["command"].(string)
	if command == "" {
		return Answer{}, nil
	}
	cwd, err := where(e)
	if err != nil {
		return Answer{}, err
	}
	text, ok := commitMessage(command, cwd)
	if !ok {
		return Answer{}, nil
	}

	problems, _ := check.Commit(check.Message{Text: text})
	if len(problems) > 0 {
		var b strings.Builder
		b.WriteString("The commit message needs a shape fix before this runs:\n")
		for _, problem := range problems {
			fmt.Fprintf(&b, "  %s\n", problem)
		}
		b.WriteString("The shape is in the commits-and-prs skill, and `tstorm check commit-message` reads it.")
		return decision(Deny, b.String()), nil
	}

	settings, err := config.Load(cwd)
	if err != nil {
		return Answer{}, err
	}
	findings, err := check.ProseText(text, settings.Rules())
	if err != nil || len(findings) == 0 {
		// A missing tropius reads the same as a clean message.
		return Answer{}, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "The commit message carries %d of the tells writing-docs catalogues:\n", len(findings))
	for _, finding := range findings {
		fmt.Fprintf(&b, "  line %d: %s: %s\n", finding.Line, finding.Rule, finding.Matched)
	}
	b.WriteString("A commit message is read far more often than the diff. Rewrite it, or say why it stands.")
	return decision(Ask, b.String()), nil
}

// decision is a PreToolUse answer; the harness shows the reason.
func decision(what, why string) Answer {
	return Answer{HookSpecificOutput: &Output{
		HookEventName:            PreToolUse,
		PermissionDecision:       what,
		PermissionDecisionReason: why,
	}}
}

// where is the directory the event came from, or this process's own.
func where(e Event) (string, error) {
	if e.CWD != "" {
		return e.CWD, nil
	}
	return os.Getwd()
}

// decideWrite answers for a file a session just wrote.
func decideWrite(e Event) (Answer, error) {
	path := writtenPath(e)
	if path == "" {
		return Answer{}, nil
	}

	cwd, err := where(e)
	if err != nil {
		return Answer{}, err
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}

	settings, err := config.Load(cwd)
	if err != nil {
		return Answer{}, err
	}

	var said []string
	if root := settings.DocumentsDir(); root != "" && under(root, path) {
		findings, err := check.FrontmatterFile(root, path)
		if err != nil {
			return Answer{}, err
		}
		if len(findings) > 0 {
			var b strings.Builder
			fmt.Fprintf(&b, "%s does not carry the frontmatter the specify skill asks for:\n",
				filepath.Base(path))
			for _, finding := range findings {
				fmt.Fprintf(&b, "  %s\n", finding)
			}
			b.WriteString("An issue cites a document by its identifier, so a document without one cannot be cited.")
			said = append(said, b.String())
		}
	}
	if note := prose(settings, cwd, path); note != "" {
		said = append(said, note)
	}
	if len(said) == 0 {
		return Answer{}, nil
	}
	return Answer{HookSpecificOutput: &Output{
		HookEventName:     PostToolUse,
		AdditionalContext: strings.Join(said, "\n\n"),
	}}, nil
}

// prose reports what tropius found in a file a session just wrote. A missing
// tropius and a clean file read the same: nothing to add.
func prose(settings config.Config, cwd, path string) string {
	if !slices.Contains(check.ProseFiles, strings.ToLower(filepath.Ext(path))) {
		return ""
	}
	if !under(cwd, path) {
		return ""
	}
	findings, err := check.Prose([]string{path}, settings.Rules())
	if err != nil || len(findings) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s carries %d of the tells writing-docs catalogues:\n", filepath.Base(path), len(findings))
	for _, finding := range findings {
		if rel, err := filepath.Rel(cwd, finding.Path); err == nil {
			finding.Path = rel
		}
		fmt.Fprintf(&b, "  %s\n", finding)
	}
	b.WriteString("Read them against references/tells.md and decide; the gate counts tropes and judges nothing.")
	return b.String()
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
