package config

import (
	"os"
	"path/filepath"
	"strings"
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

// A repository configuring the board for the first time has usually left out
// more than one setting. One error names them all.
func TestMissingBoardSettingsAreNamedTogether(t *testing.T) {
	err := Board{Owner: "stormlightlabs", Number: 13}.Validate()
	if err == nil {
		t.Fatal("a board with no status field validated")
	}
	for _, setting := range []string{"board.statusField", "board.status.todo", "board.status.done"} {
		if !strings.Contains(err.Error(), setting) {
			t.Errorf("the error does not name %s: %v", setting, err)
		}
	}
	if strings.Contains(err.Error(), "board.owner") {
		t.Errorf("the error names a setting that is there: %v", err)
	}
}

// A board that carries one repository needs no grouping field. One that names
// a field and no value would file work where no read finds it.
func TestAGroupFieldWithoutAValueIsAnError(t *testing.T) {
	complete := Board{
		Owner: "stormlightlabs", Number: 13, StatusField: "Status",
		Status: Status{Todo: "Todo", InProgress: "In Progress", Done: "Done"},
	}
	if err := complete.Validate(); err != nil {
		t.Fatalf("a board carrying one repository failed: %v", err)
	}
	complete.GroupField = "Track"
	if err := complete.Validate(); err == nil {
		t.Error("a grouping field with no value validated")
	}
}

// The board is read from the same file as everything else, so a repository
// configures the loop in one place.
func TestTheBoardIsReadFromTheSameFile(t *testing.T) {
	root := t.TempDir()
	write(t, root, `{"documents":"docs","board":{"owner":"stormlightlabs","number":13,
		"statusField":"Status","status":{"todo":"Todo","inProgress":"In Progress","done":"Done"},
		"groupField":"Track","groupValue":"Thunderstorm"}}`)

	settings, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Board.Number != 13 || settings.Board.Status.InProgress != "In Progress" {
		t.Errorf("read %+v", settings.Board)
	}
	if err := settings.Board.Validate(); err != nil {
		t.Errorf("a complete board failed: %v", err)
	}
}
