package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These cases are ported from workflow/scripts/check-commit-message-test.py,
// which the Go check replaced.
//
// The ones that matter most are about severity. A shape error has to fail the
// hook and a length finding has to not, and the two are one branch apart in
// the check. If that branch ever inverts, a long body starts rejecting
// commits, authors learn --no-verify, and the shape checks stop running at
// all. The exit-status assertions below are what stands between that and the
// repository.

// message writes one message to a file and checks it.
func message(t *testing.T, text string, args ...string) (string, string, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return run(t, append([]string{"check", "commit-message", path}, args...)...)
}

// pullRequest checks a title and a body, each held in its own file.
func pullRequest(t *testing.T, title, body string, args ...string) (string, string, error) {
	t.Helper()
	dir := t.TempDir()
	titlePath := filepath.Join(dir, "title.txt")
	bodyPath := filepath.Join(dir, "body.txt")
	for path, text := range map[string]string{titlePath: title, bodyPath: body} {
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return run(t, append([]string{
		"check", "commit-message", "--pr",
		"--title-file", titlePath, "--body-file", bodyPath,
	}, args...)...)
}

func body(lines int) string {
	out := make([]string, lines)
	for i := range out {
		out[i] = fmt.Sprintf("Sentence %d of the body.", i)
	}
	return strings.Join(out, "\n")
}

func TestACleanMessagePassesSilently(t *testing.T) {
	_, stderr, err := message(t, "fix: collapse nested if in percentage check\n\nClippy fires here.\n")
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v", code, err)
	}
	if !strings.Contains(stderr, "clean") {
		t.Errorf("did not say it was clean: %q", stderr)
	}
}

// Shape is not a judgement call, so it fails the hook.
func TestShapeFailsTheHook(t *testing.T) {
	for name, text := range map[string]string{
		"no type":         "Fix the thing\n",
		"capitalised":     "fix: Collapse the nested if\n",
		"trailing period": "fix: collapse the nested if.\n",
		"body glued on":   "fix: collapse it\nWhy it changed\n",
		"wide body line":  "fix: collapse it\n\n" + strings.Repeat("w", 40) + " " + strings.Repeat("w", 40) + "\n",
		"unclosed fence":  "fix: collapse it\n\n```text\nsome output\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, stderr, err := message(t, text)
			if code := ExitCode(err); code != 1 {
				t.Fatalf("exit %d, want 1: %v", code, err)
			}
			if !strings.Contains(stderr, "error:") {
				t.Errorf("no error reported: %q", stderr)
			}
		})
	}
}

// The squash discards a branch commit body, so it carries no target.
func TestALongBranchBodySaysNothing(t *testing.T) {
	_, stderr, err := message(t, "fix: collapse the nested if\n\n"+body(30)+"\n")
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v", code, err)
	}
	if strings.Contains(stderr, "length:") {
		t.Errorf("graded a body the squash drops: %q", stderr)
	}
}

// Every overlong subject on this repository's trunk arrived this way: a title
// inside the limit, plus the suffix GitHub appends. Blaming the author for it
// sends them to cut a title that was already short enough.
func TestASuffixInducedOverflowIsAdvice(t *testing.T) {
	_, stderr, err := message(t, "docs: sharpen review passes and add writing length targets (#23)\n")
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v", code, err)
	}
	if !strings.Contains(stderr, "length:") {
		t.Fatalf("said nothing about the width: %q", stderr)
	}
	if !strings.Contains(stderr, "53") || !strings.Contains(stderr, "(#NN)") {
		t.Errorf("advice does not name the title budget: %q", stderr)
	}
}

func TestATitleOverTheLimitAloneIsRejected(t *testing.T) {
	_, stderr, err := message(t, "docs: "+strings.Repeat("w", 60)+"\n")
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit %d, want 1: %v", code, err)
	}
	if !strings.Contains(stderr, "error:") {
		t.Errorf("no error reported: %q", stderr)
	}
}

// A title heading for overflow is worth saying before the merge, not after.
func TestATitlePastTheSquashBudgetIsFlaggedEarly(t *testing.T) {
	_, stderr, err := message(t, "docs: "+strings.Repeat("w", 50)+"\n")
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v", code, err)
	}
	if !strings.Contains(stderr, "squash") {
		t.Errorf("advice does not mention the squash: %q", stderr)
	}
}

// CI reports and never fails, whatever it finds.
func TestWarnReportsAShapeErrorWithoutFailing(t *testing.T) {
	stdout, _, err := message(t, "Fix the thing\n", "--warn")
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v", code, err)
	}
	if !strings.Contains(stdout, "error:") {
		t.Errorf("--warn did not report on stdout: %q", stdout)
	}
}

