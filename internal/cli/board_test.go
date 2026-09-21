package cli

import (
	"strings"
	"testing"
)

// Every board command builds its client from the repository it runs in, so a
// test that is not in one reaches the configuration error and stops there,
// before any request goes out.
func elsewhere(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
}

// A repository configuring the board for the first time has usually left out
// more than one setting, and learning about them one run at a time is three
// runs.
func TestBoardNamesEverySettingTheRepositoryLeftOut(t *testing.T) {
	elsewhere(t)
	_, _, err := run(t, "board", "list")
	if err == nil {
		t.Fatal("a repository with no board configured read one")
	}
	for _, setting := range []string{"board.owner", "board.number", "board.statusField", "board.status.todo"} {
		if !strings.Contains(err.Error(), setting) {
			t.Errorf("the error does not name %s: %v", setting, err)
		}
	}
	if code := ExitCode(err); code != 2 {
		t.Errorf("exit %d, want 2: the command could not run", code)
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
