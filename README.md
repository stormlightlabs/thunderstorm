# thunderstorm

An agent agnostic loop harness.

A development loop that installs into a repository and runs on whichever coding
agent is available. A run covers one issue and the sub-issues under it: it
claims the issue on a GitHub Projects board, creates a git worktree for each
worker, takes each change through a fixed sequence of review passes, and stops
once a pull request is open. A person starts every run, and merging is left to
a person as well.

The loop began inside [thunderus](https://github.com/stormlightlabs/thunderus)
as six checked-in directories. A second repository could copy them, and the
copies drifted within weeks. This repository is the installable version.

## Install

```sh
GOPROXY=direct go install github.com/stormlightlabs/thunderstorm/cmd/tstorm@main
tstorm install
```

`tstorm install` puts the loop in the repository you run it in. The binary
carries the workflow, so nothing has to be checked out anywhere. It also does
the three things a payload cannot do for itself: merge the commands the
workflow reserves for a person into the repository's deny list, register the
gate hook against the same settings file, and write a `.tstorm.toml` from what
you name on the command line.

```sh
tstorm install --target pi --dir ../other-repo
tstorm install --board stormlightlabs/13 --documents docs/internal
tstorm install --check      # says what it would do, writes nothing
tstorm update               # moves a payload to what the binary carries
tstorm uninstall            # takes it back out, leaving what was yours
```

Claude Code can install it as a plugin instead, which keeps the payload in the
harness's own cache:

```sh
claude plugin marketplace add stormlightlabs/thunderstorm
claude plugin install thunderstorm@stormlightlabs
```

Both commands take `--scope project`, which writes the declaration into the
project's `.claude/settings.json` so everyone working there gets the workflow;
this repository installs itself that way. A plugin carries no permissions, so
the deny rules are still a merge you make yourself.

For Codex:

```sh
codex plugin marketplace add stormlightlabs/thunderstorm
codex plugin add thunderstorm@stormlightlabs
```

For Pi:

```sh
pi install git:github.com/stormlightlabs/thunderstorm
```

The [install guide](docs/src/content/docs/start/install.md) also covers
project-scoped Codex setup and per-project enablement.

## The stages

Each stage is a skill the agent loads. Claude Code and Pi expose the commands
below. Codex invokes the installed skill as `$thunderstorm:<skill>`; for
example, `$thunderstorm:thunderstorm Run issue 123` starts the full loop.

| Command         | What it does                                               |
| --------------- | ---------------------------------------------------------- |
| `/rubber-duck`  | Rubber-duck a design before any code exists                |
| `/specify`      | Turn an idea into a spec when issues need a decision first |
| `/decompose`    | Cut an idea or spec into issues and their sub-issues       |
| `/triage`       | Rank the board into a dispatch plan                        |
| `/thunderstorm` | Run one loop over an issue and its sub-issues              |
| `/implement`    | Work one issue on its own branch and open a pull request   |
| `/rev`          | Standard review pass                                       |
| `/adv-rev`      | Adversarial review pass                                    |
| `/revise`       | Address the findings a pass returned                       |

## tstorm

A skill is an instruction. When an agent ignores one, nothing records that it
happened, which is how a review protocol contradicted itself across seven files
with CI green throughout. `tstorm` is where the parts that have to be true
live, as exit codes rather than as prose:

```sh
GOPROXY=direct go install github.com/stormlightlabs/thunderstorm/cmd/tstorm@main
tstorm install
tstorm version
```

`tstorm version` names the tag a build descends from and the commit under it,
`v0.1.0-rc.1+g1969674`, so two builds of one tag are told apart.

The binary also dispatches Pi roles through tmux or Zellij, and carries the
checks a run depends on: commit and pull request shape, document identity,
worktree isolation, the denied commands a repository has to merge into its own
settings, and the writing tells
[tropius](https://github.com/stormlightlabs/trps) reports. It reads and writes
the board as well.

The only tag is a prerelease, so `@main` is what the line above fetches.
Releases carry linux and macOS binaries; `@latest` through the module proxy
and Homebrew arrive with `v0.1.0`
([#19](https://github.com/stormlightlabs/thunderstorm/issues/19)).

Each harness reads skills from its own directory, names commands its own way,
and means its own thing by a subagent. The workflow is written once under
`workflow/`, and one render per harness builds a payload from it. `install`
does that from the tree the binary carries; `render` does it from a checkout,
which is how this repository builds the payloads it publishes:

```sh
tstorm render --target claude --out payloads/claude
tstorm render --target claude --out payloads/claude --check
```

A path that differs between harnesses is written `{{ROOT}}` in the source, and
the render substitutes the resource directory the reader will actually have.
The manifest beside the source says what each artifact is and which harness
capabilities it needs. An artifact that needs one the target does not provide
stops the render and names the gap, because a payload that installs and then
skips the review fan-out is worse than no payload.

## Harness support

|          | Claude Code       | Codex                               | Pi                               |
| -------- | ----------------- | ----------------------------------- | -------------------------------- |
| Skills   | `.claude/skills/` | plugin skills                       | `.agents/skills/`, `.pi/skills/` |
| Dispatch | subagents         | built-in agents with packaged roles | a session per tmux or Zellij tab |
| Checks   | hooks             | a plugin hook                       | an extension                     |

`docs/internal/hosts.md` records how each row was verified, including the
fixture used and the versions it was checked against.

## Layout

```text
AGENTS.md       gates, commit shape, and where things live (CLAUDE.md links to it)
.githooks/      commit-msg, checking the shape before a message lands
CHANGELOG.md    what has landed
TODO.md         what is left
cmd/tstorm      entry point
internal/       tstorm source
workflow/       the skills, commands, agents and scripts, written once
payloads/       what `tstorm render` builds from them, one directory per harness
docs/           the published site
docs/internal/  working documents, not published
```

## License

Apache-2.0. See [LICENSE](./LICENSE).