// git deletes comments and the scissors block after the hook runs, so grading
// them rejects an author for a diff that never becomes part of the message.
func TestTheDiffUnderAScissorsLineIsNotGraded(t *testing.T) {
	var b strings.Builder
	b.WriteString("fix: collapse the nested if\n\nWhy it changed.\n\n")
	b.WriteString("# ------------------------ >8 ------------------------\n")
	b.WriteString("diff --git a/very/long/path/that/would/blow/the/column/limit.rs b/x.rs\n")
	for i := range 40 {
		fmt.Fprintf(&b, "+    line %d\n", i)
	}
	_, stderr, err := message(t, b.String())
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v", code, err)
	}
	if strings.Contains(stderr, "length:") || strings.Contains(stderr, "error:") {
		t.Errorf("graded the diff: %q", stderr)
	}
}

// What reaches the trunk is the pull request title and body, joined the way
// git joins a subject and a body. Everything below grades that text.
const prTitle = "fix: stop dropping skills over a name mismatch"

func TestACleanPullRequestPasses(t *testing.T) {
	_, stderr, err := pullRequest(t, prTitle, "Skill discovery keyed on the directory name.\n")
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v", code, err)
	}
	if !strings.Contains(stderr, "clean") {
		t.Errorf("did not say it was clean: %q", stderr)
	}
}

// The body is the commit body now, so its target applies here and nowhere else.
func TestALongPullRequestBodyAdvisesAndStillPasses(t *testing.T) {
	_, stderr, err := pullRequest(t, prTitle, body(30))
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v", code, err)
	}
	for _, want := range []string{"length:", "commit body", "fails nothing"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("advice does not say %q: %q", want, stderr)
		}
	}
	if strings.Contains(stderr, "error:") {
		t.Errorf("a long body was called an error: %q", stderr)
	}
}

// The limit is inclusive.
func TestABodyExactlyAtTheTargetSaysNothing(t *testing.T) {
	_, stderr, err := pullRequest(t, prTitle, body(20))
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v", code, err)
	}
	if strings.Contains(stderr, "length:") {
		t.Errorf("graded a body at its target: %q", stderr)
	}
}

// GitHub appends " (#NN)" to the title. The author never sees it, so the
// advice has to name the merged width rather than the width they typed.
func TestTheAdviceNamesTheMergedWidth(t *testing.T) {
	_, stderr, err := pullRequest(t, "docs: "+strings.Repeat("w", 50), "Why it changed.\n")
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v", code, err)
	}
	if !strings.Contains(stderr, "length:") || !strings.Contains(stderr, "62") {
		t.Errorf("advice does not name the merged width: %q", stderr)
	}
}

// Shape is shape wherever the text came from.
func TestAPullRequestTitleWithNoTypeIsRejected(t *testing.T) {
	_, stderr, err := pullRequest(t, "Stop dropping skills", "Why it changed.\n")
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit %d, want 1: %v", code, err)
	}
	if !strings.Contains(stderr, "error:") {
		t.Errorf("no error reported: %q", stderr)
	}
	if _, _, err := pullRequest(t, "Stop dropping skills", "Why.\n", "--warn"); ExitCode(err) != 0 {
		t.Errorf("--warn failed a pull request shape error: %v", err)
	}
}

// A body wrapped past 72 columns lands in git log wrapped past 72 columns.
func TestAWidePullRequestBodyLineIsRejected(t *testing.T) {
	_, stderr, err := pullRequest(t, prTitle, strings.Repeat("w", 40)+" "+strings.Repeat("w", 40)+"\n")
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit %d, want 1: %v", code, err)
	}
	if !strings.Contains(stderr, "error:") {
		t.Errorf("no error reported: %q", stderr)
	}
}

// A pull request with no body is a title alone, not a message glued to one.
func TestATitleWithNoBodyIsClean(t *testing.T) {
	_, stderr, err := pullRequest(t, prTitle, "")
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v", code, err)
	}
	if strings.Contains(stderr, "error:") {
		t.Errorf("a title alone was rejected: %q", stderr)
	}
}

// The title is read from a file precisely so it never reaches a shell, and an
// invocation that names only half of one is the gate failing to run rather
// than the text being wrong.
func TestPullRequestModeNeedsBothFiles(t *testing.T) {
	_, _, err := run(t, "check", "commit-message", "--pr", "--title-file", "only.txt")
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit %d, want 2: %v", code, err)
	}
	if !strings.Contains(err.Error(), "--body-file") {
		t.Errorf("error does not name the missing flag: %v", err)
	}
}

func TestAMessageWithNoFileIsAUsageError(t *testing.T) {
	_, _, err := run(t, "check", "commit-message")
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit %d, want 2: %v", code, err)
	}
}
