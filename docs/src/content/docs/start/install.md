---
title: Install
description: Add thunderstorm to Claude Code, Codex, or Pi.
sidebar:
  order: 2
---

Thunderstorm ships payloads for Claude Code, Codex, and Pi. `tstorm install`
puts one in a repository; each harness can also install it as a plugin of its
own.

## tstorm install

```sh
GOPROXY=direct go install github.com/stormlightlabs/thunderstorm/cmd/tstorm@main
cd <your repository>
tstorm install
```

The binary carries the workflow, so nothing has to be checked out anywhere.
Claude Code is the default target; `--target codex` and `--target pi` write
theirs, and `--dir` installs into a repository you are not standing in.

Four things happen, and `--check` reports all of them without writing:

1. The payload is rendered into `.claude`, `.codex`, or `.pi`.
2. The commands the workflow reserves for a person are merged into the
   repository's `permissions.deny`.
3. The gate is registered in the same settings file, against
   `$CLAUDE_PROJECT_DIR/.claude/hooks/gate.sh`.
4. A `.tstorm.toml` is written from what you named, and its documents tree is
   created.

Steps 2 and 3 are Claude Code's. Codex reads an execution policy and Pi its
extension, so `--target codex` and `--target pi` skip them and say so.

Step 4 invents nothing. With no flags no config is written, because a board
nobody named cannot be guessed:

```sh
tstorm install --board stormlightlabs/13 --documents docs/internal
tstorm install --board stormlightlabs/13 --track Tropius   # a shared board
```

`--no-settings` and `--no-config` turn off a step whose file you keep yourself.

The install writes its own files and leaves every other file in the directory
alone, so a settings file, a hook and a worktrees directory beside the payload
survive it. `.tstorm-payload` records which files the render owns and which
version wrote them. The next install replaces those and removes the ones the
source stopped producing. A path the repository owns and the payload also
wants stops the install and names the file.

A directory from an older copy carries no marker, which is every repository
that installed the workflow before `tstorm render` existed. `--adopt` takes
one over:

```sh
tstorm install --adopt --check   # says what it would do
tstorm install --adopt
```

It replaces the files the payload writes, leaves the rest, and writes the
marker so no later install needs the flag. What it leaves includes the older
payload's own files, listed by path: an install removes only what it wrote, and
a directory with no marker has no record of that. Read that list and delete
what the workflow replaced.

## tstorm update

```sh
tstorm update            # the payload under the current directory
tstorm update --check    # says what it would move, writes nothing
tstorm update --dir ../other-repo
```

Update finds the payload by its marker, names the version that wrote it and
the one this binary carries, re-renders, and merges the settings again, so a
command the workflow newly reserves reaches a repository that installed months
ago. A payload installed before the marker carried a version reports an
unknown one and updates anyway.

Your config is left alone. A repository's board and documents tree are its own
after the first install.

Update ends by saying when a newer `tstorm` has been released. That lookup is
cached for a day and gives up after three seconds, and `--offline` skips it.
`tstorm update --self` takes the new one: it downloads the release for your
machine, checks it against the `checksums.txt` published beside it, and
renames it over the binary you are running.

## Claude Code as a plugin

```sh
claude plugin marketplace add stormlightlabs/thunderstorm
claude plugin install thunderstorm@stormlightlabs
```

This keeps the payload in Claude Code's own cache rather than in the
repository. `/plugin marketplace add` and `/plugin install` do the same from
inside a session. `marketplace add` also takes an HTTPS URL, an SSH URL, or a
local path. Twelve skills, nine commands and four agents arrive; `claude plugin
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

The four denied commands are not in it, because no plugin mechanism carries a
permission. `tstorm install --no-config` merges them into the same file, or
add the `permissions.deny` block by hand. Either way, check the merge:

```sh
tstorm check policy
```

It reads the deny list from the workflow the binary carries and names every
reserved command `.claude/settings.json` does not deny. Run it in CI as well: a
command added to the workflow reaches the payload on the next update and the
repository's settings never.

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

## Render from a checkout

`tstorm render` builds a payload from a workflow directory rather than from the
one the binary carries, which is how thunderstorm builds the payloads it
publishes and how anyone editing the loop sees the result:

```sh
tstorm render --target claude --out payloads/claude
tstorm render --target claude --out payloads/claude --check
```

`--source <dir>` names the workflow tree, and `install --source <dir>` reads
one too, which is how an edit reaches a repository before it is committed.

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

`tstorm version` names the tag a build descends from and the commit under it,
`v0.1.0-rc.1+g1969674`, so two builds of one tag are told apart. A release
names its tag alone. Homebrew is the other route planned, through a tap in
this organization, and it is the only other one.

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

A build from a checkout names the tag its tree descends from and the commit it
was built at, and appends `.dirty` where that tree had uncommitted changes.
`go build -o ~/.local/bin/tstorm ./cmd/tstorm` puts it somewhere else.
`TSTORM_BIN` names the binary for the hooks where it is on neither `PATH` nor
the payload's `bin/`, which is how to run a build under test without
installing it.

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
crates/cli` builds the tree you have. `TRPS_BIN` points at it when it lives
outside `PATH`.

## What stays with the repository

The loop is the same everywhere; what it runs against is not.
[Configuration](/reference/configuration/) covers what a repository keeps for
itself. The gates `AGENTS.md` names are there, along with the documents tree,
the deny rules, the board, and which model carries which role.
