package ui

import (
	"bytes"
	"os"
	"testing"
)

func TestBufferIsNeverColored(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	os.Unsetenv("NO_COLOR")
	if New(&bytes.Buffer{}, false).Colored() {
		t.Error("a buffer is not a terminal, so it must not be colored")
	}
}

// no-color.org: any value counts, the empty string included. Testing the
// empty string is the case a naive os.Getenv("NO_COLOR") != "" check fails.
func TestNoColorEnvCountsWhenEmpty(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	if !noColorEnv() {
		t.Error("NO_COLOR set to the empty string must still disable color")
	}
}

func TestNoColorEnvAbsent(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	os.Unsetenv("NO_COLOR")
	if noColorEnv() {
		t.Error("NO_COLOR unset must not disable color")
	}
}
