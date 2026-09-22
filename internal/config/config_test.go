package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A check runs from wherever the harness started it, which is rarely the
// repository root.
func TestASettingIsFoundFromASubdirectory(t *testing.T) {
	root := t.TempDir()
	write(t, root, Name, `documents = "docs/internal"`)
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
	write(t, root, Name, "documents = nope")
	if _, err := Load(root); err == nil {
		t.Error("a malformed file loaded without complaint")
	}

	json := t.TempDir()
	write(t, json, ".tstorm.json", "{documents: nope}")
	if _, err := Load(json); err == nil {
		t.Error("a malformed JSON file loaded without complaint")
	}
}

// A repository configuring the board for the first time has usually left out
// more than one setting. One error names them all.
func TestMissingBoardSettingsAreNamedTogether(t *testing.T) {
	b := Board{Owner: "stormlightlabs", Number: 13}
	b.file = Name
	err := b.Validate()
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
	write(t, root, Name, `
documents = "docs"

[board]
owner = "stormlightlabs"
number = 13
statusField = "Status"
groupField = "Track"
groupValue = "Thunderstorm"

[board.status]
todo = "Todo"
inProgress = "In Progress"
done = "Done"
`)

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

// A repository that installed the loop before TOML already has a .tstorm.json,
// and it keeps being read under the names it used.
func TestAJSONFileStillLoads(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".tstorm.json", `{"documents":"docs","board":{"owner":"stormlightlabs",
		"number":13,"statusField":"Status","status":{"todo":"Todo","inProgress":"In Progress",
		"done":"Done"}},"prose":{"mute":[{"rule":"structure.fragments","why":"noisy"}]}}`)

	settings, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Board.Number != 13 || settings.Board.Status.InProgress != "In Progress" {
		t.Errorf("read %+v", settings.Board)
	}
	if got := settings.Rules().Mute; len(got) != 1 || got[0] != "structure.fragments" {
		t.Errorf("muted %v", got)
	}
}

// Both formats in one directory is a repository midway through converting.
// The order of Names decides, so the answer does not depend on the filesystem.
func TestTOMLWinsOverJSONInTheSameDirectory(t *testing.T) {
	root := t.TempDir()
	write(t, root, Name, `documents = "from-toml"`)
	write(t, root, ".tstorm.json", `{"documents":"from-json"}`)

	settings, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "from-toml"); settings.DocumentsDir() != want {
		t.Errorf("documents resolved to %q, want %q", settings.DocumentsDir(), want)
	}
}

// The walk upward looks for both names at each level, so a nested repository
// on JSON is not overridden by a TOML file further up.
func TestTheNearestDirectoryWinsWhicheverFormatItUses(t *testing.T) {
	root := t.TempDir()
	write(t, root, Name, `documents = "outer"`)
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, nested, ".tstorm.json", `{"documents":"inner"}`)

	settings, err := Load(nested)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(nested, "inner"); settings.DocumentsDir() != want {
		t.Errorf("documents resolved to %q, want %q", settings.DocumentsDir(), want)
	}
}

// TOML has comments, so a muted rule is the id alone and the reason sits above
// it. The rest of the binary sees the same list either format produced.
func TestAMutedRuleInTOMLIsTheIDAlone(t *testing.T) {
	root := t.TempDir()
	write(t, root, Name, `
[prose]
dictionary = "trps.toml"
mute = [
  # 77 findings against prose a reader called clean
  "structure.short_punchy_fragments",
  "repetition.anaphora_abuse",
]
`)

	settings, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	rules := settings.Rules()
	if want := filepath.Join(root, "trps.toml"); rules.Dictionary != want {
		t.Errorf("dictionary resolved to %q, want %q", rules.Dictionary, want)
	}
	if len(rules.Mute) != 2 || rules.Mute[0] != "structure.short_punchy_fragments" {
		t.Errorf("muted %v", rules.Mute)
	}
}

// Somebody converting a file by hand will carry the JSON table across. Saying
// what the setting takes beats muting nothing and reporting success.
func TestAMutedRuleInTOMLRejectsATable(t *testing.T) {
	root := t.TempDir()
	write(t, root, Name, "[prose]\nmute = [{ rule = \"structure.fragments\", why = \"noisy\" }]\n")

	_, err := Load(root)
	if err == nil {
		t.Fatal("a muted rule written as a table loaded without complaint")
	}
	if !strings.Contains(err.Error(), "prose.mute") {
		t.Errorf("the error does not name the setting: %v", err)
	}
}

// Converting a repository starts with a file that holds nothing yet. The first
// file found is the only one read, so an empty one that loaded would take the
// settings of the .tstorm.json beside it with no sign it had.
func TestAnEmptyFileDoesNotShadowTheOneBesideIt(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".tstorm.json", `{"documents":"docs","board":{"owner":"acme","number":7}}`)
	write(t, root, Name, "")

	_, err := Load(root)
	if err == nil {
		t.Fatal("an empty file loaded and hid the settings beside it")
	}
	if !strings.Contains(err.Error(), Name) {
		t.Errorf("the error does not name the empty file: %v", err)
	}

	// A file carrying only the comments somebody started with says as little.
	write(t, root, Name, "# the board lives on 13\n")
	if _, err := Load(root); err == nil {
		t.Error("a file of nothing but comments loaded")
	}
}

// A repository still on .tstorm.json is not helped by being told to set a
// value in a file it does not have.
func TestADiagnosticNamesTheFileThatWasRead(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".tstorm.json", `{"board":{"owner":"acme","number":7}}`)

	settings, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if settings.File() != ".tstorm.json" {
		t.Errorf("File() is %q, want .tstorm.json", settings.File())
	}
	err = settings.Board.Validate()
	if err == nil {
		t.Fatal("an incomplete board validated")
	}
	if !strings.Contains(err.Error(), ".tstorm.json") {
		t.Errorf("the error names a file this repository does not have: %v", err)
	}
}

// With no file anywhere there is no file to name, so a message asking for a
// board setting names the command that writes one instead of six keys a
// reader has to map back to the docs on their own.
func TestWithNoFileTheDiagnosticNamesInstall(t *testing.T) {
	settings, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if settings.File() != Name {
		t.Errorf("File() is %q, want %q", settings.File(), Name)
	}
	err = settings.Board.Validate()
	if err == nil {
		t.Fatal("an incomplete board with no file validated")
	}
	if !strings.Contains(err.Error(), "tstorm install") {
		t.Errorf("the error does not name install: %v", err)
	}
	if strings.Contains(err.Error(), "board.owner") {
		t.Errorf("the error lists keys instead of the command that writes them: %v", err)
	}
}

// A repository that has not moved to TOML keeps its JSON, and can put the
// reason for a setting beside it.
func TestACommentedJSONConfigIsRead(t *testing.T) {
	root := t.TempDir()
	body := `{
  // the tree an issue cites a document by
  "documents": "docs/internal",
  "board": {
    "owner": "stormlightlabs",
    "number": 13, // project 13 carries several repositories
  },
}`
	if err := os.WriteFile(filepath.Join(root, ".tstorm.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	settings, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if settings.Documents != "docs/internal" || settings.Board.Number != 13 {
		t.Errorf("read %+v", settings)
	}
}
