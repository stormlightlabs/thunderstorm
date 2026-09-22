package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stormlightlabs/thunderstorm/internal/config"
)

// Every board command builds its client from the repository it runs in, so a
// test that is not in one reaches the configuration error and stops there,
// before any request goes out.
func elsewhere(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
}

// A repository with no config file at all is told what writes one, rather
// than six key names it has to map back to the docs on its own.
func TestBoardWithNoConfigNamesInstall(t *testing.T) {
	elsewhere(t)
	_, _, err := run(t, "board", "list")
	if err == nil {
		t.Fatal("a repository with no board configured read one")
	}
	if !strings.Contains(err.Error(), "tstorm install") {
		t.Errorf("the error does not name install: %v", err)
	}
	if code := ExitCode(err); code != 2 {
		t.Errorf("exit %d, want 2: the command could not run", code)
	}
}

// A repository that already has a config file naming some board settings is
// told which ones it still left out.
func TestBoardWithAConfigNamesWhatItLeftOut(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, config.Name), []byte("[board]\nowner = \"acme\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	_, _, err := run(t, "board", "list")
	if err == nil {
		t.Fatal("a repository with an incomplete board configured read one")
	}
	for _, setting := range []string{"board.number", "board.statusField", "board.status.todo"} {
		if !strings.Contains(err.Error(), setting) {
			t.Errorf("the error does not name %s: %v", setting, err)
		}
	}
	if !strings.Contains(err.Error(), config.Name) {
		t.Errorf("the error does not name the file that has it: %v", err)
	}
}

func TestBoardRejectsAStatusTheLoopDoesNotUse(t *testing.T) {
	elsewhere(t)
	_, _, err := run(t, "board", "move", "15", "--to", "shipped")
	if err == nil {
		t.Fatal("a status outside the loop's three was accepted")
	}
	if !strings.Contains(err.Error(), "in-progress") {
		t.Errorf("the error does not list the states: %v", err)
	}
}

func TestBoardMoveNeedsAState(t *testing.T) {
	elsewhere(t)
	_, _, err := run(t, "board", "move", "15")
	if err == nil {
		t.Fatal("a move with no destination ran")
	}
	if !strings.Contains(err.Error(), "to") {
		t.Errorf("the error does not name the missing flag: %v", err)
	}
}

func TestBoardRejectsSomethingThatIsNotAnIssueNumber(t *testing.T) {
	elsewhere(t)
	_, _, err := run(t, "board", "claim", "fifteen")
	if err == nil {
		t.Fatal("a claim on something that is not an issue ran")
	}
	if !strings.Contains(err.Error(), "fifteen") {
		t.Errorf("the error does not quote the argument: %v", err)
	}
}

// A hook or a workflow is a likelier caller than a person, so every read
// takes --json.
func TestEveryBoardReadTakesJSON(t *testing.T) {
	root := Root(strings.NewReader(""), nil, nil)
	for _, command := range []string{"list", "show", "sub", "blocked-by"} {
		found, _, err := root.Find([]string{"board", command})
		if err != nil {
			t.Fatalf("no board %s command: %v", command, err)
		}
		if found.InheritedFlags().Lookup("json") == nil {
			t.Errorf("board %s takes no --json", command)
		}
	}
}
