package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These cases are ported from workflow/scripts/check-frontmatter-test.py.
//
// They are the ways a hand-written block goes wrong: a file that never got
// one, a key copied from a neighbour and not edited, an identifier that reads
// like a ULID without being one. The later ones are the ways a document walks
// past the check entirely, which is the worse failure and the quieter one.

const (
	goodID     = "01M2RFP6G4NBXT94SAAYZ1D1FH"
	alsoGoodID = "01M2RFP6G4GR61PWC6AR0WVSTR"
)

func block(name string) string { return blockWith(name, goodID, "2026-09-17") }

func blockWith(name, id, date string) string {
	return "---\nname: " + name + "\nlast_updated: " + date + "\nid: " + id + "\n---\n\n# Body\n"
}

// tree writes a fixture and returns its root, so a failure names the fixture
// that produced it.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for relative, text := range files {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func frontmatter(t *testing.T, files map[string]string, args ...string) (string, string, error) {
	t.Helper()
	return run(t, append([]string{"check", "frontmatter", tree(t, files)}, args...)...)
}

func TestATreeCarryingFrontmatterEverywherePasses(t *testing.T) {
	stdout, stderr, err := frontmatter(t, map[string]string{
		"models.md":    block("models"),
		"qa/README.md": blockWith("qa", alsoGoodID, "2026-09-17"),
	})
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v %q", code, err, stderr)
	}
	if !strings.Contains(stdout, "2 documents") {
		t.Errorf("did not count the tree: %q", stdout)
	}
}

func TestAWrongBlockIsNamed(t *testing.T) {
	for name, fixture := range map[string]struct {
		files map[string]string
		want  string
	}{
		"no block":        {map[string]string{"models.md": "# Body\n"}, "does not open with frontmatter"},
		"never closed":    {map[string]string{"models.md": "---\nname: models\n\n# Body\n"}, "never closed"},
		"missing key":     {map[string]string{"models.md": "---\nname: models\nid: " + goodID + "\n---\n"}, "no last_updated"},
		"empty key":       {map[string]string{"models.md": "---\nname:\nlast_updated: 2026-09-17\nid: " + goodID + "\n---\n"}, "name is empty"},
		"date shape":      {map[string]string{"models.md": blockWith("models", goodID, "17-09-2026")}, "expected YYYY-MM-DD"},
		"date not real":   {map[string]string{"models.md": blockWith("models", goodID, "2026-13-45")}, "not a real date"},
		"letter omitted":  {map[string]string{"models.md": blockWith("models", "01M2RFP6G4NBXT94SAAYZ1D1FI", "2026-09-17")}, "expected a 26-character ULID"},
		"wrong length":    {map[string]string{"models.md": blockWith("models", goodID[:25], "2026-09-17")}, "expected a 26-character ULID"},
		"shared id":       {map[string]string{"models.md": block("models"), "qa/brew.md": block("brew")}, "id is also on"},
		"name mismatch":   {map[string]string{"models.md": block("mdoels")}, `expected "models"`},
		"README misnamed": {map[string]string{"qa/README.md": block("readme")}, `expected "qa"`},
		"key twice":       {map[string]string{"models.md": "---\nname: models\nname: models\nlast_updated: 2026-09-17\nid: " + goodID + "\n---\n"}, "name appears twice"},
	} {
		t.Run(name, func(t *testing.T) {
			_, stderr, err := frontmatter(t, fixture.files)
			if code := ExitCode(err); code != 1 {
				t.Fatalf("exit %d, want 1: %v", code, err)
			}
			if !strings.Contains(stderr, fixture.want) {
				t.Errorf("did not report %q: %q", fixture.want, stderr)
			}
		})
	}
}

func TestABlockTheConventionAllowsPasses(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"upper-case filename": {"BUGS.md": block("bugs")},
		"per-feature plan":    {"models.md": block("models"), "features/mcp/plan.md": "# MCP\n"},
		"per-feature tasks":   {"models.md": block("models"), "features/mcp/tasks.md": "# MCP\n"},
		"list under a key": {"models.md": "---\nname: models\nlast_updated: 2026-09-17\nid: " +
			goodID + "\ntags:\n  - one\n  - two\n---\n\n# Body\n"},
		"comment and hyphenated key": {"models.md": "---\n# the identifier never changes\nname: models\n" +
			"last_updated: 2026-09-17\nid: " + goodID + "\nfeature-id: mcp\n---\n\n# Body\n"},
		"trailing comment": {"models.md": blockWith("models", goodID+" # assigned in #8", "2026-09-17")},
		"quoted scalars": {"models.md": "---\nname: \"models\"\nlast_updated: \"2026-09-17\"\nid: \"" +
			goodID + "\"\n---\n\n# Body\n"},
		"milestone URL": {"tracked.md": "---\nname: tracked\nlast_updated: 2026-09-17\nid: " + goodID +
			"\nmilestone: https://github.com/stormlightlabs/trps/milestone/1\n---\n\n# Body\n"},
	} {
		t.Run(name, func(t *testing.T) {
			_, stderr, err := frontmatter(t, files)
			if code := ExitCode(err); code != 0 {
				t.Fatalf("exit %d, want 0: %v %q", code, err, stderr)
			}
		})
	}
}

