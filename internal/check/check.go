// Package check holds the gates the loop runs.
//
// Each gate was a Python or shell script that a repository got by copying and
// fixed by copying again, and that needed an interpreter on PATH to run at
// all. A hook that fails because the interpreter moved teaches an operator to
// ignore hooks, so the gates live in the binary the hooks already call.
//
// Every gate reports the same three things through its exit code: 0 for clean,
// 1 for findings, 2 for a gate that could not run. A caller can tell "your
// prose is bad" from "I could not read the file".
package check

import (
	"os/exec"
	"strings"
)

// git runs a git command and returns its output, or false when git failed.
// Every caller here treats a failure as an absent answer rather than an error,
// because each one has a sensible reading for "git would not say".
func git(args ...string) (string, bool) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

// lines splits text the way a file's reader sees it, with no trailing empty
// element for a file that ends in a newline.
func lines(text string) []string {
	text = strings.TrimSuffix(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
