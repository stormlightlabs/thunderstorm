package check

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The limits below are the whole policy; change them here and the hook, CI,
// and the commits-and-prs skill stay in step.
const (
	// SubjectLimit is "under 60 characters", so 59 is the longest allowed.
	SubjectLimit = 60
	BodyLimit    = 72
	// PRBodyLines is the target for a pull request body. That body becomes the
	// squash commit body verbatim, so it is counted in lines at 72 columns
	// like any other commit body. Twenty lines at that width is about 150
	// words, the figure docs/internal cites for a focused change.
	//
	// A branch commit body has no target. The squash discards it, so grading
	// it would spend a reader's attention on text nobody reads.
	PRBodyLines = 20
	// squashSuffixWidth covers the " (#NN)" GitHub appends to a squash subject
	// server-side: six characters for a two-digit issue, seven past #99. The
	// author never sees them, so the title they wrote is the only part the
	// budget covers.
	squashSuffixWidth = 6
	titleBudget       = SubjectLimit - 1 - squashSuffixWidth
)

// Types are the commit types the subject may open with.
var Types = []string{"feat", "fix", "docs", "refactor", "test", "chore", "perf"}

var (
	subjectPattern = regexp.MustCompile(`^(` + strings.Join(Types, "|") + `): (.+)$`)
	// A trailer key is hyphenated (Co-Authored-By, Signed-off-by) or one of
	// the few single words git tooling writes. `^[A-Za-z][A-Za-z-]*: ` also
	// matched "Note: ..." and "Result: ...", so a closing paragraph of
	// ordinary sentences exempted itself from the column limit.
	trailerPattern = regexp.MustCompile(`^(?:[A-Za-z][A-Za-z0-9]*(?:-[A-Za-z0-9]+)+|Closes|Fixes|Refs|Cc|Change-Id): .+$`)
	squashSuffix   = regexp.MustCompile(` \(#\d+\)$`)
)

// Message is one commit message and where it came from.
//
// FromFile says the text is still in an editor, so the comment lines git is
// about to delete have to be stripped before anything is graded.
// FromPullRequest says the subject is a pull request title, which is the one
// subject the " (#NN)" budget is certain to apply to.
type Message struct {
	Text            string
	FromFile        bool
	FromPullRequest bool
}

// Commit returns what is wrong with a message: problems that fail a run, and
// advice that never does.
//
// A problem is a shape a reader cannot recover from: a missing type, a subject
// that overflows the column `git log --oneline` gives it, a body glued to its
// subject. Shape is not a judgement call, so it exits non-zero, which is what
// the commit-msg hook wants: the message is still in the editor and costs
// nothing to fix.
//
// Advice is length. A body over its target is usually padding, but sometimes a
// change earns the room, and no check can tell those apart. Rejecting a
// message for length would teach authors to reach for --no-verify, which skips
// the shape checks too, so the check that cannot be certain stays out of the
// way of the one that can.
func Commit(m Message) (problems, advice []string) {
	body := messageLines(m.Text, m.FromFile)
	if len(body) == 0 {
		return []string{"the message is empty"}, nil
	}

	subject := body[0]
	if match := subjectPattern.FindStringSubmatch(subject); match == nil {
		problems = append(problems, "subject must read '<type>: <what changed>', where type is one of "+
			strings.Join(Types, ", "))
	} else {
		rest := match[2]
		if first, _ := utf8.DecodeRuneInString(rest); unicode.IsUpper(first) {
			problems = append(problems, "subject starts with a capital after the type")
		}
		if strings.HasSuffix(rest, ".") {
			problems = append(problems, "subject ends with a period")
		}
	}

	// Measure the title the author wrote, not the suffix GitHub bolted on.
	// Every overlong subject on this repository's trunk got there the second
	// way, and reporting those as the author's error sends them to shorten a
	// title that was already inside the limit.
	title := squashSuffix.ReplaceAllString(subject, "")
	switch {
	case width(title) >= SubjectLimit:
		problems = append(problems, fmt.Sprintf("subject is %d characters, over the %d allowed",
			width(title), SubjectLimit-1))
	case width(subject) >= SubjectLimit:
		advice = append(advice, fmt.Sprintf("subject reaches %d characters once GitHub's '(#NN)' suffix "+
			"is added. A pull request title has about %d characters before the squash overflows",
			width(subject), titleBudget))
	case m.FromPullRequest && width(title) > titleBudget:
		advice = append(advice, fmt.Sprintf("title is %d characters; the merged subject will be %d once "+
			"GitHub appends '(#NN)', over the %d allowed",
			width(title), width(title)+squashSuffixWidth, SubjectLimit-1))
	case m.FromFile && width(title) > titleBudget:
		// In the editor nothing knows whether this subject becomes a title, so
		// the budget is advice rather than the verdict it is on a pull request.
		advice = append(advice, fmt.Sprintf("subject is %d characters; if it becomes a pull request title, "+
			"the squash appends '(#NN)' and lands over the limit", width(title)))
	}

	if len(body) > 1 && strings.TrimSpace(body[1]) != "" {
		problems = append(problems, "no blank line between the subject and the body")
	}

	trailersFrom := trailerBlock(body)
	problems = append(problems, columnProblems(body, trailersFrom)...)

	if m.FromPullRequest {
		if length := bodyLength(body, trailersFrom); length > PRBodyLines {
			advice = append(advice, fmt.Sprintf("body is %d lines against a %d-line target, and it is the "+
				"commit body. Cut what the diff already says; keep what it cannot say", length, PRBodyLines))
		}
	}

	return problems, advice
}

