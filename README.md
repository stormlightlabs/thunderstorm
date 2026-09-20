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
# Claude Code
/plugin marketplace add stormlightlabs/thunderstorm

# Codex
codex plugin marketplace add stormlightlabs/thunderstorm
codex plugin add thunderstorm@stormlightlabs

# Pi
pi install git:github.com/stormlightlabs/thunderstorm
```

## The stages

Each stage is a skill the agent loads and a command you type. Most carry a
short alias and a spelled-out one, so `/decomp` and `/decompose` reach the same
skill.

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
with CI green throughout. `tstorm` holds the parts that have to be true:

- board reads and writes against GitHub Projects
- commit and pull request shape
- document identity, so an issue can cite the document it came from
- worktree isolation, so two workers cannot take each other's commits
- prose, checked against a catalogue of writing tells

```sh
go install github.com/stormlightlabs/tstorm/cmd/tstorm@latest
```

Hooks installed with the payload call it by name, so it needs to be on `PATH`.
Released builds will also ship through Homebrew, `.deb`, `.rpm`, `.apk`, and
the NUR.

## Harness support

| | Claude Code | Codex | Pi |
| --- | --- | --- | --- |
| Skills | `.claude/skills/` | `.agents/skills/`, `.codex/skills/` | `.agents/skills/`, `.pi/skills/` |
| Dispatch | subagents | `spawn_agent` | a session per tmux pane |
| Checks | hooks | hooks | an extension |

`docs/internal/hosts.md` records how each row was verified, including the
fixture used and the versions it was checked against.

## Layout

```text
cmd/tstorm      entry point
internal/       tstorm source
docs/           the published site
docs/internal/  working documents, not published
```

## Status

The binary is early: `tstorm version` is the only command implemented so far,
and the rest of the surface is on the board under the **Thunderstorm** track of
[project 13](https://github.com/orgs/stormlightlabs/projects/13).

## License

Apache-2.0. See [LICENSE](./LICENSE).
