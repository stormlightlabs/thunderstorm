---
title: Checks
description: What each check reads, what its exit code means, where it runs on its own, and what to type where it does not.
---

In order to verify that each skill is applied, the `tstorm` CLI discretely
runs the following checks to confirm that they were used. Exit codes are used
to communicate status.

| Check | Reads | Says |
| --- | --- | --- |
| `tstorm check commit-message <file>` | a commit message, or a pull request's title and body with `--pr` | the type, the 60-character subject, the blank line, the 72-column body |
| `tstorm check prose <path>` | Markdown, a file or a tree | the writing tells [tropius](https://github.com/stormlightlabs/trps) matches, minus the rules you mute |
| `tstorm check frontmatter [dir]` | the documents tree `.tstorm.toml` names | that every document carries the identifier an issue cites it by |
| `tstorm check policy [settings]` | a repository's permissions | which of the four reserved commands it does not deny |
| `tstorm check isolation [dir]` | skills and role definitions | that none of them asks the harness for a worktree |
| `tstorm check version` | the manifest, `BaseVersion`, the last tag, the rendered payloads | whether a payload changed without a version bump, and whether a tag left the binary's version behind |

`0` is nothing found, `1` is something found, and `2` is a check that could
not run. A caller reading only "non-zero" cannot tell a bad commit message
from an unreadable file, and the two need different answers. `--warn` reports
everything and exits `0`, which is what CI wants for the checks that cannot be
certain.

## What runs on its own

The payload registers one script for two events, and `tstorm hook` picks the
check from the event it is given.

A write to a file runs two checks and refuses neither. A document under your
configured tree is checked for its frontmatter, and a Markdown file is checked
for tells; both are reported into the session, which decides what to do. A
hook that blocks an editor gets uninstalled, so this one never does.

A `git commit` runs one check that does refuse. A message whose shape is wrong
is denied before git takes it, because a subject that will not read in
`git log --oneline` costs nothing to fix now and cannot be fixed later. What
the prose check finds in the same message is put to you instead: a trope count
is not a verdict, and the person writing is the one who decides whether the
message says what it means.

| Harness | Write check | Commit check |
| --- | --- | --- |
| Claude Code | on a `Write` or `Edit` | on a `Bash` command |
| Pi | through the package extension | through the same extension, refusals only |
| Codex | registered, never fires | on a `Bash` command |

Pi has no third answer between blocking a command and allowing it, so a check
that asks is read there as an allow and its finding is dropped. Codex writes
through its `exec` tool rather than a `Write` or `Edit` tool, so the write
check's matcher never matches; `tstorm render --target codex` says so in its
report, and [#18](https://github.com/stormlightlabs/thunderstorm/issues/18) is
where the extraction that fixes it goes.

CI runs the same binary. This repository's `check.yml` runs the frontmatter,
isolation, policy and version checks, and prose with `--warn`. The release
workflow runs the version check again with the tag it is building.

A harness caches an installed plugin by version and reports it current while
that string is unchanged, so a re-rendered payload reaches nobody until
`version` in the manifest moves. That is what the version check is for.

## When nothing fires

A hook is a session-start affair and a binary is a `PATH` affair, so there are
four ordinary ways to be working with no check behind you:

- The session started before the plugin was installed or enabled. Restart it.
  `claude plugin list` names what the next session will load.
- `tstorm` is not on `PATH`. The hook lets everything through and says so
  once per machine, since it runs on every shell command a session makes.
  `TSTORM_BIN` names the binary where it is somewhere unexpected.
- Tropius is not installed, so the prose check has nothing to report.
  `TRPS_BIN` names it. The install page has both, including from a checkout.
- The commit was written in an editor, or its message was built by a command
  substitution or a heredoc. The check reads what a shell would give it and
  says nothing about the rest, which is the safe direction for a check that
  refuses.

The editor case is what a git hook covers, and it is yours to add. This
repository keeps one in `.githooks/commit-msg`, enabled with
`git config core.hooksPath .githooks`. That setting replaces `.git/hooks`
wholesale, so every hook you already had stops firing; move them across first.

## Running them by hand

```sh
tstorm check prose docs README.md      # a tree and a file
tstorm check prose --warn <file>       # report, exit 0
tstorm check commit-message .git/COMMIT_EDITMSG
tstorm check frontmatter               # reads the tree from .tstorm.toml
tstorm check policy .claude/settings.json
tstorm check isolation .claude
```

Before a commit, the one worth typing is the pair the commit check runs:

```sh
git log -1 --format=%B > /tmp/msg && tstorm check commit-message /tmp/msg
tstorm check prose /tmp/msg
```

The prose check reports part of what `writing-docs` catalogues: phrase
patterns, bold-first leads, tricolons, negative parallelism, and the words that
name a judgment instead of a property. Structure, length, and whether a
paragraph earns its place are not in it. A clean run means those rules matched
nothing, which is a smaller claim than the prose being good.

## Muting a rule

`.tstorm.toml` carries the rules your repository drops, with the reason for
each in a comment above it:

```toml
[prose]
dictionary = "trps.toml"
paths = ["README.md", "docs", "workflow"]
mute = [
  # uncalibrated: 77 findings against prose a reader called clean
  "structure.short_punchy_fragments",
]
```

A `.tstorm.json` has nowhere to put a comment, so it names each rule as
`{ "rule": "...", "why": "..." }` instead.

A muted rule is one you cannot read yet rather than one you disagree with, so
the list is a record of what to fix upstream and it shrinks as the detector
improves. Vocabulary belongs in `trps.toml` instead, where `allow` takes a word
out of the bundled pattern that carried it and `exclude` keeps generated or
quoted prose out of the scan.