func TestAMilestoneThatIsNotAURLIsRejected(t *testing.T) {
	_, stderr, err := frontmatter(t, map[string]string{
		"tracked.md": "---\nname: tracked\nlast_updated: 2026-09-17\nid: " + goodID +
			"\nmilestone: UI Polish\n---\n\n# Body\n",
	})
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit %d, want 1: %v", code, err)
	}
	if !strings.Contains(stderr, "milestone") {
		t.Errorf("did not name the key: %q", stderr)
	}
}

// The exemption names features/ at the top of the tree being checked, not any
// directory called features.
func TestAFeaturesDirectoryElsewhereInheritsNoExemption(t *testing.T) {
	_, stderr, err := frontmatter(t, map[string]string{
		"models.md":                    block("models"),
		"archive/features/old/plan.md": "# Old\n",
	})
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit %d, want 1: %v", code, err)
	}
	if !strings.Contains(stderr, "does not open with frontmatter") {
		t.Errorf("did not report the file: %q", stderr)
	}
}

// A waived file that has a block still answers for everything but its name.
func TestAWaivedFileWithABlockStillAnswersForIt(t *testing.T) {
	_, stderr, err := frontmatter(t, map[string]string{
		"models.md": block("models"),
		"features/mcp/plan.md": "---\nname: anything\nlast_updated: not-a-date\nid: " +
			goodID + "\n---\n\n# Plan\n",
	})
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit %d, want 1: %v", code, err)
	}
	for _, want := range []string{"id is also on", "expected YYYY-MM-DD"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("did not report %q: %q", want, stderr)
		}
	}
	if strings.Contains(stderr, "name is") {
		t.Errorf("graded the name of a waived file: %q", stderr)
	}
}

func TestOneRunReportsEveryFailingFile(t *testing.T) {
	_, stderr, err := frontmatter(t, map[string]string{"a.md": "# A\n", "b.md": "# B\n", "c.md": "# C\n"})
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit %d, want 1: %v", code, err)
	}
	if got := strings.Count(stderr, "does not open with frontmatter"); got != 3 {
		t.Errorf("reported %d files, want 3: %q", got, stderr)
	}
}

// Everything below is a way a document walks past the check rather than a way
// it is written wrong. A false rejection is loud; these are silent.
func TestCapitalisingTheExtensionDoesNotHideADocument(t *testing.T) {
	_, stderr, err := frontmatter(t, map[string]string{
		"models.md": block("models"),
		"NOTES.MD":  "# no block at all\n",
	})
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit %d, want 1: %v", code, err)
	}
	for _, want := range []string{`extension is ".MD"`, "does not open with frontmatter"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("did not report %q: %q", want, stderr)
		}
	}
}

func TestAFileThatIsNotUTF8FailsOnItsOwnLine(t *testing.T) {
	root := tree(t, map[string]string{"good.md": block("good"), "zzz.md": "# no block\n"})
	if err := os.WriteFile(filepath.Join(root, "binary.md"),
		[]byte("---\nname: binary\n---\n\ncaf\xe9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, err := run(t, "check", "frontmatter", root)
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit %d, want 1: %v", code, err)
	}
	for _, want := range []string{"not valid UTF-8", "zzz.md"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("did not report %q: %q", want, stderr)
		}
	}
}

// Nothing found and everything correct produce the same empty list, and only
// one of them means the check did its job.
func TestATreeWithNoDocumentsFails(t *testing.T) {
	_, stderr, err := frontmatter(t, map[string]string{})
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit %d, want 1: %v", code, err)
	}
	if !strings.Contains(stderr, "no documents found") {
		t.Errorf("did not say the tree was empty: %q", stderr)
	}
}

func TestADirectoryThatIsNotThereIsAUsageError(t *testing.T) {
	_, _, err := run(t, "check", "frontmatter", filepath.Join(t.TempDir(), "nowhere"))
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit %d, want 2: %v", code, err)
	}
	if !strings.Contains(err.Error(), "is not a directory") {
		t.Errorf("error does not say what is wrong: %v", err)
	}
}

// A README at the root of the tree is named for the directory being checked.
func TestARootREADMEIsNamedForItsDirectory(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "fixtures")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(block("fixtures")), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, err := run(t, "check", "frontmatter", root)
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v %q", code, err, stderr)
	}
}

// Uniqueness within one tree is not immutability across time, so these cases
// mutate a committed identifier rather than duplicating a live one.
func TestAnIdentifierEditedInPlaceFailsSince(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "internal")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	document := filepath.Join(root, "models.md")
	if err := os.WriteFile(document, []byte(block("models")), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "fixture"},
	} {
		cmd := exec.Command("git", append([]string{"-C", parent}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}

	if _, stderr, err := run(t, "check", "frontmatter", root, "--since", "HEAD"); ExitCode(err) != 0 {
		t.Fatalf("an unchanged identifier failed --since: %v %q", err, stderr)
	}

	if err := os.WriteFile(document, []byte(blockWith("models", alsoGoodID, "2026-09-17")), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, err := run(t, "check", "frontmatter", root, "--since", "HEAD")
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit %d, want 1: %v", code, err)
	}
	if !strings.Contains(stderr, "never changes once assigned") {
		t.Errorf("did not report the edit: %q", stderr)
	}

	// The same edit passes without --since, which is why --since exists.
	if _, stderr, err := run(t, "check", "frontmatter", root); ExitCode(err) != 0 {
		t.Fatalf("the tree alone failed: %v %q", err, stderr)
	}
}
