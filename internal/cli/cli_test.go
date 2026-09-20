package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), args, &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func TestNoArgumentsIsUsage(t *testing.T) {
	_, stderr, err := run(t)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("usage not written to stderr, got %q", stderr)
	}
}

func TestUnknownCommandNamesIt(t *testing.T) {
	_, stderr, err := run(t, "brew")
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
	if !strings.Contains(stderr, `unknown command "brew"`) {
		t.Errorf("stderr does not name the command, got %q", stderr)
	}
}

func TestHelpSucceedsOnStdout(t *testing.T) {
	stdout, _, err := run(t, "--help")
	if err != nil {
		t.Fatalf("help returned %v, want nil", err)
	}
	if !strings.Contains(stdout, "version") {
		t.Errorf("help does not list commands, got %q", stdout)
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
