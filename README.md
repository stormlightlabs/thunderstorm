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
claude plugin marketplace add stormlightlabs/thunderstorm
claude plugin install thunderstorm@stormlightlabs
```

Or `/plugin marketplace add` and `/plugin install` from inside a session. Both
commands take `--scope project`, which writes the declaration into the
project's `.claude/settings.json` so everyone working there gets the workflow;
this repository installs itself that way.

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

| Command | What it does |
| --- | --- |
| `/r-d` | Rubber-duck a design before any code exists |
| `/spec-ify` | Turn an idea into a spec when issues need a decision first |
| `/decomp` | Cut an idea or spec into issues and their sub-issues |
| `/forecast`, `/triage` | Rank the board into a dispatch plan |
| `/storm` | Run one loop over an issue and its sub-issues |
| `/impl` | Work one issue on its own branch and open a pull request |
| `/rev` | Standard review pass |
| `/adv-rev` | Adversarial review pass |
| `/edit` | Address the findings a pass returned |

## tstorm

A skill is an instruction. When an agent ignores one, nothing records that it
happened, which is how a review protocol contradicted itself across seven files
with CI green throughout. `tstorm` is where the parts that have to be true
live, as exit codes rather than as prose:

```sh
GOPROXY=direct go install github.com/stormlightlabs/thunderstorm/cmd/tstorm@main
tstorm render --target claude
tstorm render --target codex
tstorm render --target pi
tstorm version
```

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
`workflow/`, and `tstorm render` builds one payload per harness from it:

```sh
tstorm render --target claude          # writes payloads/claude
tstorm render --target claude --check  # does the committed payload still match?
tstorm render --target codex           # writes payloads/codex
tstorm render --target pi              # writes payloads/pi
```

A path that differs between harnesses is written `{{ROOT}}` in the source, and
the render substitutes the resource directory the reader will actually have.
The manifest beside the source says what each artifact is and which harness
capabilities it needs. An artifact that needs one the target does not provide
stops the render and names the gap, because a payload that installs and then
skips the review fan-out is worse than no payload.

## Harness support

| | Claude Code | Codex | Pi |
| --- | --- | --- | --- |
| Skills | `.claude/skills/` | plugin skills | `.agents/skills/`, `.pi/skills/` |
| Dispatch | subagents | built-in agents with packaged roles | a session per tmux or Zellij tab |
| Checks | hooks | a plugin hook | an extension |

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

## Status

The binary is early: `tstorm render`, `tstorm dispatch`, and `tstorm version`
are what exist. The rest of the surface is on the board under the
**Thunderstorm** track of
[project 13](https://github.com/orgs/stormlightlabs/projects/13). Claude Code,
Codex, and Pi payloads render from the shared workflow. Cursor remains
unsupported.

## License

Apache-2.0. See [LICENSE](./LICENSE).
