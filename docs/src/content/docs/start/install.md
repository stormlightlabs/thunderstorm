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

## Commit the payload instead

A repository that wants the workflow in its own tree renders it there. Target
names the destination:

```sh
tstorm render --target claude   # .claude
tstorm render --target codex    # .codex
tstorm render --target pi       # .pi
```

`--out <dir>` takes somewhere else, which is what thunderstorm itself does to
build the payloads it publishes.

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
tstorm render --target claude --adopt --check   # says what it would do
tstorm render --target claude --adopt
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

## Checks

Skills call `tstorm` subcommands, so the binary has to be on `PATH` wherever
the loop runs. Pi needs it for role dispatch as well. Install it with:

```sh
GOPROXY=direct go install github.com/stormlightlabs/thunderstorm/cmd/tstorm@main
```

That fetches the current `main` straight from GitHub and builds it, which is
what `@latest` does not do yet: the only tag is the `v0.1.0-rc.1` prerelease,
and the module proxy resolves `@latest` to a release. `@v0.1.0-rc.1` takes the
prerelease.

A machine with no Go toolchain takes a binary from the
[releases](https://github.com/stormlightlabs/thunderstorm/releases) instead.
Each release carries linux and macOS builds for amd64 and arm64, with
`checksums.txt` beside them:

```sh
tar -xzf tstorm_0.1.0-rc.1_linux_amd64.tar.gz
install -m 755 tstorm ~/.local/bin/tstorm
tstorm version
```

`tstorm version` names the tag and the commit it was built from. Homebrew is
the other route planned, through a tap in this organization, and it is the
only other one.

Two of them run on their own once the payload is installed. A write is checked
for the frontmatter an issue cites a document by and for the writing tells the
catalogue lists, and both are reported into the session rather than refused. A
`git commit` is checked for its shape, and that one refuses: a subject that
will not read in `git log --oneline` cannot be fixed afterwards.

Hooks load when a session starts, so the session you install from gets none of
this. Start a new one. A binary the hook cannot find lets everything through
and says so once per machine, since it runs on every shell command a session
makes.

[Checks](/reference/checks/) says what each check reads, what its exit code
means, which harness runs which, and what to type where none of them runs.

### From a checkout

Working on the loop means building what you are editing:

```sh
git clone https://github.com/stormlightlabs/thunderstorm
cd thunderstorm
go install ./cmd/tstorm     # to $GOBIN, or ~/go/bin
tstorm version
```

A build from a checkout takes its version from the repository's tags, so
`tstorm version` on a clone that has fetched none reports `(devel)`. `go build
-o ~/.local/bin/tstorm ./cmd/tstorm` puts it somewhere else. `TSTORM_BIN`
names the binary for the hooks where it is on neither `PATH` nor the payload's
`bin/`, which is how to run a build under test without installing it.

### Prose gate

`tstorm check prose` runs [tropius](https://github.com/stormlightlabs/trps),
which is a separate Rust binary and an optional one: without it the gate
reports nothing and says so. Install it from source, which is the only route
it has today:

```sh
cargo install --git https://github.com/stormlightlabs/trps trps-cli
```

`--rev <sha>` pins it, which is what CI does so a detector change does not
land under a job nobody ran. From a checkout, `cargo install --path
crates/cli` builds the tree you have. `TRPS_BIN` points at it when it lives outside
`PATH`.

## Not in the package

The loop is the same everywhere. What it runs against is not, so five things
stay with the repository rather than arriving with the install.

### Test and lint commands

`implement` and `revise` run the narrowest relevant test, then the gates
`AGENTS.md` names. Name them there: formatter, linter and the strictness it
runs at, test command. A repository that names none leaves an agent to
guess.

### Documents tree

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

### Deny rules

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

### Board

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

### Model assignments

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
