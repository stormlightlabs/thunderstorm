---
title: Install
description: Add thunderstorm to Claude Code, Codex, or Pi.
sidebar:
  order: 2
---

Every harness installs from this repository, and Claude Code is the one with a
payload today.

## Claude Code

```sh
claude plugin marketplace add stormlightlabs/thunderstorm
claude plugin install thunderstorm@stormlightlabs
```

`/plugin marketplace add` and `/plugin install` do the same from inside a
session. `marketplace add` also takes an HTTPS URL, an SSH URL, or a local
path. Twelve skills, sixteen commands and four agents arrive; `claude plugin
details thunderstorm` lists them and what they cost a session.

## Codex and Pi

Neither installs yet. Codex reads prompts from `~/.codex/prompts`, which a
plugin does not write, and Pi packages extensions and skills but not prompts,
so the commands have nowhere to go on either. Pi has no subagents at all, so
the review fan-out has nothing to dispatch with.

`tstorm render --target codex` stops and says so rather than building a payload
that installs and then skips half the loop. The issues that would close each
gap are named in the failure.

## The checks

The checks the skills call are Python scripts inside the installed payload, so
they need `python3`. `tstorm` itself is what builds a payload, and you need it
only to render one:

```sh
go install github.com/stormlightlabs/thunderstorm/cmd/tstorm@latest
```

Nothing is released yet, so that command builds from source. Homebrew is the
other route planned, and it is the only other one.

## What the package does not carry

The loop is the same everywhere. What it runs against is not, so four things
stay with the repository rather than arriving with the install.

### Your gates

The `implement` and `revise` skills run the narrowest relevant test and then
the gates your `AGENTS.md` names. Name them there: the formatter, the linter
and the strictness you hold it to, and the test command. A repository that
names none leaves an agent to guess.

### The deny rules

No plugin mechanism carries a permission, so an installed payload cannot stop a
session merging its own work. The payload ships them as `settings.json` for you
to merge into your repository's `.claude/settings.json`:

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

A bare `git push` is denied because pushing goes through `push-verified.sh`,
which compares the remote ref to what it is about to overwrite. Without these
rules the review sequence is a convention an agent can skip.

### Your board

The `github-board` and `triage` skills read and write a GitHub Projects board.
The board, its `Todo`/`In Progress`/`Done` status field, and the labels are
yours to create. Name the board's number and owner in your `AGENTS.md`, beside
the gates: the skills are written against `<project>`, `<owner>` and
`<owner>/<repo>`, and take all three from what your repository says.

### Your model assignments

Which model runs which role is a decision per organization, not per package.
One rule travels with the loop whatever you decide: an implementer and the
reviewer reading its work never share a model within a run, because a model
reviewing its own diff inherits the gap that produced the defect. The
[harness reference](/reference/harnesses/) covers what an agent must provide
before it can carry a role at all.
