package config

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, Name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A check runs from wherever the harness started it, which is rarely the
// repository root.
func TestASettingIsFoundFromASubdirectory(t *testing.T) {
	root := t.TempDir()
	write(t, root, `{"documents": "docs/internal"}`)
	deep := filepath.Join(root, "internal", "cli")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	settings, err := Load(deep)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "docs", "internal"); settings.DocumentsDir() != want {
		t.Errorf("documents resolved to %q, want %q", settings.DocumentsDir(), want)
	}
}

// A repository that has configured nothing has not asked for these checks,
// which is not the same as a repository that is broken.
func TestNoFileIsNotAnError(t *testing.T) {
	settings, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("a missing file failed: %v", err)
	}
	if settings.DocumentsDir() != "" {
		t.Errorf("invented a documents tree: %q", settings.DocumentsDir())
	}
}

// A file that is there and unreadable is a repository saying something wrong,
// which is worth stopping for.
func TestAMalformedFileIsAnError(t *testing.T) {
	root := t.TempDir()
	write(t, root, "{documents: nope}")
	if _, err := Load(root); err == nil {
		t.Error("a malformed file loaded without complaint")
	}
}
