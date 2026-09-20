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

Or `/plugin marketplace add` and `/plugin install` from inside a session.
Claude Code is the only harness with a payload today; `docs/start/install.md`
says what stays with your repository, and what Codex and Pi are still waiting
on.

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
go install github.com/stormlightlabs/thunderstorm/cmd/tstorm@latest
```

The checks call it by name, so it needs to be on `PATH`. Homebrew is the other
route once [#19](https://github.com/stormlightlabs/thunderstorm/issues/19)
lands; nothing is released yet.

Each harness reads skills from its own directory, names commands its own way,
and means its own thing by a subagent. The workflow is written once under
`workflow/`, and `tstorm render` builds one payload per harness from it:

```sh
tstorm render --target claude          # writes payloads/claude
tstorm render --target claude --check  # does the committed payload still match?
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
| Skills | `.claude/skills/` | `.agents/skills/`, `.codex/skills/` | `.agents/skills/`, `.pi/skills/` |
| Dispatch | subagents | `spawn_agent` | a session per tmux pane |
| Checks | hooks | hooks | an extension |

`docs/internal/hosts.md` records how each row was verified, including the
fixture used and the versions it was checked against.

## Layout

```text
AGENTS.md       gates, commit shape, and where things live (CLAUDE.md links to it)
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

The binary is early: `tstorm render` and `tstorm version` are what exist, and
the rest of the surface is on the board under the **Thunderstorm** track of
[project 13](https://github.com/orgs/stormlightlabs/projects/13). Only the
Claude Code payload renders today. Codex and Pi have no place to put a command,
and Pi has no subagents, so the render stops and names the issue that would
close each gap.

## License

Apache-2.0. See [LICENSE](./LICENSE).
