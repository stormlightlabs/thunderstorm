package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// These cases are ported from workflow/scripts/check-isolation-test.py.
//
// They are the ways the setting gets declared, and the ways a check for it
// goes wrong: the word in prose, the word in a value, and a copy of the tree
// under worktrees/. Each of those three must keep passing.

const cleanDefinition = `---
name: implementer
description: Work one issue to a pull request, with build isolation per tree.
tools: Bash, Read
---

Use the ` + "`implement`" + ` skill. The run gives you a directory; work there.
`

func isolation(t *testing.T, files map[string]string) (string, string, error) {
	t.Helper()
	return run(t, "check", "isolation", tree(t, files))
}

func TestADeclarationIsReported(t *testing.T) {
	for name, fixture := range map[string]struct {
		files map[string]string
		want  string
		why   string
	}{
		"the frontmatter key": {
			files: map[string]string{"agents/implementer.md": "---\nname: implementer\nisolation: worktree\n---\n\nBody.\n"},
			want:  "frontmatter declares isolation: worktree",
		},
		"the key with no value": {
			files: map[string]string{"agents/implementer.md": "---\nname: a\nisolation:\n---\n\nBody.\n"},
			want:  "isolation: (empty)",
		},
		"an indented key": {
			files: map[string]string{"agents/a.md": "---\nname: a\nagent:\n  isolation: worktree\n---\n\nB.\n"},
			want:  "frontmatter declares isolation",
			why: "The harness reads the key whatever it is indented under. Requiring column zero " +
				"would let one space through, and one space is what a hand-edited block has.",
		},
		"a flow mapping": {
			files: map[string]string{"agents/a.md": "---\n{name: a, isolation: worktree}\n---\n\nB.\n"},
			want:  "frontmatter declares isolation",
			why: "Valid YAML that a real frontmatter parser reads, and no line in it reduces to a " +
				"leading `isolation:`. A line-oriented check passes it.",
		},
		"a flow mapping inside a value": {
			files: map[string]string{"agents/a.md": "---\nname: a\nagent: {isolation: x}\n---\n\nB.\n"},
			want:  "frontmatter declares isolation",
			why:   "The line's own key is `agent`, so matching only the key misses it.",
		},
		"a byte order mark in front of the fence": {
			files: map[string]string{"agents/a.md": "\ufeff---\nname: a\nisolation: worktree\n---\n\nB.\n"},
			want:  "frontmatter declares isolation",
			why:   "An editor that writes a BOM puts a character before the fence.",
		},
		"the setting in a fenced block": {
			files: map[string]string{"agents/a.md": "---\nname: a\n---\n\n```json\n{\"isolation\": \"x\"}\n```\n"},
			want:  "code block sets isolation",
		},
		"the setting as a flag in a shell block": {
			files: map[string]string{"skills/s/SKILL.md": "---\nname: s\n---\n\n```sh\nAgent --isolation=worktree\n```\n"},
			want:  "code block sets isolation",
		},
		"a nested fence": {
			files: map[string]string{"skills/s/SKILL.md": "---\nname: s\n---\n\n````md\n```yaml\nisolation: x\n```\n````\n"},
			want:  "code block sets isolation",
			why: "A fence inside a block is written with a longer fence. Closing on the inner one " +
				"would leave the rest of the block read as prose.",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, stderr, err := isolation(t, fixture.files)
			if code := ExitCode(err); code != 1 {
				t.Fatalf("exit %d, want 1: %v. %s", code, err, fixture.why)
			}
			if !strings.Contains(stderr, fixture.want) {
				t.Errorf("did not report %q: %q. %s", fixture.want, stderr, fixture.why)
			}
		})
	}
}

func TestATreeThatDeclaresNothingPasses(t *testing.T) {
	stdout, stderr, err := isolation(t, map[string]string{
		"agents/implementer.md":    cleanDefinition,
		"skills/worktree/SKILL.md": cleanDefinition,
	})
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v %q", code, err, stderr)
	}
	if !strings.Contains(stdout, "2 definitions") {
		t.Errorf("did not count the tree: %q", stdout)
	}
}

// The rule itself is a sentence containing the word. A check that fails the
// place the rule is written down cannot be the place the rule is written down.
func TestTheWordInProsePasses(t *testing.T) {
	_, stderr, err := isolation(t, map[string]string{
		"skills/worktree/SKILL.md": "---\nname: worktree\n---\n\n" +
			"An `isolation` setting on a dispatch places the worktree inside\n" +
			"the repository root, so this repository uses none.\n",
	})
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v %q", code, err, stderr)
	}
}

// The worktree skill's own description says "build isolation". This is the
// line the flow-mapping fallback could false-positive on.
func TestTheWordInsideAValuePasses(t *testing.T) {
	_, stderr, err := isolation(t, map[string]string{
		"agents/a.md": "---\nname: a\ndescription: Remove it with Rust build isolation. Use when.\n---\n\nB.\n",
	})
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v %q", code, err, stderr)
	}
}

// A worktree landing under the tree carries a second copy of it, whose
// findings would be duplicates against paths nothing tracks.
func TestACopyUnderWorktreesIsNotReported(t *testing.T) {
	stdout, stderr, err := isolation(t, map[string]string{
		"agents/a.md":              cleanDefinition,
		"worktrees/12/agents/a.md": "---\nisolation: worktree\n---\n",
	})
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v %q", code, err, stderr)
	}
	if !strings.Contains(stdout, "1 definition ") {
		t.Errorf("counted the copy: %q", stdout)
	}
}

// An unclosed block is left to the frontmatter check, which owns block shape.
func TestAnUnclosedBlockIsLeftToTheFrontmatterCheck(t *testing.T) {
	_, stderr, err := isolation(t, map[string]string{"agents/a.md": "---\nname: a\n\nisolation: worktree\n"})
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v %q", code, err, stderr)
	}
}

func TestATreeWithNoMarkdownFails(t *testing.T) {
	_, stderr, err := isolation(t, map[string]string{"agents/notes.txt": "isolation: worktree\n"})
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit %d, want 1: %v", code, err)
	}
	if !strings.Contains(stderr, "no definitions found") {
		t.Errorf("did not say the tree was empty: %q", stderr)
	}
}

func TestAnIsolationRootThatIsNotThereIsAUsageError(t *testing.T) {
	_, _, err := run(t, "check", "isolation", filepath.Join(t.TempDir(), "no-such-tree"))
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit %d, want 2: %v", code, err)
	}
}

// The workflow source is what the payload renders from, so it is the tree that
// has to stay clean.
func TestTheWorkflowSourcePassesItsOwnCheck(t *testing.T) {
	_, stderr, err := run(t, "check", "isolation", filepath.Join("..", "..", "workflow"))
	if code := ExitCode(err); code != 0 {
		t.Fatalf("exit %d, want 0: %v %q", code, err, stderr)
	}
}