// columnProblems reports every body line over the column limit.
//
// A fenced block holds output or commands that wrapping would corrupt, and a
// line without spaces is a URL or a path that cannot be wrapped at all.
func columnProblems(body []string, trailersFrom int) []string {
	var problems []string
	fenced := false
	fenceOpenedAt := 0
	for i, line := range body[1:] {
		number := i + 2
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "```") {
			fenced = !fenced
			if fenced {
				fenceOpenedAt = number
			}
			continue
		}
		if fenced || !strings.Contains(strings.TrimSpace(line), " ") {
			continue
		}
		if number-1 >= trailersFrom {
			continue
		}
		if width(line) > BodyLimit {
			problems = append(problems, fmt.Sprintf("line %d is %d characters, over the %d allowed",
				number, width(line), BodyLimit))
		}
	}
	if fenced {
		problems = append(problems, fmt.Sprintf("the code fence opened on line %d is never closed, "+
			"so everything below it skipped the column check", fenceOpenedAt))
	}
	return problems
}

// PullRequestMessage joins a pull request's title and body the way a squash
// merge does.
//
// This repository merges with squash_merge_commit_title=PR_TITLE and
// squash_merge_commit_message=PR_BODY, so the two reach the trunk exactly as
// git joins a subject and a body, and the branch's own messages are discarded.
// GitHub appends " (#NN)" at merge time; it is not added here, because the
// budget covers the part the author controls.
func PullRequestMessage(title, body string) string {
	title = strings.TrimSpace(title)
	body = strings.Trim(body, "\n")
	if body == "" {
		return title
	}
	return title + "\n\n" + body
}

// messageLines returns the lines of a message that survive into history.
func messageLines(text string, fromFile bool) []string {
	var out []string
	if fromFile {
		// A stored message carries no comments, so stripping them there would
		// drop a real subject that happens to start with the comment character.
		out = editableLines(text)
	} else {
		out = lines(text)
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	return out
}

// editableLines strips what git itself removes from a message being edited.
//
// Comments and everything below the scissors line are deleted by git after the
// commit-msg hook runs, so grading them rejects an author for a verbose diff
// that never becomes part of the message.
func editableLines(text string) []string {
	char := commentChar()
	var out []string
	for _, line := range lines(text) {
		if strings.HasPrefix(line, char) {
			// "# ------------------------ >8 ------------------------"
			if strings.Contains(line, " >8 ") {
				break
			}
			continue
		}
		out = append(out, line)
	}
	return out
}

// commentChar is the character git strips from a message being edited.
func commentChar() string {
	value, ok := git("config", "--get", "core.commentChar")
	value = strings.TrimSpace(value)
	// "auto" asks git to pick one per message; it picks '#' unless the message
	// already starts a line with it, which a message about to be rejected for
	// its subject will not.
	if !ok || value == "" || value == "auto" {
		return "#"
	}
	return string([]rune(value)[0])
}

// trailerBlock returns the index of the first line of the trailing trailer
// paragraph, or len(body).
//
// Only the final paragraph can hold trailers. Exempting every `Word: value`
// line anywhere in the body would let an ordinary sentence opening with
// "Note: " run to any width.
func trailerBlock(body []string) int {
	end := len(body)
	start := end
	for start > 0 && strings.TrimSpace(body[start-1]) != "" {
		start--
	}
	if start == end {
		return end
	}
	for _, line := range body[start:end] {
		if !trailerPattern.MatchString(line) {
			return end
		}
	}
	return start
}

// bodyLength is the lines of body a reader actually reads.
//
// The trailers the harness appends are excluded because the author does not
// write them and cannot shorten them. Blank lines between paragraphs are
// counted, because a reader scrolls past those too.
func bodyLength(body []string, trailersFrom int) int {
	text := body[1:trailersFrom]
	for len(text) > 0 && strings.TrimSpace(text[0]) == "" {
		text = text[1:]
	}
	for len(text) > 0 && strings.TrimSpace(text[len(text)-1]) == "" {
		text = text[:len(text)-1]
	}
	return len(text)
}

// width counts what a terminal shows rather than what the file stores, so an
// accented character costs one column and not two bytes.
func width(s string) int { return utf8.RuneCountInString(s) }
