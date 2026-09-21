package check

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fakeTropius puts a script on TRPS_BIN that writes what the real one would,
// so these cases cover the gate rather than the detector. The report shape is
// tropius's --json output, version 1.
func fakeTropius(t *testing.T, status int, stdout string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "trps")
	script := "#!/bin/sh\ncat <<'REPORT'\n" + stdout + "\nREPORT\nexit " + strconv.Itoa(status) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRPS_BIN", path)
}

const twoFindings = `{
  "version": 1,
  "dictionary": null,
  "findings": [
    {"rule_id": "formatting.bold_first_leads", "rule_name": "Bold-First Leads",
     "severity": "medium", "kind": "markdown", "path": "a.md", "line": 3, "column": 1,
     "matched": "- **One:** a thing.\n- **Two:** another."},
    {"rule_id": "structure.short_punchy_fragments", "rule_name": "Short Punchy Fragments",
     "severity": "low", "kind": "structure", "path": "a.md", "line": 9, "column": 1,
     "matched": "Every time."}
  ]
}`

func write(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestProseReportsWhatTropiusFound(t *testing.T) {
	fakeTropius(t, 1, twoFindings)
	found, err := Prose([]string{write(t, "a.md", "# A\n")}, ProseRules{})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Fatalf("reported %d findings, want both: %v", len(found), found)
	}
	// A structural finding quotes the whole run it matched, and a report a
	// hook hands back cannot carry a newline in the middle of a line.
	if line := found[0].String(); strings.Contains(line, "\n") {
		t.Errorf("a finding spans lines: %q", line)
	}
}

// A muted rule is one this repository cannot read yet. Dropping it here keeps
// the decision in one file instead of in whoever is reading the output.
func TestProseDropsAMutedRule(t *testing.T) {
	fakeTropius(t, 1, twoFindings)
	found, err := Prose([]string{write(t, "a.md", "# A\n")}, ProseRules{
		Mute: []string{"structure.short_punchy_fragments"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].Rule != "formatting.bold_first_leads" {
		t.Errorf("reported %v, want the rule that is not muted", found)
	}
}

// A gate that stops a session because a tool is missing gets uninstalled, so
// the caller is told which case this is and decides.
func TestProseSaysWhenTropiusIsNotInstalled(t *testing.T) {
	t.Setenv("TRPS_BIN", filepath.Join(t.TempDir(), "absent"))
	_, err := Prose([]string{write(t, "a.md", "# A\n")}, ProseRules{})
	if !errors.Is(err, ProseNotInstalled) {
		t.Errorf("error is %v, want the one that says tropius is missing", err)
	}
}

// Exit 2 is tropius saying it could not run, which is not the same answer as
// a clean file and must not be read as one.
func TestProseCarriesAFailureOutOfTropius(t *testing.T) {
	fakeTropius(t, 2, "")
	_, err := Prose([]string{write(t, "a.md", "# A\n")}, ProseRules{})
	if err == nil {
		t.Fatal("a tropius that could not run was read as a clean file")
	}
}

// The report shape carries a version tropius raises when a consumer would
// have to change. Reading a later one as if it were this one loses findings
// quietly.
func TestProseRefusesAReportShapeItDoesNotKnow(t *testing.T) {
	fakeTropius(t, 1, `{"version": 2, "findings": []}`)
	_, err := Prose([]string{write(t, "a.md", "# A\n")}, ProseRules{})
	if err == nil || !strings.Contains(err.Error(), "version") {
		t.Errorf("error is %v, want one that names the report version", err)
	}
}

// A directory is walked for the files this gate reads, because tropius takes
// files and the configured trees are directories.
func TestProseWalksATreeForMarkdown(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"a.md", "deep/b.mdx", "c.go", ".hidden/d.md"} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("# A\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	found, err := proseInputs([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Errorf("walked to %v, want the Markdown outside the dot directory", found)
	}
}
