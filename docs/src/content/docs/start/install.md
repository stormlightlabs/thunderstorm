---
title: Install
description: Add thunderstorm to Claude Code, Codex, or Pi.
sidebar:
  order: 2
---

Thunderstorm ships payloads for Claude Code, Codex, and Pi.

## Claude Code

```sh
claude plugin marketplace add stormlightlabs/thunderstorm
claude plugin install thunderstorm@stormlightlabs
```

`/plugin marketplace add` and `/plugin install` do the same from inside a
session. `marketplace add` also takes an HTTPS URL, an SSH URL, or a local
path. Twelve skills, sixteen commands and four agents arrive; `claude plugin
details thunderstorm` lists them and what they cost a session.

## Codex

### Enable it everywhere

```sh
codex plugin marketplace add stormlightlabs/thunderstorm
codex plugin add thunderstorm@stormlightlabs
```

Start a new session, review the plugin hook, and allow it before running the
workflow. The hook blocks the four merge, review, and push command prefixes
while the plugin is enabled.

Invoke a skill with its plugin-qualified name:

```text
$thunderstorm:thunderstorm Run issue 123
$thunderstorm:implement Work issue 456
```

### Enable it only in one project

Add this to the trusted project's `.codex/config.toml`:

```toml
[marketplaces.stormlightlabs]
source_type = "git"
source = "https://github.com/stormlightlabs/thunderstorm.git"

[plugins."thunderstorm@stormlightlabs"]
enabled = true
```

Codex loads the marketplace and enables the plugin only while it works in that
project. Commit the file when every Codex user on the project should get the
workflow.

### Install it globally and choose projects

Register and install the plugin with the two global commands above. Then set
the user-level default in `~/.codex/config.toml`:

```toml
[plugins."thunderstorm@stormlightlabs"]
enabled = false
```

Turn it on in each trusted project's `.codex/config.toml`:

```toml
[plugins."thunderstorm@stormlightlabs"]
enabled = true
```

Project configuration overrides user configuration. Use `/plugins` to inspect
the installed plugin, or remove it with `codex plugin remove
thunderstorm@stormlightlabs`.

## Pi

Install `tstorm`, then install the repository as a Pi package for the current
user:

```sh
GOPROXY=direct go install github.com/stormlightlabs/thunderstorm/cmd/tstorm@main
pi install git:github.com/stormlightlabs/thunderstorm
```

For one project, run this from the project root instead:

```sh
pi install git:github.com/stormlightlabs/thunderstorm -l
```

The local command writes `.pi/settings.json`. Pi asks the user to trust the
project before it loads the package.

The package loads the skills, prompts, and extension from `payloads/pi`. The
extension supplies the package path used by the scripts and blocks the four
merge and push command prefixes. `tstorm dispatch` starts each role in its own
Pi session. When Pi runs inside tmux or Zellij, dispatch opens a new window or
tab in that session so the operator can watch and control it.

## The checks

The checks the skills call are Python scripts inside the installed payload, so
they need `python3`. Pi also needs `tstorm` for role dispatch. Install it with:

```sh
GOPROXY=direct go install github.com/stormlightlabs/thunderstorm/cmd/tstorm@main
```

Nothing is tagged yet, so that fetches the current `main` straight from GitHub
and builds it. Once a version is tagged, `go install
github.com/stormlightlabs/thunderstorm/cmd/tstorm@latest` is the same thing
through the module proxy. Homebrew is the other route planned, and it is the
only other one.

## What the package does not carry

The loop is the same everywhere. What it runs against is not, so four things
stay with the repository rather than arriving with the install.

### Your gates

The `implement` and `revise` skills run the narrowest relevant test and then
the gates your `AGENTS.md` names. Name them there: the formatter, the linter
and the strictness you hold it to, and the test command. A repository that
names none leaves an agent to guess.

### The deny rules

Claude Code carries a settings file for the repository to merge. Codex loads a
`PreToolUse` hook from the enabled plugin. Pi loads its command gate from the
package extension.

Claude Code reads them as one rule per command. Merge the payload's
`settings.json` into your repository's `.claude/settings.json`:

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

### Your board

The `github-board` and `triage` skills read and write a GitHub Projects board.
The board, its `Todo`/`In Progress`/`Done` status field, and the labels are
yours to create. Name the board's number and owner in your `AGENTS.md`, beside
the gates: the skills are written against `<project>`, `<owner>` and
`<owner>/<repo>`, and take all three from what your repository says.

### Your model assignments

Which model runs which role is a decision per organization, not per package.
Pass either `--model provider/model` or separate `--provider` and `--model`
values to `tstorm dispatch`. The values come from `pi --list-models`, including
custom providers declared in `~/.pi/agent/models.json`. Pi's `models.json`
supports OpenAI-, Anthropic-, and Google-compatible endpoints; a Pi extension
can register other APIs or OAuth flows.

An implementer and the reviewer reading its work never share a model within a
run, because a model reviewing its own diff inherits the gap that produced the
defect. The [harness reference](/reference/harnesses/) covers what an agent
must provide before it can carry a role at all.
