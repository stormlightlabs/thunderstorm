package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	err := Execute(context.Background(), args, &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func TestBareInvocationShowsHelp(t *testing.T) {
	stdout, _, err := run(t)
	if err != nil {
		t.Fatalf("bare invocation returned %v, want nil", err)
	}
	if !strings.Contains(stdout, "Usage:") {
		t.Errorf("help not on stdout, got %q", stdout)
	}
}

func TestUnknownCommandFails(t *testing.T) {
	_, _, err := run(t, "brew")
	if err == nil {
		t.Fatal("unknown command returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "brew") {
		t.Errorf("error does not name the command: %v", err)
	}
}

func TestVersionPrintsABuild(t *testing.T) {
	stdout, _, err := run(t, "version")
	if err != nil {
		t.Fatalf("version returned %v", err)
	}
	if strings.TrimSpace(stdout) == "" {
		t.Error("version printed nothing")
	}
}

// A buffer is not a terminal, so nothing tstorm writes to one may carry escape
// sequences. This is the check that keeps hook output and CI logs clean.
func TestOutputToABufferIsPlain(t *testing.T) {
	stdout, _, err := run(t, "version")
	if err != nil {
		t.Fatalf("version returned %v", err)
	}
	if strings.Contains(stdout, "\x1b[") {
		t.Errorf("escape sequence written to a non-terminal: %q", stdout)
	}
}

func TestRenderNeedsATarget(t *testing.T) {
	_, _, err := run(t, "render")
	if err == nil {
		t.Fatal("render without --target returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "target") {
		t.Errorf("error does not name the missing flag: %v", err)
	}
}

func TestRenderRejectsAnUnknownTarget(t *testing.T) {
	_, _, err := run(t, "render", "--target", "emacs")
	if err == nil {
		t.Fatal("an unknown target returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "claude") {
		t.Errorf("error does not list the targets that exist: %v", err)
	}
}

func TestDispatchRefusesAHarnessItCannotDrive(t *testing.T) {
	_, _, err := run(t, "dispatch", "--harness", "claude", "--role", "reviewer",
		"--worktree", t.TempDir(), "--model", "m", "review it")
	if err == nil {
		t.Fatal("dispatch on claude returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "subagent") {
		t.Errorf("the error does not say what drives a role there: %v", err)
	}
}

func TestDispatchRefusesAnUnknownRole(t *testing.T) {
	_, _, err := run(t, "dispatch", "--role", "orchestrator",
		"--worktree", t.TempDir(), "--model", "m", "run it")
	if err == nil {
		t.Fatal("dispatch of an unknown role returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "implementer") {
		t.Errorf("the error does not list the roles: %v", err)
	}
}
