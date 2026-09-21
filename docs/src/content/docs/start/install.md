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

### Enable it for everyone on a project

Both commands take `--scope project`, which writes the declaration into the
project's `.claude/settings.json` instead of the machine's:

```sh
claude plugin marketplace add stormlightlabs/thunderstorm --scope project
claude plugin install thunderstorm@stormlightlabs --scope project
```

```json
{
  "extraKnownMarketplaces": {
    "stormlightlabs": {
      "source": { "source": "github", "repo": "stormlightlabs/thunderstorm" }
    }
  },
  "enabledPlugins": { "thunderstorm@stormlightlabs": true }
}
```

Commit that file and everyone working in the project gets the workflow on their
next session. A `directory` source takes a path, and a relative one is read
from the project root, which is how this repository installs the payload it
renders.

The four denied commands are not in it. No plugin mechanism carries a
permission, so merge the `permissions.deny` block from
[`payloads/claude/settings.json`](https://github.com/stormlightlabs/thunderstorm/blob/main/payloads/claude/settings.json)
into the same file by hand, and check the merge:

```sh
tstorm check policy --expected <payload>/settings.json .claude/settings.json
```

It names every reserved command the settings do not deny. Run it in CI as
well: a command added to the workflow reaches the payload on the next update
and the repository's settings never.

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

## Commit the payload instead of installing it

A repository that wants the workflow in its own tree renders it there:

```sh
tstorm render --target claude --out .claude
```

The render writes its own files and leaves every other file in the directory
alone, so a settings file, a hook and a worktrees directory beside the payload
survive it. `.tstorm-payload` is the record of which files the render owns, and
the next render replaces those and removes the ones the source stopped
producing. A path the repository owns and the payload also wants stops the
render and names the file. `settings.json` is the exception: the payload writes
it where there is none and leaves the one it finds, because the permissions
block is merged by hand.

A directory from an older copy carries no marker, which is every repository
that installed the workflow before `tstorm render` existed. `--adopt` takes
one over:

```sh
tstorm render --target claude --out .claude --adopt --check   # says what it would do
tstorm render --target claude --out .claude --adopt
```

It replaces the files the payload writes, leaves the rest, and writes the
marker so no later render needs the flag. What it leaves includes the older
payload's own files, listed by path: a render removes only what it wrote, and
a directory with no marker has no record of that. Read that list and delete
what the workflow replaced.

Hooks are the one thing the copy route does not carry. `hooks/hooks.json` is
read by a plugin install, so a repository rendering into `.claude` registers
the hook in its own `settings.json`, against
`$CLAUDE_PROJECT_DIR/.claude/hooks/gate.sh`.

## The checks

The checks the skills call are `tstorm` subcommands, so the binary has to be on
`PATH` wherever the loop runs. Pi needs it for role dispatch as well. Install
it with:

```sh
GOPROXY=direct go install github.com/stormlightlabs/thunderstorm/cmd/tstorm@main
```

Nothing is tagged yet, so that fetches the current `main` straight from GitHub
and builds it. Once a version is tagged, `go install
github.com/stormlightlabs/thunderstorm/cmd/tstorm@latest` is the same thing
through the module proxy. Homebrew is the other route planned, and it is the
only other one.

One check runs on its own. Claude Code registers a `PostToolUse` hook from the
payload, and Pi's package extension does the same job from TypeScript: a
document written under your configured tree is checked for the frontmatter an
issue cites it by, and what it finds is reported to the session. It never
refuses a write. Where the payload puts the binary is a setting: `TSTORM_BIN`
first, then the payload's own `bin/`, then `PATH`, and a binary it cannot find
is a warning rather than a blocked editor.

Codex installs the same hook and does not yet run it. Codex writes files
through its `exec` tool rather than a `Write` or `Edit` tool, so the matcher
does not reach it, and `tstorm render --target codex` says so in its report.

## What the package does not carry

The loop is the same everywhere. What it runs against is not, so five things
stay with the repository rather than arriving with the install.

### Your gates

The `implement` and `revise` skills run the narrowest relevant test and then
the gates your `AGENTS.md` names. Name them there: the formatter, the linter
and the strictness you hold it to, and the test command. A repository that
names none leaves an agent to guess.

### Your documents tree

The `specify` skill writes plans and ideas that issues cite by identifier, and
`tstorm check frontmatter` is what keeps those identifiers real. Which
directory holds them is yours. Name it in `.tstorm.json` at the repository
root:

```json
{
  "documents": "docs/internal"
}
```

A repository that names none is a clean skip: the check has nothing to walk,
and says so rather than failing.

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

### Your board

`tstorm board` reads and writes a GitHub Projects board, and the `github-board`
and `triage` skills decide what it writes. The board and the single-select
field carrying status are yours to create. Name them in the same
`.tstorm.json`:

```json
{
  "board": {
    "owner": "your-org",
    "number": 13,
    "statusField": "Status",
    "status": {
      "todo": "Todo",
      "inProgress": "In Progress",
      "done": "Done"
    }
  }
}
```

The three option names are whatever your board calls the states the loop moves
an issue between. A project carrying several repositories' work adds
`groupField` and `groupValue` to separate them; a project that is this
repository's alone leaves both out. Reads are filtered to the repository the
command runs in, taken from its `origin` remote unless `repository` names one.

A repository that configures none of this gets an error naming every setting it
left out. Reading and writing a project also needs the `project` scope on the
token: `gh auth status` lists what yours carries, and `gh auth refresh -s
project` adds it.

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
