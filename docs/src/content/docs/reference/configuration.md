---
title: Configuration
description: The five things a repository keeps for itself.
---

The loop is the same everywhere. What it runs against is not, so five things
stay with the repository rather than arriving with the install.

## Test and lint commands

`implement` and `revise` run the narrowest relevant test, then the gates
`AGENTS.md` names. Name them there: formatter, linter and the strictness it
runs at, test command. A repository that names none leaves an agent to
guess.

## Documents tree

`specify` writes plans and ideas that issues cite by identifier, and `tstorm
check frontmatter` keeps those identifiers real. Which directory holds them is
a repository's own choice. Name it in `.tstorm.toml` at the repository
root:

```toml
documents = "docs/internal"
```

A repository that names none is a clean skip: the check has nothing to walk,
and says so rather than failing.

`tstorm` looks for `.tstorm.toml` first and `.tstorm.json` second, in the
directory a command runs in and then each directory above it. The nearest
directory holding either one wins. Both names take the same settings, so a
repository that installed the loop when JSON was the only format keeps working
until somebody converts the dozen lines by hand.

The first file found is the only one read; two files do not merge. A file that
names no settings is an error rather than an empty answer, so a `.tstorm.toml`
created and not yet filled in says so instead of shadowing the `.tstorm.json`
beside it.

## Deny rules

Claude Code carries a settings file for the repository to merge. Codex loads a
`PreToolUse` hook from the enabled plugin. Pi loads its command gate from the
package extension.

Claude Code reads them as one rule per command. Merge the payload's
`settings.json` into `.claude/settings.json`:

```json
{
  "permissions": {
    "deny": [
      "Bash(gh pr merge:*)",
      "Bash(gh pr review:*)",
      "Bash(git push:*)",
      "Bash(git merge:*)"
    ]
  }
}
```

A bare `git push` is denied because pushing goes through `tstorm push`, which
compares the remote ref to what it is about to overwrite. Without these
rules the review sequence is a convention an agent can skip.

The Codex payload also carries `rules/thunderstorm.rules`, one execution-policy
rule per command:

```starlark
prefix_rule(
    pattern = ["gh", "pr", "merge"],
    decision = "forbidden",
    justification = "only a human merges or approves a thunderstorm run",
)
```

Copy that file into `~/.codex/rules/` or a trusted project's `.codex/rules/`
only when the policy should remain active while the plugin is disabled. A
`forbidden` rule blocks the command without a prompt. To inspect a command:

```sh
codex execpolicy check --rules ~/.codex/rules/thunderstorm.rules -- gh pr merge 12
```

Pi ships no sandbox, but its thunderstorm extension rejects the same four
command prefixes before its `bash` tool runs. The extension is not process
isolation: keep Pi inside an operating-system sandbox or container when the
repository needs one.

## Board

`tstorm board` reads and writes a GitHub Projects board, and `github-board`
and `triage` decide what it writes. Create the board and the single-select
field carrying status, then name them in the same `.tstorm.toml`:

```toml
[board]
owner = "your-org"
number = 13
statusField = "Status"

[board.status]
todo = "Todo"
inProgress = "In Progress"
done = "Done"
```

The three option names are whatever the board calls the states an issue moves
between. A project carrying several repositories' work adds
`groupField` and `groupValue` to separate them; a project that is this
repository's alone leaves both out. Reads are filtered to the repository the
command runs in, taken from its `origin` remote unless `repository` names one.

A repository that configures none of this gets an error naming every setting
it left out. Reading and writing a project also needs the `project` scope on
the token: `gh auth status` lists the scopes, `gh auth refresh -s project`
adds it.

## Model assignments

Which model runs which role is a decision per organization, not per package.
Pass `--model provider/model` to `tstorm dispatch`, or `--provider` and
`--model` separately. The values come from `pi --list-models`, including
custom providers declared in `~/.pi/agent/models.json`. Pi's `models.json`
supports OpenAI-, Anthropic-, and Google-compatible endpoints; a Pi extension
can register other APIs or OAuth flows.

An implementer and the reviewer reading its work never share a model within a
run, because a model reviewing its own diff inherits the gap that produced the
defect. The [harness reference](/reference/harnesses/) covers what an agent
must provide before it can carry a role at all.
